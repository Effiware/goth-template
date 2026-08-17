package services

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/effiware/goth-template/internal/db"
	"github.com/effiware/goth-template/internal/version"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Run-state status values persisted in tech_service_runs.last_status.
const (
	RunStatusOK    = "ok"
	RunStatusError = "error"
)

// DefaultRunTimeout is a wedge-breaker, not an SLA: it bounds one service run so a
// hung upstream can't hold the gate's session lock for the process's lifetime.
const DefaultRunTimeout = 30 * time.Minute

// ServiceRunStore is the slice of *db.Store that RecordRun needs.
type ServiceRunStore interface {
	UpsertServiceRun(ctx context.Context, arg db.UpsertServiceRunParams) error
}

// GatedRunStore adds RunIfDue's two primitives: the gate lock and the freshness check.
type GatedRunStore interface {
	ServiceRunStore
	IsServiceRunFresh(ctx context.Context, arg db.IsServiceRunFreshParams) (bool, error)
	TryWithSessionLock(ctx context.Context, key int64, fn func() error) (bool, error)
}

// serviceGateLockSalt keeps gate keys disjoint from the per-org advisory-lock
// keyspace (OrgTickLockKey, taskOrgLockKey) and goose's migration lock.
const serviceGateLockSalt = uint64(0x53c7_0000_0000_0002)

// serviceGateFreshnessFactor shrinks the window below the interval so an instance's
// own next tick never self-skips on timer jitter.
const serviceGateFreshnessFactor = 0.9

// serviceGateLockKey derives the advisory-lock key for one service's gate.
func serviceGateLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("svcgate:" + name))
	return int64(h.Sum64() ^ serviceGateLockSalt)
}

// wedgedGateFactor: a lock-busy skip whose run row is older than this many intervals
// means the holder is wedged. Without it, a wedged gate is invisible (peers skip at Debug).
const wedgedGateFactor = 2.0

// Lazily created: RunIfDue has no InitMetrics hook, and the otel global delegates
// instruments created before the MeterProvider is set.
var (
	gateMetricsOnce sync.Once
	gateSkipCounter metric.Int64Counter
)

func recordGateSkip(ctx context.Context, name, reason string) {
	gateMetricsOnce.Do(func() {
		var err error
		gateSkipCounter, err = otel.Meter(version.ServiceName).Int64Counter(
			"services.gate.skips",
			metric.WithDescription("Service runs skipped by the multi-instance gate, by reason"),
			metric.WithUnit("{skip}"),
		)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to create service gate skip counter", "error", err)
		}
	})
	if gateSkipCounter != nil {
		gateSkipCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("service", name),
			attribute.String("reason", reason),
		))
	}
}

// warnIfGateWedged is the one skip signal above Debug. Best-effort: errors are
// logged, never propagated.
func warnIfGateWedged(ctx context.Context, store GatedRunStore, name string, interval time.Duration) {
	if interval <= 0 {
		return
	}
	fresh, err := store.IsServiceRunFresh(ctx, db.IsServiceRunFreshParams{
		ServiceName:      name,
		FreshnessSeconds: interval.Seconds() * wedgedGateFactor,
	})
	if err != nil {
		slog.DebugContext(ctx, "Service gate wedge check failed", "service", name, "error", err)
		return
	}
	if !fresh {
		slog.WarnContext(ctx, "Service gate lock held with no run recorded within twice the interval — possible wedged run",
			"service", name, "interval", interval)
	}
}

// RunIfDue is the multi-instance gate around RecordRun: svc runs only when no peer
// is mid-run (session advisory lock on name) AND none started within the freshness
// window (~90% of interval, read while holding the lock). Both halves are
// load-bearing — the lock alone misses staggered tickers, the freshness check alone
// is a TOCTOU race. interval <= 0 keeps the lock but skips freshness ("run now,
// never two at once" — the manual /admin trigger). A skip writes nothing to
// tech_service_runs. Reports whether svc ran.
func RunIfDue(ctx context.Context, store GatedRunStore, name string, svc Servicer, interval time.Duration) (bool, error) {
	ran := false
	acquired, err := store.TryWithSessionLock(ctx, serviceGateLockKey(name), func() error {
		if interval > 0 {
			fresh, err := store.IsServiceRunFresh(ctx, db.IsServiceRunFreshParams{
				ServiceName:      name,
				FreshnessSeconds: interval.Seconds() * serviceGateFreshnessFactor,
			})
			if err != nil {
				return err
			}
			if fresh {
				recordGateSkip(ctx, name, "fresh")
				slog.DebugContext(ctx, "Service run skipped: ran within the freshness window", "service", name, "interval", interval)
				return nil
			}
		}
		ran = true
		return RecordRun(ctx, store, name, svc)
	})
	if !acquired && err == nil {
		recordGateSkip(ctx, name, "lock_busy")
		slog.DebugContext(ctx, "Service run skipped: another instance holds the gate lock", "service", name)
		warnIfGateWedged(ctx, store, name, interval)
	}
	return ran, err
}

// RunDetailer lets a Servicer contribute per-run counts into the run-state details
// JSONB; non-implementers record an empty object.
type RunDetailer interface {
	LastRunDetails() map[string]int64
}

// RecordRun runs svc.Sync and persists the outcome into tech_service_runs, returning
// Sync's error unchanged. Single write path for both the periodic runner and the
// manual /admin trigger; a persist failure is logged, never masking the Sync result.
func RecordRun(ctx context.Context, store ServiceRunStore, name string, svc Servicer) error {
	start := time.Now()
	syncErr := svc.Sync(ctx)
	finished := time.Now()

	status := RunStatusOK
	var lastErr pgtype.Text
	if syncErr != nil {
		status = RunStatusError
		lastErr = pgtype.Text{String: syncErr.Error(), Valid: true}
	}

	details := []byte("{}")
	if d, ok := svc.(RunDetailer); ok {
		if b, err := json.Marshal(d.LastRunDetails()); err == nil {
			details = b
		}
	}

	if err := store.UpsertServiceRun(ctx, db.UpsertServiceRunParams{
		ServiceName:    name,
		LastStartedAt:  start,
		LastFinishedAt: pgtype.Timestamptz{Time: finished, Valid: true},
		LastStatus:     status,
		LastError:      lastErr,
		DurationMs:     pgtype.Int8{Int64: finished.Sub(start).Milliseconds(), Valid: true},
		Details:        details,
	}); err != nil {
		slog.ErrorContext(ctx, "RecordRun: failed to persist run-state", "service", name, "error", err)
	}

	return syncErr
}
