package di

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type testConfig struct {
	DSN string
}

type testDB struct {
	dsn string
}

type testRepo struct {
	db *testDB
}

type testService struct {
	repo *testRepo
}

func newTestConfig() *testConfig {
	return &testConfig{DSN: "file:test.db"}
}

func newTestDB(cfg *testConfig) *testDB {
	return &testDB{dsn: cfg.DSN}
}

func newTestRepo(db *testDB) *testRepo {
	return &testRepo{db: db}
}

func newTestService(repo *testRepo) *testService {
	return &testService{repo: repo}
}

func TestResolveDependencyChain(t *testing.T) {
	c := New()
	for _, fn := range []any{newTestConfig, newTestDB, newTestRepo, newTestService} {
		if err := c.Provide(fn); err != nil {
			t.Fatalf("Provide(%T): %v", fn, err)
		}
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	service, err := Get[*testService](c)
	if err != nil {
		t.Fatalf("Get[*testService]: %v", err)
	}
	if service == nil || service.repo == nil || service.repo.db == nil {
		t.Fatal("resolved service chain contains nil")
	}
	if got := service.repo.db.dsn; got != "file:test.db" {
		t.Fatalf("db DSN = %q, want %q", got, "file:test.db")
	}
}

func TestProviderRunsOncePerContainer(t *testing.T) {
	var dbCalls int
	c := New()
	if err := c.Provide(func() *testConfig { return newTestConfig() }); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(func(cfg *testConfig) *testDB {
		dbCalls++
		return newTestDB(cfg)
	}); err != nil {
		t.Fatal(err)
	}

	first, err := Get[*testDB](c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Get[*testDB](c)
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatal("container returned different singleton instances")
	}
	if dbCalls != 1 {
		t.Fatalf("provider called %d times, want 1", dbCalls)
	}
}

func TestSharedDependencyIsConstructedOnce(t *testing.T) {
	type left struct{ cfg *testConfig }
	type right struct{ cfg *testConfig }
	type both struct {
		left  *left
		right *right
	}

	var configCalls int
	c := New()
	if err := c.Provide(func() *testConfig {
		configCalls++
		return newTestConfig()
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(func(cfg *testConfig) *left { return &left{cfg: cfg} }); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(func(cfg *testConfig) *right { return &right{cfg: cfg} }); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(func(l *left, r *right) *both { return &both{left: l, right: r} }); err != nil {
		t.Fatal(err)
	}

	value, err := Get[*both](c)
	if err != nil {
		t.Fatal(err)
	}
	if value.left.cfg != value.right.cfg {
		t.Fatal("shared dependency was constructed more than once")
	}
	if configCalls != 1 {
		t.Fatalf("config provider called %d times, want 1", configCalls)
	}
}

func TestMissingDependency(t *testing.T) {
	c := New()
	if err := c.Provide(newTestService); err != nil {
		t.Fatal(err)
	}

	err := c.Validate()
	var missing *MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("Validate error = %v, want MissingDependencyError", err)
	}
	if missing.Type != reflect.TypeFor[*testRepo]() {
		t.Fatalf("missing type = %s, want %s", missing.Type, reflect.TypeFor[*testRepo]())
	}
	wantPath := []reflect.Type{
		reflect.TypeFor[*testService](),
		reflect.TypeFor[*testRepo](),
	}
	if !reflect.DeepEqual(missing.Path, wantPath) {
		t.Fatalf("missing path = %v, want %v", missing.Path, wantPath)
	}
	if !strings.Contains(missing.RequiredBy, "newTestService") {
		t.Fatalf("RequiredBy = %q, want provider name", missing.RequiredBy)
	}
	if !strings.Contains(missing.Error(), "*di.testRepo") {
		t.Fatalf("error text lacks missing type: %v", missing)
	}
}

func TestMissingRootDependency(t *testing.T) {
	c := New()

	_, err := c.Resolve(reflect.TypeFor[*testService]())
	var missing *MissingDependencyError
	if !errors.As(err, &missing) {
		t.Fatalf("Resolve error = %v, want MissingDependencyError", err)
	}
	if missing.RequiredBy != "" {
		t.Fatalf("RequiredBy = %q, want empty for root request", missing.RequiredBy)
	}
	wantPath := []reflect.Type{reflect.TypeFor[*testService]()}
	if !reflect.DeepEqual(missing.Path, wantPath) {
		t.Fatalf("missing path = %v, want %v", missing.Path, wantPath)
	}
}

func TestDuplicateProvider(t *testing.T) {
	c := New()
	if err := c.Provide(newTestConfig); err != nil {
		t.Fatal(err)
	}
	err := c.Provide(func() *testConfig { return &testConfig{DSN: "other"} })

	var duplicate *DuplicateProviderError
	if !errors.As(err, &duplicate) {
		t.Fatalf("Provide error = %v, want DuplicateProviderError", err)
	}
	if duplicate.Type != reflect.TypeFor[*testConfig]() {
		t.Fatalf("duplicate type = %s", duplicate.Type)
	}
	if duplicate.Existing == "" || duplicate.Incoming == "" {
		t.Fatalf("duplicate error lacks provider names: %+v", duplicate)
	}
	if !strings.Contains(duplicate.Error(), "duplicate provider") {
		t.Fatalf("unexpected duplicate error text: %v", duplicate)
	}
}

type cycleA struct{}
type cycleB struct{}

func newCycleA(b *cycleB) *cycleA { return &cycleA{} }
func newCycleB(a *cycleA) *cycleB { return &cycleB{} }

func TestDependencyCycle(t *testing.T) {
	c := New()
	if err := c.Provide(newCycleA); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(newCycleB); err != nil {
		t.Fatal(err)
	}

	err := c.Validate()
	var cycle *CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("Validate error = %v, want CycleError", err)
	}
	if len(cycle.Path) != 3 {
		t.Fatalf("cycle path = %v, want three entries", cycle.Path)
	}
	if cycle.Path[0] != cycle.Path[len(cycle.Path)-1] {
		t.Fatalf("cycle path does not close: %v", cycle.Path)
	}
	if !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("unexpected error text: %v", err)
	}

	_, resolveErr := c.Resolve(reflect.TypeFor[*cycleA]())
	if !errors.As(resolveErr, &cycle) {
		t.Fatalf("Resolve error = %v, want CycleError", resolveErr)
	}
}

func TestInvalidProviders(t *testing.T) {
	var nilFunc func() int

	tests := []struct {
		name string
		fn   any
	}{
		{name: "nil interface", fn: nil},
		{name: "not a function", fn: 42},
		{name: "nil typed function", fn: nilFunc},
		{name: "no result", fn: func() {}},
		{name: "too many results", fn: func() (int, error, error) { return 0, nil, nil }},
		{name: "error only", fn: func() error { return nil }},
		{name: "error first", fn: func() (error, error) { return nil, nil }},
		{name: "second result is not error", fn: func() (int, int) { return 0, 0 }},
		{name: "variadic", fn: func(...int) int { return 0 }},
		{name: "error parameter", fn: func(error) int { return 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			err := c.Provide(tt.fn)
			var invalid *InvalidProviderError
			if !errors.As(err, &invalid) {
				t.Fatalf("Provide error = %v, want InvalidProviderError", err)
			}
			if invalid.Reason == "" || invalid.Provider == "" {
				t.Fatalf("invalid provider error lacks context: %+v", invalid)
			}
			if !strings.Contains(invalid.Error(), invalid.Reason) {
				t.Fatalf("invalid provider error text lacks reason: %v", invalid)
			}
		})
	}
}

type failingService struct{}

func TestProviderErrorIsWrapped(t *testing.T) {
	sentinel := errors.New("constructor failed")
	var calls int
	c := New()
	if err := c.Provide(func() (*failingService, error) {
		calls++
		return nil, sentinel
	}); err != nil {
		t.Fatal(err)
	}

	_, err := Get[*failingService](c)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Get error = %v, want wrapped sentinel", err)
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("Get error = %v, want ProviderError", err)
	}
	if providerErr.Type != reflect.TypeFor[*failingService]() {
		t.Fatalf("provider error type = %s", providerErr.Type)
	}
	if !strings.Contains(providerErr.Error(), "constructor failed") {
		t.Fatalf("provider error lacks cause: %v", providerErr)
	}
	wantPath := []reflect.Type{reflect.TypeFor[*failingService]()}
	if !reflect.DeepEqual(providerErr.Path, wantPath) {
		t.Fatalf("provider error path = %v, want %v", providerErr.Path, wantPath)
	}

	// Failed constructions are not cached, so a later attempt can retry.
	_, _ = Get[*failingService](c)
	if calls != 2 {
		t.Fatalf("failed provider called %d times, want 2", calls)
	}
}

type greeter interface {
	Greet() string
}

type englishGreeter struct{}

func (englishGreeter) Greet() string { return "hello" }

type greeterConsumer struct {
	greeter greeter
}

func TestInterfaceProviderAndDependency(t *testing.T) {
	c := New()
	if err := c.Provide(func() greeter { return englishGreeter{} }); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(func(g greeter) *greeterConsumer { return &greeterConsumer{greeter: g} }); err != nil {
		t.Fatal(err)
	}

	consumer, err := Get[*greeterConsumer](c)
	if err != nil {
		t.Fatal(err)
	}
	if got := consumer.greeter.Greet(); got != "hello" {
		t.Fatalf("Greet() = %q, want hello", got)
	}

	direct, err := Get[greeter](c)
	if err != nil {
		t.Fatal(err)
	}
	if direct.Greet() != "hello" {
		t.Fatalf("direct interface resolution failed: %v", direct)
	}
}

func TestNilInterfaceResultIsRejected(t *testing.T) {
	c := New()
	if err := c.Provide(func() greeter { return nil }); err != nil {
		t.Fatal(err)
	}

	_, err := Get[greeter](c)
	if err == nil {
		t.Fatal("Get returned nil error for nil interface result")
	}
	if !strings.Contains(err.Error(), "returned a nil value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDoesNotConstructValues(t *testing.T) {
	var calls int
	c := New()
	if err := c.Provide(func() *testConfig {
		calls++
		return newTestConfig()
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(newTestDB); err != nil {
		t.Fatal(err)
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if calls != 0 {
		t.Fatalf("Validate constructed %d values, want 0", calls)
	}
}

func TestConcurrentResolveUsesSingleton(t *testing.T) {
	var calls atomic.Int32
	c := New()
	if err := c.Provide(func() *testConfig {
		calls.Add(1)
		return newTestConfig()
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Provide(newTestDB); err != nil {
		t.Fatal(err)
	}

	const workers = 32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	results := make(chan *testDB, workers)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := Get[*testDB](c)
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}
	wg.Wait()
	close(errs)
	close(results)

	for err := range errs {
		t.Errorf("concurrent Get: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider called %d times, want 1", got)
	}

	var first *testDB
	for value := range results {
		if first == nil {
			first = value
			continue
		}
		if value != first {
			t.Fatal("concurrent Get returned different instances")
		}
	}
}

func TestZeroValueContainerCanProvide(t *testing.T) {
	var c Container
	if err := c.Provide(newTestConfig); err != nil {
		t.Fatalf("zero value Container Provide: %v", err)
	}
	config, err := Get[*testConfig](&c)
	if err != nil {
		t.Fatalf("zero value Container Get: %v", err)
	}
	if config.DSN != "file:test.db" {
		t.Fatalf("DSN = %q", config.DSN)
	}
}

func TestNilTypesAreRejected(t *testing.T) {
	c := New()
	if _, err := c.Resolve(nil); err == nil {
		t.Fatal("Resolve(nil) returned nil error")
	}

	var nilContainer *Container
	if err := nilContainer.Provide(newTestConfig); err == nil {
		t.Fatal("nil container Provide returned nil error")
	}
	if err := nilContainer.Validate(); err == nil {
		t.Fatal("nil container Validate returned nil error")
	}
	if _, err := nilContainer.Resolve(reflect.TypeFor[*testConfig]()); err == nil {
		t.Fatal("nil container Resolve returned nil error")
	}
	if _, err := Get[*testConfig](nilContainer); err == nil {
		t.Fatal("nil container Get returned nil error")
	}
}

func TestProviderErrorMessageIncludesType(t *testing.T) {
	c := New()
	if err := c.Provide(func() (*testConfig, error) {
		return nil, fmt.Errorf("bad config")
	}); err != nil {
		t.Fatal(err)
	}

	_, err := c.Resolve(reflect.TypeFor[*testConfig]())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), reflect.TypeFor[*testConfig]().String()) {
		t.Fatalf("error lacks type name: %v", err)
	}
}

type testGreeter interface {
	Greet() string
}

type testHelloGreeter struct {
	msg string
}

func (h *testHelloGreeter) Greet() string {
	return h.msg
}

func newTestHelloGreeter() *testHelloGreeter {
	return &testHelloGreeter{msg: "hello world"}
}

type testGreeterConsumer struct {
	greeter testGreeter
}

func newTestGreeterConsumer(g testGreeter) *testGreeterConsumer {
	return &testGreeterConsumer{greeter: g}
}

func TestProvideAsAndSameSingletonInstance(t *testing.T) {
	c := New()
	if err := c.ProvideAs(newTestHelloGreeter, (*testGreeter)(nil)); err != nil {
		t.Fatalf("ProvideAs: %v", err)
	}
	if err := c.Provide(newTestGreeterConsumer); err != nil {
		t.Fatalf("Provide consumer: %v", err)
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	concrete, err := Get[*testHelloGreeter](c)
	if err != nil {
		t.Fatalf("Get concrete: %v", err)
	}
	iface, err := Get[testGreeter](c)
	if err != nil {
		t.Fatalf("Get interface: %v", err)
	}

	if concrete != iface {
		t.Fatal("concrete and interface did not resolve to the same singleton instance")
	}

	consumer, err := Get[*testGreeterConsumer](c)
	if err != nil {
		t.Fatalf("Get consumer: %v", err)
	}
	if consumer.greeter != concrete {
		t.Fatal("consumer did not receive the singleton greeter instance")
	}
}

func TestGenericProvideAsAndBind(t *testing.T) {
	c := New()
	if err := c.Provide(newTestHelloGreeter); err != nil {
		t.Fatal(err)
	}
	if err := Bind[testGreeter, *testHelloGreeter](c); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	concrete, err := Get[*testHelloGreeter](c)
	if err != nil {
		t.Fatal(err)
	}
	iface, err := Get[testGreeter](c)
	if err != nil {
		t.Fatal(err)
	}
	if concrete != iface {
		t.Fatal("different instances returned")
	}
}

func TestInvalidBindings(t *testing.T) {
	c := New()

	// Target not an interface
	err := c.Bind(testConfig{}, (*testHelloGreeter)(nil))
	if err == nil {
		t.Fatal("expected error binding to non-interface target")
	}
	var bindErr *InvalidBindingError
	if !errors.As(err, &bindErr) {
		t.Fatalf("expected InvalidBindingError, got: %T", err)
	}

	// Concrete does not implement interface
	err = c.Bind((*testGreeter)(nil), (*testConfig)(nil))
	if err == nil {
		t.Fatal("expected error when source does not implement interface")
	}
	if !errors.As(err, &bindErr) {
		t.Fatalf("expected InvalidBindingError, got: %T", err)
	}

	// Nil target
	if err := c.Bind(nil, (*testHelloGreeter)(nil)); err == nil {
		t.Fatal("expected error on nil target")
	}

	// Nil source
	if err := c.Bind((*testGreeter)(nil), nil); err == nil {
		t.Fatal("expected error on nil source")
	}

	// ProvideAs target not interface
	if err := c.ProvideAs(newTestHelloGreeter, testConfig{}); err == nil {
		t.Fatal("expected error when ProvideAs target is not interface")
	}
}

func TestDuplicateBindingAndProvider(t *testing.T) {
	c := New()
	if err := c.ProvideAs(newTestHelloGreeter, (*testGreeter)(nil)); err != nil {
		t.Fatal(err)
	}

	// Duplicate binding for same interface
	err := c.Bind((*testGreeter)(nil), (*testHelloGreeter)(nil))
	if err == nil {
		t.Fatal("expected error on duplicate binding")
	}
	var dupErr *DuplicateProviderError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateProviderError, got: %T", err)
	}

	// Direct provider for already bound interface
	err = c.Provide(func() testGreeter { return &testHelloGreeter{} })
	if err == nil {
		t.Fatal("expected error registering provider for bound interface")
	}
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateProviderError, got: %T", err)
	}
}

type cycleIfaceA interface{ MethodA() }
type cycleIfaceB interface{ MethodB() }

type cycleImplA struct{ b cycleIfaceB }

func (a *cycleImplA) MethodA() {}

type cycleImplB struct{ a cycleIfaceA }

func (b *cycleImplB) MethodB() {}

func TestInterfaceBindingDependencyCycle(t *testing.T) {
	c := New()
	if err := c.ProvideAs(func(b cycleIfaceB) *cycleImplA { return &cycleImplA{b: b} }, (*cycleIfaceA)(nil)); err != nil {
		t.Fatal(err)
	}
	if err := c.ProvideAs(func(a cycleIfaceA) *cycleImplB { return &cycleImplB{a: a} }, (*cycleIfaceB)(nil)); err != nil {
		t.Fatal(err)
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var cycleErr *CycleError
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected CycleError, got: %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("unexpected message: %v", err)
	}
}

func TestInterfaceBindingMissingDependency(t *testing.T) {
	c := New()
	// Bind testGreeter to *testHelloGreeter, but do not provide *testHelloGreeter
	if err := Bind[testGreeter, *testHelloGreeter](c); err != nil {
		t.Fatal(err)
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	var missErr *MissingDependencyError
	if !errors.As(err, &missErr) {
		t.Fatalf("expected MissingDependencyError, got: %T (%v)", err, err)
	}
}

func TestInterfaceConcurrentResolve(t *testing.T) {
	c := New()
	if err := ProvideAs[testGreeter](c, newTestHelloGreeter); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var instances sync.Map
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var ptr uintptr
			if id%2 == 0 {
				g, err := Get[testGreeter](c)
				if err != nil {
					t.Errorf("Get interface: %v", err)
					return
				}
				ptr = reflect.ValueOf(g).Pointer()
			} else {
				g, err := Get[*testHelloGreeter](c)
				if err != nil {
					t.Errorf("Get concrete: %v", err)
					return
				}
				ptr = reflect.ValueOf(g).Pointer()
			}
			instances.Store(ptr, true)
		}(i)
	}
	wg.Wait()

	count := 0
	instances.Range(func(_, _ any) bool {
		count++
		return true
	})
	if count != 1 {
		t.Fatalf("expected 1 unique instance pointer, got %d", count)
	}
}

func TestErrorUnwrapAndErrorsIsAs(t *testing.T) {
	rootErr := errors.New("underlying DB failure")
	c := New()
	if err := c.Provide(func() (*testConfig, error) {
		return nil, rootErr
	}); err != nil {
		t.Fatal(err)
	}

	_, err := Get[*testConfig](c)
	if err == nil {
		t.Fatal("expected provider error")
	}

	// Verify errors.Is unwraps properly
	if !errors.Is(err, rootErr) {
		t.Fatalf("errors.Is failed to find root error in: %v", err)
	}

	// Verify errors.As
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("errors.As failed to match *ProviderError in: %v", err)
	}
	if provErr.Err != rootErr {
		t.Fatalf("ProviderError.Err = %v, want %v", provErr.Err, rootErr)
	}
}
