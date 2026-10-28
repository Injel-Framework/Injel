package injel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type appConfig struct {
	env string
}

type appService struct {
	cfg *appConfig
}

func newAppConfig() *appConfig {
	return &appConfig{env: "test"}
}

func newAppService(cfg *appConfig) *appService {
	return &appService{cfg: cfg}
}

func TestAppProvideAndGet(t *testing.T) {
	app := New()
	if app.State() != StateNew {
		t.Fatalf("initial state = %s, want %s", app.State(), StateNew)
	}

	if err := app.Provide(newAppConfig); err != nil {
		t.Fatalf("Provide config: %v", err)
	}
	if err := app.Provide(newAppService); err != nil {
		t.Fatalf("Provide service: %v", err)
	}

	if err := app.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	svc, err := Get[*appService](app)
	if err != nil {
		t.Fatalf("Get service: %v", err)
	}
	if svc == nil || svc.cfg == nil || svc.cfg.env != "test" {
		t.Fatalf("unexpected service value: %+v", svc)
	}
}

func TestNilAppCalls(t *testing.T) {
	var app *App
	if app.Container() != nil {
		t.Fatal("expected nil container from nil app")
	}
	if app.State() != StateFailed {
		t.Fatalf("expected StateFailed for nil app, got %s", app.State())
	}
	if err := app.Provide(newAppConfig); err == nil {
		t.Fatal("expected error providing to nil app")
	}
	if err := app.ProvideAs(newAppConfig, nil); err == nil {
		t.Fatal("expected error ProvideAs on nil app")
	}
	if err := app.Bind(nil, nil); err == nil {
		t.Fatal("expected error Bind on nil app")
	}
	if err := app.Validate(); err == nil {
		t.Fatal("expected error validating nil app")
	}
	if _, err := app.Resolve(nil); err == nil {
		t.Fatal("expected error resolving nil app")
	}
	if _, err := Get[*appService](app); err == nil {
		t.Fatal("expected error getting from nil app")
	}
	if err := app.Start(context.Background()); err == nil {
		t.Fatal("expected error starting nil app")
	}
	if err := app.Stop(context.Background()); err == nil {
		t.Fatal("expected error stopping nil app")
	}
	app.AppendHook(Hook{}) // Should not panic
}

// Lifecycle test fixtures

type recordedEvent struct {
	component string
	action    string
}

type lifecycleTracker struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (tr *lifecycleTracker) record(comp, action string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.events = append(tr.events, recordedEvent{component: comp, action: action})
}

func (tr *lifecycleTracker) list() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	res := make([]string, len(tr.events))
	for i, e := range tr.events {
		res[i] = e.component + ":" + e.action
	}
	return res
}

type dbComponent struct {
	tr *lifecycleTracker
}

func (db *dbComponent) OnStart(ctx context.Context) error {
	db.tr.record("db", "start")
	return nil
}

func (db *dbComponent) OnStop(ctx context.Context) error {
	db.tr.record("db", "stop")
	return nil
}

type repoComponent struct {
	db *dbComponent
	tr *lifecycleTracker
}

func (repo *repoComponent) OnStart(ctx context.Context) error {
	repo.tr.record("repo", "start")
	return nil
}

func (repo *repoComponent) OnStop(ctx context.Context) error {
	repo.tr.record("repo", "stop")
	return nil
}

type serviceComponent struct {
	repo *repoComponent
	tr   *lifecycleTracker
}

func (svc *serviceComponent) OnStart(ctx context.Context) error {
	svc.tr.record("service", "start")
	return nil
}

func (svc *serviceComponent) OnStop(ctx context.Context) error {
	svc.tr.record("service", "stop")
	return nil
}

func TestLifecycleStartupAndShutdownOrder(t *testing.T) {
	tracker := &lifecycleTracker{}
	app := New()

	// Register in random order to prove resolution follows dependency graph, not registration order.
	if err := app.Provide(func(repo *repoComponent) *serviceComponent {
		return &serviceComponent{repo: repo, tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(func(db *dbComponent) *repoComponent {
		return &repoComponent{db: db, tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(func() *dbComponent {
		return &dbComponent{tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Start application
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if app.State() != StateRunning {
		t.Fatalf("state = %s, want %s", app.State(), StateRunning)
	}

	// Verify startup order: db -> repo -> service
	wantStart := []string{"db:start", "repo:start", "service:start"}
	gotEvents := tracker.list()
	if len(gotEvents) != len(wantStart) {
		t.Fatalf("got start events %v, want %v", gotEvents, wantStart)
	}
	for i, want := range wantStart {
		if gotEvents[i] != want {
			t.Fatalf("start event %d = %s, want %s", i, gotEvents[i], want)
		}
	}

	// 2. Stop application
	if err := app.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if app.State() != StateStopped {
		t.Fatalf("state = %s, want %s", app.State(), StateStopped)
	}

	// Verify shutdown order is exactly reverse: service -> repo -> db
	wantAll := []string{
		"db:start", "repo:start", "service:start",
		"service:stop", "repo:stop", "db:stop",
	}
	gotAll := tracker.list()
	if len(gotAll) != len(wantAll) {
		t.Fatalf("got all events %v, want %v", gotAll, wantAll)
	}
	for i, want := range wantAll {
		if gotAll[i] != want {
			t.Fatalf("event %d = %s, want %s", i, gotAll[i], want)
		}
	}

	// 3. Repeated Stop on StateStopped is idempotent
	if err := app.Stop(ctx); err != nil {
		t.Fatalf("second Stop should succeed, got: %v", err)
	}
	if app.State() != StateStopped {
		t.Fatalf("state after second stop = %s, want %s", app.State(), StateStopped)
	}
}

func TestLifecycleStopBeforeStart(t *testing.T) {
	app := New()
	err := app.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error stopping unstarted app")
	}
	if !strings.Contains(err.Error(), "has not been started") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLifecycleRepeatedStart(t *testing.T) {
	app := New()
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("expected error on repeated Start")
	}
	if !strings.Contains(err.Error(), "already started") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

type failingStartComponent struct {
	tr *lifecycleTracker
}

func (f *failingStartComponent) OnStart(ctx context.Context) error {
	f.tr.record("failing", "start")
	return errors.New("boom during start")
}

func (f *failingStartComponent) OnStop(ctx context.Context) error {
	f.tr.record("failing", "stop")
	return nil
}

func TestLifecycleRollbackOnStartupFailure(t *testing.T) {
	tracker := &lifecycleTracker{}
	app := New()

	if err := app.Provide(func() *dbComponent {
		return &dbComponent{tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(func(db *dbComponent) *repoComponent {
		return &repoComponent{db: db, tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(func(repo *repoComponent) *failingStartComponent {
		return &failingStartComponent{tr: tracker}
	}); err != nil {
		t.Fatal(err)
	}

	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("expected startup error")
	}
	if !strings.Contains(err.Error(), "boom during start") {
		t.Fatalf("expected original error preserved, got: %v", err)
	}
	if app.State() != StateFailed {
		t.Fatalf("state = %s, want %s", app.State(), StateFailed)
	}

	// Verify rollback:
	// db:start -> repo:start -> failing:start
	// then rollback repo:stop -> db:stop (failing didn't start successfully, so not stopped)
	wantEvents := []string{
		"db:start", "repo:start", "failing:start",
		"repo:stop", "db:stop",
	}
	gotEvents := tracker.list()
	if len(gotEvents) != len(wantEvents) {
		t.Fatalf("got events %v, want %v", gotEvents, wantEvents)
	}
	for i, want := range wantEvents {
		if gotEvents[i] != want {
			t.Fatalf("rollback event %d = %s, want %s", i, gotEvents[i], want)
		}
	}
}

type cleanupErrorComponent struct {
	name string
	err  error
}

func (c *cleanupErrorComponent) OnStop(ctx context.Context) error {
	return c.err
}

func TestLifecycleMultipleStopErrorsPreserved(t *testing.T) {
	app := New()
	err1 := errors.New("disk unmount failed")
	err2 := errors.New("network flush failed")

	if err := app.Provide(func() *cleanupErrorComponent {
		return &cleanupErrorComponent{name: "comp1", err: err1}
	}); err != nil {
		t.Fatal(err)
	}

	app.AppendHook(Hook{
		OnStop: func(ctx context.Context) error {
			return err2
		},
	})

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err := app.Stop(context.Background())
	if err == nil {
		t.Fatal("expected stop errors")
	}
	if app.State() != StateFailed {
		t.Fatalf("state = %s, want %s", app.State(), StateFailed)
	}

	// Verify errors.Is matches both errors via errors.Join
	if !errors.Is(err, err1) {
		t.Fatalf("expected err1 in joined error: %v", err)
	}
	if !errors.Is(err, err2) {
		t.Fatalf("expected err2 in joined error: %v", err)
	}
}

func TestLifecycleContextCancellationOnStartup(t *testing.T) {
	app := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := app.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting with cancelled context")
	}
	if app.State() != StateFailed {
		t.Fatalf("state = %s, want %s", app.State(), StateFailed)
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLifecycleContextTimeoutOnShutdown(t *testing.T) {
	app := New()
	app.AppendHook(Hook{
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})

	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	err := app.Stop(ctx)
	if err == nil {
		t.Fatal("expected shutdown context timeout error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleConcurrentStartAndStop(t *testing.T) {
	for attempt := 0; attempt < 5; attempt++ {
		app := New()
		if err := app.Provide(func() *dbComponent {
			return &dbComponent{tr: &lifecycleTracker{}}
		}); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = app.Start(context.Background())
		}()
		go func() {
			defer wg.Done()
			_ = app.Stop(context.Background())
		}()
		wg.Wait()

		// Final state must be one of the valid terminal states, never undefined
		st := app.State()
		if st != StateRunning && st != StateStopped && st != StateFailed && st != StateNew {
			t.Fatalf("unexpected state after concurrent start/stop: %s", st)
		}
	}
}

type appStore interface {
	GetVal() string
}

type memoryStore struct {
	val string
}

func (m *memoryStore) GetVal() string {
	return m.val
}

func TestAppProvideAsAndBind(t *testing.T) {
	app := New()
	if err := ProvideAs[appStore](app, func() *memoryStore {
		return &memoryStore{val: "in-memory"}
	}); err != nil {
		t.Fatalf("ProvideAs: %v", err)
	}

	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer app.Stop(context.Background())

	store, err := Get[appStore](app)
	if err != nil {
		t.Fatalf("Get appStore: %v", err)
	}
	if store.GetVal() != "in-memory" {
		t.Fatalf("store val = %q, want in-memory", store.GetVal())
	}

	concrete, err := Get[*memoryStore](app)
	if err != nil {
		t.Fatalf("Get concrete: %v", err)
	}
	if store != concrete {
		t.Fatal("interface and concrete did not return identical singleton instance")
	}
}
