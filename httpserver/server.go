package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Default configurations
const (
	DefaultAddr              = ":8080"
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultIdleTimeout       = 60 * time.Second
	DefaultShutdownTimeout   = 10 * time.Second
	DefaultMaxHeaderBytes    = 1 << 20 // 1MB
)

// Config defines the operational parameters of the HTTP server.
type Config struct {
	Addr              string        `env:"HTTP_ADDR,default=:8080"`
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT,default=5s"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT,default=60s"`
	ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT,default=10s"`
	MaxHeaderBytes    int           `env:"HTTP_MAX_HEADER_BYTES,default=1048576"`
}

// ServerOption configures an HTTP server instance.
type ServerOption func(*Server)

// WithConfig sets the server configuration.
func WithConfig(cfg Config) ServerOption {
	return func(s *Server) {
		s.config = cfg
	}
}

// WithMiddleware appends middlewares to the server.
func WithMiddleware(mw ...Middleware) ServerOption {
	return func(s *Server) {
		s.Use(mw...)
	}
}

// Server represents an Injel HTTP server managed by Application Lifecycle.
type Server struct {
	config      Config
	router      *router
	middlewares []Middleware
	handler     http.Handler

	mu         sync.Mutex
	listener   net.Listener
	httpServer *http.Server
	started    bool
	stopped    bool

	serveErrMu sync.RWMutex
	serveErr   error
	serveErrCh chan error
}

// New creates a new HTTP server. If no Config is provided, defaults are applied.
func New(opts ...ServerOption) *Server {
	s := &Server{
		config: Config{
			Addr:              DefaultAddr,
			ReadHeaderTimeout: DefaultReadHeaderTimeout,
			IdleTimeout:       DefaultIdleTimeout,
			ShutdownTimeout:   DefaultShutdownTimeout,
			MaxHeaderBytes:    DefaultMaxHeaderBytes,
		},
		router:     newRouter(),
		serveErrCh: make(chan error, 1),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	return s
}

// Use appends one or more middlewares to the server's global middleware chain.
// Middlewares are executed in the order they were registered.
func (s *Server) Use(middlewares ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mw := range middlewares {
		if mw != nil {
			s.middlewares = append(s.middlewares, mw)
		}
	}
}

// Handle registers a route pattern with an http.Handler.
// If pattern is invalid, conflicting, or server has started, an error is returned.
func (s *Server) Handle(pattern string, handler http.Handler) error {
	return s.router.handle(pattern, handler)
}

// HandleFunc registers a route pattern with an http.HandlerFunc.
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) error {
	if handler == nil {
		return &RouteError{
			Pattern: pattern,
			Reason:  "handler cannot be nil",
		}
	}
	return s.Handle(pattern, http.HandlerFunc(handler))
}

// MustHandle registers a route pattern and panics if registration fails.
func (s *Server) MustHandle(pattern string, handler http.Handler) {
	if err := s.Handle(pattern, handler); err != nil {
		panic(err)
	}
}

// MustHandleFunc registers a route pattern function and panics if registration fails.
func (s *Server) MustHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if err := s.HandleFunc(pattern, handler); err != nil {
		panic(err)
	}
}

// RegisterHealth registers a health endpoint (defaults to "GET /health").
func (s *Server) RegisterHealth(pattern ...string) error {
	pat := "GET /health"
	if len(pattern) > 0 && pattern[0] != "" {
		pat = pattern[0]
	}
	return s.HandleFunc(pat, HealthHandler())
}

// Handler returns the compiled http.Handler with all middlewares applied.
// This is especially useful for testing with net/http/httptest.
func (s *Server) Handler() http.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handler != nil {
		return s.handler
	}

	mux := s.router.buildMux()
	var h http.Handler = mux
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		h = s.middlewares[i](h)
	}
	return h
}

// Addr returns the network address the server is listening on, or nil if not listening.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// ServeErrors returns a receive-only channel yielding asynchronous serve errors.
func (s *Server) ServeErrors() <-chan error {
	return s.serveErrCh
}

// OnStart implements injel.StartHook. It validates configuration, binds the listener,
// and starts serving HTTP requests in a background goroutine.
func (s *Server) OnStart(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return &ServerError{
			Action: "start",
			Addr:   s.config.Addr,
			Err:    errors.New("server already started"),
		}
	}

	if s.config.ShutdownTimeout <= 0 {
		s.config.ShutdownTimeout = DefaultShutdownTimeout
	}
	if s.config.Addr == "" {
		s.config.Addr = DefaultAddr
	}

	// 1. Build mux and middleware chain
	mux := s.router.buildMux()
	var h http.Handler = mux
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		h = s.middlewares[i](h)
	}
	s.handler = h

	// 2. Bind listener
	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "tcp", s.config.Addr)
	if err != nil {
		return &ServerError{
			Action: "listen",
			Addr:   s.config.Addr,
			Err:    err,
		}
	}
	s.listener = l

	// 3. Setup http.Server
	s.httpServer = &http.Server{
		Addr:              s.config.Addr,
		Handler:           s.handler,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
		IdleTimeout:       s.config.IdleTimeout,
		MaxHeaderBytes:    s.config.MaxHeaderBytes,
	}

	s.started = true

	// 4. Start serving in background goroutine
	go func() {
		err := s.httpServer.Serve(s.listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serveErrMu.Lock()
			s.serveErr = err
			s.serveErrMu.Unlock()

			select {
			case s.serveErrCh <- err:
			default:
			}
		}
	}()

	return nil
}

// OnStop implements injel.StopHook. It gracefully shuts down the HTTP server.
func (s *Server) OnStop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return &ServerError{
			Action: "stop",
			Addr:   s.config.Addr,
			Err:    errors.New("server not started"),
		}
	}

	if s.stopped {
		return nil // Idempotent
	}

	// Disable keep-alives so idle connections are closed and clients do not reuse connections
	s.httpServer.SetKeepAlivesEnabled(false)

	shutdownCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.config.ShutdownTimeout > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(ctx, s.config.ShutdownTimeout)
		defer cancel()
	}

	var errs []error
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown error: %w", err))
		// Force close on shutdown error/timeout
		if closeErr := s.httpServer.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			errs = append(errs, fmt.Errorf("force close error: %w", closeErr))
		}
	}

	s.stopped = true

	if len(errs) > 0 {
		return &ServerError{
			Action: "stop",
			Addr:   s.config.Addr,
			Err:    errors.Join(errs...),
		}
	}

	return nil
}
