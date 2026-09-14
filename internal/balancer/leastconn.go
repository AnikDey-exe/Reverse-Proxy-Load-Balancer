package balancer

// LeastConnections sends traffic to whichever alive backend has the
// fewest requests in flight. Better than round robin when backends do
// uneven amounts of work per request; probably overkill otherwise.
type LeastConnections struct{}

func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

func (l *LeastConnections) NextBackend(backends []*Backend) *Backend {
	var best *Backend
	var bestConns int64

	for _, b := range backends {
		if !b.IsAlive() {
			continue
		}
		conns := b.ActiveConns()
		if best == nil || conns < bestConns {
			best = b
			bestConns = conns
		}
	}
	return best
}
