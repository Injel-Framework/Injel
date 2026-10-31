package httpserver

import (
	"fmt"
)

// RouteError is returned when a route pattern or registration is invalid.
type RouteError struct {
	Pattern string
	Reason  string
}

func (e *RouteError) Error() string {
	if e.Pattern == "" {
		return fmt.Sprintf("injel/httpserver: route error: %s", e.Reason)
	}
	return fmt.Sprintf("injel/httpserver: invalid route %q: %s", e.Pattern, e.Reason)
}

// ServerError represents an operational error in the HTTP server lifecycle.
type ServerError struct {
	Action string
	Addr   string
	Err    error
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("injel/httpserver: %s on %q failed: %v", e.Action, e.Addr, e.Err)
}

func (e *ServerError) Unwrap() error {
	return e.Err
}
