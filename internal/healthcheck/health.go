package healthcheck

import (
	"context"
	"net/http"
	"time"

	"loadbalancer/internal/balancer"
)

type Checker struct {
	Pool     *balancer.Pool
	Interval time.Duration
	Timeout  time.Duration
	client   *http.Client
}

func NewChecker(pool *balancer.Pool, interval, timeout time.Duration) *Checker {
	return &Checker{
		Pool:     pool,
		Interval: interval,
		Timeout:  timeout,
		client:   &http.Client{Timeout: timeout},
	}
}

func (c *Checker) Run(ctx context.Context) {
	c.probeAll(ctx)

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.probeAll(ctx)
		}
	}
}

func (c *Checker) probeAll(ctx context.Context) {
	for _, b := range c.Pool.Backends() {
		go c.probe(ctx, b)
	}
}

func (c *Checker) probe(ctx context.Context, b *balancer.Backend) {
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	b.SetAlive(isReachable(reqCtx, c.client, b.URL.String()))
}

func isReachable(ctx context.Context, client *http.Client, target string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}
