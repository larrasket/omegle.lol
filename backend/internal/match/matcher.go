package match

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/omegle-lol/omegle/backend/internal/session"
)

// Candidate is what callers hand to the matcher: a session and the tags they want to match on.
type Candidate struct {
	Session    *session.Session
	Tags       []string
	EnqueuedAt time.Time
}

// Room is the result of a successful match.
type Room struct {
	ID         string
	A, B       *session.Session
	SharedTags []string
	CreatedAt  time.Time
}

// Matcher pairs candidates into rooms.
//
// Enqueue returns a result channel that delivers exactly one Room (then closes) once a
// match is made, and a cancel func to remove the candidate from the queue. After cancel,
// the channel is closed without a Room.
type Matcher interface {
	Enqueue(c *Candidate) (room <-chan *Room, cancel func())
}

func newRoomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
