package balancer

import "sync/atomic"

// RoundRobin just cycles through the list, skipping dead backends. Using
// one atomic counter instead of a mutex means we don't serialize every
// request behind a lock just to pick a backend.
type RoundRobin struct {
	cursor atomic.Uint64
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (r *RoundRobin) NextBackend(backends []*Backend) *Backend {
	n := len(backends)
	if n == 0 {
		return nil
	}

	start := r.cursor.Add(1)
	for i := 0; i < n; i++ {
		idx := (start + uint64(i)) % uint64(n)
		if backends[idx].IsAlive() {
			return backends[idx]
		}
	}
	return nil
}
