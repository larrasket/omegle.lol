package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port              string
	MatchTimeout      time.Duration
	MaxTags           int
	MaxTagLen         int
	MaxMsgBytes       int
	MsgRatePerSec     int
	AllowedOrigins    []string
	LogLevel          string
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	// TrustedProxyDepth tells the WS upgrade how many trusted proxy hops are
	// between the client and us. With 0 (default), X-Forwarded-For is ignored
	// entirely and the per-IP rate limit uses r.RemoteAddr — safe but assumes
	// no proxy. With N>0, the Nth-from-right XFF entry is treated as the
	// client. For Google Cloud Run direct: 1. For Cloud Run behind Cloud
	// Load Balancer: 2. Misconfiguration just makes the rate limit slightly
	// looser; it never gives the client more privilege than r.RemoteAddr.
	TrustedProxyDepth int
	ShutdownTimeout   time.Duration
	// PauseGrace caps how long a client can be "paused" (tab backgrounded
	// or screen locked) before the server gives up and tears down the
	// connection. While paused, the normal HeartbeatInterval+Timeout
	// silence threshold is replaced by this larger window so brief mobile
	// inattention doesn't kill the chat.
	PauseGrace time.Duration
	// AdminUsername / AdminPassword gate the /admin/stats endpoint via HTTP
	// Basic Auth. Both empty → endpoint always 401s and the dashboard is
	// effectively offline. Password is the only thing required for access;
	// the username defaults to "admin" so a single-secret deployment works.
	AdminUsername string
	AdminPassword string
}

func Load() Config {
	return Config{
		Port:              env("PORT", "8080"),
		MatchTimeout:      msDuration("MATCH_TIMEOUT_MS", 8000),
		MaxTags:           envInt("MAX_TAGS", 10),
		MaxTagLen:         envInt("MAX_TAG_LEN", 30),
		MaxMsgBytes:       envInt("MAX_MSG_BYTES", 2048),
		MsgRatePerSec:     envInt("MSG_RATE_PER_SEC", 10),
		AllowedOrigins:    splitCSV(env("ALLOWED_ORIGINS", "http://localhost:5173")),
		LogLevel:          env("LOG_LEVEL", "info"),
		HeartbeatInterval: msDuration("HEARTBEAT_INTERVAL_MS", 25000),
		HeartbeatTimeout:  msDuration("HEARTBEAT_TIMEOUT_MS", 10000),
		TrustedProxyDepth: envInt("TRUSTED_PROXY_DEPTH", 0),
		ShutdownTimeout:   msDuration("SHUTDOWN_TIMEOUT_MS", 8000),
		PauseGrace:        msDuration("PAUSE_GRACE_MS", 120000),
		AdminUsername:     env("ADMIN_USERNAME", "admin"),
		AdminPassword:     env("ADMIN_PASSWORD", ""),
	}
}

func env(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func msDuration(k string, defMs int) time.Duration {
	return time.Duration(envInt(k, defMs)) * time.Millisecond
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
