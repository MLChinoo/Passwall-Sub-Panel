package handler

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/web"
)

// staticAsset holds a pre-loaded embedded SPA file. Pre-fix StaticSPA
// called fs.ReadFile + mime.TypeByExtension on every request — fine in
// theory because go:embed reads are memory-backed, but at polling-
// dashboard rate this still showed up. Loading once at init eliminates
// the syscall-shaped overhead and the per-request mime lookup.
type staticAsset struct {
	body        []byte
	contentType string
}

var (
	staticAssetsOnce sync.Once
	staticAssets     map[string]staticAsset
)

// loadStaticAssets walks the embedded dist tree once and builds a
// path → asset map. Pre-computes Content-Type so the request path
// never has to compute it.
func loadStaticAssets() map[string]staticAsset {
	staticAssetsOnce.Do(func() {
		out := map[string]staticAsset{}
		sub, err := fs.Sub(web.DistFS, "dist")
		if err != nil {
			staticAssets = out
			return
		}
		_ = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			b, rerr := fs.ReadFile(sub, path)
			if rerr != nil {
				return nil
			}
			ct := mime.TypeByExtension(filepath.Ext(path))
			if ct == "" {
				ct = "application/octet-stream"
			}
			out[path] = staticAsset{body: b, contentType: ct}
			return nil
		})
		staticAssets = out
	})
	return staticAssets
}

// StaticSPA serves the embedded SPA bundle and falls back to index.html for
// any non-asset path (so React Router's history mode works).
//
// Wire it as a NoRoute handler so /api and /sub keep their precedence.
//
// Caching policy:
//   - /assets/*  → Cache-Control: public, max-age=1y, immutable. Vite hashes
//     these filenames on every content change, so an old URL can never refer
//     to new content; long caching is safe.
//   - index.html and other root files (favicon etc.) → no-cache so a redeploy
//     propagates immediately. The hashed-asset URLs inside index.html update
//     on every build, which is what triggers the browser to pick up the new
//     bundle.
func StaticSPA(c *gin.Context) {
	assets := loadStaticAssets()
	if len(assets) == 0 {
		c.String(http.StatusInternalServerError, "frontend bundle not embedded")
		return
	}

	requested := strings.TrimPrefix(c.Request.URL.Path, "/")
	if requested == "" {
		requested = "index.html"
	}

	if a, ok := assets[requested]; ok {
		setCacheHeaders(c, requested)
		c.Data(http.StatusOK, a.contentType, a.body)
		return
	}

	// Asset-shaped paths (/assets/...) shouldn't fall back — return 404.
	if strings.HasPrefix(requested, "assets/") {
		c.String(http.StatusNotFound, "not found")
		return
	}

	// SPA fallback to index.html.
	if a, ok := assets["index.html"]; ok {
		setCacheHeaders(c, "index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", a.body)
		return
	}

	c.String(http.StatusNotFound,
		"Frontend bundle not built. Run `cd web && npm install && npm run build` then rebuild the Go binary.")
}

// setCacheHeaders applies the Vite-aware caching policy described on
// StaticSPA. Kept separate so the SPA fallback path can reuse it without
// duplicating the branch.
func setCacheHeaders(c *gin.Context, path string) {
	if strings.HasPrefix(path, "assets/") {
		// Filename carries an 8-char content hash; new content = new URL.
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	// index.html (and root files like favicon.ico) must revalidate so a new
	// deploy is picked up on the next page load instead of after the user
	// clears their cache.
	c.Header("Cache-Control", "no-cache")
}
