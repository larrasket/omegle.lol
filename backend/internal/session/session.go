package session

import (
	"crypto/rand"
	"encoding/hex"
)

// Session is the per-connection state. Held by both the WS layer and the matcher/chat layers.
type Session struct {
	ID   string
	IP   string      // client IP captured at WS upgrade; used by moderation (shadowban) when this session is reported as a partner
	Send chan []byte // outgoing bytes to the WS writer goroutine; buffered
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand should never fail
	}
	return hex.EncodeToString(b)
}

// New creates a fresh session with a random 128-bit ID and a 32-message send buffer.
// IP starts empty; callers (the WS handler) fill it after capturing the request's client IP.
func New() *Session {
	return &Session{
		ID:   newID(),
		Send: make(chan []byte, 32),
	}
}
