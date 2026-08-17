package services

import (
	"context"
	"log/slog"
	"time"
)

// PeriodicRunner calls a Servicer's Sync on a fixed interval; it owns the ticker
// goroutine so services stay scheduling-agnostic.
type PeriodicRunner struct {
	name     string
	svc      Servicer
	store    GatedRunStore
	interval time.Duration
	gated    bool
	done     chan struct{}
}

// NewPeriodicRunner returns a gated runner — each tick goes through RunIfDue, so the
// interval is honored fleet-wide, not per instance. The default for new services.
func NewPeriodicRunner(name string, svc Servicer, store GatedRunStore, interval time.Duration) *PeriodicRunner {
	return &PeriodicRunner{
		name:     name,
		svc:      svc,
		store:    store,
		interval: interval,
		gated:    true,
		done:     make(chan struct{}),
	}
}

// NewUngatedPeriodicRunner ticks on every instance. Only for services whose work
// self-partitions (e.g. FOR UPDATE SKIP LOCKED claims) — gating those would
// serialize useful parallelism.
func NewUngatedPeriodicRunner(name string, svc Servicer, store GatedRunStore, interval time.Duration) *PeriodicRunner {
	r := NewPeriodicRunner(name, svc, store, interval)
	r.gated = false
	return r
}

// Start begins the sync loop in a background goroutine. It does NOT run an initial
// sync — call svc.Sync explicitly first if you need one.
func (r *PeriodicRunner) Start() {
	ticker := time.NewTicker(r.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-r.done:
				return
			case <-ticker.C:
				slog.Debug("Periodic sync triggered", "service", r.name)
				runBounded(r.name, r.svc, r.store, r.interval, r.gated)
			}
		}
	}()
}

func (r *PeriodicRunner) ShutDown() {
	close(r.done)
}

// runBounded executes one pass under DefaultRunTimeout; shared with ScheduledRunner.
// The gate unlocks under WithoutCancel even when the timeout fires mid-run — keep
// Sync safe to interrupt.
func runBounded(name string, svc Servicer, store GatedRunStore, interval time.Duration, gated bool) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRunTimeout)
	defer cancel()

	var err error
	if gated {
		_, err = RunIfDue(ctx, store, name, svc, interval)
	} else {
		err = RecordRun(ctx, store, name, svc)
	}
	if err != nil {
		slog.ErrorContext(ctx, "Service sync failed", "service", name, "error", err)
	}
}
