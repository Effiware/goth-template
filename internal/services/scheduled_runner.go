package services

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/adhocore/gronx"

	"github.com/effiware/goth-template/internal/config"
)

// ScheduleTZ anchors schedules whatever the host's zone; the canonical value
// lives in config so Validate's gap walk agrees with the runner.
const ScheduleTZ = config.ScheduleTZ

// maxScheduleLookback bounds the backward-gap search; a spec sparser than this is unusable.
const maxScheduleLookback = 400 * 24 * time.Hour

// ScheduledRunner is the wall-clock sibling of PeriodicRunner — same RunIfDue gate,
// fired by a cron spec. For work belonging at a quiet hour rather than on a cadence.
type ScheduledRunner struct {
	name  string
	svc   Servicer
	store GatedRunStore
	spec  string
	loc   *time.Location
	done  chan struct{}
}

func NewScheduledRunner(name string, svc Servicer, store GatedRunStore, spec string) (*ScheduledRunner, error) {
	if !gronx.IsValid(spec) {
		return nil, fmt.Errorf("invalid cron spec %q for service %s", spec, name)
	}
	loc, err := time.LoadLocation(ScheduleTZ)
	if err != nil {
		return nil, fmt.Errorf("loading schedule timezone %s: %w", ScheduleTZ, err)
	}
	return &ScheduledRunner{name: name, svc: svc, store: store, spec: spec, loc: loc, done: make(chan struct{})}, nil
}

// Start arms the schedule. Every instance fires at the same instant; RunIfDue picks
// one winner, so no cross-instance coordination is needed.
func (r *ScheduledRunner) Start() {
	go func() {
		for {
			next, gap, err := r.nextFire(time.Now().In(r.loc))
			if err != nil {
				slog.Error("Scheduled sync stopped: cannot compute next fire", "service", r.name, "spec", r.spec, "error", err)
				return
			}
			slog.Debug("Scheduled sync armed", "service", r.name, "next", next)

			timer := time.NewTimer(time.Until(next))
			select {
			case <-r.done:
				timer.Stop()
				return
			case <-timer.C:
				runBounded(r.name, r.svc, r.store, gap, true)
			}
		}
	}()
}

func (r *ScheduledRunner) ShutDown() {
	close(r.done)
}

// BootGate returns the schedule gap spanning now. As the boot catch-up's RunIfDue
// interval it fires the pass only when downtime swallowed a scheduled run.
func (r *ScheduledRunner) BootGate() (time.Duration, error) {
	_, gap, err := r.nextFire(time.Now().In(r.loc))
	return gap, err
}

// nextFire returns the next fire and the gap back to the tick before it — RunIfDue's
// freshness window is 0.9 × that gap, which can never reach the previous fire, so no
// spec (uniform or irregular) can self-skip. Recomputed each pass so DST follows.
func (r *ScheduledRunner) nextFire(from time.Time) (time.Time, time.Duration, error) {
	next, err := gronx.NextTickAfter(r.spec, from, false)
	if err != nil {
		return time.Time{}, 0, err
	}
	prev, err := r.prevTickBefore(next)
	if err != nil {
		return time.Time{}, 0, err
	}
	return next, next.Sub(prev), nil
}

// prevTickBefore finds the last tick strictly before t by walking NextTickAfter forward
// from a growing lookback — gronx.PrevTickBefore is not DST-safe (returns t itself, or
// errors, when the walk crosses the fall-back transition).
func (r *ScheduledRunner) prevTickBefore(t time.Time) (time.Time, error) {
	after, err := gronx.NextTickAfter(r.spec, t, false)
	if err != nil {
		return time.Time{}, err
	}
	for lookback := 2 * after.Sub(t); ; lookback *= 4 {
		lookback = min(lookback, maxScheduleLookback)
		cand, err := gronx.NextTickAfter(r.spec, t.Add(-lookback), false)
		if err != nil {
			return time.Time{}, err
		}
		for cand.Before(t) {
			n, err := gronx.NextTickAfter(r.spec, cand, false)
			if err != nil {
				return time.Time{}, err
			}
			if !n.After(cand) { // paranoia: a stuck tick must not loop forever
				break
			}
			if !n.Before(t) {
				return cand, nil
			}
			cand = n
		}
		if lookback == maxScheduleLookback {
			return time.Time{}, fmt.Errorf("no tick of %q within %v before %v", r.spec, maxScheduleLookback, t)
		}
	}
}
