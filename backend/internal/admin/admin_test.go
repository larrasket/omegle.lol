package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

type fakeRooms struct{ n int }

func (f fakeRooms) RoomCount() int { return f.n }

func newAdminServer(password string) (*Server, *match.Memory, *session.Registry) {
	matcher := match.NewMemory(8 * time.Second)
	sessions := session.NewRegistry()
	srv := &Server{
		Username:  "admin",
		Password:  password,
		Sessions:  sessions,
		Matcher:   matcher,
		Rooms:     fakeRooms{n: 3},
		StartedAt: time.Now().Add(-90 * time.Second),
	}
	return srv, matcher, sessions
}

func TestStatsHandlerRejectsWithoutAuth(t *testing.T) {
	srv, matcher, _ := newAdminServer("hunter2")
	defer matcher.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/stats", http.NoBody)
	srv.StatsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatalf("expected WWW-Authenticate header, got empty")
	}
}

func TestStatsHandlerRejectsWrongPassword(t *testing.T) {
	srv, matcher, _ := newAdminServer("hunter2")
	defer matcher.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/stats", http.NoBody)
	req.SetBasicAuth("admin", "wrong")
	srv.StatsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong password, got %d", rec.Code)
	}
}

func TestStatsHandlerBlocksWhenPasswordUnset(t *testing.T) {
	srv, matcher, _ := newAdminServer("")
	defer matcher.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/stats", http.NoBody)
	req.SetBasicAuth("admin", "anything")
	srv.StatsHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unset password must still 401, got %d", rec.Code)
	}
}

func TestStatsHandlerReturnsSnapshot(t *testing.T) {
	srv, matcher, sessions := newAdminServer("hunter2")
	defer matcher.Close()
	// Two sessions in the registry, one of them enqueued with two tags so the
	// histogram has predictable contents.
	s1 := session.New()
	s2 := session.New()
	sessions.Add(s1)
	sessions.Add(s2)
	_, cancel := matcher.Enqueue(&match.Candidate{Session: s1, Tags: []string{"music", "books"}})
	defer cancel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/stats", http.NoBody)
	req.SetBasicAuth("admin", "hunter2")
	srv.StatsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ActiveConnections != 2 {
		t.Errorf("active_connections: want 2, got %d", body.ActiveConnections)
	}
	if body.ActiveRooms != 3 || body.PairedUsers != 6 {
		t.Errorf("room counts wrong: rooms=%d paired=%d", body.ActiveRooms, body.PairedUsers)
	}
	if body.Searching != 1 {
		t.Errorf("searching: want 1, got %d", body.Searching)
	}
	if body.UptimeSeconds < 60 {
		t.Errorf("uptime should be ~90s, got %d", body.UptimeSeconds)
	}
	gotTags := map[string]int{}
	for _, tc := range body.TopTags {
		gotTags[tc.Tag] = tc.Count
	}
	if gotTags["music"] != 1 || gotTags["books"] != 1 {
		t.Errorf("top_tags wrong: %+v", body.TopTags)
	}
}

func TestSnapshotRanksTagsByCount(t *testing.T) {
	matcher := match.NewMemory(8 * time.Second)
	defer matcher.Close()
	// Three searchers all tag "popular"; one of them also tags "rare".
	cancels := make([]func(), 0, 3)
	for range 3 {
		s := session.New()
		_, cancel := matcher.Enqueue(&match.Candidate{Session: s, Tags: []string{"popular"}})
		cancels = append(cancels, cancel)
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()
	rare := session.New()
	_, cancel := matcher.Enqueue(&match.Candidate{Session: rare, Tags: []string{"popular", "rare"}})
	defer cancel()

	snap := matcher.Snapshot(10)
	if snap.Searching != 4 {
		t.Fatalf("searching: want 4, got %d", snap.Searching)
	}
	if len(snap.TopTags) < 2 || snap.TopTags[0].Tag != "popular" || snap.TopTags[0].Count != 4 {
		t.Fatalf("popular tag should rank first with count 4, got %+v", snap.TopTags)
	}
	if snap.TopTags[1].Tag != "rare" || snap.TopTags[1].Count != 1 {
		t.Fatalf("rare tag should rank second with count 1, got %+v", snap.TopTags)
	}
}
