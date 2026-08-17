package utils

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Chain composes wrappers outermost-first: Chain(w1, w2, w3)(base) ≡ w1(w2(w3(base))).
// Empty chain is a no-op.
func Chain(wrappers ...CircuitWrapper) CircuitWrapper {
	return func(next CircuitWithKey) CircuitWithKey {
		for i := len(wrappers) - 1; i >= 0; i-- {
			next = wrappers[i](next)
		}
		return next
	}
}

// RedisCacheForKeyTTL caches results under `{keyPrefix}:{key}` (sha256 of the key
// when hashKeys — required if the key can carry a secret). Redis errors degrade to
// a miss, so Redis is never a hard runtime dependency.
func RedisCacheForKeyTTL(
	rdb redis.Cmdable,
	keyPrefix string,
	ttl time.Duration,
	hashKeys bool,
) CircuitWrapper {
	return func(next CircuitWithKey) CircuitWithKey {
		return func(ctx context.Context, key string) ([]byte, error) {
			span := trace.SpanFromContext(ctx)

			cacheKey := buildRedisKey(keyPrefix, key, hashKeys)

			cached, err := rdb.Get(ctx, cacheKey).Bytes()
			switch {
			case err == nil:
				span.SetAttributes(attribute.String("cache.status", "hit"))
				return cached, nil
			case errors.Is(err, redis.Nil):
				span.SetAttributes(attribute.String("cache.status", "miss"))
			default:
				slog.WarnContext(ctx, "redis cache get failed, falling through to circuit",
					"key", cacheKey, "error", err)
				span.SetAttributes(attribute.String("cache.status", "error"))
			}

			result, circuitErr := next(ctx, key)
			if circuitErr != nil {
				return nil, circuitErr
			}

			if setErr := rdb.Set(ctx, cacheKey, result, ttl).Err(); setErr != nil {
				slog.WarnContext(ctx, "redis cache set failed", "key", cacheKey, "error", setErr)
			}

			return result, nil
		}
	}
}

func buildRedisKey(prefix, key string, hash bool) string {
	if hash {
		return prefix + ":" + hashKey(key) // hashKey defined in httputil.go
	}
	return prefix + ":" + key
}
