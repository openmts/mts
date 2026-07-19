package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dashboard-dist
var dashboardFS embed.FS

// normalizeDashboardBase 规范化仪表盘子路径。
// 空值或 "/" 表示根路径；其他值返回以 / 开头、以 / 结尾的前缀（如 /mts/）。
func normalizeDashboardBase(base string) string {
	base = strings.TrimSpace(base)
	if base == "" || base == "/" {
		return "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

func stripDashboardBase(path, base string) (string, bool) {
	if base == "/" {
		return path, true
	}
	trimmed := strings.TrimSuffix(base, "/")
	if path == trimmed {
		return "/", true
	}
	if strings.HasPrefix(path, base) {
		rel := path[len(trimmed):]
		if rel == "" {
			rel = "/"
		}
		if !strings.HasPrefix(rel, "/") {
			rel = "/" + rel
		}
		return rel, true
	}
	return "", false
}

func dashboardHandler(basePath string) http.Handler {
	basePath = normalizeDashboardBase(basePath)
	sub, err := fs.Sub(dashboardFS, "dashboard-dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("dashboard not built"))
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if basePath != "/" {
			trimmed := strings.TrimSuffix(basePath, "/")
			if req.URL.Path == trimmed {
				http.Redirect(w, req, basePath, http.StatusFound)
				return
			}
		}

		rel, ok := stripDashboardBase(req.URL.Path, basePath)
		if !ok {
			http.NotFound(w, req)
			return
		}

		clone := *req
		u := *req.URL
		u.Path = rel
		clone.URL = &u

		reqPath := strings.TrimPrefix(rel, "/")
		f, fErr := sub.Open(reqPath)
		if fErr != nil {
			if strings.HasPrefix(rel, "/api/") || strings.HasPrefix(rel, "/debug/") {
				http.NotFound(w, req)
				return
			}
			u2 := u
			u2.Path = "/"
			clone.URL = &u2
		} else {
			_ = f.Close()
		}
		if strings.HasPrefix(clone.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, &clone)
	})
}
