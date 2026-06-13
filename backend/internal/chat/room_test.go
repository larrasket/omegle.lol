package chat

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omegle-lol/omegle/backend/internal/proto"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

func readEnvelope(t *testing.T, ch <-chan []byte) proto.Envelope {
	t.Helper()
	select {
	case raw := <-ch:
		var env proto.Envelope
		require.NoError(t, json.Unmarshal(raw, &env))
		return env
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for envelope")
		return proto.Envelope{}
	}
}

func TestRoom_RelaysMessage(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)
	defer r.Close("disconnect")

	require.NoError(t, r.RelayMessage(a.ID, "hello"))
	env := readEnvelope(t, b.Send)
	assert.Equal(t, proto.MsgPeerMsg, env.Type)
	var d proto.PeerMsgData
	require.NoError(t, json.Unmarshal(env.Data, &d))
	assert.Equal(t, "hello", d.Text)
	assert.NotZero(t, d.Ts)
}

func TestRoom_RelaysTyping(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)
	defer r.Close("disconnect")

	require.NoError(t, r.RelayTyping(a.ID, true))
	env := readEnvelope(t, b.Send)
	assert.Equal(t, proto.MsgPeerTyping, env.Type)
}

func TestRoom_ClosePeerOnly_StopReason(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)

	r.ClosePeerOnly(a.ID, "stop")
	env := readEnvelope(t, b.Send)
	assert.Equal(t, proto.MsgPeerLeft, env.Type)
	var d proto.PeerLeftData
	require.NoError(t, json.Unmarshal(env.Data, &d))
	assert.Equal(t, "stop", d.Reason)
	// initiator gets nothing
	select {
	case <-a.Send:
		t.Fatal("initiator should not receive peer_left when they initiated stop")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRoom_Close_FansOutToBoth(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)

	r.Close("disconnect")
	for _, ch := range []chan []byte{a.Send, b.Send} {
		select {
		case raw := <-ch:
			var env proto.Envelope
			require.NoError(t, json.Unmarshal(raw, &env))
			assert.Equal(t, proto.MsgPeerLeft, env.Type)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("member did not receive peer_left from Close")
		}
	}
}

func TestRoom_NotifiesPeerOnly(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)
	defer r.Close("disconnect")

	require.NoError(t, r.RelayMessage(a.ID, "hi"))
	// Sender (a) should not receive their own message back.
	select {
	case <-a.Send:
		t.Fatal("sender received its own relayed message")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRoom_RejectsUnknownSender(t *testing.T) {
	a, b := session.New(), session.New()
	r := NewRoom("room-1", a, b, nil)
	defer r.Close("disconnect")

	err := r.RelayMessage("not-a-member", "ping")
	assert.Error(t, err)
}
