package server

import (
	"net/http"

	views "github.com/effiware/goth-template/internal/views"
	components "github.com/effiware/goth-template/internal/views/components"
)

var clicks int = 0

func renderRoot(w http.ResponseWriter, r *http.Request) error {
	template := views.Index()
	return template.Render(r.Context(), w)
}

func renderClick(w http.ResponseWriter, r *http.Request) error {
	clicks++
	template := components.Click(clicks)
	return template.Render(r.Context(), w)
}
