package services

import (
	"context"

	"github.com/effiware/goth-template/internal/version"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer(version.ServiceName) //nolint:gochecknoglobals

type Servicer interface {
	InitMetrics()
	Sync(context.Context) error
}
