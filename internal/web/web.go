// Package web embeds the SvelteKit dashboard built into the web/build/
// directory and serves it as a single-page app at /dashboard.
//
// The build script (`pnpm --dir web build`) writes the static bundle into
// internal/web/dist/. A placeholder index.html is committed so the embed
// directive resolves on a fresh checkout before the JS toolchain has run;
// the real bundle replaces it during the release build.
package web

import (
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var embedded embed.FS

// MountPrefix is the URL prefix at which the dashboard is served.
const MountPrefix = "/dashboard"

// Mount installs middleware on the supplied gin engine that intercepts any
// GET request whose path is exactly MountPrefix or starts with MountPrefix+"/"
// and serves the corresponding file from the embedded dist filesystem. Paths
// that do not match a real file fall back to index.html so client-side
// routing works for deep links.
//
// We use middleware (rather than engine.GET("/dashboard/*filepath", ...))
// because gin's radix trie does not allow catch-all wildcards to coexist
// with sibling routes registered at the root prefix. Middleware sits in
// front of the trie and avoids the conflict entirely.
//
// The mount does not apply auth: the dashboard's JavaScript reads its own
// bearer token from localStorage and supplies it on each /v0/management/*
// call. The static assets themselves are not sensitive.
func Mount(engine *gin.Engine) error {
	if engine == nil {
		return nil
	}
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return err
	}

	serve := func(c *gin.Context, name string) {
		f, err := sub.Open(name)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer f.Close()
		body, rerr := io.ReadAll(f)
		if rerr != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		ct := mime.TypeByExtension(filepath.Ext(name))
		if ct == "" {
			ct = http.DetectContentType(body)
		}
		// Cache hashed asset bundles aggressively; everything else is no-store
		// so the user always sees the latest dashboard after a redeploy.
		if strings.Contains(name, "_app/immutable/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-store")
		}
		c.Data(http.StatusOK, ct, body)
		c.Abort()
	}

	engine.Use(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if path != MountPrefix && !strings.HasPrefix(path, MountPrefix+"/") {
			c.Next()
			return
		}

		raw := strings.TrimPrefix(path, MountPrefix)
		raw = strings.TrimPrefix(raw, "/")
		if raw == "" || strings.HasSuffix(raw, "/") {
			serve(c, "index.html")
			return
		}

		// Try the file as-is; on miss, serve index.html for SPA deep links.
		if f, err := sub.Open(raw); err == nil {
			_ = f.Close()
			serve(c, raw)
			return
		}
		serve(c, "index.html")
	})
	return nil
}
