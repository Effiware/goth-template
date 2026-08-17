package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// BeforeCallHook runs before each attempt (e.g. to refresh a bearer token).
type BeforeCallHook func(ctx context.Context, req *http.Request) error

// DeepCopyRequest copies a request including its body, restoring the original's
// body so the caller can still read it.
func DeepCopyRequest(ctx context.Context, reqInit *http.Request) (*http.Request, error) {
	if reqInit.Body == nil {
		return reqInit.Clone(ctx), nil
	}

	body, err := io.ReadAll(reqInit.Body)
	if err != nil {
		return nil, fmt.Errorf("error while copying request body: %w", err)
	}

	reqClone := reqInit.Clone(ctx)
	reqInit.Body = io.NopCloser(bytes.NewReader(body))
	reqClone.Body = io.NopCloser(bytes.NewReader(body))

	return reqClone, nil
}

// SendRetryableRequest retries on retriableStatuses up to maxRetryTimes; req stays
// reusable. Legacy helper — prefer the NewHTTPCircuit pipeline for new code.
func SendRetryableRequest(
	ctx context.Context,
	req *http.Request,
	retriableStatuses []int,
	maxRetryTimes int,
	hook BeforeCallHook,
	client *http.Client,
) ([]byte, error) {
	span := trace.SpanFromContext(ctx)

	if len(retriableStatuses) == 0 {
		return nil, fmt.Errorf("you must provide at least one retriable status code")
	}
	if slices.Contains(retriableStatuses, http.StatusOK) {
		return nil, fmt.Errorf("response status 200 cannot be used to trigger the retry")
	}

	var resBody []byte
	var retryNum = 0
	var statusCode = retriableStatuses[0]
	if client == nil {
		client = http.DefaultClient
	}
	for next := true; next; next = retryNum <= maxRetryTimes && slices.Contains(retriableStatuses, statusCode) {
		if retryNum > 0 {
			span.AddEvent("http.retry", trace.WithAttributes(
				attribute.Int("retry.attempt", retryNum),
				attribute.Int("http.response.status_code", statusCode),
			))
		}

		reqCopy, err := DeepCopyRequest(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to copy request")
			return nil, err
		}

		if hook != nil {
			if err := hook(ctx, reqCopy); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "hook failed")
				return nil, err
			}
		}

		resp, err := client.Do(reqCopy)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "request failed")
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		statusCode = resp.StatusCode
		resBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read response")
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if slices.Contains([]int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent}, statusCode) {
			span.SetAttributes(attribute.Int("http.retry_count", retryNum))
			return resBody, nil
		}

		retryNum++
	}

	err := fmt.Errorf("request failed after %d attempts with status %d: %s", retryNum, statusCode, string(resBody))
	span.RecordError(err)
	span.SetStatus(codes.Error, "max retries exceeded")
	span.SetAttributes(attribute.Int("http.retry_count", retryNum))
	return nil, err
}
