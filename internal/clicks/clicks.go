// Package clicks is the demo domain layer: Postgres holds the state, Redis
// fronts the reads. Delete it (and its table, queries, routes and views) when
// forking — it exists to show the wiring, not because a counter needs a package.
package clicks

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/effiware/goth-template/internal/db"
	"github.com/effiware/goth-template/utils"
	"github.com/redis/go-redis/v9"
)

// CounterName keys the single row in the clicks table.
const CounterName = "global"

// cachePrefix + CounterName is the Redis key; Increment deletes it.
const cachePrefix = "clicks"

// cacheTTL is a backstop only — a write invalidates the key immediately, so the
// TTL just bounds staleness if a peer's DEL is lost.
const cacheTTL = 5 * time.Minute

// Counter reads through Redis and writes through Postgres. rdb may be nil: the
// cache wrapper is then skipped and every read hits the DB.
type Counter struct {
	store *db.Store
	rdb   redis.Cmdable
	read  utils.CircuitWithKey
}

func NewCounter(store *db.Store, rdb redis.Cmdable) *Counter {
	c := &Counter{store: store, rdb: rdb}

	base := utils.CircuitWithKey(func(ctx context.Context, name string) ([]byte, error) {
		count, err := c.store.GetClickCount(ctx, name)
		if err != nil {
			return nil, err
		}
		return []byte(strconv.FormatInt(count, 10)), nil
	})

	c.read = base
	if rdb != nil {
		c.read = utils.RedisCacheForKeyTTL(rdb, cachePrefix, cacheTTL, false)(base)
	}
	return c
}

// Count returns the current total, served from Redis when warm.
func (c *Counter) Count(ctx context.Context) (int64, error) {
	raw, err := c.read(ctx, CounterName)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(raw), 10, 64)
}

// Increment bumps the counter and drops the cached value. The DB write already
// committed, so a failed DEL is only staleness (bounded by cacheTTL) — logged, not returned.
func (c *Counter) Increment(ctx context.Context) (int64, error) {
	count, err := c.store.IncrementClickCount(ctx, CounterName)
	if err != nil {
		return 0, err
	}
	if c.rdb != nil {
		if err := c.rdb.Del(ctx, cachePrefix+":"+CounterName).Err(); err != nil {
			slog.WarnContext(ctx, "clicks: cache invalidation failed", "error", err)
		}
	}
	return count, nil
}
