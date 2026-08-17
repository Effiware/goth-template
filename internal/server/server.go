package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/effiware/goth-template/internal/clicks"
	"github.com/effiware/goth-template/internal/db"
	_ "github.com/effiware/goth-template/internal/docs"
	mw "github.com/effiware/goth-template/internal/server/middlewares"
	"github.com/effiware/goth-template/internal/server/session"
	"github.com/effiware/goth-template/internal/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// HdaAndApi carries everything the routes need. Add dependencies here rather
// than reaching for globals.
type HdaAndApi struct {
	dbStore            *db.Store
	sessionStore       *session.Store
	redisClient        redis.Cmdable
	counter            *clicks.Counter
	serviceMap         map[string]services.Servicer
	prometheusRegistry *prometheus.Registry
	devToolsEnabled    bool
	securityHeaders    mw.SecurityHeadersOptions
	appOrigin          string
}

// Options groups the server's dependencies; HttpServer and NewHdaAndApi share it.
type Options struct {
	Host               string
	Port               int
	Timeout            int
	DbStore            *db.Store
	SessionStore       *session.Store
	RedisClient        redis.Cmdable
	Counter            *clicks.Counter
	ServiceMap         map[string]services.Servicer
	PrometheusRegistry *prometheus.Registry
	DevToolsEnabled    bool
	SecurityHeaders    mw.SecurityHeadersOptions
	AppOrigin          string // scheme://host used by the CSRF origin check
}

func NewHdaAndApi(opts Options) *HdaAndApi {
	return &HdaAndApi{
		dbStore:            opts.DbStore,
		sessionStore:       opts.SessionStore,
		redisClient:        opts.RedisClient,
		counter:            opts.Counter,
		serviceMap:         opts.ServiceMap,
		prometheusRegistry: opts.PrometheusRegistry,
		devToolsEnabled:    opts.DevToolsEnabled,
		securityHeaders:    opts.SecurityHeaders,
		appOrigin:          opts.AppOrigin,
	}
}

func HttpServer(opts Options) *http.Server {
	hdaAndApi := NewHdaAndApi(opts)
	readTimeout := time.Duration(opts.Timeout) * time.Second

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Handler:      hdaAndApi.RegisterRoutes(),
		ReadTimeout:  readTimeout,
		WriteTimeout: 3 * readTimeout,
		IdleTimeout:  6 * readTimeout,
		// Route net/http's own errors (TLS, conn) through the JSON logger.
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	server.RegisterOnShutdown(func() {
		slog.Debug("HTTP server shutdown complete")
	})

	return server
}
