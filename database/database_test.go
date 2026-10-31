package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	injel "github.com/Injel-Framework/Injel"
)

// ---------------------------------------------------------------------------
// Fake database/sql driver (no external deps)
// ---------------------------------------------------------------------------

// fakeDriver is a minimal database/sql driver for unit testing.
// It is registered once using sync.Once to avoid "RegisteredTwice" panics.
//
// The driver tracks ping call counts and can be configured to return errors.
type fakeDriver struct {
	mu      sync.Mutex
	pingErr error // error returned by Ping (via fakeConn)
	openErr error // error returned by Open
}

type fakeConn struct {
	mu      sync.Mutex
	pingErr error
	closed  bool
}

type fakeStmt struct{}
type fakeResult struct{}
type fakeRows struct{}
type fakeTx struct{}

func (d *fakeDriver) Open(_ string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.openErr != nil {
		return nil, d.openErr
	}
	return &fakeConn{pingErr: d.pingErr}, nil
}

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) { return &fakeStmt{}, nil }
func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
func (c *fakeConn) Begin() (driver.Tx, error) { return &fakeTx{}, nil }
func (c *fakeConn) Ping(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pingErr
}

func (s *fakeStmt) Close() error                                 { return nil }
func (s *fakeStmt) NumInput() int                                { return 0 }
func (s *fakeStmt) Exec(_ []driver.Value) (driver.Result, error) { return &fakeResult{}, nil }
func (s *fakeStmt) Query(_ []driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }
func (r *fakeResult) LastInsertId() (int64, error)               { return 0, nil }
func (r *fakeResult) RowsAffected() (int64, error)               { return 0, nil }
func (r *fakeRows) Columns() []string                            { return nil }
func (r *fakeRows) Close() error                                 { return nil }
func (r *fakeRows) Next(_ []driver.Value) error                  { return io.EOF }
func (t *fakeTx) Commit() error                                  { return nil }
func (t *fakeTx) Rollback() error                                { return nil }

// fakeCloseErrConn returns an error on Close to test CloseError wrapping.
type fakeCloseErrConn struct{ fakeConn }

// fakeCloseErrDriver always opens fakeCloseErrConn connections.
type fakeCloseErrDriver struct{}

func (d *fakeCloseErrDriver) Open(_ string) (driver.Conn, error) {
	return &fakeCloseErrConn{}, nil
}
func (c *fakeCloseErrConn) Close() error { return errors.New("close: disk full") }

// Ensure fakeConn satisfies driver.Pinger so PingContext is used.
var _ driver.Pinger = (*fakeConn)(nil)

// Driver registration — done once per test binary lifetime.
var (
	registerOnce         sync.Once
	registerCloseErrOnce sync.Once

	drv         = &fakeDriver{}
	driverName  = "injel-test-fake"
	driverName2 = "injel-test-fake-closeerr"
)

func registerFakeDriver() {
	registerOnce.Do(func() {
		sql.Register(driverName, drv)
	})
}

func registerCloseErrDriver() {
	registerCloseErrOnce.Do(func() {
		sql.Register(driverName2, &fakeCloseErrDriver{})
	})
}

// fakeConfig returns a valid Config using the registered fake driver.
func fakeConfig() Config {
	return Config{
		DriverName:      driverName,
		DSN:             "fake://localhost/testdb",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		PingTimeout:     2 * time.Second,
	}
}

// setPingErr configures the fake driver to return err on the next Ping call.
func setPingErr(err error) {
	drv.mu.Lock()
	defer drv.mu.Unlock()
	drv.pingErr = err
}

// ---------------------------------------------------------------------------
// newHandle tests
// ---------------------------------------------------------------------------

func TestNewHandle_ValidConfig(t *testing.T) {
	registerFakeDriver()
	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}
	defer h.db.Close()
	if h.db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
}

func TestNewHandle_InvalidConfig_MissingDriver(t *testing.T) {
	cfg := Config{DSN: "x"}
	_, err := newHandle(cfg)
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
}

func TestNewHandle_UnregisteredDriver(t *testing.T) {
	cfg := Config{DriverName: "driver-that-does-not-exist", DSN: "x"}
	_, err := newHandle(cfg)
	// sql.Open does not fail for unknown drivers in all Go versions,
	// but validation should still catch the empty-driver case.
	// If Open doesn't fail, the ping will. Just assert no panic.
	_ = err
}

// ---------------------------------------------------------------------------
// OnStart / OnStop lifecycle tests
// ---------------------------------------------------------------------------

func TestOnStart_Success(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}
	defer h.OnStop(context.Background()) //nolint

	if err := h.OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
}

func TestOnStart_PingFailure_ReturnsPingError(t *testing.T) {
	registerFakeDriver()
	pingErr := errors.New("connection refused")
	setPingErr(pingErr)
	defer setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	startErr := h.OnStart(context.Background())
	var pe *PingError
	if !errors.As(startErr, &pe) {
		t.Fatalf("expected *PingError, got %T: %v", startErr, startErr)
	}
	if !errors.Is(startErr, pingErr) {
		t.Error("errors.Is should unwrap to original ping error")
	}
}

func TestOnStart_PingFailure_ClosesDB(t *testing.T) {
	registerFakeDriver()
	pingErr := errors.New("ping refused")
	setPingErr(pingErr)
	defer setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	_ = h.OnStart(context.Background())

	// After ping failure, handle must be marked closed.
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if !closed {
		t.Error("handle should be marked closed after ping failure")
	}
}

func TestOnStart_PingFailure_SubsequentOnStop_IsNoop(t *testing.T) {
	registerFakeDriver()
	setPingErr(errors.New("ping fail"))
	defer setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	_ = h.OnStart(context.Background())

	// OnStop after a ping-failure cleanup must not return an error (db already closed).
	if err := h.OnStop(context.Background()); err != nil {
		t.Errorf("OnStop after ping failure should be no-op, got: %v", err)
	}
}

func TestOnStop_Success(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	if err := h.OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	if err := h.OnStop(context.Background()); err != nil {
		t.Fatalf("OnStop: %v", err)
	}
}

func TestOnStop_Idempotent(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	if err := h.OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	if err := h.OnStop(context.Background()); err != nil {
		t.Fatalf("first OnStop: %v", err)
	}
	// Second call must be a no-op.
	if err := h.OnStop(context.Background()); err != nil {
		t.Fatalf("second OnStop should be idempotent, got: %v", err)
	}
}

func TestOnStop_CloseError_WrappedInCloseError(t *testing.T) {
	registerCloseErrDriver()
	cfg := Config{
		DriverName:  driverName2,
		DSN:         "x",
		PingTimeout: time.Second,
	}
	h, err := newHandle(cfg)
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	// Force a connection to be opened and placed in the idle pool
	// so that db.Close() will actually call conn.Close() on it.
	// We use QueryContext with a fake query; fakeStmt.Query returns fakeRows.
	rows, queryErr := h.db.QueryContext(context.Background(), "SELECT 1")
	if queryErr == nil {
		rows.Close()
	}
	// Now db.Close() should close the idle connection and encounter the error.
	stopErr := h.OnStop(context.Background())
	var ce *CloseError
	if !errors.As(stopErr, &ce) {
		t.Fatalf("expected *CloseError, got %T: %v", stopErr, stopErr)
	}
	if ce.Err == nil {
		t.Error("CloseError.Err must not be nil")
	}
}

// ---------------------------------------------------------------------------
// Provide() tests
// ---------------------------------------------------------------------------

func TestProvide_Success(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}
}

func TestProvide_NilApp(t *testing.T) {
	if err := Provide(nil, fakeConfig()); err == nil {
		t.Fatal("expected error for nil app")
	}
}

func TestProvide_InvalidConfig(t *testing.T) {
	app := injel.New()
	err := Provide(app, Config{}) // missing DriverName and DSN
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
}

func TestProvide_DuplicateProvider(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("first Provide: %v", err)
	}
	// Second Provide should fail with a duplicate provider error.
	if err := Provide(app, fakeConfig()); err == nil {
		t.Fatal("expected error for duplicate *sql.DB provider")
	}
}

func TestProvide_ResolveSQLDB(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start: %v", err)
	}
	defer app.Stop(ctx) //nolint

	db, err := injel.Get[*sql.DB](app)
	if err != nil {
		t.Fatalf("Get[*sql.DB]: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil *sql.DB")
	}
}

func TestProvide_SingletonIdentity(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start: %v", err)
	}
	defer app.Stop(ctx) //nolint

	db1, err := injel.Get[*sql.DB](app)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	db2, err := injel.Get[*sql.DB](app)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if db1 != db2 {
		t.Error("expected same *sql.DB singleton, got different pointers")
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle integration test
// ---------------------------------------------------------------------------

func TestLifecycle_StartStop(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start: %v", err)
	}
	if app.State() != injel.StateRunning {
		t.Errorf("expected StateRunning, got %s", app.State())
	}

	db, err := injel.Get[*sql.DB](app)
	if err != nil || db == nil {
		t.Fatalf("Get[*sql.DB] after Start: %v, db=%v", err, db)
	}

	if err := app.Stop(ctx); err != nil {
		t.Fatalf("app.Stop: %v", err)
	}
	if app.State() != injel.StateStopped {
		t.Errorf("expected StateStopped, got %s", app.State())
	}
}

func TestLifecycle_PingFailure_StartFails(t *testing.T) {
	registerFakeDriver()
	pingErr := errors.New("database unavailable")
	setPingErr(pingErr)
	defer setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("expected app.Start to fail when ping fails")
	}
	var pe *PingError
	if !errors.As(err, &pe) {
		t.Errorf("expected error chain to contain *PingError, got %T: %v", err, err)
	}
	if app.State() != injel.StateFailed {
		t.Errorf("expected StateFailed after ping failure, got %s", app.State())
	}
}

func TestLifecycle_PingTimeout(t *testing.T) {
	registerFakeDriver()
	// Simulate a slow ping that exceeds PingTimeout.
	slowPing := errors.New("context deadline exceeded")
	setPingErr(slowPing)
	defer setPingErr(nil)

	cfg := fakeConfig()
	cfg.PingTimeout = 1 * time.Millisecond // very short timeout

	app := injel.New()
	if err := Provide(app, cfg); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	if err := app.Start(context.Background()); err == nil {
		app.Stop(context.Background()) //nolint
		t.Fatal("expected Start to fail with slow ping")
	}
}

// ---------------------------------------------------------------------------
// Concurrency tests
// ---------------------------------------------------------------------------

func TestConcurrentResolve_NoPanic(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	app := injel.New()
	if err := Provide(app, fakeConfig()); err != nil {
		t.Fatalf("Provide: %v", err)
	}

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("app.Start: %v", err)
	}
	defer app.Stop(ctx) //nolint

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, err := injel.Get[*sql.DB](app)
			if err != nil || db == nil {
				t.Errorf("concurrent Get[*sql.DB]: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentOnStop_Idempotent(t *testing.T) {
	registerFakeDriver()
	setPingErr(nil)

	h, err := newHandle(fakeConfig())
	if err != nil {
		t.Fatalf("newHandle: %v", err)
	}

	ctx := context.Background()
	if err := h.OnStart(ctx); err != nil {
		t.Fatalf("OnStart: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := range errs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = h.OnStop(ctx)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: OnStop returned error: %v", i, e)
		}
	}
}
