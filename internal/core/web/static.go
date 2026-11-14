package web

import (
	"net/http"
	"os"
	"strings"
)

// manifestSuffix is the extension of a web app manifest, which the system MIME tables often do not know.
const manifestSuffix string = ".webmanifest"

// manifestMediaType is the media type browsers expect for a web app manifest.
const manifestMediaType string = "application/manifest+json"

// StaticHandler serves the built assets (CSS, JS, fonts, the web app manifest) from dir under /static/.
func StaticHandler(dir string) http.Handler {
	files := http.StripPrefix("/static/", http.FileServer(http.FS(os.DirFS(dir))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, manifestSuffix) {
			w.Header().Set("Content-Type", manifestMediaType)
		}
		files.ServeHTTP(w, r)
	})
}
