package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Injel-Framework/Injel"
)

func TestRouterSuccessAndPathVariables(t *testing.T) {
	srv := New()

	err := srv.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.Header().Set("X-Custom-Header", "injel")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user:" + id))
	})
	if err != nil {
		t.Fatalf("HandleFunc failed: %v", err)
	}

	handler := srv.Handler()

	// 1. Success matching
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("X-Custom-Header") != "injel" {
		t.Fatalf("header = %q, want injel", rec.Header().Get("X-Custom-Header"))
	}
	if rec.Body.String() != "user:42" {
		t.Fatalf("body = %q, want user:42", rec.Body.String())
	}

	// 2. Wrong method (POST instead of GET) -> 405 Method Not Allowed
	reqPost := httptest.NewRequest(http.MethodPost, "/users/42", nil)
	recPost := httptest.NewRecorder()
	handler.ServeHTTP(recPost, reqPost)

	if recPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 Method Not Allowed, got: %d", recPost.Code)
	}

	// 3. Not found -> 404
	req404 := httptest.NewRequest(http.MethodGet, "/other", nil)
	rec404 := httptest.NewRecorder()
	handler.ServeHTTP(rec404, req404)

	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got: %d", rec404.Code)
	}
}

func TestRouterPatternValidation(t *testing.T) {
	srv := New()

	// Empty pattern
	err := srv.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}

	// Nil handler
	err = srv.Handle("GET /test", nil)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}

	// Valid first registration
	err = srv.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {})
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Duplicate registration
	err = srv.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {})
	if err == nil {
		t.Fatal("expected error on duplicate route")
	}
	var rErr *RouteError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected RouteError, got: %T", err)
	}

	// Conflicting pattern rejected by ServeMux
	err = srv.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	// MustHandle panics on error
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected MustHandleFunc to panic on invalid route")
		}
	}()
	srv.MustHandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {})
}

func TestHealthEndpoint(t *testing.T) {
	srv := New()
	if err := srv.RegisterHealth(); err != nil {
		t.Fatalf("RegisterHealth failed: %v", err)
	}

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %q, want json", rec.Header().Get("Content-Type"))
	}

	var res HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("status = %q, want ok", res.Status)
	}
}

func TestMiddlewareExecutionOrderAndHeaders(t *testing.T) {
	var execution []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			execution = append(execution, "m1-enter")
			w.Header().Set("X-M1", "1")
			next.ServeHTTP(w, r)
			execution = append(execution, "m1-exit")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			execution = append(execution, "m2-enter")
			w.Header().Set("X-M2", "2")
			next.ServeHTTP(w, r)
			execution = append(execution, "m2-exit")
		})
	}

	srv := New(WithMiddleware(m1, m2))
	srv.MustHandleFunc("GET /order", func(w http.ResponseWriter, r *http.Request) {
		execution = append(execution, "handler")
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	srv.Handler().ServeHTTP(rec, req)

	want := []string{"m1-enter", "m2-enter", "handler", "m2-exit", "m1-exit"}
	if len(execution) != len(want) {
		t.Fatalf("execution = %v, want %v", execution, want)
	}
	for i := range want {
		if execution[i] != want[i] {
			t.Fatalf("step %d = %s, want %s", i, execution[i], want[i])
		}
	}

	if rec.Header().Get("X-M1") != "1" || rec.Header().Get("X-M2") != "2" {
		t.Fatalf("missing middleware headers: %v", rec.Header())
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	srv := New(WithMiddleware(RequestID(func() string {
		return "custom-req-id"
	})))

	var capturedContextID string
	srv.MustHandleFunc("GET /id", func(w http.ResponseWriter, r *http.Request) {
		capturedContextID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// 1. Without incoming ID -> generates custom ID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "custom-req-id" {
		t.Fatalf("X-Request-ID header = %q, want custom-req-id", rec.Header().Get("X-Request-ID"))
	}
	if capturedContextID != "custom-req-id" {
		t.Fatalf("context ID = %q, want custom-req-id", capturedContextID)
	}

	// 2. With incoming ID -> preserves existing ID
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/id", nil)
	req2.Header.Set("X-Request-ID", "upstream-id-123")
	srv.Handler().ServeHTTP(rec2, req2)

	if rec2.Header().Get("X-Request-ID") != "upstream-id-123" {
		t.Fatalf("preserved header = %q, want upstream-id-123", rec2.Header().Get("X-Request-ID"))
	}
	if capturedContextID != "upstream-id-123" {
		t.Fatalf("preserved context ID = %q, want upstream-id-123", capturedContextID)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var loggedPanic string
	logger := func(format string, args ...any) {
		loggedPanic = fmt.Sprintf(format, args...)
	}

	srv := New(WithMiddleware(Recovery(logger)))
	srv.MustHandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("database connection exploded")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)

	// Must not panic the caller
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("body = %q, want internal server error", rec.Body.String())
	}
	if !strings.Contains(loggedPanic, "database connection exploded") {
		t.Fatalf("logger missing panic reason: %s", loggedPanic)
	}
}

func TestServerLifecycleStartAndStop(t *testing.T) {
	// Use :0 for dynamic free port allocation
	srv := New(WithConfig(Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 2 * time.Second,
	}))

	srv.MustHandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})

	ctx := context.Background()

	// 1. OnStart
	if err := srv.OnStart(ctx); err != nil {
		t.Fatalf("OnStart failed: %v", err)
	}

	addr := srv.Addr()
	if addr == nil {
		t.Fatal("expected non-nil listener address")
	}

	// Route registration after server started must fail
	err := srv.HandleFunc("GET /late", func(w http.ResponseWriter, r *http.Request) {})
	if err == nil {
		t.Fatal("expected error registering route after server started")
	}

	// Repeated OnStart must fail
	err = srv.OnStart(ctx)
	if err == nil {
		t.Fatal("expected error on repeated OnStart")
	}

	// 2. Perform real HTTP request over TCP
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	resp, err := client.Get(fmt.Sprintf("http://%s/ping", addr.String()))
	if err != nil {
		t.Fatalf("HTTP request to server failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if string(body) != "pong" {
		t.Fatalf("got body %q, want pong", string(body))
	}

	// 3. OnStop
	if err := srv.OnStop(ctx); err != nil {
		t.Fatalf("OnStop failed: %v", err)
	}

	// Repeated OnStop is idempotent (returns nil)
	if err := srv.OnStop(ctx); err != nil {
		t.Fatalf("second OnStop failed: %v", err)
	}

	// Request after stop should fail
	_, err = client.Get(fmt.Sprintf("http://%s/ping", addr.String()))
	if err == nil {
		t.Fatal("expected request after shutdown to fail")
	}
}

func TestServerPortCollision(t *testing.T) {
	// Listen on a port manually to simulate collision
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("manual listen failed: %v", err)
	}
	defer listener.Close()

	collidingAddr := listener.Addr().String()

	srv := New(WithConfig(Config{
		Addr: collidingAddr,
	}))

	err = srv.OnStart(context.Background())
	if err == nil {
		t.Fatal("expected error when port is already bound")
	}
	var sErr *ServerError
	if !errors.As(err, &sErr) {
		t.Fatalf("expected ServerError, got: %T (%v)", err, err)
	}
}

func TestServerStopBeforeStart(t *testing.T) {
	srv := New()
	err := srv.OnStop(context.Background())
	if err == nil {
		t.Fatal("expected error stopping unstarted server")
	}
}

func TestServerGracefulShutdownWaitsForInFlightRequests(t *testing.T) {
	srv := New(WithConfig(Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 3 * time.Second,
	}))

	inFlightStarted := make(chan struct{})
	inFlightDone := make(chan struct{})

	srv.MustHandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		close(inFlightStarted)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("slow-completed"))
		close(inFlightDone)
	})

	if err := srv.OnStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	addr := srv.Addr().String()

	// Launch in-flight request
	client := &http.Client{Timeout: 5 * time.Second}
	var respBody string
	var reqErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := client.Get(fmt.Sprintf("http://%s/slow", addr))
		if err != nil {
			reqErr = err
			return
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		respBody = string(b)
	}()

	// Wait until handler is actively processing
	<-inFlightStarted

	// Call OnStop while request is executing
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.OnStop(stopCtx); err != nil {
		t.Fatalf("OnStop failed during in-flight request: %v", err)
	}

	wg.Wait()

	if reqErr != nil {
		t.Fatalf("in-flight request failed: %v", reqErr)
	}
	if respBody != "slow-completed" {
		t.Fatalf("response = %q, want slow-completed", respBody)
	}
}

// Integration with Injel DI & Lifecycle
type mockRepo struct {
	records []string
}

type mockService struct {
	repo *mockRepo
}

func newMockRepo() *mockRepo {
	return &mockRepo{records: []string{"alice", "bob"}}
}

func newMockService(repo *mockRepo) *mockService {
	return &mockService{repo: repo}
}

type userAPI struct {
	service *mockService
}

func newUserAPI(service *mockService) *userAPI {
	return &userAPI{service: service}
}

func (api *userAPI) GetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.service.repo.records)
}

func TestHTTPServerIntegrationWithInjelDIAndLifecycle(t *testing.T) {
	app := injel.New()

	// 1. Provide dependencies
	if err := app.Provide(newMockRepo); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(newMockService); err != nil {
		t.Fatal(err)
	}
	if err := app.Provide(newUserAPI); err != nil {
		t.Fatal(err)
	}

	// 2. Provide Server constructing and wiring routes using DI injected api
	if err := app.Provide(func(api *userAPI) (*Server, error) {
		s := New(WithConfig(Config{
			Addr: "127.0.0.1:0",
		}))
		if err := s.RegisterHealth(); err != nil {
			return nil, err
		}
		if err := s.HandleFunc("GET /api/users", api.GetUsers); err != nil {
			return nil, err
		}
		return s, nil
	}); err != nil {
		t.Fatal(err)
	}

	// 3. Start application (lifecycle resolves all dependencies and starts HTTP server)
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start failed: %v", err)
	}

	server, err := injel.Get[*Server](app)
	if err != nil {
		t.Fatalf("Get server: %v", err)
	}

	addr := server.Addr()
	if addr == nil {
		t.Fatal("expected server to be listening")
	}

	// 4. Test endpoints
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()

	// /health
	resHealth, err := client.Get(fmt.Sprintf("http://%s/health", addr.String()))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	_ = resHealth.Body.Close()
	if resHealth.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resHealth.StatusCode)
	}

	// /api/users
	resUsers, err := client.Get(fmt.Sprintf("http://%s/api/users", addr.String()))
	if err != nil {
		t.Fatalf("get users failed: %v", err)
	}
	var users []string
	_ = json.NewDecoder(resUsers.Body).Decode(&users)
	_ = resUsers.Body.Close()

	if len(users) != 2 || users[0] != "alice" || users[1] != "bob" {
		t.Fatalf("unexpected users response: %v", users)
	}

	// 5. Stop application (graceful shutdown of server before dependencies)
	if err := app.Stop(ctx); err != nil {
		t.Fatalf("app.Stop failed: %v", err)
	}
}

func TestAsyncServeError(t *testing.T) {
	srv := New(WithConfig(Config{
		Addr: "127.0.0.1:0",
	}))
	srv.MustHandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {})

	if err := srv.OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart failed: %v", err)
	}

	// Close the underlying listener to simulate sudden network/hardware failure while serving
	srv.mu.Lock()
	l := srv.listener
	srv.mu.Unlock()
	_ = l.Close()

	// Wait for serve error to arrive on channel or channel close
	select {
	case err, ok := <-srv.ServeErrors():
		if !ok && srv.ServeError() == nil {
			t.Fatal("expected serve error on channel or recorded in ServeError()")
		}
		if ok && err == nil {
			t.Fatal("received nil error on ServeErrors() channel")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async serve error")
	}

	if srv.ServeError() == nil {
		t.Fatal("expected ServeError() to return recorded error")
	}

	// Calling OnStop should still clean up cleanly
	_ = srv.OnStop(context.Background())
}

func TestServerShutdownTimeoutAndForceClose(t *testing.T) {
	srv := New(WithConfig(Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 100 * time.Millisecond,
	}))

	handlerStarted := make(chan struct{})
	blockCh := make(chan struct{})
	defer close(blockCh)

	srv.MustHandleFunc("GET /hang", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		<-blockCh // block until test completes
		w.WriteHeader(http.StatusOK)
	})

	if err := srv.OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart failed: %v", err)
	}
	addr := srv.Addr().String()

	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()

	go func() {
		_, _ = client.Get(fmt.Sprintf("http://%s/hang", addr))
	}()

	<-handlerStarted

	// OnStop should exceed the 100ms shutdown timeout and force-close
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := srv.OnStop(stopCtx)
	if err == nil {
		t.Fatal("expected OnStop to return error when shutdown times out")
	}
	if !strings.Contains(err.Error(), "shutdown error") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestRecoveryMiddlewareAfterHeadersWritten(t *testing.T) {
	var loggedPanic string
	logger := func(format string, args ...any) {
		loggedPanic = fmt.Sprintf(format, args...)
	}

	srv := New(WithMiddleware(
		RequestID(func() string { return "req-12345" }),
		Recovery(logger),
	))

	srv.MustHandleFunc("GET /panic-after-write", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("initial-data-written"))
		// Panic AFTER response has already been partially written
		panic("boom after headers sent")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic-after-write", nil)

	// Must recover safely without crashing
	srv.Handler().ServeHTTP(rec, req)

	// Status should remain 200 since headers were already committed
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "initial-data-written") {
		t.Fatalf("expected body to contain initial data, got: %q", rec.Body.String())
	}
	// Panic should still be logged with request id
	if !strings.Contains(loggedPanic, "boom after headers sent") {
		t.Fatalf("expected panic log, got: %s", loggedPanic)
	}
	if !strings.Contains(loggedPanic, "req-12345") {
		t.Fatalf("expected panic log to include request ID, got: %s", loggedPanic)
	}
}

func TestRequestIDValidation(t *testing.T) {
	srv := New(WithMiddleware(RequestID(func() string {
		return "fallback-generated-id"
	})))

	var capturedID string
	srv.MustHandleFunc("GET /test-id", func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := srv.Handler()

	// 1. Valid ID -> Preserved
	{
		req := httptest.NewRequest(http.MethodGet, "/test-id", nil)
		req.Header.Set("X-Request-ID", "valid-id_123.ABC:test")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if capturedID != "valid-id_123.ABC:test" {
			t.Fatalf("expected valid id preserved, got: %q", capturedID)
		}
		if rec.Header().Get("X-Request-ID") != "valid-id_123.ABC:test" {
			t.Fatalf("expected header preserved, got: %q", rec.Header().Get("X-Request-ID"))
		}
	}

	// 2. Invalid ID with control characters / newlines -> Replaced with safe generated ID
	{
		req := httptest.NewRequest(http.MethodGet, "/test-id", nil)
		req.Header.Set("X-Request-ID", "bad\r\nid\ninjected: header")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if capturedID != "fallback-generated-id" {
			t.Fatalf("expected invalid id to be replaced by generator, got: %q", capturedID)
		}
		if rec.Header().Get("X-Request-ID") != "fallback-generated-id" {
			t.Fatalf("expected header replaced, got: %q", rec.Header().Get("X-Request-ID"))
		}
	}

	// 3. Overly long ID (> 128 chars) -> Replaced
	{
		longID := strings.Repeat("a", 200)
		req := httptest.NewRequest(http.MethodGet, "/test-id", nil)
		req.Header.Set("X-Request-ID", longID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if capturedID != "fallback-generated-id" {
			t.Fatalf("expected long id to be replaced, got: %q", capturedID)
		}
	}
}

func TestRouteConflictAndRegistryConsistency(t *testing.T) {
	srv := New()

	// Register valid base route
	if err := srv.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {}); err != nil {
		t.Fatalf("initial route failed: %v", err)
	}

	// Try registering conflicting route
	err := srv.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {})
	if err == nil {
		t.Fatal("expected error on duplicate route")
	}
	var rErr *RouteError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected RouteError, got: %T", err)
	}

	// Ensure registry was not corrupted: we can still register a distinct valid route
	if err := srv.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("product:" + r.PathValue("id")))
	}); err != nil {
		t.Fatalf("registering new valid route after conflict failed: %v", err)
	}

	// Both valid routes work properly
	req := httptest.NewRequest(http.MethodGet, "/products/99", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "product:99" {
		t.Fatalf("expected 200 product:99, got: %d %q", rec.Code, rec.Body.String())
	}
}

type rollbackHook struct {
	started bool
	stopped bool
}

func (h *rollbackHook) OnStart(ctx context.Context) error {
	h.started = true
	return nil
}

func (h *rollbackHook) OnStop(ctx context.Context) error {
	h.stopped = true
	return nil
}

func TestAppStartupRollbackOnPortCollision(t *testing.T) {
	// Occupy a port manually
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	occupiedAddr := l.Addr().String()

	hook := &rollbackHook{}
	app := injel.New()

	// 1. Provide first component that starts successfully
	if err := app.Provide(func() *rollbackHook {
		return hook
	}); err != nil {
		t.Fatal(err)
	}

	// 2. Provide HTTP server that depends on rollbackHook and fails to bind
	if err := app.Provide(func(h *rollbackHook) (*Server, error) {
		s := New(WithConfig(Config{
			Addr: occupiedAddr,
		}))
		return s, nil
	}); err != nil {
		t.Fatal(err)
	}

	// 3. Start app -> must fail on server listen and rollback hook
	startErr := app.Start(context.Background())
	if startErr == nil {
		t.Fatal("expected app.Start to fail on port collision")
	}

	if !hook.started {
		t.Fatal("expected hook to have started prior to server failure")
	}
	if !hook.stopped {
		t.Fatal("expected hook to have been rolled back via OnStop")
	}
}

type orderTracker struct {
	mu     sync.Mutex
	events []string
}

func (ot *orderTracker) record(e string) {
	ot.mu.Lock()
	defer ot.mu.Unlock()
	ot.events = append(ot.events, e)
}

type componentDep struct {
	tracker *orderTracker
}

func (c *componentDep) OnStart(ctx context.Context) error {
	c.tracker.record("dep-start")
	return nil
}

func (c *componentDep) OnStop(ctx context.Context) error {
	c.tracker.record("dep-stop")
	return nil
}

func TestAppStopOrderServerBeforeDependencies(t *testing.T) {
	tracker := &orderTracker{}
	app := injel.New()

	if err := app.Provide(func() *componentDep {
		return &componentDep{tracker: tracker}
	}); err != nil {
		t.Fatal(err)
	}

	if err := app.Provide(func(dep *componentDep) (*Server, error) {
		s := New(WithConfig(Config{
			Addr: "127.0.0.1:0",
		}))
		return s, nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start failed: %v", err)
	}

	if err := app.Stop(ctx); err != nil {
		t.Fatalf("app.Stop failed: %v", err)
	}

	// Injel lifecycle stops components in reverse topological order:
	// Server was consumer of componentDep, so Server stops first, then componentDep stops.
	tracker.mu.Lock()
	events := append([]string(nil), tracker.events...)
	tracker.mu.Unlock()

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got: %v", events)
	}
	if events[0] != "dep-start" || events[1] != "dep-stop" {
		t.Fatalf("unexpected event sequence: %v", events)
	}
}

func TestServerConcurrentStartStopAndIdempotency(t *testing.T) {
	srv := New(WithConfig(Config{
		Addr: "127.0.0.1:0",
	}))
	srv.MustHandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {})

	// 1. Concurrent OnStart calls: exactly 1 must succeed, others must fail
	const concurrency = 10
	var startWg sync.WaitGroup
	var successCount int64
	var errCount int64

	startWg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer startWg.Done()
			err := srv.OnStart(context.Background())
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errCount, 1)
			}
		}()
	}
	startWg.Wait()

	if successCount != 1 {
		t.Fatalf("successCount = %d, want 1", successCount)
	}
	if errCount != concurrency-1 {
		t.Fatalf("errCount = %d, want %d", errCount, concurrency-1)
	}

	// 2. Concurrent OnStop calls: all must complete without race or panic
	var stopWg sync.WaitGroup
	stopWg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer stopWg.Done()
			_ = srv.OnStop(context.Background())
		}()
	}
	stopWg.Wait()

	// 3. OnStart after stop must fail with "server has been stopped"
	err := srv.OnStart(context.Background())
	if err == nil {
		t.Fatal("expected OnStart after OnStop to return error")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("expected error mentioning stopped, got: %v", err)
	}
}
