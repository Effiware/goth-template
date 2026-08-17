package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPError means a response arrived, carrying status + body for wrappers to
// inspect. A plain error means no response at all (network, DNS, timeout).
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	const maxBodyLen = 200
	body := e.Body
	if len(body) > maxBodyLen {
		body = body[:maxBodyLen]
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, body)
}

// NewHTTPCircuit runs makeRequest (called per attempt, key interpreted by the
// caller) and always returns *HTTPError — even for 2xx. Deciding which codes are
// success belongs to HTTPSuccess, which every pipeline must therefore include.
// A nil client falls back to http.DefaultClient.
func NewHTTPCircuit(
	client *http.Client,
	makeRequest func(context.Context, string) (*http.Request, error),
) CircuitWithKey {
	if client == nil {
		client = http.DefaultClient
	}
	return func(ctx context.Context, key string) ([]byte, error) {
		span := trace.SpanFromContext(ctx)

		req, err := makeRequest(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to build request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: body}
	}
}

// HTTPSuccess promotes responses whose status matches successPattern — a regex over
// the 3-digit code, e.g. "2.." or "2..|304" — to success, letting the rest propagate
// as *HTTPError for Retry to inspect. Compiled at construction: invalid patterns panic.
func HTTPSuccess(successPattern string) CircuitWrapper {
	re := regexp.MustCompile(successPattern)
	return func(next CircuitWithKey) CircuitWithKey {
		return func(ctx context.Context, key string) ([]byte, error) {
			result, err := next(ctx, key)
			if err == nil {
				return result, nil
			}

			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				return nil, err
			}

			if re.MatchString(strconv.Itoa(httpErr.StatusCode)) {
				return httpErr.Body, nil
			}

			return nil, httpErr
		}
	}
}
