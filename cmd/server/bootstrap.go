package main

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/effiware/goth-template/internal/clicks"
	"github.com/effiware/goth-template/internal/config"
	"github.com/effiware/goth-template/internal/db"
	mw "github.com/effiware/goth-template/internal/server/middlewares"
	"github.com/effiware/goth-template/internal/server/session"
	"github.com/effiware/goth-template/internal/services"
	"github.com/effiware/goth-template/internal/version"
	"github.com/effiware/goth-template/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer(version.ServiceName) //nolint:gochecknoglobals

var containerID string //nolint:gochecknoglobals

// getContainerID returns HOSTNAME (the container ID under K8s/Docker) or a
// random name; memoized so the log `instance` field and the trace resource agree.
func getContainerID(defaultName string) string {
	if containerID == "" {
		if containerID = os.Getenv("HOSTNAME"); containerID == "" {
			containerID = defaultName + "-" + utils.RandomString(7)
		}
	}
	return containerID
}

// traceHandler stamps trace_id/span_id from a ctx-carried span onto every record.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}
func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}
func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// bootLogger installs the JSON logger at the configured level, with constant identity fields on every line.
func bootLogger(cfg *config.Config) {
	var level = new(slog.Level)
	if err := level.UnmarshalText([]byte(cfg.Server.LogLevel)); err == nil {
		slog.SetLogLoggerLevel(*level)
	} else {
		slog.Error("Error while unmarshalling log level", "error", err)
	}
	handler := traceHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})}
	slog.SetDefault(slog.New(handler).With(
		"service", serviceName(cfg), "env", cfg.Server.Environment, "instance", getContainerID(cfg.Server.Name)))
	slog.Info("Initialized slog with", "level", level)
}

// serviceName resolves service identity for traces AND logs: the standard
// OTEL_SERVICE_NAME wins over config — the SDK resource builder never reads it.
func serviceName(cfg *config.Config) string {
	return cmp.Or(os.Getenv("OTEL_SERVICE_NAME"), cfg.Server.Name)
}

// otlpEndpointFromEnv reports whether the standard OTLP endpoint env is set — then
// the exporter gets no endpoint/TLS options (the env URL's scheme decides TLS).
func otlpEndpointFromEnv() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// bootOtelResource builds the resource shared by the tracer and meter providers.
func bootOtelResource(cfg *config.Config) *resource.Resource {
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName(cfg)),
		semconv.ServiceVersionKey.String(version.Version),
		attribute.String("vcs.ref.head.revision", version.BuildHash), // not yet in semconv/v1.26.0
		semconv.DeploymentEnvironmentKey.String(cfg.Server.Environment),
		semconv.ContainerIDKey.String(getContainerID(cfg.Server.Name)),
		semconv.TelemetrySDKLanguageGo,
		semconv.TelemetrySDKNameKey.String("opentelemetry"),
		semconv.TelemetrySDKVersionKey.String("1.26.0"),
	)
	// Honor OTEL_RESOURCE_ATTRIBUTES on top; on schema conflict keep our resource.
	if merged, err := resource.Merge(res, resource.Environment()); err == nil {
		res = merged
	}
	return res
}

// bootOtel initializes tracing and sets the global provider. Returns nil (tracing
// off) when neither otlp.url nor the OTEL_* endpoint env is set.
func bootOtel(cfg *config.Config, otelResource *resource.Resource) *sdktrace.TracerProvider {
	envEndpoint := otlpEndpointFromEnv()
	if cfg.Otlp.Url == "" && !envEndpoint {
		return nil
	}

	var exporter sdktrace.SpanExporter
	var err error
	switch cfg.Otlp.Protocol {
	case "http":
		var opts []otlptracehttp.Option
		if !envEndpoint {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Otlp.Url))
			if !cfg.Otlp.Secure {
				opts = append(opts, otlptracehttp.WithInsecure())
			}
		}
		exporter, err = otlptracehttp.New(context.Background(), opts...)
	default: // grpc
		var opts []otlptracegrpc.Option
		if !envEndpoint {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Otlp.Url))
			if !cfg.Otlp.Secure {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}
		}
		exporter, err = otlptracegrpc.New(context.Background(), opts...)
	}
	if err != nil {
		slog.Error("Failed to create OTLP trace exporter", "error", err)
		os.Exit(1)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(otelResource),
	)
	otel.SetTracerProvider(tracerProvider)

	// The batch exporter drops spans silently on failure — log it, max once per 30 s.
	var lastErrLog atomic.Int64
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		now := time.Now().Unix()
		if last := lastErrLog.Load(); now-last >= 30 && lastErrLog.CompareAndSwap(last, now) {
			slog.Error("OTel export error", "error", err)
		}
	}))

	// Self-check span: forces one export attempt after boot so a dead pipeline is loud.
	_, span := tracerProvider.Tracer(version.ServiceName).Start(context.Background(), "otel.selfcheck")
	span.End()

	slog.Info("OTLP trace provider initialized",
		"protocol", cfg.Otlp.Protocol, "url", cfg.Otlp.Url, "secure", cfg.Otlp.Secure, "endpoint_from_env", envEndpoint)

	return tracerProvider
}

// bootMeter initializes metrics behind a Prometheus exporter and sets the global
// provider. Returns nils when metrics are disabled.
func bootMeter(cfg *config.Config, otelResource *resource.Resource) (*sdkmetric.MeterProvider, *prometheus.Registry) {
	if !cfg.Metrics.Enabled {
		return nil, nil
	}

	registry := prometheus.NewRegistry()
	// Standard go_* / process_* families — a custom registry starts empty.
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	exporter, err := otelprometheus.New(
		otelprometheus.WithRegisterer(registry),
		otelprometheus.WithoutScopeInfo(),
		otelprometheus.WithNamespace(strings.ReplaceAll(cfg.Server.Name, "-", "_")),
	)
	if err != nil {
		slog.Error("Failed to create Prometheus exporter", "error", err)
		os.Exit(1)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(otelResource),
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(meterProvider)
	slog.Info("Prometheus MeterProvider initialized")

	return meterProvider, registry
}

// bootDbStore opens the pool, runs migrations and registers pool metrics.
func bootDbStore(cfg *config.Config) *db.Store {
	dbCtx := context.Background()
	dbStore, err := db.NewStore(dbCtx, cfg.Database.URL, cfg.Database.PoolMaxConns)
	if err != nil {
		slog.Error("Failed to create DB store", "error", err)
		os.Exit(1)
	}
	if !cfg.Database.SkipMigrations {
		if err := dbStore.RunMigrations(dbCtx); err != nil {
			slog.Error("Failed to run DB migrations", "error", err)
			os.Exit(1)
		}
		slog.Info("DB store created and migrations ran successfully")
	} else {
		slog.Info("DB store created")
	}
	if err := dbStore.RegisterPoolMetrics(); err != nil {
		slog.Error("Failed to register DB pool metrics", "error", err)
	}

	return dbStore
}

// bootRedis returns a client built from redis.url, or nil when no URL is
// configured. Every consumer must treat nil as "skip cache, hit source".
func bootRedis(cfg *config.Config) redis.Cmdable {
	if cfg.Redis.URL == "" {
		slog.Warn("No redis.url configured — session store and caches disabled")
		return nil
	}
	opts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		slog.Error("Failed to parse redis URL", "error", err)
		os.Exit(1)
	}
	return redis.NewClient(opts)
}

// bootSessionStore returns nil when Redis is not configured; handlers degrade.
func bootSessionStore(cfg *config.Config) *session.Store {
	if cfg.Redis.URL == "" {
		return nil
	}
	sessionStore, err := session.NewStore(context.Background(), session.StoreConfig{
		MaxAge:   cfg.Session.MaxAge,
		Secure:   cfg.Session.Secure,
		RedisURL: cfg.Redis.URL,
	})
	if err != nil {
		slog.Error("Failed to create session store", "error", err)
		os.Exit(1)
	}
	slog.Info("Session store initialized", "max_age", cfg.Session.MaxAge, "secure", cfg.Session.Secure)

	return sessionStore
}

// securityHeaderOptions builds the CSP middleware options from config.
func securityHeaderOptions(cfg *config.Config) mw.SecurityHeadersOptions {
	return mw.SecurityHeadersOptions{ReportOnly: cfg.Server.CSPReportOnly}
}

// bootServices registers the background Servicers by name.
func bootServices(counter *clicks.Counter) map[string]services.Servicer {
	svc := make(map[string]services.Servicer)
	svc["demo"] = services.NewDemoService(counter)
	return svc
}
