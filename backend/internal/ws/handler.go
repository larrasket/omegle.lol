package ws

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	cws "github.com/coder/websocket"

	"github.com/omegle-lol/omegle/backend/internal/chat"
	"github.com/omegle-lol/omegle/backend/internal/config"
	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/metrics"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

// Server holds shared deps for the WS handler.
type Server struct {
	Cfg       config.Config
	Sessions  *session.Registry
	Matcher   match.Matcher
	Shadowban *Shadowban

	// Rooms is the map of active rooms keyed by ID. Members track room ID per session.
	roomsMu       sync.Mutex
	rooms         map[string]*chat.Room
	sessionToRoom map[string]string // sessionID -> roomID

	// connectionRate limits per-IP WS upgrade attempts.
	connectRateMu sync.Mutex
	connectRate   map[string]*TokenBucket // IP -> bucket

	// activeConns tracks every live connection so the server can force-close
	// them all at shutdown time, keeping the SIGTERM-to-exit budget tight.
	activeConnsMu sync.Mutex
	activeConns   map[*connection]struct{}
}

func NewServer(cfg config.Config, sessions *session.Registry, matcher match.Matcher, shadowban *Shadowban) *Server {
	return &Server{
		Cfg:           cfg,
		Sessions:      sessions,
		Matcher:       matcher,
		Shadowban:     shadowban,
		rooms:         make(map[string]*chat.Room),
		sessionToRoom: make(map[string]string),
		connectRate:   make(map[string]*TokenBucket),
		activeConns:   make(map[*connection]struct{}),
	}
}

// HandleUpgrade is registered at /ws.
func (s *Server) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	if !s.allowConnect(s.clientIP(r)) {
		http.Error(w, "connect_rate_limited", http.StatusTooManyRequests)
		return
	}
	c, err := cws.Accept(w, r, &cws.AcceptOptions{
		OriginPatterns: s.Cfg.AllowedOrigins,
	})
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}

	sess := session.New()
	// Capture the client IP at upgrade time so dispatch can hand it to
	// the shadowban store when this session is reported as a partner.
	sess.IP = s.clientIP(r)
	s.Sessions.Add(sess)
	metrics.ConnectionsActive.Inc()
	slog.Info("ws connected", "session_id", sess.ID[:8])

	conn := newConnection(c, sess, s)
	s.registerConn(conn)

	defer func() {
		// Mark session dead FIRST so any awaitMatch goroutine that wakes up
		// from a racing match delivery can see that this side is gone.
		s.Sessions.Remove(sess.ID)
		metrics.ConnectionsActive.Dec()

		// If the user was sitting in the matcher queue when the WS dropped,
		// cancel the pending entry so future searchers don't get paired with
		// this dead session.
		cancelPendingSearch(conn)

		s.endRoomIfAny(sess.ID, "disconnect")
		_ = c.CloseNow()
		s.unregisterConn(conn)
		slog.Info("ws disconnected", "session_id", sess.ID[:8])
	}()

	conn.run(r.Context())
}

func (s *Server) registerConn(c *connection) {
	s.activeConnsMu.Lock()
	s.activeConns[c] = struct{}{}
	s.activeConnsMu.Unlock()
}

func (s *Server) unregisterConn(c *connection) {
	s.activeConnsMu.Lock()
	delete(s.activeConns, c)
	s.activeConnsMu.Unlock()
}

// CloseAllConnections force-closes every live WS. Used at shutdown so
// srv.Shutdown doesn't have to wait the full deadline for clients to leave on
// their own.
func (s *Server) CloseAllConnections() {
	s.activeConnsMu.Lock()
	conns := make([]*connection, 0, len(s.activeConns))
	for c := range s.activeConns {
		conns = append(conns, c)
	}
	s.activeConnsMu.Unlock()
	for _, c := range conns {
		_ = c.ws.CloseNow()
	}
}

func (s *Server) allowConnect(ip string) bool {
	s.connectRateMu.Lock()
	defer s.connectRateMu.Unlock()
	bucket, ok := s.connectRate[ip]
	if !ok {
		bucket = NewTokenBucket(1, 30) // 1/sec sustained, burst 30 → ~30/min
		s.connectRate[ip] = bucket
	}
	return bucket.Allow()
}

// clientIP returns the IP we should rate-limit on. When TrustedProxyDepth is 0
// (default), X-Forwarded-For is ignored entirely and we use r.RemoteAddr — safe
// but assumes no proxy. With N>0, the Nth-from-right XFF entry is treated as
// the client; entries further right are assumed to be added by trusted infra
// (Cloud Run = 1, Cloud LB + Cloud Run = 2). Misconfiguration just makes the
// limiter slightly looser — never tighter than r.RemoteAddr alone.
func (s *Server) clientIP(r *http.Request) string {
	depth := s.Cfg.TrustedProxyDepth
	if depth <= 0 {
		return hostFromAddr(r.RemoteAddr)
	}
	h := r.Header.Get("X-Forwarded-For")
	if h == "" {
		return hostFromAddr(r.RemoteAddr)
	}
	parts := strings.Split(h, ",")
	// We want the entry at index len(parts) - depth: with depth=1 that's the
	// rightmost element, with depth=2 the second-from-right, etc.
	idx := len(parts) - depth
	if idx < 0 || idx >= len(parts) {
		return hostFromAddr(r.RemoteAddr)
	}
	return strings.TrimSpace(parts[idx])
}

// hostFromAddr strips an optional port from a "host:port" address.
func hostFromAddr(addr string) string {
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// --- Room registry helpers (used by dispatch.go) ---

func (s *Server) registerRoomIfNew(r *chat.Room) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()
	if _, exists := s.rooms[r.ID]; exists {
		return
	}
	s.rooms[r.ID] = r
	a, b := r.Members()
	s.sessionToRoom[a.ID] = r.ID
	s.sessionToRoom[b.ID] = r.ID
}

func (s *Server) getRoom(sessionID string) (*chat.Room, bool) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()
	roomID, ok := s.sessionToRoom[sessionID]
	if !ok {
		return nil, false
	}
	r, ok := s.rooms[roomID]
	return r, ok
}

func (s *Server) endRoomIfAny(sessionID, reason string) {
	s.roomsMu.Lock()
	roomID, ok := s.sessionToRoom[sessionID]
	if !ok {
		s.roomsMu.Unlock()
		return
	}
	r := s.rooms[roomID]
	delete(s.rooms, roomID)
	a, b := r.Members()
	delete(s.sessionToRoom, a.ID)
	delete(s.sessionToRoom, b.ID)
	s.roomsMu.Unlock()

	if reason == "stop" || reason == "next" {
		r.ClosePeerOnly(sessionID, reason)
	} else {
		r.Close(reason)
	}
}

// RoomCount returns the number of active rooms. Cheap, lock-protected — safe
// to call from monitoring/admin paths.
func (s *Server) RoomCount() int {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()
	return len(s.rooms)
}

// CloseAllRooms ends all active rooms with the given reason. Sends peer_left to both
// members of each room. Intended for graceful shutdown — call this before shutting down
// the HTTP server so writer pumps can flush the peer_left envelopes.
func (s *Server) CloseAllRooms(reason string) {
	s.roomsMu.Lock()
	rooms := make([]*chat.Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		rooms = append(rooms, r)
	}
	s.rooms = make(map[string]*chat.Room)
	s.sessionToRoom = make(map[string]string)
	s.roomsMu.Unlock()

	for _, r := range rooms {
		r.Close(reason)
	}
}

// heartbeatTimings returns the heartbeat interval/timeout from config for convenience.
func (s *Server) heartbeatTimings() (every, timeout time.Duration) {
	return s.Cfg.HeartbeatInterval, s.Cfg.HeartbeatTimeout
}
