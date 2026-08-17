package hda

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/effiware/goth-template/internal/views/components"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ViewHandlerT func(w http.ResponseWriter, request *http.Request) error

// WithBrowserCache sets Cache-Control: private, max-age=N plus Vary on the
// cookie: a new session must miss, never replay another user's variant.
func WithBrowserCache(maxAge time.Duration, next ViewHandlerT) ViewHandlerT {
	directive := fmt.Sprintf("private, max-age=%d", int(maxAge.Seconds()))
	return func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", directive)
		w.Header().Add("Vary", "Cookie")
		return next(w, r)
	}
}

// isExpectedClientError: sentinels that express a deliberate refusal.
func isExpectedClientError(err error) bool {
	return errors.As(err, new(*UnauthorizedError)) ||
		errors.As(err, new(*ForbiddenError)) ||
		errors.As(err, new(*ConflictError)) ||
		errors.As(err, new(*NotFoundError))
}

// WithHTMLFallback turns a ViewHandlerT into an http.HandlerFunc, rendering an
// error fragment HTMX can swap. Status codes not in the layout's htmx
// responseHandling config are dropped by the browser, so keep the two in sync.
func WithHTMLFallback(viewHandler ViewHandlerT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := viewHandler(w, r)
		if err == nil {
			return
		}

		level := slog.LevelError
		span := trace.SpanFromContext(r.Context())
		if isExpectedClientError(err) {
			level = slog.LevelWarn
			// Deliberate refusal — an event keeps the span green.
			span.AddEvent("request refused", trace.WithAttributes(attribute.String("error.type", fmt.Sprintf("%T", err))))
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, "handler error")
		}
		slog.Log(r.Context(), level, "ViewHandler error", "error", err, "path", r.URL.Path)

		// Boosted navs swap hx-select="#main-content": a response missing that id
		// would select nothing and delete the target.
		boosted := r.Header.Get("HX-Boosted") != ""
		writeFragment := func(status int, message string) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			if boosted {
				fmt.Fprint(w, `<div id="main-content">`)
			}
			_ = components.ErrorPanel(message).Render(r.Context(), w)
			if boosted {
				fmt.Fprint(w, `</div>`)
			}
		}

		var ce *ConflictError
		switch {
		case errors.As(err, new(*UnauthorizedError)):
			w.Header().Set("HX-Redirect", "/")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		case errors.As(err, new(*ForbiddenError)):
			writeFragment(http.StatusForbidden, "You do not have access to this.")
		case errors.As(err, new(*NotFoundError)):
			writeFragment(http.StatusNotFound, "The requested resource was not found.")
		case errors.As(err, &ce):
			writeFragment(http.StatusConflict, ce.Message)
		default:
			writeFragment(http.StatusInternalServerError, "Something went wrong. Please try again.")
		}
	}
}
