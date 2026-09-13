package balancer

// Pool is just the backend list plus whatever strategy picks among them.
// The slice is fixed at construction time so reads don't need locking -
// only each Backend's own alive/conn counters change at runtime, and
// those are already atomic.
type Pool struct {
	backends []*Backend
	strategy Strategy
}

func NewPool(backends []*Backend, strategy Strategy) *Pool {
	return &Pool{backends: backends, strategy: strategy}
}

func (p *Pool) Next() *Backend {
	return p.strategy.NextBackend(p.backends)
}

func (p *Pool) Backends() []*Backend {
	return p.backends
}

func (p *Pool) AliveCount() int {
	n := 0
	for _, b := range p.backends {
		if b.IsAlive() {
			n++
		}
	}
	return n
}
