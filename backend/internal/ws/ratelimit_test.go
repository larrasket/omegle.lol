package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucket_AllowsBurst(t *testing.T) {
	tb := NewTokenBucket(10, 5) // 10/sec, burst 5
	for i := 0; i < 5; i++ {
		assert.True(t, tb.Allow(), "should allow burst token %d", i)
	}
	assert.False(t, tb.Allow(), "should deny after burst exhausted")
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	tb := NewTokenBucket(100, 1) // 100/sec, burst 1
	assert.True(t, tb.Allow())
	assert.False(t, tb.Allow())
	time.Sleep(15 * time.Millisecond) // ~1.5 tokens accrued
	assert.True(t, tb.Allow())
}
