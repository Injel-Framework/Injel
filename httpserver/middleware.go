package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDContextKey contextKey = "injel_request_id"

// Middleware defines a standard HTTP middleware function.
type Middleware func(http.Handler) http.Handler

// RequestIDFromContext extracts the request ID from the given context, if present.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDContextKey).(string); ok {
		return id
	}
	return ""
}

var reqCounter uint64

// defaultIDGenerator generates a random 16-byte hex string.
func defaultIDGenerator() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp + atomic counter
		c := atomic.AddUint64(&reqCounter, 1)
		return fmt.Sprintf("%x-%x", time.Now().UnixNano(), c)
	}
	return hex.EncodeToString(b)
}

// RequestID returns a middleware that ensures every request has an X-Request-ID header.
// If an existing X-Request-ID is present in the request headers, it is preserved.
func RequestID(generator ...func() string) Middleware {
	gen := defaultIDGenerator
	if len(generator) > 0 && generator[0] != nil {
		gen = generator[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = gen()
			}

			w.Header().Set("X-Request-ID", reqID)
			ctx := context.WithValue(r.Context(), requestIDContextKey, reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseWriterTracker wraps http.ResponseWriter to capture status code and write status.
type responseWriterTracker struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriterTracker) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriterTracker) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Recovery returns a middleware that catches panics in downstream handlers,
// logs them if a logger is provided, and returns a 500 Internal Server Error response.
func Recovery(logger ...func(format string, args ...any)) Middleware {
	var logFn func(format string, args ...any)
	if len(logger) > 0 {
		logFn = logger[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracker := &responseWriterTracker{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				if rec := recover(); rec != nil {
					if logFn != nil {
						logFn("injel/httpserver: panic recovered: %v\nstack:\n%s", rec, debug.Stack())
					}

					if !tracker.written {
						tracker.Header().Set("Content-Type", "application/json; charset=utf-8")
						tracker.WriteHeader(http.StatusInternalServerError)
						_, _ = tracker.Write([]byte(`{"error":"internal server error"}`))
					}
				}
			}()

			next.ServeHTTP(tracker, r)
		})
	}
}

// Logging returns a middleware that logs incoming HTTP requests and response durations.
func Logging(logFn func(format string, args ...any)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			tracker := &responseWriterTracker{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(tracker, r)

			if logFn != nil {
				duration := time.Since(start)
				reqID := RequestIDFromContext(r.Context())
				idPart := ""
				if reqID != "" {
					idPart = fmt.Sprintf(" [req_id=%s]", reqID)
				}
				logFn("%s %s %d %s (%v)%s", r.Method, r.URL.Path, tracker.statusCode, http.StatusText(tracker.statusCode), duration, idPart)
			}
		})
	}
}
