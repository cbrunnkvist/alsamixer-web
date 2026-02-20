package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

// StaticFileServer returns an http.Handler that serves static files from the embedded filesystem.
func StaticFileServer() http.Handler {
	return http.FileServer(http.FS(StaticFS()))
}

// TemplateFS returns the embedded filesystem for templates.
func TemplateFS() fs.FS {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		panic(err)
	}
	return sub
}
