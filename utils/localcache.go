package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
)

// Circuit is the keyless call shape the pipeline wraps.
type Circuit func(context.Context) ([]byte, error)

// CircuitWithKey is the keyed call shape the pipeline wraps.
type CircuitWithKey func(context.Context, string) ([]byte, error)

// CircuitWrapper adds one cross-cutting concern (cache, retry, …) to a circuit.
type CircuitWrapper func(CircuitWithKey) CircuitWithKey

// CacheFirstTTL caches a single result for ttl; errors do not extend the window.
func CacheFirstTTL(circuit Circuit, ttl time.Duration) Circuit {
	var expires time.Time
	var result []byte
	var err error
	var m sync.Mutex

	return func(ctx context.Context) ([]byte, error) {
		span := trace.SpanFromContext(ctx)

		m.Lock()
		defer m.Unlock()

		if time.Now().After(expires) {
			span.SetAttributes(attribute.String("cache.status", "miss"))
			result, err = circuit(ctx)
			if err == nil {
				expires = time.Now().Add(ttl)
			}
			return result, err
		}

		span.SetAttributes(attribute.String("cache.status", "hit"))
		return result, err
	}
}

type resultWithTTL struct {
	expires time.Time
	result  []byte
	err     error
}

// CacheFirstForKeyTTL caches in process memory for ttl, collapsing concurrent
// misses per key via singleflight. Closing done stops the eviction goroutine.
func CacheFirstForKeyTTL(done chan struct{}, ttl time.Duration, hashKeys bool) CircuitWrapper {
	results := map[string]*resultWithTTL{}
	cleanupTicker := time.NewTicker(2 * ttl)
	var m sync.RWMutex
	var sf singleflight.Group

	go func() {
		for {
			select {
			case <-done:
				cleanupTicker.Stop()
				return
			case <-cleanupTicker.C:
				now := time.Now()
				m.Lock()
				for key, res := range results {
					if now.After(res.expires) {
						delete(results, key)
					}
				}
				m.Unlock()
			}
		}
	}()

	return func(next CircuitWithKey) CircuitWithKey {
		return func(ctx context.Context, key string) ([]byte, error) {
			span := trace.SpanFromContext(ctx)
			_key := key
			if hashKeys {
				_key = hashKey(key)
			}

			m.RLock()
			cache := results[_key]
			m.RUnlock()

			if cache == nil || time.Now().After(cache.expires) {
				span.SetAttributes(attribute.String("cache.status", "miss"))

				v, err, _ := sf.Do(_key, func() (interface{}, error) {
					res, err := next(ctx, key)
					var expires time.Time
					if err == nil {
						expires = time.Now().Add(ttl)
					}
					m.Lock()
					results[_key] = &resultWithTTL{
						expires: expires,
						result:  res,
						err:     err,
					}
					m.Unlock()
					return res, err
				})
				if err != nil {
					return nil, err
				}
				return v.([]byte), nil
			}

			span.SetAttributes(attribute.String("cache.status", "hit"))
			return cache.result, cache.err
		}
	}
}

// hashKey reduces a key to a fixed-size SHA-256 hex digest.
func hashKey(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
