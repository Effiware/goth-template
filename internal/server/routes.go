package server

import (
	"net/http"

	"github.com/effiware/goth-template/internal"
	"github.com/effiware/goth-template/internal/server/api"
	"github.com/effiware/goth-template/internal/server/hda"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (hdaAndApi *HdaAndApi) RegisterRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Heartbeat("/ping"))
	r.Use(middleware.Logger)

	r.HandleFunc("/", hda.WithJsonFallback(hda.RenderRoot))
	r.Handle("/static/*", http.FileServer(http.FS(internal.StaticFiles)))
	r.Post("/clicked", hda.WithJsonFallback(hda.RenderClick))

	r.Get("/api/v1/clicks", api.JsonHandler(api.GetClicks))
	r.Post("/api/v1/clicks/increment", api.JsonHandler(api.IncrementClicks))

	return r
}
