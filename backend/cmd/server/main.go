package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/omegle-lol/omegle/backend/internal/admin"
	"github.com/omegle-lol/omegle/backend/internal/config"
	"github.com/omegle-lol/omegle/backend/internal/match"
	"github.com/omegle-lol/omegle/backend/internal/metrics"
	"github.com/omegle-lol/omegle/backend/internal/session"
	"github.com/omegle-lol/omegle/backend/internal/webroot"
	"github.com/omegle-lol/omegle/backend/internal/ws"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.LogLevel)

	matcher := match.NewMemory(cfg.MatchTimeout)
	defer matcher.Close()
	sessions := session.NewRegistry()
	// Let the matcher drop stale candidates each scan — a candidate whose
	// session has already left the registry should never be paired with the
	// next searcher. Closes the heartbeat-detection window during which
	// ghost matches were possible.
	matcher.SetLivenessCheck(func(id string) bool {
		_, ok := sessions.Get(id)
		return ok
	})
	shadowban := ws.NewShadowban()
	defer shadowban.Close()
	wsServer := ws.NewServer(cfg, sessions, matcher, shadowban)

	var ready atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/ws", wsServer.HandleUpgrade)

	adminSrv := &admin.Server{
		Username:  cfg.AdminUsername,
		Password:  cfg.AdminPassword,
		Sessions:  sessions,
		Matcher:   matcher,
		Rooms:     wsServer,
		StartedAt: time.Now(),
	}
	// Mounted at an unguessable path so anonymous scanners don't even land on
	// the auth prompt. Keep this in sync with the SvelteKit /<slug> route.
	mux.HandleFunc("/705812da16d3edd0/stats", adminSrv.StatsHandler)
	// Catch-all serves the embedded SvelteKit SPA. Explicit routes above
	// (/healthz, /readyz, /metrics, /ws) take precedence because net/http
	// resolves more-specific patterns first.
	mux.Handle("/", webroot.Handler())

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		// IdleTimeout closes /healthz, /readyz, /metrics clients that don't
		// send keep-alive activity. WebSocket connections are upgraded out of
		// this code path so they don't share this timeout.
		IdleTimeout: 60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "port", cfg.Port, "allowed_origins", cfg.AllowedOrigins)
		ready.Store(true)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	ready.Store(false)
	slog.Info("shutdown requested")

	// 1. Tell every active room their partner is leaving. This pushes
	//    peer_left envelopes into the writer pumps.
	wsServer.CloseAllRooms("disconnect")
	// 2. Brief pause so writer pumps have a chance to flush those envelopes
	//    over the wire before their connections are torn down.
	time.Sleep(500 * time.Millisecond)
	// 3. Force-close every WS so srv.Shutdown isn't stuck waiting for them
	//    to disconnect on their own. Cloud Run's default SIGTERM grace is
	//    10 s; the old 30 s shutdown deadline was effectively dead code.
	wsServer.CloseAllConnections()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("shutdown complete")
}

func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)
}
