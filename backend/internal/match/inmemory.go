package match

import (
	"sort"
	"sync"
	"time"
)

// Memory is an in-memory Matcher. Safe for concurrent Enqueue/Cancel.
// All matching happens on a single internal goroutine.
type Memory struct {
	mu       sync.Mutex
	queue    []*Candidate
	byID     map[string]*Candidate
	tagIndex map[string]map[string]struct{} // tag -> set of session IDs
	pending  map[string]chan *Room          // session ID -> notification channel
	timeout  time.Duration

	// isAlive is an optional liveness predicate. If set, scan() drops any
	// queued candidate whose session is no longer alive — preventing
	// ghost matches when a WS dropped before cancelPendingSearch could
	// fire (the heartbeat-detection window).
	isAlive func(sessionID string) bool

	signal chan struct{}
	closed chan struct{}
}

func NewMemory(timeout time.Duration) *Memory {
	m := &Memory{
		byID:     make(map[string]*Candidate),
		tagIndex: make(map[string]map[string]struct{}),
		pending:  make(map[string]chan *Room),
		timeout:  timeout,
		signal:   make(chan struct{}, 1),
		closed:   make(chan struct{}),
	}
	go m.loop()
	return m
}

// SetLivenessCheck registers a callback used by scan() to filter out queue
// entries whose underlying session has already gone away. Optional but
// strongly recommended in production — without it a candidate can sit in
// the queue for up to (HEARTBEAT_INTERVAL + HEARTBEAT_TIMEOUT) seconds
// after its WS dies, during which a new searcher can be paired with it.
func (m *Memory) SetLivenessCheck(isAlive func(sessionID string) bool) {
	m.mu.Lock()
	m.isAlive = isAlive
	m.mu.Unlock()
}

func (m *Memory) Close() {
	close(m.closed)
}

// TagCount pairs a tag with how many waiting candidates currently list it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// Stats is a point-in-time snapshot of the matcher queue. Read-only —
// taking it does not perturb the queue. topN caps the tag histogram; pass
// 0 for "no cap".
type Stats struct {
	Searching int        `json:"searching"`
	TopTags   []TagCount `json:"top_tags"`
}

// Snapshot returns the current queue size and the most-popular tags among
// waiting candidates. Used by the admin endpoint; cheap enough to call on
// every poll because the tag index is already maintained inline.
func (m *Memory) Snapshot(topN int) Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	tags := make([]TagCount, 0, len(m.tagIndex))
	for t, set := range m.tagIndex {
		tags = append(tags, TagCount{Tag: t, Count: len(set)})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Tag < tags[j].Tag
	})
	if topN > 0 && len(tags) > topN {
		tags = tags[:topN]
	}
	return Stats{Searching: len(m.queue), TopTags: tags}
}

func (m *Memory) Enqueue(c *Candidate) (room <-chan *Room, cancel func()) {
	ch := make(chan *Room, 1)
	m.mu.Lock()
	if c.EnqueuedAt.IsZero() {
		c.EnqueuedAt = time.Now()
	}
	m.queue = append(m.queue, c)
	m.byID[c.Session.ID] = c
	for _, t := range c.Tags {
		if m.tagIndex[t] == nil {
			m.tagIndex[t] = make(map[string]struct{})
		}
		m.tagIndex[t][c.Session.ID] = struct{}{}
	}
	m.pending[c.Session.ID] = ch
	m.mu.Unlock()

	select {
	case m.signal <- struct{}{}:
	default:
	}

	id := c.Session.ID
	cancel = func() {
		m.mu.Lock()
		m.removeLocked(id)
		m.mu.Unlock()
	}
	return ch, cancel
}

func (m *Memory) removeLocked(id string) {
	c, ok := m.byID[id]
	if !ok {
		return
	}
	delete(m.byID, id)
	for i, q := range m.queue {
		if q.Session.ID == id {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}
	for _, t := range c.Tags {
		if set, ok := m.tagIndex[t]; ok {
			delete(set, id)
			if len(set) == 0 {
				delete(m.tagIndex, t)
			}
		}
	}
	if ch, ok := m.pending[id]; ok {
		close(ch)
		delete(m.pending, id)
	}
}

func (m *Memory) loop() {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-m.closed:
			return
		case <-m.signal:
			// Small debounce: drain any pending signals and wait briefly so
			// rapid back-to-back Enqueue calls batch into one scan.
			time.Sleep(5 * time.Millisecond)
		drainSignal:
			for {
				select {
				case <-m.signal:
				default:
					break drainSignal
				}
			}
			m.scan()
		case <-t.C:
			m.scan()
		}
	}
}

func (m *Memory) scan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropDeadCandidatesLocked()

	now := time.Now()
	matched := make(map[string]struct{})

	m.matchTaggedPairsLocked(matched)
	m.applyTimeoutFallbacksLocked(now, matched)
	m.pruneMatchedLocked(matched)
}

// dropDeadCandidatesLocked walks the queue once and removes candidates whose
// session is no longer alive (per the registered liveness check). Each dropped
// entry has its pending channel closed so the corresponding awaitMatch
// goroutine reads (nil, false) and exits without registering a phantom room.
func (m *Memory) dropDeadCandidatesLocked() {
	if m.isAlive == nil || len(m.queue) == 0 {
		return
	}
	survivors := m.queue[:0]
	for _, c := range m.queue {
		if m.isAlive(c.Session.ID) {
			survivors = append(survivors, c)
			continue
		}
		// Dead: detach from byID/tagIndex and close the pending channel.
		delete(m.byID, c.Session.ID)
		for _, t := range c.Tags {
			if set, ok := m.tagIndex[t]; ok {
				delete(set, c.Session.ID)
				if len(set) == 0 {
					delete(m.tagIndex, t)
				}
			}
		}
		if ch, ok := m.pending[c.Session.ID]; ok {
			close(ch)
			delete(m.pending, c.Session.ID)
		}
	}
	m.queue = survivors
}

// matchTaggedPairsLocked greedily pairs candidates with the globally best tag overlap,
// preventing a low-overlap pair from stealing a partner that could make a better match.
func (m *Memory) matchTaggedPairsLocked(matched map[string]struct{}) {
	for {
		a, b := m.findGlobalBestPairLocked(matched)
		if a == nil {
			break
		}
		m.pairLocked(a, b)
		matched[a.Session.ID] = struct{}{}
		matched[b.Session.ID] = struct{}{}
	}
}

// findGlobalBestPairLocked scans the queue and returns the pair with the highest
// tag overlap. Ties break by earliest EnqueuedAt of the first candidate.
func (m *Memory) findGlobalBestPairLocked(matched map[string]struct{}) (bestA, bestB *Candidate) {
	bestScore := 0
	for _, u := range m.queue {
		if _, done := matched[u.Session.ID]; done {
			continue
		}
		partner := m.findBestPartnerLocked(u, matched)
		if partner == nil {
			continue
		}
		score := overlapCount(u.Tags, partner.Tags)
		if score > bestScore || (score == bestScore && bestA != nil && u.EnqueuedAt.Before(bestA.EnqueuedAt)) {
			bestScore = score
			bestA = u
			bestB = partner
		}
	}
	return bestA, bestB
}

// applyTimeoutFallbacksLocked pairs candidates that have waited longer than the
// timeout with any available unmatched partner, regardless of tag overlap.
func (m *Memory) applyTimeoutFallbacksLocked(now time.Time, matched map[string]struct{}) {
	for _, u := range m.queue {
		if _, done := matched[u.Session.ID]; done {
			continue
		}
		if now.Sub(u.EnqueuedAt) < m.timeout {
			continue
		}
		fb := m.findFallbackLocked(u, matched)
		if fb != nil {
			m.pairLocked(u, fb)
			matched[u.Session.ID] = struct{}{}
			matched[fb.Session.ID] = struct{}{}
		}
	}
}

// pruneMatchedLocked removes paired candidates from the queue.
func (m *Memory) pruneMatchedLocked(matched map[string]struct{}) {
	if len(matched) == 0 {
		return
	}
	nq := m.queue[:0]
	for _, q := range m.queue {
		if _, gone := matched[q.Session.ID]; !gone {
			nq = append(nq, q)
		}
	}
	m.queue = nq
}

// findBestPartnerLocked picks the queued candidate with the highest tag overlap > 0.
// Ties break by earliest EnqueuedAt.
func (m *Memory) findBestPartnerLocked(u *Candidate, matched map[string]struct{}) *Candidate {
	var best *Candidate
	bestScore := 0
	seen := make(map[string]struct{})
	for _, t := range u.Tags {
		for vID := range m.tagIndex[t] {
			if vID == u.Session.ID {
				continue
			}
			if _, ok := matched[vID]; ok {
				continue
			}
			if _, ok := seen[vID]; ok {
				continue
			}
			seen[vID] = struct{}{}
			v := m.byID[vID]
			if v == nil {
				continue
			}
			score := overlapCount(u.Tags, v.Tags)
			if isBetterCandidate(score, v, bestScore, best) {
				best = v
				bestScore = score
			}
		}
	}
	if bestScore == 0 {
		return nil
	}
	return best
}

// isBetterCandidate returns true when (score, candidate) beats (bestScore, best).
// Ties break by earliest EnqueuedAt.
func isBetterCandidate(score int, candidate *Candidate, bestScore int, best *Candidate) bool {
	if score > bestScore {
		return true
	}
	return score == bestScore && best != nil && candidate.EnqueuedAt.Before(best.EnqueuedAt)
}

// findFallbackLocked picks the oldest queued candidate other than u.
func (m *Memory) findFallbackLocked(u *Candidate, matched map[string]struct{}) *Candidate {
	for _, v := range m.queue {
		if v.Session.ID == u.Session.ID {
			continue
		}
		if _, gone := matched[v.Session.ID]; gone {
			continue
		}
		return v
	}
	return nil
}

func (m *Memory) pairLocked(a, b *Candidate) {
	shared := intersect(a.Tags, b.Tags)
	room := &Room{
		ID:         newRoomID(),
		A:          a.Session,
		B:          b.Session,
		SharedTags: shared,
		CreatedAt:  time.Now(),
	}
	for _, c := range [2]*Candidate{a, b} {
		// Remove from index.
		for _, t := range c.Tags {
			if set, ok := m.tagIndex[t]; ok {
				delete(set, c.Session.ID)
				if len(set) == 0 {
					delete(m.tagIndex, t)
				}
			}
		}
		delete(m.byID, c.Session.ID)
		if ch, ok := m.pending[c.Session.ID]; ok {
			ch <- room
			close(ch)
			delete(m.pending, c.Session.ID)
		}
	}
}

func overlapCount(a, b []string) int {
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	count := 0
	for _, y := range b {
		if _, ok := set[y]; ok {
			count++
		}
	}
	return count
}

func intersect(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	out := make([]string, 0)
	for _, y := range b {
		if _, ok := set[y]; ok {
			out = append(out, y)
		}
	}
	return out
}
