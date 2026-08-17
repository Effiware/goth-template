package middlewares

import (
	"net/http"
	"time"

	"github.com/effiware/goth-template/internal/version"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RequestMetrics records http.server.request.duration{method, route, status} — the
// RED signal per endpoint. Mount inside otelchi (exemplars need the span in ctx) and
// outside Recoverer (a recovered panic must count as its 500). skip filters probes.
func RequestMetrics(skip func(*http.Request) bool) func(http.Handler) http.Handler {
	hist, err := otel.Meter(version.ServiceName).Float64Histogram("http.server.request.duration",
		metric.WithDescription("HTTP server request duration"), metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10))
	if err != nil {
		otel.Handle(err)
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip(r) {
				next.ServeHTTP(w, r)
				return
			}
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 { // nothing written — net/http sends 200
				status = http.StatusOK
			}
			// RoutePattern is read post-serve (chi fills it during routing); empty on 404s.
			hist.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(
				attribute.String("http.request.method", requestMethod(r.Method)),
				attribute.String("http.route", chi.RouteContext(r.Context()).RoutePattern()),
				attribute.Int("http.response.status_code", status)))
		})
	}
}

// requestMethod folds nonstandard methods to _OTHER (semconv; caps label cardinality).
func requestMethod(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return m
	}
	return "_OTHER"
}
