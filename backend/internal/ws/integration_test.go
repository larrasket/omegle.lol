package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cws "github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omegle-lol/omegle/backend/internal/config"
	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/proto"
	"github.com/omegle-lol/omegle/backend/internal/session"
	"github.com/omegle-lol/omegle/backend/internal/ws"
)

func newTestServer(t *testing.T) (ts *httptest.Server, teardown func()) {
	t.Helper()
	cfg := config.Config{
		MatchTimeout:      500 * time.Millisecond,
		MaxTags:           10,
		MaxTagLen:         30,
		MaxMsgBytes:       2048,
		MsgRatePerSec:     100,
		AllowedOrigins:    []string{"*"},
		HeartbeatInterval: 60 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
	}
	matcher := match.NewMemory(cfg.MatchTimeout)
	shadowban := ws.NewShadowban()
	srv := ws.NewServer(cfg, session.NewRegistry(), matcher, shadowban)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", srv.HandleUpgrade)
	ts = httptest.NewServer(mux)
	return ts, func() {
		ts.Close()
		matcher.Close()
		shadowban.Close()
	}
}

func dial(t *testing.T, ts *httptest.Server) *cws.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	c, resp, err := cws.Dial(context.Background(), url, nil)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	return c
}

func send(t *testing.T, c *cws.Conn, msgType string, data any) {
	t.Helper()
	raw, err := proto.Encode(msgType, data)
	require.NoError(t, err)
	require.NoError(t, c.Write(context.Background(), cws.MessageText, raw))
}

func recv(t *testing.T, c *cws.Conn, timeout time.Duration) proto.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, raw, err := c.Read(ctx)
	require.NoError(t, err)
	var env proto.Envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	return env
}

// recvUntil reads messages until one with the given type arrives or timeout fires.
func recvUntil(t *testing.T, c *cws.Conn, wantType string, timeout time.Duration) proto.Envelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		env := recv(t, c, time.Until(deadline))
		if env.Type == wantType {
			return env
		}
	}
	t.Fatalf("never received %s", wantType)
	return proto.Envelope{}
}

func TestTwoClients_MatchAndChat(t *testing.T) {
	ts, done := newTestServer(t)
	defer done()

	a := dial(t, ts)
	b := dial(t, ts)
	defer a.CloseNow()
	defer b.CloseNow()

	// Consume welcome.
	_ = recvUntil(t, a, proto.MsgWelcome, time.Second)
	_ = recvUntil(t, b, proto.MsgWelcome, time.Second)

	send(t, a, proto.MsgSearch, proto.SearchData{Tags: []string{"tech"}})
	send(t, b, proto.MsgSearch, proto.SearchData{Tags: []string{"tech"}})

	// Both get matched.
	envA := recvUntil(t, a, proto.MsgMatched, 2*time.Second)
	envB := recvUntil(t, b, proto.MsgMatched, 2*time.Second)
	var md proto.MatchedData
	require.NoError(t, json.Unmarshal(envA.Data, &md))
	assert.Equal(t, []string{"tech"}, md.SharedTags)
	require.NoError(t, json.Unmarshal(envB.Data, &md))
	assert.Equal(t, []string{"tech"}, md.SharedTags)

	// A sends a message, B receives it.
	send(t, a, proto.MsgMessage, proto.MessageData{Text: "hello"})
	env := recvUntil(t, b, proto.MsgPeerMsg, 2*time.Second)
	var pm proto.PeerMsgData
	require.NoError(t, json.Unmarshal(env.Data, &pm))
	assert.Equal(t, "hello", pm.Text)

	// B stops.
	send(t, b, proto.MsgStop, nil)
	env = recvUntil(t, a, proto.MsgPeerLeft, 2*time.Second)
	var pl proto.PeerLeftData
	require.NoError(t, json.Unmarshal(env.Data, &pl))
	assert.Equal(t, "stop", pl.Reason)
}

func TestSearch_TagValidationRejects(t *testing.T) {
	ts, done := newTestServer(t)
	defer done()
	a := dial(t, ts)
	defer a.CloseNow()
	_ = recvUntil(t, a, proto.MsgWelcome, time.Second)

	send(t, a, proto.MsgSearch, proto.SearchData{Tags: []string{"hello!world"}})
	env := recvUntil(t, a, proto.MsgError, time.Second)
	var e proto.ErrorData
	require.NoError(t, json.Unmarshal(env.Data, &e))
	assert.Equal(t, proto.ErrInvalidTag, e.Code)
}
