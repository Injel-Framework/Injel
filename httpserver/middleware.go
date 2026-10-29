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

// maxRequestIDLength defines the maximum allowable length for an incoming X-Request-ID header.
const maxRequestIDLength = 128

// isValidRequestID checks whether an incoming request ID is within length limits
// and contains only safe ASCII characters (alphanumeric, hyphen, underscore, period, colon).
func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == ':' {
			continue
		}
		return false
	}
	return true
}

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
// If a valid X-Request-ID is present in the request headers (non-empty, safe characters, <= 128 bytes),
// it is preserved. Otherwise, a new request ID is generated.
func RequestID(generator ...func() string) Middleware {
	gen := defaultIDGenerator
	if len(generator) > 0 && generator[0] != nil {
		gen = generator[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if !isValidRequestID(reqID) {
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

func wrapResponseWriter(w http.ResponseWriter) *responseWriterTracker {
	if tracker, ok := w.(*responseWriterTracker); ok {
		return tracker
	}
	return &responseWriterTracker{ResponseWriter: w, statusCode: http.StatusOK}
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

func (rw *responseWriterTracker) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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
			tracker := wrapResponseWriter(w)

			defer func() {
				if rec := recover(); rec != nil {
					if logFn != nil {
						reqID := RequestIDFromContext(r.Context())
						idPart := ""
						if reqID != "" {
							idPart = fmt.Sprintf(" [req_id=%s]", reqID)
						}
						logFn("injel/httpserver: panic recovered%s: %v\nstack:\n%s", idPart, rec, debug.Stack())
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
			tracker := wrapResponseWriter(w)

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
