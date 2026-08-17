package middlewares

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Recoverer logs handler panics as single-line JSON (stack as a field) and marks the
// request span as errored. Mount immediately after otelchi — only there does the ctx
// carry the span for trace_id correlation.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec) // net/http contract: let the server abort the connection
				}
				ctx := r.Context()
				slog.ErrorContext(ctx, "panic recovered",
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
					"method", r.Method, "path", r.URL.Path)
				span := trace.SpanFromContext(ctx)
				span.RecordError(fmt.Errorf("panic: %v", rec))
				span.SetStatus(codes.Error, "panic")
				if ww.Status() == 0 { // headers not sent yet
					ww.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

// RequestLogger emits one Debug line per request. Mount inside otelchi so lines carry trace_id.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.DebugContext(r.Context(), "request",
			"method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"bytes", ww.BytesWritten(), "duration_ms", time.Since(start).Milliseconds())
	})
}
