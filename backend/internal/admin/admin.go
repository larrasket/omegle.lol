// Package admin serves the in-process operator dashboard. There's exactly
// one endpoint — /admin/stats — returning a JSON snapshot of the matcher
// and connection state. It's intentionally pull-based (no streaming) so
// the page can be a single static SPA route that polls.
//
// Auth is HTTP Basic. Without ADMIN_PASSWORD set, every request is 401 —
// the dashboard is offline by default. Username defaults to "admin".
package admin

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

// TopTagsCap bounds the tag histogram so a server with a wide tag
// distribution can't blow up the response size or expose a long tail of
// niche tags. 20 is a round number — plenty for a dashboard.
const TopTagsCap = 20

// roomCounter is the slice of ws.Server we need without taking a hard
// import dependency on the ws package (which would cycle through chat ←
// session). The handler builds its own counter at construction time.
type roomCounter interface {
	RoomCount() int
}

// Server bundles the live state the dashboard reads from.
type Server struct {
	Username  string
	Password  string
	Sessions  *session.Registry
	Matcher   *match.Memory
	Rooms     roomCounter
	StartedAt time.Time
}

type statsResponse struct {
	ActiveConnections int              `json:"active_connections"`
	ActiveRooms       int              `json:"active_rooms"`
	PairedUsers       int              `json:"paired_users"`
	Searching         int              `json:"searching"`
	TopTags           []match.TagCount `json:"top_tags"`
	UptimeSeconds     int64            `json:"uptime_seconds"`
	StartedAt         string           `json:"started_at"`
	ServerTime        string           `json:"server_time"`
}

// StatsHandler is mounted at /admin/stats. Returns 401 when auth fails so
// browsers can pop the native credential dialog on first visit.
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="omegle-admin", charset="UTF-8"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	snap := s.Matcher.Snapshot(TopTagsCap)
	rooms := s.Rooms.RoomCount()
	body := statsResponse{
		ActiveConnections: s.Sessions.Count(),
		ActiveRooms:       rooms,
		PairedUsers:       rooms * 2,
		Searching:         snap.Searching,
		TopTags:           snap.TopTags,
		UptimeSeconds:     int64(time.Since(s.StartedAt).Seconds()),
		StartedAt:         s.StartedAt.UTC().Format(time.RFC3339),
		ServerTime:        time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(body)
}

// authorized returns true only when both username and password match.
// Constant-time compare keeps response time from leaking which half was
// wrong. A blank configured password always fails — the dashboard stays
// off when the operator hasn't set the secret yet.
func (s *Server) authorized(r *http.Request) bool {
	if s.Password == "" {
		return false
	}
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.Username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.Password)) == 1
	return userOK && passOK
}
