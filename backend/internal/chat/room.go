package chat

import (
	"errors"
	"sync"
	"time"

	"github.com/omegle-lol/omegle/backend/internal/proto"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

var ErrNotMember = errors.New("sender is not a member of this room")

// Room holds the two members of a chat. Methods are safe for concurrent use.
type Room struct {
	ID         string
	SharedTags []string
	createdAt  time.Time

	mu     sync.Mutex
	a, b   *session.Session
	closed bool
}

func NewRoom(id string, a, b *session.Session, sharedTags []string) *Room {
	return &Room{
		ID:         id,
		SharedTags: sharedTags,
		createdAt:  time.Now(),
		a:          a,
		b:          b,
	}
}

func (r *Room) Members() (a, b *session.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.a, r.b
}

func (r *Room) peer(senderID string) (*session.Session, error) {
	switch senderID {
	case r.a.ID:
		return r.b, nil
	case r.b.ID:
		return r.a, nil
	default:
		return nil, ErrNotMember
	}
}

// RelayMessage sends a chat message from senderID to the other member.
func (r *Room) RelayMessage(senderID, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	peer, err := r.peer(senderID)
	if err != nil {
		return err
	}
	raw, err := proto.Encode(proto.MsgPeerMsg, proto.PeerMsgData{Text: text, Ts: time.Now().UnixMilli()})
	if err != nil {
		return err
	}
	trySend(peer, raw)
	return nil
}

// RelayTyping forwards a typing indicator to the other member.
func (r *Room) RelayTyping(senderID string, active bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	peer, err := r.peer(senderID)
	if err != nil {
		return err
	}
	raw, err := proto.Encode(proto.MsgPeerTyping, proto.PeerTypingData{Active: active})
	if err != nil {
		return err
	}
	trySend(peer, raw)
	return nil
}

// RelayPresence forwards a pause/resume signal to the other member so their
// UI can show "Stranger is away…" while the peer's tab is backgrounded.
// envType is proto.MsgPeerPaused or proto.MsgPeerResumed.
func (r *Room) RelayPresence(senderID, envType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	peer, err := r.peer(senderID)
	if err != nil {
		return err
	}
	raw, err := proto.Encode(envType, nil)
	if err != nil {
		return err
	}
	trySend(peer, raw)
	return nil
}

// Close marks the room closed and sends peer_left to BOTH members with the given reason.
// Callers may want to send peer_left to only the non-initiating side; this method
// fans out to both because it's used for disconnect/shutdown paths. For stop/next,
// dispatch sends the message to just the peer and then calls Close with a no-fanout
// variant if needed — for MVP, simpler to fan out and let the initiator ignore extras.
func (r *Room) Close(reason string) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()

	raw, err := proto.Encode(proto.MsgPeerLeft, proto.PeerLeftData{Reason: reason})
	if err != nil {
		return
	}
	trySend(r.a, raw)
	trySend(r.b, raw)
}

// ClosePeerOnly sends peer_left only to the non-initiator (used for stop/next so the
// initiator who *chose* to leave doesn't get a "peer left" message about themselves).
func (r *Room) ClosePeerOnly(initiatorID, reason string) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	peer, err := r.peer(initiatorID)
	r.mu.Unlock()
	if err != nil {
		return
	}
	raw, err := proto.Encode(proto.MsgPeerLeft, proto.PeerLeftData{Reason: reason})
	if err != nil {
		return
	}
	trySend(peer, raw)
}

// trySend writes to the session's Send channel, dropping the message if the buffer is full
// (the connection writer is presumably stuck or dead; the connection layer will close it).
func trySend(s *session.Session, raw []byte) {
	select {
	case s.Send <- raw:
	default:
	}
}
