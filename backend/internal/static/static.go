// Package static serves the built single-page application.
//
// In production the SPA and the API share one origin: the browser asks for
// /api/... (handled by the router before this package is reached) and for
// everything else this handler answers with a file, falling back to index.html
// so client-side routes survive a refresh.
package static

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler returns a handler that serves dir, or nil when dir is empty so the
// caller can skip mounting it entirely.
func Handler(dir string) http.HandlerFunc {
	if strings.TrimSpace(dir) == "" {
		return nil
	}

	root := http.Dir(dir)
	files := http.FileServer(root)
	indexPath := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(r.URL.Path)

		// Hashed build output never changes, so it can be cached forever.
		// Everything else (index.html especially) must be revalidated or a
		// deploy would leave clients on stale hashes.
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		if served := serveFile(w, r, root, files, clean); served {
			return
		}

		// Unknown path: hand back the shell and let the client router decide.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, indexPath)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, root http.Dir, files http.Handler, clean string) bool {
	if clean == "/" {
		return false
	}

	file, err := root.Open(clean)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}

	// ServeFile would happily serve a directory listing; the explicit file
	// check above prevents that, and this keeps range requests working.
	files.ServeHTTP(w, r)
	return true
}

// Exists reports whether a usable index.html is present. The API refuses to
// start with an unusable STATIC_DIR rather than serving 404s in production.
func Exists(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}
