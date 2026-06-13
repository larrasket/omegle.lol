package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	cws "github.com/coder/websocket"

	"github.com/omegle-lol/omegle/backend/internal/metrics"
	"github.com/omegle-lol/omegle/backend/internal/proto"
	"github.com/omegle-lol/omegle/backend/internal/session"
)

// connection wraps a single live ws and runs three goroutines: reader, writer, heartbeat.
type connection struct {
	ws       *cws.Conn
	sess     *session.Session
	srv      *Server
	limiter  *TokenBucket
	dispatch *dispatchState

	lastPong atomic.Int64 // unix-ms timestamp of last pong/message received
	// paused is set by the client's pause message — once true, the
	// heartbeat watcher allows up to PauseGrace of silence instead of the
	// normal interval+timeout. Cleared by the matching resume.
	paused atomic.Bool
}

func newConnection(ws *cws.Conn, sess *session.Session, srv *Server) *connection {
	c := &connection{
		ws:       ws,
		sess:     sess,
		srv:      srv,
		limiter:  NewTokenBucket(srv.Cfg.MsgRatePerSec, 5),
		dispatch: &dispatchState{},
	}
	c.lastPong.Store(time.Now().UnixMilli())
	return c
}

func (c *connection) run(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Greet the client. welcome is the first message any client expects;
	// dropping it leaves the UI stuck on "connecting". Include the live
	// connection count so the Intro screen can show "N people here right
	// now" as social proof (the UI hides it below a threshold so an empty
	// server doesn't advertise its own emptiness).
	if raw, err := proto.Encode(proto.MsgWelcome, proto.WelcomeData{
		SessionID:   c.sess.ID,
		OnlineCount: c.srv.Sessions.Count(),
	}); err == nil {
		c.sendCritical(ctx, raw)
	}

	go c.writePump(ctx, cancel)
	go c.heartbeat(ctx, cancel)
	c.readPump(ctx, cancel)
}

func (c *connection) readPump(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		typ, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != cws.MessageText {
			continue
		}
		if len(data) > c.srv.Cfg.MaxMsgBytes*2 {
			c.sendError(proto.ErrMessageTooLarge, "payload too large")
			return
		}
		c.lastPong.Store(time.Now().UnixMilli())

		var env proto.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.sendError(proto.ErrInvalidJSON, err.Error())
			continue
		}
		c.handle(env)
	}
}

func (c *connection) writePump(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-c.sess.Send:
			if !ok {
				return
			}
			wctx, c2 := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Write(wctx, cws.MessageText, raw)
			c2()
			if err != nil {
				return
			}
		}
	}
}

func (c *connection) heartbeat(ctx context.Context, cancel context.CancelFunc) {
	every, timeout := c.srv.heartbeatTimings()
	pingT := time.NewTicker(every)
	checkT := time.NewTicker(timeout / 2)
	defer pingT.Stop()
	defer checkT.Stop()

	activeSilence := every + timeout
	pauseSilence := c.srv.Cfg.PauseGrace

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingT.C:
			// Skip pinging a paused tab — JS is throttled, the pong won't
			// come, and the ping itself just wakes the radio for nothing.
			if c.paused.Load() {
				continue
			}
			if raw, encErr := proto.Encode(proto.MsgPing, nil); encErr == nil {
				select {
				case c.sess.Send <- raw:
				default:
				}
			}
		case <-checkT.C:
			last := time.UnixMilli(c.lastPong.Load())
			limit := activeSilence
			if c.paused.Load() {
				limit = pauseSilence
			}
			if time.Since(last) > limit {
				slog.Info("heartbeat timeout, closing", "session_id", c.sess.ID[:8], "paused", c.paused.Load())
				cancel()
				return
			}
		}
	}
}

func (c *connection) sendError(code, msg string) {
	metrics.ErrorsTotal.WithLabelValues(code).Inc()
	raw, err := proto.Encode(proto.MsgError, proto.ErrorData{Code: code, Message: msg})
	if err != nil {
		return
	}
	select {
	case c.sess.Send <- raw:
	default:
	}
}

// sendCritical is for control-plane messages (welcome, matched) where silent
// buffer-full drops would leave the UI stuck. Waits up to ~2 s for the writer
// pump to make room; if the send still blocks the connection is stuck and we
// force-close it so the defer in HandleUpgrade fires the standard cleanup
// path. Uses NewTimer + Stop so the timer is GC'd promptly when the send wins
// the race (time.After would hold the channel until the timer fires).
func (c *connection) sendCritical(ctx context.Context, raw []byte) {
	t := time.NewTimer(2 * time.Second)
	defer t.Stop()
	select {
	case c.sess.Send <- raw:
	case <-ctx.Done():
	case <-t.C:
		slog.Warn("critical send blocked, closing", "session_id", c.sess.ID[:8])
		_ = c.ws.CloseNow()
	}
}
