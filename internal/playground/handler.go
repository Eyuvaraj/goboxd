package playground

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html favicon.svg
var staticFiles embed.FS

// Handler returns an http.Handler that serves the playground SPA.
func Handler() http.Handler {
	fSys, err := fs.Sub(staticFiles, ".")
	if err != nil {
		panic(err) // Should never happen with valid embed
	}
	return http.FileServer(http.FS(fSys))
}
