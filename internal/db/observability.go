package db

import (
	"context"
	"strings"

	"github.com/effiware/goth-template/internal/version"
	"github.com/exaring/otelpgx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// poolTracer builds the otelpgx query tracer: spans named db.{sqlc query name},
// no SQL text or params recorded, no per-acquire spans (pool gauges cover waits).
func poolTracer() *otelpgx.Tracer {
	return otelpgx.NewTracer(
		otelpgx.WithTrimSQLInSpanName(), // required — the name func is only consulted when trimming is on
		otelpgx.WithSpanNameFunc(sqlcSpanName),
		otelpgx.WithDisableQuerySpanNamePrefix(),
		otelpgx.WithDisableSQLStatementInAttributes(),
		otelpgx.WithDisableAcquireTracer(),
	)
}

// sqlcSpanName maps sqlc's leading `-- name: X :type` comment to db.X;
// non-sqlc SQL (advisory locks, goose) falls back to db.{first word}.
func sqlcSpanName(stmt string) string {
	if rest, ok := strings.CutPrefix(stmt, "-- name: "); ok {
		if name, _, found := strings.Cut(rest, " "); found && name != "" {
			return "db." + name
		}
	}
	if fields := strings.Fields(stmt); len(fields) > 0 {
		return "db." + strings.ToLower(fields[0])
	}
	return "db.query"
}

// RegisterPoolMetrics exports pool health as observable instruments (semconv
// db.client.* names — otelpgx.RecordStats ships pgxpool.* names, skipped on
// purpose). No-op on a pool-less store.
func (s *Store) RegisterPoolMetrics() error {
	if s.pool == nil {
		return nil
	}
	meter := otel.Meter(version.ServiceName)

	usage, err := meter.Int64ObservableGauge("db.client.connections.usage",
		metric.WithDescription("Connections currently acquired"), metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	idle, err := meter.Int64ObservableGauge("db.client.connections.idle",
		metric.WithDescription("Connections currently idle"), metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	maxConns, err := meter.Int64ObservableGauge("db.client.connections.max",
		metric.WithDescription("Maximum pool size"), metric.WithUnit("{connection}"))
	if err != nil {
		return err
	}
	waitCount, err := meter.Int64ObservableCounter("db.client.connections.acquire_wait",
		metric.WithDescription("Acquires that waited for a free connection"), metric.WithUnit("{acquire}"))
	if err != nil {
		return err
	}
	waitTime, err := meter.Float64ObservableCounter("db.client.connections.acquire_wait.duration",
		metric.WithDescription("Cumulative time acquires spent waiting for a free connection"), metric.WithUnit("s"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		stat := s.pool.Stat()
		o.ObserveInt64(usage, int64(stat.AcquiredConns()))
		o.ObserveInt64(idle, int64(stat.IdleConns()))
		o.ObserveInt64(maxConns, int64(stat.MaxConns()))
		o.ObserveInt64(waitCount, stat.EmptyAcquireCount())
		o.ObserveFloat64(waitTime, stat.EmptyAcquireWaitTime().Seconds())
		return nil
	}, usage, idle, maxConns, waitCount, waitTime)
	return err
}
