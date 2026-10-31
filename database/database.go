package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	injel "github.com/Injel-Framework/Injel"
)

// handle manages a *sql.DB instance along with its closed state.
// It implements injel.StartHook and injel.StopHook.
type handle struct {
	mu     sync.Mutex
	db     *sql.DB
	cfg    Config
	closed bool
}

// newHandle opens a database connection with the given configuration.
// It calls sql.Open to create the handle and configures the connection pool,
// but does NOT ping the database — that is deferred to OnStart.
//
// If configuration validation fails, or sql.Open returns an error, newHandle
// returns an error immediately so the issue surfaces at registration time.
func newHandle(cfg Config) (*handle, error) {
	cfg = cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	sqlDB, err := sql.Open(cfg.DriverName, cfg.DSN)
	if err != nil {
		return nil, &OpenError{Driver: cfg.DriverName, Err: err}
	}

	// Configure connection pool parameters.
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return &handle{db: sqlDB, cfg: cfg}, nil
}

// OnStart pings the database to verify connectivity during application startup.
// If the ping fails, the database handle is closed immediately to release
// resources, and a *PingError is returned.
func (h *handle) OnStart(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, h.cfg.PingTimeout)
	defer cancel()

	if err := h.db.PingContext(pingCtx); err != nil {
		// Mark closed and release resources before returning the error
		// so the subsequent OnStop (rollback) becomes a no-op.
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		_ = h.db.Close()
		return &PingError{Err: err}
	}
	return nil
}

// OnStop closes the database connection. It is idempotent: calling it multiple
// times (e.g. after a ping failure rollback) is safe and returns nil.
func (h *handle) OnStop(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}
	h.closed = true

	if err := h.db.Close(); err != nil {
		return &CloseError{Err: err}
	}
	return nil
}

// Provide registers a *sql.DB provider into the Injel application.
// It validates the configuration and opens the database handle at registration
// time so errors surface early, before app.Start is called.
//
// The *sql.DB instance is a singleton within the application context; all
// components that inject *sql.DB receive the same underlying connection pool.
//
// Lifecycle hooks are registered automatically:
//   - OnStart: pings the database; startup fails if the ping fails.
//   - OnStop: closes the database; called in reverse startup order.
//
// Usage:
//
//	err := database.Provide(app, database.Config{
//	    DriverName: "postgres",
//	    DSN:        os.Getenv("DATABASE_URL"),
//	})
//
// The application must separately import the desired database driver:
//
//	import _ "github.com/lib/pq" // PostgreSQL
func Provide(app *injel.App, cfg Config) error {
	if app == nil {
		return fmt.Errorf("database: app cannot be nil")
	}

	h, err := newHandle(cfg)
	if err != nil {
		return err
	}

	// Register the *sql.DB singleton provider. The closure captures h, ensuring
	// a single *sql.DB instance is shared across all dependents.
	if err := app.Provide(func() *sql.DB {
		return h.db
	}); err != nil {
		// If registration fails (e.g. duplicate provider), clean up the handle
		// we just created to avoid a resource leak.
		_ = h.db.Close()
		return fmt.Errorf("database: register *sql.DB provider: %w", err)
	}

	// Register lifecycle hooks so OnStart/OnStop are invoked by the application.
	app.AppendHook(injel.Hook{
		OnStart: h.OnStart,
		OnStop:  h.OnStop,
	})

	return nil
}
