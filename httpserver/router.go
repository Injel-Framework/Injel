package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type routeEntry struct {
	pattern string
	handler http.Handler
}

// router manages route registration and validates patterns against net/http.ServeMux.
type router struct {
	mu       sync.RWMutex
	routes   []routeEntry
	patterns map[string]bool
	started  bool
}

func newRouter() *router {
	return &router{
		patterns: make(map[string]bool),
	}
}

func (r *router) handle(pattern string, handler http.Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return &RouteError{
			Pattern: pattern,
			Reason:  "cannot register route after server has started",
		}
	}

	trimmedPattern := strings.TrimSpace(pattern)
	if trimmedPattern == "" {
		return &RouteError{
			Pattern: pattern,
			Reason:  "pattern cannot be empty",
		}
	}

	if handler == nil {
		return &RouteError{
			Pattern: trimmedPattern,
			Reason:  "handler cannot be nil",
		}
	}

	if r.patterns[trimmedPattern] {
		return &RouteError{
			Pattern: trimmedPattern,
			Reason:  "route pattern already registered",
		}
	}

	// Validate pattern and conflict against a sandbox ServeMux
	if err := validateRoute(r.routes, trimmedPattern, handler); err != nil {
		return err
	}

	r.routes = append(r.routes, routeEntry{
		pattern: trimmedPattern,
		handler: handler,
	})
	r.patterns[trimmedPattern] = true
	return nil
}

func validateRoute(existing []routeEntry, newPattern string, newHandler http.Handler) (retErr error) {
	testMux := http.NewServeMux()

	for _, entry := range existing {
		testMux.Handle(entry.pattern, entry.handler)
	}

	defer func() {
		if rec := recover(); rec != nil {
			retErr = &RouteError{
				Pattern: newPattern,
				Reason:  fmt.Sprintf("pattern rejected by ServeMux: %v", rec),
			}
		}
	}()

	testMux.Handle(newPattern, newHandler)
	return nil
}

func (r *router) buildMux() *http.ServeMux {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.started = true
	mux := http.NewServeMux()
	for _, entry := range r.routes {
		mux.Handle(entry.pattern, entry.handler)
	}
	return mux
}

// HealthResponse represents the standard payload for the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// HealthHandler returns a standard HTTP handler that responds with {"status":"ok"}.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	}
}
