// Package internal GOTH Template API
// @title GOTH Template API
// @version 1.0
// @description This is an example server for a GOTH template application.
// @contact.name Effi-Support
// @contact.url https://www.effiware.com/contact
// @contact.email contact@effiware.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @schemes http
// @BasePath /api/v1
package internal

import (
	"embed"

	_ "github.com/effiware/goth-template/internal/docs"
)

// This file exists for three reasons:
// 1. to embed the static files for the sever
// 2. to embed the swagger doc JSON file
// 3. to register the entry point for the swagger docs via the blank import above

//go:embed static/*
var StaticFiles embed.FS

//go:embed docs/swagger.json
var DocsFS embed.FS
