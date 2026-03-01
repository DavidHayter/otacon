package api

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed ui-dist/*
var uiFiles embed.FS

// UIHandler returns an HTTP handler that serves the embedded UI
// Falls back to index.html for SPA routing
func UIHandler() http.Handler {
	distFS, err := fs.Sub(uiFiles, "ui-dist")
	if err != nil {
		log.Printf("[ui] Warning: embedded UI not found, UI will be unavailable: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"UI not built. Run 'cd ui && npm run build' first."}`))
		})
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve API requests normally (handled by API mux)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		f, err := distFS.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// SPA fallback — serve index.html for all non-file routes
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
