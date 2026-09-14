package balancer

import "testing"

func TestLeastConnections_PicksFewestConns(t *testing.T) {
	a := mustBackend(t, "http://a")
	b := mustBackend(t, "http://b")
	c := mustBackend(t, "http://c")
	backends := []*Backend{a, b, c}

	a.IncConns()
	a.IncConns()
	b.IncConns()
	// c has 0 active conns, should win

	lc := NewLeastConnections()
	got := lc.NextBackend(backends)
	if got != c {
		t.Fatalf("expected backend c (0 conns), got %v", got.URL)
	}
}

func TestLeastConnections_SkipsDeadBackends(t *testing.T) {
	a := mustBackend(t, "http://a")
	b := mustBackend(t, "http://b")
	a.SetAlive(false) // fewer conns but dead, shouldn't matter

	lc := NewLeastConnections()
	got := lc.NextBackend([]*Backend{a, b})
	if got != b {
		t.Fatalf("expected backend b (only alive), got %v", got)
	}
}

func TestLeastConnections_AllDeadReturnsNil(t *testing.T) {
	a := mustBackend(t, "http://a")
	a.SetAlive(false)

	lc := NewLeastConnections()
	if got := lc.NextBackend([]*Backend{a}); got != nil {
		t.Fatalf("expected nil, got %v", got.URL)
	}
}
