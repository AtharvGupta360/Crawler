package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// frontendFS holds the embedded frontend build output.
// It is set from cmd/server via SetFrontendFS when the embedded dist directory
// is available. If nil, no static serving is configured (dev mode uses Vite).
var frontendFS fs.FS

// SetFrontendFS registers the embedded frontend filesystem for serving.
// Should be called from cmd/server/main.go before NewServer.
func SetFrontendFS(f fs.FS) {
	frontendFS = f
}

// setupStaticRoutes mounts the embedded frontend if available.
// It must be called AFTER all API routes so the catch-all doesn't shadow them.
func (s *Server) setupStaticRoutes() {
	if frontendFS == nil {
		s.logger.Info("no embedded frontend found, static serving disabled (use Vite dev server)")
		return
	}

	// Verify the embedded FS has content by checking for index.html
	if _, err := fs.Stat(frontendFS, "index.html"); err != nil {
		s.logger.Warn("embedded frontend has no index.html, static serving disabled")
		return
	}

	fileServer := http.FileServer(http.FS(frontendFS))

	s.logger.Info("serving embedded frontend from Go binary")

	// Catch-all: serve static files or fall back to index.html for SPA routing
	s.router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the requested file from the embedded FS
		if file, err := frontendFS.Open(path); err == nil {
			file.Close()

			// Set cache headers based on path
			if strings.HasPrefix(path, "assets/") {
				// Vite hashed assets — cache immutably
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}

			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found → serve index.html for SPA client-side routing
		// Ensure index.html is never cached so updates propagate immediately
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}))
}
