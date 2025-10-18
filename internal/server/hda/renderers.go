package hda

import (
	"net/http"

	"github.com/effiware/goth-template/internal/server/models"
	views "github.com/effiware/goth-template/internal/views"
	components "github.com/effiware/goth-template/internal/views/components"
)

var clicks *models.Clicks = models.ClicksStore

func RenderRoot(w http.ResponseWriter, r *http.Request) error {
	template := views.Index()
	return template.Render(r.Context(), w)
}

func RenderClick(w http.ResponseWriter, r *http.Request) error {
	clicks.Increment()
	template := components.Click()
	return template.Render(r.Context(), w)
}
