package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // the alpine runtime image has no /usr/share/zoneinfo

	"github.com/effiware/goth-template/internal/clicks"
	"github.com/effiware/goth-template/internal/config"
	"github.com/effiware/goth-template/internal/server"
	"github.com/effiware/goth-template/internal/server/api"
	"github.com/effiware/goth-template/internal/services"
	"github.com/effiware/goth-template/internal/version"
)

func main() {
	//-------------------------------------------------------------------------
	//----------------------------------PREP-----------------------------------
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration file", "error", err)
		os.Exit(1)
	}

	bootLogger(cfg)

	// Shared OTel resource for both tracing and metrics.
	otelResource := bootOtelResource(cfg)
	tracerProvider := bootOtel(cfg, otelResource)
	meterProvider, prometheusRegistry := bootMeter(cfg, otelResource)

	dbStore := bootDbStore(cfg)
	redisClient := bootRedis(cfg)
	sessionStore := bootSessionStore(cfg)
	counter := clicks.NewCounter(dbStore, redisClient)
	serviceMap := bootServices(counter)

	// Instruments must be created after the providers are set: the global
	// no-op meter would otherwise hand out permanently no-op instruments.
	for _, svc := range serviceMap {
		svc.InitMetrics()
	}

	httpServer := server.HttpServer(server.Options{
		Host:               cfg.Server.Host,
		Port:               cfg.Server.Port,
		Timeout:            cfg.Server.Timeout,
		DbStore:            dbStore,
		SessionStore:       sessionStore,
		RedisClient:        redisClient,
		Counter:            counter,
		ServiceMap:         serviceMap,
		PrometheusRegistry: prometheusRegistry,
		DevToolsEnabled:    !slices.Contains([]string{"production", "prod"}, strings.ToLower(cfg.Server.Environment)),
		SecurityHeaders:    securityHeaderOptions(cfg),
		AppOrigin:          cfg.Server.BaseURL,
	})

	slog.Info("Starting server", "address", httpServer.Addr, "version", version.Version, "build", version.BuildHash)

	//-------------------------------------------------------------------------
	//---------------------------------START-----------------------------------
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	syncInterval := time.Duration(cfg.Server.SyncIntervalMin) * time.Minute

	// Boot-time sync catches up after downtime; with a live peer there is none,
	// so it is gated on the service's own cadence.
	syncCtx, span := tracer.Start(context.Background(), "SyncServices")
	if _, err := services.RunIfDue(syncCtx, dbStore, "demo", serviceMap["demo"], syncInterval); err != nil {
		slog.Error("Initial sync failed", "service", "demo", "error", err)
	}
	span.End()

	// Periodic runners: gated, so the cadence holds fleet-wide, not per instance.
	demoRunner := services.NewPeriodicRunner("demo", serviceMap["demo"], dbStore, syncInterval)
	demoRunner.Start()

	// Wall-clock sibling of the above; disabled when server.sync_at is empty.
	var scheduledRunner *services.ScheduledRunner
	if spec := cfg.Server.SyncAt; spec != "" {
		if scheduledRunner, err = services.NewScheduledRunner("demo-scheduled", serviceMap["demo"], dbStore, spec); err != nil {
			slog.Error("Scheduled sync disabled", "spec", spec, "error", err)
		} else {
			scheduledRunner.Start()
			slog.Info("Scheduled sync armed", "spec", spec, "timezone", services.ScheduleTZ)
		}
	}

	//-------------------------------------------------------------------------
	//----------------------------------QUIT-----------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	case sig := <-quit:
		slog.Info("Received shutdown signal", "signal", sig)
	}

	// Fail readiness before closing the listener: k8s removes the pod from
	// Endpoints asynchronously, so requests keep arriving for a moment after SIGTERM.
	api.BeginDraining()
	if grace := time.Duration(cfg.Server.DrainGraceSec) * time.Second; grace > 0 {
		slog.Info("Draining: /readyz now failing, still serving", "grace", grace)
		time.Sleep(grace)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	slog.Info("Shutting down HTTP server...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Runners stop after the server — they may still emit spans/metrics.
	demoRunner.ShutDown()
	if scheduledRunner != nil {
		scheduledRunner.ShutDown()
	}

	// Providers last, so the flush carries everything above.
	if meterProvider != nil {
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("MeterProvider shutdown error", "error", err)
		}
	}
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("TracerProvider shutdown error", "error", err)
		}
	}

	dbStore.Close()
	slog.Info("Graceful shutdown complete")
}
