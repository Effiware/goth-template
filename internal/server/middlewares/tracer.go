package middlewares

import (
	"github.com/effiware/goth-template/internal/version"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer(version.ServiceName) //nolint:gochecknoglobals
