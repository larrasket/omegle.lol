package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omegle-lol/omegle/backend/internal/config"
)

func newClientIPServer(depth int) *Server {
	return &Server{Cfg: config.Config{TrustedProxyDepth: depth}}
}

func TestClientIP_NoProxyTrust_UsesRemoteAddr(t *testing.T) {
	s := newClientIPServer(0)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "203.0.113.42:54321"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	assert.Equal(t, "203.0.113.42", s.clientIP(r))
}

func TestClientIP_Depth1_TakesRightmostXFF(t *testing.T) {
	s := newClientIPServer(1)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "5.5.5.5, 8.8.8.8")
	assert.Equal(t, "8.8.8.8", s.clientIP(r))
}

func TestClientIP_Depth2_TakesSecondFromRight(t *testing.T) {
	s := newClientIPServer(2)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Forwarded-For", "client, gclb, gfe")
	assert.Equal(t, "gclb", s.clientIP(r))
}

func TestClientIP_DepthExceedsXFF_FallsBackToRemoteAddr(t *testing.T) {
	s := newClientIPServer(5)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	assert.Equal(t, "10.0.0.1", s.clientIP(r))
}

func TestClientIP_MissingXFF_UsesRemoteAddr(t *testing.T) {
	s := newClientIPServer(1)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "10.0.0.1:1234"
	assert.Equal(t, "10.0.0.1", s.clientIP(r))
}

func TestSanitizeMessageText_PreservesNormal(t *testing.T) {
	out, ok := sanitizeMessageText("hello world!")
	assert.True(t, ok)
	assert.Equal(t, "hello world!", out)
}

func TestSanitizeMessageText_KeepsWhitespace(t *testing.T) {
	out, ok := sanitizeMessageText("line one\nline two\tand a tab")
	assert.True(t, ok)
	assert.Equal(t, "line one\nline two\tand a tab", out)
}

func TestSanitizeMessageText_StripsControlChars(t *testing.T) {
	// U+0001 START OF HEADING, U+007F DEL — both should be stripped.
	out, ok := sanitizeMessageText("hi\x01there\x7f")
	assert.True(t, ok)
	assert.Equal(t, "hithere", out)
}

func TestSanitizeMessageText_RejectsBidiOverride(t *testing.T) {
	// U+202E RIGHT-TO-LEFT OVERRIDE is the classic spoof codepoint.
	_, ok := sanitizeMessageText("user‮gnp.exe")
	assert.False(t, ok)
}

func TestSanitizeMessageText_RejectsInvalidUTF8(t *testing.T) {
	// Standalone continuation byte: not valid UTF-8.
	_, ok := sanitizeMessageText("hi\x80there")
	assert.False(t, ok)
}

func TestSanitizeMessageText_TrimsWhitespace(t *testing.T) {
	out, ok := sanitizeMessageText("   hello   ")
	assert.True(t, ok)
	assert.Equal(t, "hello", out)
}
