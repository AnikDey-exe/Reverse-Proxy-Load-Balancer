package balancer

import (
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

// Backend is one upstream server the proxy can send traffic to.
type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy

	alive       atomic.Bool
	activeConns atomic.Int64
}

func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

func (b *Backend) IncConns() int64 {
	return b.activeConns.Add(1)
}

func (b *Backend) DecConns() int64 {
	return b.activeConns.Add(-1)
}

func (b *Backend) ActiveConns() int64 {
	return b.activeConns.Load()
}
