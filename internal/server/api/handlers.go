package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/effiware/goth-template/internal/clicks"
	_ "github.com/effiware/goth-template/internal/docs"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// EndpointHandlerT returns (status, payload, err); a non-nil err is a bodiless 500.
type EndpointHandlerT func(w http.ResponseWriter, request *http.Request) (int, any, error)

// markSpanError flags the request span — the wrapper otherwise swallows failures into a 500.
func markSpanError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, "handler error")
}

func JsonHandler(endpointHandler EndpointHandlerT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, payload, err := endpointHandler(w, r)
		if err != nil {
			slog.ErrorContext(r.Context(), "EndpointHandler", "error", err)
			markSpanError(r.Context(), err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// 204 and 304 must not carry a body (RFC 9110).
		if payload == nil || code == http.StatusNoContent || code == http.StatusNotModified {
			w.WriteHeader(code)
			return
		}

		body, err := json.Marshal(payload)
		if err != nil {
			slog.ErrorContext(r.Context(), "When marshaling JSON", "error", err)
			markSpanError(r.Context(), err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(body)
	}
}

// Clicks is the JSON shape of the demo counter.
type Clicks struct {
	Count int64 `json:"count" example:"17"`
}

// GetClicks returns the click total.
//
//	@Summary		Get the number of clicks
//	@Description	Total number of clicks recorded in the system
//	@Tags			clicks
//	@Produce		json
//	@Success		200	{object}	Clicks
//	@Router			/clicks [get]
func GetClicks(counter *clicks.Counter) EndpointHandlerT {
	return func(_ http.ResponseWriter, r *http.Request) (int, any, error) {
		count, err := counter.Count(r.Context())
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, Clicks{Count: count}, nil
	}
}

// IncrementClicks bumps the click total.
//
//	@Summary		Increment the number of clicks
//	@Description	Increment the click total by one and return the new value
//	@Tags			clicks
//	@Produce		json
//	@Success		200	{object}	Clicks
//	@Router			/clicks/increment [post]
func IncrementClicks(counter *clicks.Counter) EndpointHandlerT {
	return func(_ http.ResponseWriter, r *http.Request) (int, any, error) {
		count, err := counter.Increment(r.Context())
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, Clicks{Count: count}, nil
	}
}
