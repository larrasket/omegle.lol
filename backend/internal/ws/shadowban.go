package ws

import (
	"sync"
	"time"
)

// Shadowban is a lightweight in-process moderation store: peers report
// each other, and once an IP collects enough reports inside a sliding
// window we silently refuse to match it. The user keeps seeing the
// "looking for someone…" indicator — they never learn they're banned,
// they just don't get matched, which avoids tipping them off into
// rotating IPs/devices.
//
// All thresholds are intentionally small for an MVP. They tune by
// changing the consts; no env vars yet because the right values depend
// entirely on traffic patterns we don't have data on.
const (
	reportWindow    = 1 * time.Hour
	reportThreshold = 3
	banDuration     = 24 * time.Hour
	pruneInterval   = 10 * time.Minute
)

// Shadowban tracks recent reports and active bans, keyed by client IP.
// Safe for concurrent use. State is purely in-memory: a restart resets
// everything, which is fine for the single-instance Cloud Run topology.
type Shadowban struct {
	mu      sync.Mutex
	reports map[string][]time.Time // ip -> timestamps of recent reports (inside reportWindow)
	bans    map[string]time.Time   // ip -> ban expiry
	closed  chan struct{}
}

// NewShadowban constructs the store and kicks off the prune goroutine
// that drops stale reports and expired bans every pruneInterval.
func NewShadowban() *Shadowban {
	s := &Shadowban{
		reports: make(map[string][]time.Time),
		bans:    make(map[string]time.Time),
		closed:  make(chan struct{}),
	}
	go s.pruneLoop()
	return s
}

// Close stops the prune goroutine. Safe to call multiple times.
func (s *Shadowban) Close() {
	select {
	case <-s.closed:
		// already closed
	default:
		close(s.closed)
	}
}

// Report records a fresh complaint against ip and returns true if the
// IP is now banned (either by this report crossing the threshold or by
// a prior ban still in effect). Empty IP is a no-op.
func (s *Shadowban) Report(ip string) (nowBanned bool) {
	if ip == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()

	if until, ok := s.bans[ip]; ok && until.After(now) {
		return true
	}

	cutoff := now.Add(-reportWindow)
	prev := s.reports[ip]
	live := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			live = append(live, t)
		}
	}
	live = append(live, now)

	if len(live) >= reportThreshold {
		s.bans[ip] = now.Add(banDuration)
		delete(s.reports, ip)
		return true
	}
	s.reports[ip] = live
	return false
}

// IsBanned reports whether the IP currently has an active ban. Empty IP
// is never banned (so a session that somehow has no IP just behaves
// normally rather than being silently locked out).
func (s *Shadowban) IsBanned(ip string) bool {
	if ip == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.bans[ip]
	if !ok {
		return false
	}
	if until.Before(time.Now()) {
		delete(s.bans, ip)
		return false
	}
	return true
}

func (s *Shadowban) pruneLoop() {
	t := time.NewTicker(pruneInterval)
	defer t.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
			s.prune()
		}
	}
}

func (s *Shadowban) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-reportWindow)
	for ip, ts := range s.reports {
		live := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				live = append(live, t)
			}
		}
		if len(live) == 0 {
			delete(s.reports, ip)
		} else {
			s.reports[ip] = live
		}
	}
	for ip, until := range s.bans {
		if until.Before(now) {
			delete(s.bans, ip)
		}
	}
}
