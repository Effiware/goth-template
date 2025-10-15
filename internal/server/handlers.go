package server

import (
	"net/http"

	"github.com/effiware/goth-template/internal/components"
)

var clicks int = 0

func root(w http.ResponseWriter, r *http.Request) error {
	return Render(w, r, components.Home())
}

func click(w http.ResponseWriter, r *http.Request) error {
	clicks++
	return Render(w, r, components.Click(clicks))
}
