// Package internal GOTH Template API
//
//	@title			GOTH Template API
//	@version		1.0
//	@description	JSON API of the GOTH stack template.
//	@contact.name	Effi-Support
//	@contact.url	https://www.effiware.com/contact
//	@contact.email	contact@effiware.com
//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html
//	@host			localhost:8080
//	@schemes		http
//	@BasePath		/api/v1
package internal

import (
	"embed"

	_ "github.com/effiware/goth-template/internal/docs"
)

// This file exists to embed the static assets and the swagger JSON, and to
// register the swagger docs entry point via the blank import above.
// DB migrations are embedded separately, in db/store.go.

//go:embed static/*
var StaticFiles embed.FS

//go:embed docs/swagger.json
var DocsFS embed.FS
