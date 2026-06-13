// Package webroot serves the SvelteKit static build that's embedded into
// the Go binary at compile time. During the Docker build the placeholder
// `files/` directory is replaced with the real SPA build before `go build`
// runs, so the production binary carries the SPA bytes inside.
//
// During local development (`make dev`) this embed is rarely exercised —
// Vite serves the SPA directly on :5173 and proxies non-static routes to
// the Go server on :8080. The placeholder in `files/index.html` is only
// what `go run ./cmd/server` will hand out if you hit `/` without the
// frontend dev server running.
package webroot

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:files
var embedded embed.FS

// Handler returns an http.Handler backed by the embedded SPA build.
func Handler() http.Handler {
	sub, err := fs.Sub(embedded, "files")
	if err != nil {
		panic("webroot: " + err.Error())
	}
	return HandlerFS(sub)
}

// HandlerFS is the same logic with an injectable FS for tests. Unknown
// paths fall back to the SPA shell so SvelteKit deep links resolve
// client-side.
func HandlerFS(sub fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalise the request path.
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		// Static file present → serve as-is.
		if _, err := fs.Stat(sub, clean); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SvelteKit's adapter-static prerenders pages as bare-name files
		// (`/privacy` → `privacy.html`, not `privacy/index.html`). Try the
		// `.html` extension before falling through to the SPA shell so
		// prerendered routes serve their actual content.
		if !strings.HasSuffix(clean, ".html") {
			if _, err := fs.Stat(sub, clean+".html"); err == nil {
				r.URL.Path = "/" + clean + ".html"
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Unknown path (deep link / refresh on a Svelte route) → SPA fallback.
		// Use "/" rather than "/index.html": http.FileServer hardcodes a
		// "strip trailing /index.html" rule that 301-redirects to "./",
		// which from any deep-link URL resolves to the parent (i.e. /),
		// sending the user to the home page instead of their actual route.
		// Pointing at "/" makes FileServer resolve the directory index
		// without the redirect, and the browser keeps the original URL so
		// the SvelteKit client router can route to the right page.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
