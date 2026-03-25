package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var dashboardFS embed.FS

// WebHandler returns an http.Handler that serves the embedded web dashboard.
func WebHandler() http.Handler {
	fSys, err := fs.Sub(dashboardFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fSys))
}
