package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	for _, k := range []string{
		"PORT",
		"MATCH_TIMEOUT_MS",
		"MAX_TAGS",
		"MAX_TAG_LEN",
		"MAX_MSG_BYTES",
		"MSG_RATE_PER_SEC",
		"ALLOWED_ORIGINS",
		"LOG_LEVEL",
		"HEARTBEAT_INTERVAL_MS",
		"HEARTBEAT_TIMEOUT_MS",
		"TRUSTED_PROXY_DEPTH",
		"SHUTDOWN_TIMEOUT_MS",
	} {
		t.Setenv(k, "")
	}
	c := Load()
	assert.Equal(t, "8080", c.Port)
	assert.Equal(t, 8*time.Second, c.MatchTimeout)
	assert.Equal(t, 10, c.MaxTags)
	assert.Equal(t, 30, c.MaxTagLen)
	assert.Equal(t, 2048, c.MaxMsgBytes)
	assert.Equal(t, 10, c.MsgRatePerSec)
	assert.Equal(t, []string{"http://localhost:5173"}, c.AllowedOrigins)
	assert.Equal(t, "info", c.LogLevel)
	assert.Equal(t, 25*time.Second, c.HeartbeatInterval)
	assert.Equal(t, 10*time.Second, c.HeartbeatTimeout)
	assert.Equal(t, 0, c.TrustedProxyDepth)
	assert.Equal(t, 8*time.Second, c.ShutdownTimeout)
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("MATCH_TIMEOUT_MS", "3000")
	t.Setenv("ALLOWED_ORIGINS", "https://omegle.lol,https://staging.omegle.lol")
	t.Setenv("TRUSTED_PROXY_DEPTH", "1")
	t.Setenv("SHUTDOWN_TIMEOUT_MS", "12000")
	c := Load()
	assert.Equal(t, "9000", c.Port)
	assert.Equal(t, 3*time.Second, c.MatchTimeout)
	assert.Equal(t, []string{"https://omegle.lol", "https://staging.omegle.lol"}, c.AllowedOrigins)
	assert.Equal(t, 1, c.TrustedProxyDepth)
	assert.Equal(t, 12*time.Second, c.ShutdownTimeout)
}
