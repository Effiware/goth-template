package utils

import (
	"context"
	"sync"
	"time"

	"github.com/effiware/goth-template/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer(version.ServiceName) //nolint:gochecknoglobals

type Executor func(context.Context)

// ThrottledExecutor runs at most one execution per key per interval. Call ShutDown
// on app exit to stop the eviction goroutine.
type ThrottledExecutor struct {
	done          chan struct{}
	sMap          sync.Map
	interval      time.Duration
	cleanupTicker *time.Ticker
}

func NewThrottledExecutor(interval time.Duration) *ThrottledExecutor {
	e := &ThrottledExecutor{
		done:          make(chan struct{}),
		sMap:          sync.Map{},
		interval:      interval,
		cleanupTicker: time.NewTicker(2 * interval),
	}
	e.startCleanupRoutine()
	return e
}

func (e *ThrottledExecutor) startCleanupRoutine() {
	go func() {
		for {
			select {
			case <-e.done:
				e.cleanupTicker.Stop()
				return
			case <-e.cleanupTicker.C:
				threshold := time.Now().Add(-e.interval)
				e.sMap.Range(func(key, value any) bool {
					if value.(time.Time).Before(threshold) {
						e.sMap.Delete(key)
					}
					return true
				})
			}
		}
	}()
}

func (e *ThrottledExecutor) shouldProceed(key string, actionTime time.Time) bool {
	if lastAction, ok := e.sMap.Load(key); ok {
		return actionTime.Sub(lastAction.(time.Time)) >= e.interval
	}
	return true
}

// Exec runs fn on the caller's context when the interval has elapsed for key.
func (e *ThrottledExecutor) Exec(ctx context.Context, key string, name string, fn Executor) {
	ctx, span := tracer.Start(ctx, "throttledExecutor."+name+".Exec")
	defer span.End()

	now := time.Now()
	if e.shouldProceed(key, now) {
		span.SetAttributes(attribute.String("execution.status", "passthrough"))
		e.sMap.Store(key, now)
		fn(ctx)
	} else {
		span.SetAttributes(attribute.String("execution.status", "throttled"))
	}
}

// NbExec is fire-and-forget on its own short-lived context. Cold start may let a
// few concurrent executions through — callers must be idempotent.
func (e *ThrottledExecutor) NbExec(key string, name string, fn Executor) {
	ctx := context.Background()
	_, span := tracer.Start(ctx, "throttledExecutor."+name+".NbExec")
	defer span.End()

	now := time.Now()
	if e.shouldProceed(key, now) {
		span.SetAttributes(attribute.String("execution.status", "passthrough"))
		e.sMap.Store(key, now)

		go func() {
			dctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			fn(dctx)
		}()

	} else {
		span.SetAttributes(attribute.String("execution.status", "throttled"))
	}

}

func (e *ThrottledExecutor) ShutDown() {
	close(e.done)
}
