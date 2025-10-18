package server

import (
	"net/http"

	"github.com/effiware/goth-template/internal"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (hdaAndApi *HdaAndApi) RegisterRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Heartbeat("/ping"))
	r.Use(middleware.Logger)

	r.Handle("/static/*", http.FileServer(http.FS(internal.StaticFiles)))

	r.HandleFunc("/", HdaHandlerWithJsonFallback(renderRoot))

	r.Post("/clicked", HdaHandlerWithJsonFallback(renderClick))

	return r
}
