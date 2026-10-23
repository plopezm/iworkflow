package web

import (
	"net/http"
	"os"
)

// StaticHandler serves the built assets (CSS, JS, fonts) from dir under /static/.
func StaticHandler(dir string) http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.FS(os.DirFS(dir))))
}
