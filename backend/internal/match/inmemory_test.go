package match

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omegle-lol/omegle/backend/internal/session"
)

func enqWith(t *testing.T, m *Memory, tags []string) (roomCh <-chan *Room, cancel func()) {
	s := session.New()
	ch, cancel := m.Enqueue(&Candidate{Session: s, Tags: tags, EnqueuedAt: time.Now()})
	t.Cleanup(cancel)
	return ch, cancel
}

func TestInMemory_PairsOnSharedTag(t *testing.T) {
	m := NewMemory(8 * time.Second)
	defer m.Close()

	chA, _ := enqWith(t, m, []string{"tech"})
	chB, _ := enqWith(t, m, []string{"tech"})

	select {
	case roomA := <-chA:
		require.NotNil(t, roomA)
		assert.Equal(t, []string{"tech"}, roomA.SharedTags)
	case <-time.After(2 * time.Second):
		t.Fatal("A did not get a match")
	}
	select {
	case roomB := <-chB:
		require.NotNil(t, roomB)
		assert.Equal(t, []string{"tech"}, roomB.SharedTags)
	case <-time.After(2 * time.Second):
		t.Fatal("B did not get a match")
	}
}

func TestInMemory_PrefersHigherOverlap(t *testing.T) {
	m := NewMemory(8 * time.Second)
	defer m.Close()

	chWeak, _ := enqWith(t, m, []string{"music"})           // shares 1 with chooser
	chStrong, _ := enqWith(t, m, []string{"music", "tech"}) // shares 2 with chooser
	chChooser, _ := enqWith(t, m, []string{"music", "tech"})

	select {
	case r := <-chChooser:
		require.NotNil(t, r)
		assert.ElementsMatch(t, []string{"music", "tech"}, r.SharedTags)
	case <-time.After(2 * time.Second):
		t.Fatal("chooser did not match")
	}
	select {
	case r := <-chStrong:
		require.NotNil(t, r)
	case <-time.After(2 * time.Second):
		t.Fatal("strong did not match")
	}
	// weak should still be waiting.
	select {
	case <-chWeak:
		t.Fatal("weak should not have matched yet")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestInMemory_NoOverlap_NoMatchBeforeTimeout(t *testing.T) {
	m := NewMemory(8 * time.Second)
	defer m.Close()

	chA, _ := enqWith(t, m, []string{"music"})
	chB, _ := enqWith(t, m, []string{"tech"})

	select {
	case <-chA:
		t.Fatal("A should not have matched without overlap")
	case <-chB:
		t.Fatal("B should not have matched without overlap")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestInMemory_Cancel_RemovesFromQueue(t *testing.T) {
	m := NewMemory(8 * time.Second)
	defer m.Close()

	chA, cancelA := enqWith(t, m, []string{"tech"})
	cancelA()

	chB, _ := enqWith(t, m, []string{"tech"})

	// A's channel should be closed (no room).
	select {
	case r, ok := <-chA:
		assert.False(t, ok, "A's channel should be closed")
		assert.Nil(t, r)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("A's channel was not closed after cancel")
	}
	// B should be waiting (nobody else there).
	select {
	case <-chB:
		t.Fatal("B should not have matched")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestInMemory_FallbackPairsAfterTimeout(t *testing.T) {
	m := NewMemory(300 * time.Millisecond)
	defer m.Close()

	chA, _ := enqWith(t, m, []string{"music"})
	chB, _ := enqWith(t, m, []string{"tech"})

	select {
	case r := <-chA:
		require.NotNil(t, r)
		assert.Empty(t, r.SharedTags)
	case <-time.After(2 * time.Second):
		t.Fatal("A never matched on fallback")
	}
	select {
	case r := <-chB:
		require.NotNil(t, r)
	case <-time.After(2 * time.Second):
		t.Fatal("B never matched on fallback")
	}
}

func TestInMemory_EmptyTagsUsesFallback(t *testing.T) {
	m := NewMemory(300 * time.Millisecond)
	defer m.Close()

	chA, _ := enqWith(t, m, nil)
	chB, _ := enqWith(t, m, nil)

	for _, ch := range []<-chan *Room{chA, chB} {
		select {
		case r := <-ch:
			require.NotNil(t, r)
		case <-time.After(2 * time.Second):
			t.Fatal("no fallback match for empty-tag pair")
		}
	}
}

func TestInMemory_LivenessDropsGhost(t *testing.T) {
	m := NewMemory(8 * time.Second)
	defer m.Close()

	// Mark every session alive by default. Tests can flip individual IDs
	// to dead by removing them from the map.
	alive := map[string]bool{}
	m.SetLivenessCheck(func(id string) bool { return alive[id] })

	// Ghost: enqueued, then "session disconnected" — flag it dead.
	ghost := session.New()
	alive[ghost.ID] = true
	_, _ = m.Enqueue(&Candidate{Session: ghost, Tags: []string{"tech"}, EnqueuedAt: time.Now()})
	alive[ghost.ID] = false

	// Live searcher arriving after the ghost.
	live := session.New()
	alive[live.ID] = true
	chLive, _ := m.Enqueue(&Candidate{Session: live, Tags: []string{"tech"}, EnqueuedAt: time.Now()})

	// scan() should drop the ghost before pairing, leaving live with no
	// partner. So the live searcher must NOT be matched within a short
	// window (matcher's 500 ms tick is the budget).
	select {
	case r := <-chLive:
		t.Fatalf("live searcher was paired with a ghost: %+v", r)
	case <-time.After(700 * time.Millisecond):
		// expected: nobody to pair with
	}
}
