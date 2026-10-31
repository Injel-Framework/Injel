package database

import "fmt"

// ConfigError is returned when a database Config field fails validation.
type ConfigError struct {
	// Field is the name of the configuration field that failed validation.
	Field string
	// Message describes the violation. It never contains DSN or credentials.
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("database: invalid config field %q: %s", e.Field, e.Message)
}

// OpenError wraps the error returned by sql.Open.
type OpenError struct {
	// Driver is the DriverName that was used.
	Driver string
	// Err is the underlying error from sql.Open.
	Err error
}

func (e *OpenError) Error() string {
	return fmt.Sprintf("database: failed to open driver %q: %v", e.Driver, e.Err)
}

// Unwrap enables errors.Is and errors.As on the underlying error.
func (e *OpenError) Unwrap() error { return e.Err }

// PingError wraps the error returned by db.PingContext.
// It deliberately omits DSN to avoid leaking credentials.
type PingError struct {
	// Err is the underlying ping error.
	Err error
}

func (e *PingError) Error() string {
	return fmt.Sprintf("database: ping failed: %v", e.Err)
}

// Unwrap enables errors.Is and errors.As on the underlying error.
func (e *PingError) Unwrap() error { return e.Err }

// CloseError wraps the error returned by db.Close.
type CloseError struct {
	// Err is the underlying close error.
	Err error
}

func (e *CloseError) Error() string {
	return fmt.Sprintf("database: close failed: %v", e.Err)
}

// Unwrap enables errors.Is and errors.As on the underlying error.
func (e *CloseError) Unwrap() error { return e.Err }
