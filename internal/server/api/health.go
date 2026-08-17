package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/effiware/goth-template/internal/db"
	"github.com/redis/go-redis/v9"
)

// readyzTimeout keeps a hung dependency from hanging the probe.
const readyzTimeout = 2 * time.Second

// One server per process, so package-level.
var draining atomic.Bool //nolint:gochecknoglobals

// BeginDraining flips /readyz to 503; call it on SIGTERM and wait out
// server.drain_grace_sec before Shutdown.
func BeginDraining() { draining.Store(true) }

// IsProbePath keeps probes out of traces — they'd otherwise dominate span volume.
func IsProbePath(r *http.Request) bool {
	return r.URL.Path == "/readyz" || r.URL.Path == "/ping"
}

// Readyz reports whether this instance should receive traffic. Liveness stays on
// chi's /ping: a pod that can't reach Postgres wants pulling from the LB, not
// restarting.
//
//	@Summary		Readiness probe
//	@Description	200 when Postgres and Redis are reachable and the instance is not draining; 503 otherwise.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string	"ready"
//	@Failure		503	{object}	map[string]string	"draining or a dependency is unreachable"
//	@Router			/readyz [get]
func Readyz(store *db.Store, rdb redis.Cmdable) EndpointHandlerT {
	return func(_ http.ResponseWriter, r *http.Request) (int, any, error) {
		if draining.Load() {
			return http.StatusServiceUnavailable, map[string]string{"status": "draining"}, nil
		}

		ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
		defer cancel()

		// Unready is a 503 body, never a returned error — JsonHandler maps errors to a
		// bodiless 500, which reads as a bug rather than a probe result.
		if err := store.Ping(ctx); err != nil {
			slog.WarnContext(ctx, "readyz: database unreachable", "error", err)
			return http.StatusServiceUnavailable, map[string]string{"status": "unready", "check": "database"}, nil
		}
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				slog.WarnContext(ctx, "readyz: redis unreachable", "error", err)
				return http.StatusServiceUnavailable, map[string]string{"status": "unready", "check": "redis"}, nil
			}
		}

		return http.StatusOK, map[string]string{"status": "ready"}, nil
	}
}
