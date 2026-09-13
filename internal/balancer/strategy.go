package balancer

// Strategy picks the next backend from the pool. Implementations need to
// be safe for concurrent use and skip anything that isn't alive. Returns
// nil if nothing's available.
type Strategy interface {
	NextBackend(backends []*Backend) *Backend
}
