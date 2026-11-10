// Package web serves the built React client. The TypeScript frontend stays the
// UI; this package only embeds or locates its build output.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed dist
var embedded embed.FS

// Handler returns the static file handler. A GHL_WEB_DIST directory or the
// repository's apps/web/dist wins over the embedded placeholder so a real build
// is served without recompiling.
func Handler(repoRoot string) http.Handler {
	if dir := os.Getenv("GLH_WEB_DIST"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return http.FileServer(http.Dir(dir))
		}
	}
	if repoRoot != "" {
		candidate := repoRoot + "/apps/web/dist"
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return http.FileServer(http.Dir(candidate))
		}
	}
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
