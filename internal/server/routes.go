package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/effiware/goth-template/internal"
	_ "github.com/effiware/goth-template/internal/docs"
	"github.com/effiware/goth-template/internal/server/api"
	"github.com/effiware/goth-template/internal/server/hda"
	mw "github.com/effiware/goth-template/internal/server/middlewares"
	"github.com/effiware/goth-template/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (hdaAndApi *HdaAndApi) RegisterRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Heartbeat("/ping"))
	r.Use(otelchi.Middleware(version.ServiceName, otelchi.WithChiRoutes(r),
		otelchi.WithFilter(func(req *http.Request) bool { return !api.IsProbePath(req) })))
	r.Use(mw.RequestMetrics(api.IsProbePath))
	// Inside otelchi so the panic log line and span error carry the trace context.
	r.Use(mw.Recoverer)
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		r.Use(mw.RequestLogger)
	}
	r.Use(mw.SecurityHeaders(hdaAndApi.securityHeaders))

	// Public: probes, assets, metrics.
	r.Get("/readyz", api.JsonHandler(api.Readyz(hdaAndApi.dbStore, hdaAndApi.redisClient)))
	r.Handle("/static/*", http.FileServer(http.FS(internal.StaticFiles)))
	if hdaAndApi.prometheusRegistry != nil {
		// Public; secure it at the network layer (K8s NetworkPolicy).
		r.Handle("/metrics", promhttp.HandlerFor(hdaAndApi.prometheusRegistry, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}))
	}

	// Hypermedia (HDA) routes — mutations carry the CSRF origin check.
	r.Group(func(r chi.Router) {
		r.Use(mw.CSRFOriginCheck(hdaAndApi.appOrigin))

		r.Get("/", hda.WithHTMLFallback(hda.RenderRoot(hdaAndApi.counter, hdaAndApi.sessionStore, hdaAndApi.appOrigin)))
		r.Post("/clicked", hda.WithHTMLFallback(hda.RenderClick(hdaAndApi.counter, hdaAndApi.sessionStore)))
	})

	// JSON API routes.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/clicks", api.JsonHandler(api.GetClicks(hdaAndApi.counter)))
		r.Post("/clicks/increment", api.JsonHandler(api.IncrementClicks(hdaAndApi.counter)))
	})

	// Dev tools: off outside non-production environments.
	if hdaAndApi.devToolsEnabled {
		r.Handle("/docs/*", http.FileServer(http.FS(internal.DocsFS)))
		r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/docs/swagger.json")))
	}

	return r
}
