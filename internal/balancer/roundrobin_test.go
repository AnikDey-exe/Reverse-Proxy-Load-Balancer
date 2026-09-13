package balancer

import (
	"net/url"
	"sync"
	"testing"
)

func mustBackend(t *testing.T, raw string) *Backend {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	b := &Backend{URL: u}
	b.SetAlive(true)
	return b
}

func TestRoundRobin_DistributesEvenly(t *testing.T) {
	backends := []*Backend{
		mustBackend(t, "http://a"),
		mustBackend(t, "http://b"),
		mustBackend(t, "http://c"),
	}
	rr := NewRoundRobin()

	counts := map[string]int{}
	const n = 300
	for i := 0; i < n; i++ {
		b := rr.NextBackend(backends)
		if b == nil {
			t.Fatalf("iteration %d: got nil backend", i)
		}
		counts[b.URL.String()]++
	}

	for _, b := range backends {
		if got := counts[b.URL.String()]; got != n/len(backends) {
			t.Errorf("backend %s got %d requests, want %d", b.URL, got, n/len(backends))
		}
	}
}

func TestRoundRobin_SkipsDeadBackends(t *testing.T) {
	backends := []*Backend{
		mustBackend(t, "http://a"),
		mustBackend(t, "http://b"),
		mustBackend(t, "http://c"),
	}
	backends[1].SetAlive(false)
	rr := NewRoundRobin()

	for i := 0; i < 20; i++ {
		b := rr.NextBackend(backends)
		if b == nil {
			t.Fatalf("iteration %d: got nil backend", i)
		}
		if b.URL.String() == "http://b" {
			t.Fatalf("iteration %d: dead backend got picked", i)
		}
	}
}

func TestRoundRobin_AllDeadReturnsNil(t *testing.T) {
	backends := []*Backend{mustBackend(t, "http://a"), mustBackend(t, "http://b")}
	for _, b := range backends {
		b.SetAlive(false)
	}
	rr := NewRoundRobin()
	if b := rr.NextBackend(backends); b != nil {
		t.Fatalf("expected nil when all backends dead, got %v", b.URL)
	}
}

func TestRoundRobin_EmptyPool(t *testing.T) {
	rr := NewRoundRobin()
	if b := rr.NextBackend(nil); b != nil {
		t.Fatalf("expected nil for empty pool, got %v", b.URL)
	}
}

func TestRoundRobin_ConcurrentSafe(t *testing.T) {
	backends := []*Backend{
		mustBackend(t, "http://a"),
		mustBackend(t, "http://b"),
		mustBackend(t, "http://c"),
	}
	rr := NewRoundRobin()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if b := rr.NextBackend(backends); b == nil {
					t.Error("got nil backend under concurrent load")
				}
			}
		}()
	}
	wg.Wait()
}
