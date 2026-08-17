package utils

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RetryIf retries with full-jitter exponential backoff only while predicate(err)
// holds — use it to skip 4xx, which never self-heal. The first call is immediate,
// so total calls are at most maxRetries+1; a cancelled context returns ctx.Err().
func RetryIf(predicate func(error) bool, maxRetries int, baseDelay, maxDelay time.Duration) CircuitWrapper {
	return func(next CircuitWithKey) CircuitWithKey {
		return func(ctx context.Context, key string) ([]byte, error) {
			span := trace.SpanFromContext(ctx)

			for attempt := 0; ; attempt++ {
				result, err := next(ctx, key)
				if err == nil {
					if attempt > 0 {
						span.SetAttributes(attribute.Int("retry.attempts", attempt))
					}
					return result, nil
				}

				if attempt >= maxRetries || !predicate(err) {
					return nil, err
				}

				sleep := jitteredBackoff(attempt, baseDelay, maxDelay)

				slog.WarnContext(ctx, "circuit call failed, retrying",
					"attempt", attempt+1,
					"maxRetries", maxRetries,
					"sleep", sleep,
					"error", err,
				)
				span.AddEvent("retry", trace.WithAttributes(
					attribute.Int("retry.attempt", attempt+1),
					attribute.String("retry.sleep", sleep.String()),
				))

				select {
				case <-time.After(sleep):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
	}
}

// Retry is RetryIf over every error. Full jitter (random within the backoff, not
// the backoff itself) keeps multiple instances from recovering in lockstep.
func Retry(maxRetries int, baseDelay, maxDelay time.Duration) CircuitWrapper {
	return RetryIf(func(error) bool { return true }, maxRetries, baseDelay, maxDelay)
}

// jitteredBackoff returns a random sleep in [0, min(maxDelay, baseDelay*2^attempt)];
// 0 when either delay is 0, which is how callers ask for an immediate retry.
func jitteredBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if maxDelay == 0 || baseDelay == 0 {
		return 0
	}
	shift := attempt
	if shift > 62 {
		shift = 62
	}
	backoff := baseDelay << shift
	if backoff <= 0 || backoff > maxDelay { // <= 0 catches int64 overflow
		backoff = maxDelay
	}
	return time.Duration(rand.Int64N(int64(backoff)))
}
