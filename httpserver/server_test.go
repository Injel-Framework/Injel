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
