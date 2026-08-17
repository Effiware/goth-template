# utils/circuit — Composable resilience primitives

This package implements a lightweight, composable resilience pattern for Go based on
function wrapping. The core idea: any fallible operation can be expressed as a
`CircuitWithKey`, and cross-cutting concerns (caching, retry, HTTP success detection)
are applied as `CircuitWrapper` functions that chain together without coupling.

## Overview

```go
type Circuit        func(context.Context) ([]byte, error)
type CircuitWithKey func(context.Context, string) ([]byte, error)
type CircuitWrapper func(CircuitWithKey) CircuitWithKey
```

Wrappers compose via `Chain`, outermost-first:

```go
pipeline := utils.Chain(
    utils.RedisCacheForKeyTTL(rdb, "kc:orgs", 5*time.Minute, false),
    utils.RetryIf(isRetryable, 3, 500*time.Millisecond, 10*time.Second),
    utils.HTTPSuccess("2.."),
)
circuit := pipeline(utils.NewHTTPCircuit(client, makeRequest))
```

## Available primitives

| Name | File | Description |
|---|---|---|
| `Chain` | `rediscache.go` | Composes multiple `CircuitWrapper`s outermost-first |
| `RedisCacheForKeyTTL` | `rediscache.go` | Redis-backed TTL cache, shared across all instances |
| `CacheFirstForKeyTTL` | `localcache.go` | In-process TTL cache, single-instance only |
| `Retry` | `retry.go` | Retries on any error with exponential backoff + full jitter |
| `RetryIf(predicate)` | `retry.go` | Retries only when predicate returns true |
| `NewHTTPCircuit` | `httpcircuit.go` | Base circuit factory: executes an HTTP request, always returns `*HTTPError` |
| `HTTPSuccess(pattern)` | `httpcircuit.go` | Promotes matching HTTP status codes to success; rest propagate as `*HTTPError` |

---

## Candidate for extraction: `effiware/circuit`

The pattern is generic enough to live as a standalone open-source library.
The following work should happen before that extraction.

### 1. Generics — highest priority

The current `[]byte` return type is a specialisation chosen for Redis storage and HTTP
response bodies. A general-purpose library should use Go generics so that typed responses
work without marshalling overhead:

```go
// proposed generic core
type CircuitWithKey[K, V any]  func(context.Context, K) (V, error)
type CircuitWrapper[K, V any]  func(CircuitWithKey[K, V]) CircuitWithKey[K, V]
```

Under this design the current `[]byte` specialisation becomes one concrete instance,
and `Retry`, `RetryIf`, `Chain` work equally well over typed DB results, gRPC responses,
or any other fallible operation.

The HTTP-specific pieces (`NewHTTPCircuit`, `HTTPSuccess`, `HTTPError`) would move to a
`httpcircuit` sub-package as a `[string, []byte]` specialisation, keeping the core
dependency-free.

### 2. Production proof

The API was designed and first used in `auditee-app-v1`, and ships here as the shared
baseline. Before publishing:

- [ ] Remove `SendRetryableRequest` / `DeepCopyRequest` from `httputil.go` and confirm
      all call sites are covered by the new pattern
- [ ] Run the Keycloak 401-refresh retry path under real traffic and verify the
      token-refresh behaviour is preserved
- [ ] Exercise `RetryIf` and `HTTPSuccess` against a staging environment to catch any
      edge cases not covered by unit tests
- [ ] Add unit tests for `RedisCacheForKeyTTL`, `Retry`, `RetryIf`, `HTTPSuccess`, and
      `jitteredBackoff` (currently no test coverage)

### 3. Naming

`http-circuit` undersells the library if the core is made generic.
Suggested structure:

```
github.com/effiware/circuit          # core: CircuitWithKey, CircuitWrapper, Chain, Retry, RetryIf
github.com/effiware/circuit/redis    # RedisCacheForKeyTTL (depends on go-redis)
github.com/effiware/circuit/http     # NewHTTPCircuit, HTTPSuccess, HTTPError (depends on net/http)
```

This keeps the core import free of Redis and HTTP dependencies, making it usable in
contexts where only retry or in-process caching is needed.

### 4. API stability pass

Before tagging v1.0, review:

- `CacheFirstForKeyTTL` — the `done chan struct{}` lifetime management is awkward;
  consider accepting a `context.Context` instead, which is more idiomatic for Go
  background goroutine lifetimes
- `NewHTTPCircuit` — currently has no support for a before-call hook (e.g. injecting
  auth headers); the `makeRequest` factory covers this use case but it is worth
  documenting the pattern explicitly
- `jitteredBackoff` — `maxDelay = 0` means "immediate retry"; make this a named option
  or a separate `RetryImmediate` constructor to avoid a surprising zero-value behaviour

### 5. Prior art to document

Acknowledge the libraries that occupy adjacent space so users can make an informed choice:

| Library | Relationship |
|---|---|
| [go-kit/kit](https://github.com/go-kit/kit) | Same `Middleware = func(Endpoint) Endpoint` pattern but coupled to `interface{}` request/response and the wider go-kit ecosystem |
| [failsafe-go](https://github.com/failsafe-go/failsafe-go) | Feature-complete resilience library (circuit breaker, hedge, timeout); more opinionated API |
| [cenkalti/backoff](https://github.com/cenkalti/backoff) | Backoff/retry only; widely used; no chaining model |
