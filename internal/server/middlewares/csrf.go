package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/codes"
)

// CSRFOriginCheck rejects non-safe methods whose Origin (or Referer) does not
// prefix-match the app's own origin — sufficient for a cookie-session app whose
// mutations all go over XHR, where browsers always send Origin.
func CSRFOriginCheck(allowedOrigin string) func(http.Handler) http.Handler {
	allowedOrigin = strings.TrimRight(allowedOrigin, "/")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			ctx, span := tracer.Start(r.Context(), "csrf.OriginCheck")
			defer span.End()

			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
			}

			if origin == "" || !strings.HasPrefix(origin, allowedOrigin) {
				span.SetStatus(codes.Error, "CSRF origin mismatch")
				slog.WarnContext(ctx, "CSRF check failed",
					"method", r.Method,
					"path", r.URL.Path,
					"origin", origin,
					"expected", allowedOrigin,
				)
				http.Error(w, "CSRF origin check failed", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
