package embed

import (
	"embed"
	"net/http"
)

//go:embed ../../web/static/.*
var staticFS embed.FS

//go:embed ../../web/templates/*
var templateFS embed.FS

// StaticFileServer returns an http.Handler that serves static files from the embedded filesystem.
func StaticFileServer() http.Handler {
	return http.FileServer(http.FS(staticFS))
}

// TemplateFS returns the embedded filesystem for templates.
func TemplateFS() embed.FS {
	return templateFS
}
