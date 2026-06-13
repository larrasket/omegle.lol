package webroot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// Minimal SPA layout: index.html as the fallback, privacy.html as a
// prerendered route, and a favicon. Mirrors what adapter-static produces.
func mockFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":   &fstest.MapFile{Data: []byte(`<html><body data-sentinel="SPA SHELL"></body></html>`)},
		"privacy.html": &fstest.MapFile{Data: []byte(`<html><body data-sentinel="PRIVACY"></body></html>`)},
		"favicon.svg":  &fstest.MapFile{Data: []byte("<svg/>")},
	}
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	h.ServeHTTP(rec, req)
	return rec
}

// Regression test for the bug where SPA-fallback routes 301-redirected
// to ./ (resolving to the home page). The path /<unknown> must serve the
// SPA shell with HTTP 200 — never a 301 — so the client router can pick
// up the original URL.
func TestSPAFallbackServesShellWithoutRedirect(t *testing.T) {
	h := HandlerFS(mockFS())
	for _, p := range []string{
		"/705812da16d3edd0",  // admin route slug
		"/705812da16d3edd0/", // same with trailing slash
		"/some-deep-link",    // any unknown route
		"/nested/deep/link",  // multi-segment
	} {
		rec := get(t, h, p)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d (location=%q)", p, rec.Code, rec.Header().Get("Location"))
		}
		if !strings.Contains(rec.Body.String(), "SPA SHELL") {
			t.Errorf("GET %s: expected SPA shell content, got %q", p, rec.Body.String())
		}
	}
}

func TestPrerenderedRouteServesItsOwnHTML(t *testing.T) {
	h := HandlerFS(mockFS())
	rec := get(t, h, "/privacy")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /privacy: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PRIVACY") {
		t.Errorf("GET /privacy: expected privacy.html content, got %q", rec.Body.String())
	}
}

func TestExplicitHTMLPathServesFile(t *testing.T) {
	h := HandlerFS(mockFS())
	rec := get(t, h, "/privacy.html")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /privacy.html: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PRIVACY") {
		t.Errorf("expected privacy.html content, got %q", rec.Body.String())
	}
}

func TestStaticAssetServesDirectly(t *testing.T) {
	h := HandlerFS(mockFS())
	rec := get(t, h, "/favicon.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /favicon.svg: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<svg/>") {
		t.Errorf("expected favicon body, got %q", rec.Body.String())
	}
}

func TestRootServesShell(t *testing.T) {
	h := HandlerFS(mockFS())
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA SHELL") {
		t.Errorf("expected SPA shell content, got %q", rec.Body.String())
	}
}
