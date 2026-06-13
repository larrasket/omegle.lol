package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/omegle-lol/omegle/backend/internal/chat"
	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/metrics"
	"github.com/omegle-lol/omegle/backend/internal/proto"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

type connState int

const (
	stateIdle connState = iota
	stateSearching
	stateChatting
)

// dispatchState holds per-connection state for the search/chat state machine.
// One per *connection, allocated in newConnection.
type dispatchState struct {
	mu             sync.Mutex
	state          connState
	cancelSearch   func()
	enqueuedAt     time.Time
	tagsHadOverlap bool
}

func (c *connection) handle(env proto.Envelope) {
	switch env.Type {
	case proto.MsgSearch:
		var d proto.SearchData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			c.sendError(proto.ErrInvalidJSON, err.Error())
			return
		}
		c.handleSearch(d.Tags)
	case proto.MsgCancel:
		c.handleCancel()
	case proto.MsgMessage:
		var d proto.MessageData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			c.sendError(proto.ErrInvalidJSON, err.Error())
			return
		}
		c.handleMessage(d.Text)
	case proto.MsgTyping:
		var d proto.TypingData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			c.sendError(proto.ErrInvalidJSON, err.Error())
			return
		}
		c.handleTyping(d.Active)
	case proto.MsgNext:
		var d proto.NextData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			c.sendError(proto.ErrInvalidJSON, err.Error())
			return
		}
		c.handleNext(d.Tags)
	case proto.MsgStop:
		c.handleStop()
	case proto.MsgReport:
		c.handleReport()
	case proto.MsgPause:
		c.handlePresence(true)
	case proto.MsgResume:
		c.handlePresence(false)
	case proto.MsgPong:
		// liveness already updated by readPump
	default:
		c.sendError(proto.ErrInvalidType, "unknown message type: "+env.Type)
	}
}

// handlePresence flips the connection's paused flag and — when the client
// is currently in a chat — forwards the change to the partner so their UI
// can show "Stranger is away…" or clear it. Idempotent: sending pause
// twice or resume without a prior pause is harmless.
func (c *connection) handlePresence(paused bool) {
	prev := c.paused.Swap(paused)
	if prev == paused {
		return
	}
	ds := c.dispatch
	ds.mu.Lock()
	inChat := ds.state == stateChatting
	ds.mu.Unlock()
	if !inChat {
		return
	}
	envType := proto.MsgPeerResumed
	if paused {
		envType = proto.MsgPeerPaused
	}
	if room, ok := c.srv.getRoom(c.sess.ID); ok {
		_ = room.RelayPresence(c.sess.ID, envType)
	}
}

func (c *connection) handleSearch(rawTags []string) {
	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateIdle {
		ds.mu.Unlock()
		c.sendError(proto.ErrInvalidState, "already in queue or chat")
		return
	}

	tags, err := match.NormalizeTags(rawTags, c.srv.Cfg.MaxTags, c.srv.Cfg.MaxTagLen)
	if err != nil {
		ds.mu.Unlock()
		c.sendError(mapTagErr(err), err.Error())
		return
	}

	// Shadowbanned IPs are accepted into "searching" but never enqueued, so
	// the UI shows the usual looking-for-someone state forever. Never tip
	// the user off — abuse mitigation only works if the abuser thinks the
	// service is just dead.
	if c.srv.Shadowban != nil && c.srv.Shadowban.IsBanned(c.sess.IP) {
		ds.state = stateSearching
		ds.mu.Unlock()
		if raw, err := proto.Encode(proto.MsgSearching, nil); err == nil {
			select {
			case c.sess.Send <- raw:
			default:
			}
		}
		return
	}

	cand := &match.Candidate{Session: c.sess, Tags: tags, EnqueuedAt: time.Now()}
	roomCh, cancel := c.srv.Matcher.Enqueue(cand)
	ds.state = stateSearching
	ds.cancelSearch = cancel
	ds.enqueuedAt = cand.EnqueuedAt
	ds.tagsHadOverlap = len(tags) > 0
	ds.mu.Unlock()

	metrics.QueueDepth.Inc()
	if raw, err := proto.Encode(proto.MsgSearching, nil); err == nil {
		select {
		case c.sess.Send <- raw:
		default:
		}
	}

	go c.awaitMatch(roomCh)
}

func (c *connection) awaitMatch(roomCh <-chan *match.Room) {
	room, ok := <-roomCh
	metrics.QueueDepth.Dec()
	if !ok {
		// Cancelled or matcher closed.
		return
	}

	// If the WS dropped between the matcher pairing us and this goroutine
	// waking up, the session was already removed from the registry. Tear
	// down the room so the partner sees peer_left instead of being stuck
	// in a chat with a ghost.
	if _, alive := c.srv.Sessions.Get(c.sess.ID); !alive {
		cr := chat.NewRoom(room.ID, room.A, room.B, room.SharedTags)
		c.srv.registerRoomIfNew(cr)
		c.srv.endRoomIfAny(c.sess.ID, "disconnect")
		return
	}

	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateSearching {
		// User cancelled (or stopped) between match send and our read.
		// Register the room briefly so endRoomIfAny can find it, then end it
		// as a disconnect for the peer.
		ds.mu.Unlock()
		cr := chat.NewRoom(room.ID, room.A, room.B, room.SharedTags)
		c.srv.registerRoomIfNew(cr)
		c.srv.endRoomIfAny(c.sess.ID, "disconnect")
		return
	}
	ds.state = stateChatting
	ds.cancelSearch = nil
	wait := time.Since(ds.enqueuedAt)
	hadOverlap := ds.tagsHadOverlap && len(room.SharedTags) > 0
	ds.mu.Unlock()

	metrics.MatchWaitMs.Observe(float64(wait.Milliseconds()))
	if hadOverlap {
		metrics.MatchesTotal.WithLabelValues("tagged").Inc()
	} else {
		metrics.MatchesTotal.WithLabelValues("fallback").Inc()
	}

	// Only the side that performs the pairing creates the chat.Room object.
	// We use room.ID as the dedupe key: registerRoomIfNew is idempotent.
	cr := chat.NewRoom(room.ID, room.A, room.B, room.SharedTags)
	c.srv.registerRoomIfNew(cr)

	// matched is the control-plane signal that the chat has started. Drop it
	// and the UI sits on "Looking for someone…" forever. Use sendCritical
	// instead of a silent default-drop so we either deliver or kill the
	// connection.
	if raw, err := proto.Encode(proto.MsgMatched, proto.MatchedData{SharedTags: room.SharedTags}); err == nil {
		c.sendCritical(context.Background(), raw)
	}
	slog.Info("matched", "session_id", c.sess.ID[:8], "room_id", room.ID, "wait_ms", wait.Milliseconds(), "shared_tags", len(room.SharedTags))
}

func (c *connection) handleCancel() {
	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateSearching || ds.cancelSearch == nil {
		ds.mu.Unlock()
		c.sendError(proto.ErrInvalidState, "not searching")
		return
	}
	cancel := ds.cancelSearch
	ds.cancelSearch = nil
	ds.state = stateIdle
	ds.mu.Unlock()
	cancel()
}

func (c *connection) handleMessage(text string) {
	if !c.limiter.Allow() {
		c.sendError(proto.ErrRateLimited, "too many messages")
		return
	}
	if len(text) > c.srv.Cfg.MaxMsgBytes {
		c.sendError(proto.ErrMessageTooLarge, "message too large")
		return
	}
	clean, ok := sanitizeMessageText(text)
	if !ok {
		c.sendError(proto.ErrInvalidJSON, "message contained invalid characters")
		return
	}
	if clean == "" {
		// All-whitespace or fully-stripped — silently drop, no error.
		return
	}
	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateChatting {
		ds.mu.Unlock()
		c.sendError(proto.ErrInvalidState, "not in chat")
		return
	}
	ds.mu.Unlock()

	room, ok := c.srv.getRoom(c.sess.ID)
	if !ok {
		c.sendError(proto.ErrInvalidState, "no active room")
		return
	}
	if err := room.RelayMessage(c.sess.ID, clean); err != nil {
		c.sendError(proto.ErrInvalidState, err.Error())
		return
	}
	metrics.MessagesTotal.Inc()
}

func (c *connection) handleTyping(active bool) {
	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateChatting {
		ds.mu.Unlock()
		return // silently ignore
	}
	ds.mu.Unlock()
	if room, ok := c.srv.getRoom(c.sess.ID); ok {
		_ = room.RelayTyping(c.sess.ID, active)
	}
}

func (c *connection) handleStop() {
	c.srv.endRoomIfAny(c.sess.ID, "stop")
	ds := c.dispatch
	ds.mu.Lock()
	ds.state = stateIdle
	ds.mu.Unlock()
}

// handleReport records a moderation report against the current chat
// partner's IP and ends the chat for the reporter. The reportee gets a
// normal peer_left envelope ("stop" reason) — they don't learn they've
// been reported, which is by design: an abuser who knows they were
// flagged can rotate connections; one who doesn't keeps using the same
// IP until the shadowban kicks in.
func (c *connection) handleReport() {
	ds := c.dispatch
	ds.mu.Lock()
	if ds.state != stateChatting {
		ds.mu.Unlock()
		c.sendError(proto.ErrInvalidState, "not in chat")
		return
	}
	ds.mu.Unlock()

	room, ok := c.srv.getRoom(c.sess.ID)
	if !ok {
		c.sendError(proto.ErrInvalidState, "no active room")
		return
	}

	// Identify the partner. Members() returns the pair in registration
	// order; the partner is whichever one isn't us.
	a, b := room.Members()
	var partner *session.Session
	switch c.sess.ID {
	case a.ID:
		partner = b
	case b.ID:
		partner = a
	default:
		// Shouldn't happen — sessionID is in sessionToRoom so it must
		// be a member of the room. Defensive bail.
		return
	}

	if c.srv.Shadowban != nil && partner.IP != "" {
		nowBanned := c.srv.Shadowban.Report(partner.IP)
		slog.Info("session reported",
			"reporter", c.sess.ID[:8],
			"reportee", partner.ID[:8],
			"now_banned", nowBanned,
		)
	}

	// End the chat for the reporter. "stop" reason → partner gets
	// peer_left, reporter does not (their UI already transitioned
	// optimistically).
	c.srv.endRoomIfAny(c.sess.ID, "stop")
	ds.mu.Lock()
	ds.state = stateIdle
	ds.mu.Unlock()
}

func (c *connection) handleNext(rawTags []string) {
	c.srv.endRoomIfAny(c.sess.ID, "next")
	ds := c.dispatch
	ds.mu.Lock()
	ds.state = stateIdle
	ds.mu.Unlock()
	c.handleSearch(rawTags)
}

// cancelPendingSearch is called from HandleUpgrade's defer on WS disconnect.
// If the connection was in the matcher queue at the moment the socket dropped,
// this drops the entry and closes its pending channel — preventing future
// searchers from being paired with this dead session.
func cancelPendingSearch(c *connection) {
	ds := c.dispatch
	ds.mu.Lock()
	cancel := ds.cancelSearch
	ds.cancelSearch = nil
	ds.state = stateIdle
	ds.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func mapTagErr(err error) string {
	switch {
	case errors.Is(err, match.ErrTooManyTags):
		return proto.ErrTooManyTags
	case errors.Is(err, match.ErrTagTooLong):
		return proto.ErrTagTooLong
	case errors.Is(err, match.ErrInvalidTag):
		return proto.ErrInvalidTag
	default:
		return proto.ErrInvalidState
	}
}

// sanitizeMessageText cleans up incoming chat text:
//   - rejects invalid UTF-8 (returns ok=false)
//   - rejects bidi override / isolate codepoints, used in spoof attacks
//   - strips C0 control characters except tab/CR/LF
//   - trims leading and trailing whitespace
//
// Returns the cleaned string and ok=false only when the input itself is
// malformed (invalid UTF-8 or contains an explicit attack codepoint).
func sanitizeMessageText(s string) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		// Bidi override / isolate codepoints — used to disguise text direction.
		case '‪', '‫', '‬', '‭', '‮',
			'⁦', '⁧', '⁨', '⁩':
			return "", false
		// Allow common whitespace.
		case '\t', '\n', '\r':
			b.WriteRune(r)
			continue
		}
		// Strip remaining C0 (U+0000-U+001F) and DEL (U+007F).
		if r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String()), true
}
