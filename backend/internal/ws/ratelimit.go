package ws

import (
	"sync"
	"time"
)

// TokenBucket is a tiny per-session rate limiter. Safe for concurrent use.
type TokenBucket struct {
	mu       sync.Mutex
	rate     float64 // tokens per second
	burst    float64
	tokens   float64
	lastFill time.Time
}

func NewTokenBucket(ratePerSec, burst int) *TokenBucket {
	return &TokenBucket{
		rate:     float64(ratePerSec),
		burst:    float64(burst),
		tokens:   float64(burst),
		lastFill: time.Now(),
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.lastFill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
