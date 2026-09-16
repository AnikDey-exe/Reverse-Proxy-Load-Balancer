package healthcheck

import (
	"context"
	"log"
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

	// fixed interval for now, could switch to backoff if this ever needs
	// to run against something flakier than a couple local processes
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

	alive := isReachable(reqCtx, c.client, b.URL.String())
	wasAlive := b.IsAlive()
	b.SetAlive(alive)

	if alive == wasAlive {
		return
	}
	if alive {
		log.Printf("backend %s is back up", b.URL)
	} else {
		log.Printf("backend %s stopped responding, marking it down", b.URL)
	}
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
	// any response at all means the process is up and answering, even
	// if it's a 4xx/5xx 
	return true
}