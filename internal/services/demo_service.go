package services

import (
	"context"
	"log/slog"

	"github.com/effiware/goth-template/internal/clicks"
)

// DemoService is a minimal Servicer: it shows what a background job wires up to
// (runner, gate, tech_service_runs row) without doing anything real. Replace it.
type DemoService struct {
	counter  *clicks.Counter
	lastSeen int64
}

func NewDemoService(counter *clicks.Counter) *DemoService {
	return &DemoService{counter: counter}
}

// InitMetrics is called once after the MeterProvider is set; create instruments here.
func (s *DemoService) InitMetrics() {}

func (s *DemoService) Sync(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "DemoService.Sync")
	defer span.End()

	count, err := s.counter.Count(ctx)
	if err != nil {
		return err
	}
	s.lastSeen = count
	slog.InfoContext(ctx, "Demo service tick", "clicks", count)
	return nil
}

// LastRunDetails feeds tech_service_runs.details (the RunDetailer hook).
func (s *DemoService) LastRunDetails() map[string]int64 {
	return map[string]int64{"clicks": s.lastSeen}
}
