package ws

import (
	"testing"
	"time"
)

func TestReportUnderThresholdNotBanned(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	for i := 0; i < reportThreshold-1; i++ {
		if s.Report("1.2.3.4") {
			t.Fatalf("ip should not be banned after %d reports", i+1)
		}
	}
	if s.IsBanned("1.2.3.4") {
		t.Fatal("ip should still be unbanned")
	}
}

func TestReportAtThresholdBans(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	var banned bool
	for i := 0; i < reportThreshold; i++ {
		banned = s.Report("1.2.3.4")
	}
	if !banned {
		t.Fatal("report at threshold should return banned=true")
	}
	if !s.IsBanned("1.2.3.4") {
		t.Fatal("IsBanned should agree")
	}
}

func TestEmptyIPNeverBanned(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	for i := 0; i < reportThreshold*2; i++ {
		if s.Report("") {
			t.Fatal("empty ip should never be reportable")
		}
	}
	if s.IsBanned("") {
		t.Fatal("empty ip should never be banned")
	}
}

func TestDifferentIPsTrackedIndependently(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	for i := 0; i < reportThreshold; i++ {
		s.Report("1.1.1.1")
	}
	if !s.IsBanned("1.1.1.1") {
		t.Fatal("1.1.1.1 should be banned")
	}
	if s.IsBanned("2.2.2.2") {
		t.Fatal("2.2.2.2 should not be banned by 1.1.1.1 reports")
	}
}

func TestExpiredBanCleansUpOnQuery(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	// Stuff a ban with an already-elapsed expiry.
	s.mu.Lock()
	s.bans["3.3.3.3"] = time.Now().Add(-1 * time.Second)
	s.mu.Unlock()
	if s.IsBanned("3.3.3.3") {
		t.Fatal("expired ban should be treated as not banned")
	}
	// And the entry should be removed.
	s.mu.Lock()
	_, present := s.bans["3.3.3.3"]
	s.mu.Unlock()
	if present {
		t.Fatal("expired ban should be evicted from the map on query")
	}
}

func TestPruneEvictsStaleReportsAndBans(t *testing.T) {
	s := NewShadowban()
	defer s.Close()
	old := time.Now().Add(-(reportWindow + time.Hour))
	s.mu.Lock()
	s.reports["a"] = []time.Time{old}
	s.bans["b"] = old
	s.mu.Unlock()
	s.prune()
	s.mu.Lock()
	_, hasReport := s.reports["a"]
	_, hasBan := s.bans["b"]
	s.mu.Unlock()
	if hasReport {
		t.Errorf("stale report should be pruned")
	}
	if hasBan {
		t.Errorf("expired ban should be pruned")
	}
}
