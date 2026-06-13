// Package proto defines the WebSocket wire format.
//
// IMPORTANT: keep frontend/src/lib/proto.ts in sync with this file.
package proto

import "encoding/json"

// Envelope wraps every message in both directions.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Client -> Server message types.
const (
	MsgSearch  = "search"
	MsgCancel  = "cancel"
	MsgMessage = "message"
	MsgTyping  = "typing"
	MsgNext    = "next"
	MsgStop    = "stop"
	MsgPong    = "pong"
	// Pause / Resume let a mobile client tell the server "I'm backgrounded,
	// don't kick me on heartbeat timeout for a while". The server uses an
	// extended grace window (config.PauseGrace) until the matching Resume
	// arrives or the grace expires. Forwarded to the chat partner so they
	// see "Stranger is away…" instead of guessing why messages stopped.
	MsgPause  = "pause"
	MsgResume = "resume"
	// Report flags the current chat partner for abuse. The server records
	// the partner's IP in the shadowban store; if the IP collects enough
	// reports, future matches silently fail. The chat ends for the
	// reporter; the reportee gets a normal peer_left (no signal that
	// they've been reported).
	MsgReport = "report"
)

// Server -> Client message types.
const (
	MsgWelcome     = "welcome"
	MsgSearching   = "searching"
	MsgMatched     = "matched"
	MsgPeerMsg     = "peer_msg"
	MsgPeerTyping  = "peer_typing"
	MsgPeerLeft    = "peer_left"
	MsgPeerPaused  = "peer_paused"
	MsgPeerResumed = "peer_resumed"
	MsgError       = "error"
	MsgPing        = "ping"
)

type SearchData struct {
	Tags []string `json:"tags"`
}

type MessageData struct {
	Text string `json:"text"`
}

type TypingData struct {
	Active bool `json:"active"`
}

type NextData struct {
	Tags []string `json:"tags"`
}

type WelcomeData struct {
	SessionID string `json:"sessionId"`
	// OnlineCount is the live registry count at handshake time. The
	// frontend uses it as social proof on the Intro screen ("N people
	// here right now"), gated by a UI threshold so a near-empty server
	// doesn't broadcast its emptiness.
	OnlineCount int `json:"onlineCount"`
}

type MatchedData struct {
	SharedTags []string `json:"sharedTags"`
}

type PeerMsgData struct {
	Text string `json:"text"`
	Ts   int64  `json:"ts"`
}

type PeerTypingData struct {
	Active bool `json:"active"`
}

// Reason values: "stop", "next", "disconnect".
type PeerLeftData struct {
	Reason string `json:"reason"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Encode is a convenience to build an Envelope with a JSON-encoded data field.
func Encode(msgType string, data any) ([]byte, error) {
	if data == nil {
		return json.Marshal(Envelope{Type: msgType, Data: json.RawMessage("{}")})
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Data: raw})
}
