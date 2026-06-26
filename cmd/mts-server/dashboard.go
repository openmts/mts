package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dashboard-dist
var dashboardFS embed.FS

func dashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dashboard-dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("dashboard not built"))
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		reqPath := strings.TrimPrefix(req.URL.Path, "/")
		f, fErr := sub.Open(reqPath)
		if fErr != nil {
			if strings.HasPrefix(req.URL.Path, "/api/") ||
				strings.HasPrefix(req.URL.Path, "/debug/") {
				http.NotFound(w, req)
				return
			}
			req.URL.Path = "/"
		} else {
			_ = f.Close()
		}
		if strings.HasPrefix(req.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, req)
	})
}
