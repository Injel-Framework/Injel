// Package httpserver provides an HTTP server and REST API toolkit built on top
// of the Go standard library net/http with zero external dependencies.
//
// # Routing and Validation
//
// The router wraps [net/http.ServeMux] (Go 1.22+ method matching and path parameters)
// and incorporates sandbox pre-validation. Route registration errors and pattern conflicts
// are caught early and returned as [*RouteError] without causing panics or corrupting state.
//
// # Middleware Chain
//
// Middlewares adhere to the idiomatic Go signature:
//
//	type Middleware func(http.Handler) http.Handler
//
// Built-in middlewares include:
//   - [RequestID]: Inspects or generates safe X-Request-ID headers and injects them into the context.
//   - [Logging]: Logs method, path, status, duration, and request ID via standard loggers.
//   - [Recovery]: Catches panics in downstream handlers, logs stack traces, and returns HTTP 500 JSON.
//
// # Application Lifecycle Integration
//
// [Server] directly implements [injel.StartHook] ([Server.OnStart]) and [injel.StopHook] ([Server.OnStop]),
// enabling non-blocking port binding at startup, early collision detection, and graceful
// shutdown with deadline timeouts.
package httpserver
