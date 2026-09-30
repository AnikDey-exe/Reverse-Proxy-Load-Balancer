package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"loadbalancer/internal/balancer"
	"loadbalancer/internal/healthcheck"
	"loadbalancer/internal/proxyhandler"
)

func main() {
	addr := flag.String("addr", ":8080", "address for the load balancer to listen on")
	backendList := flag.String("backends", "", "comma-separated backend URLs, e.g. http://localhost:9001,http://localhost:9002")
	strategyName := flag.String("strategy", "round-robin", "round-robin | least-conn")
	healthInterval := flag.Duration("health-interval", 5*time.Second, "active health check interval")
	healthTimeout := flag.Duration("health-timeout", 2*time.Second, "active health check timeout")
	maxRetries := flag.Int("max-retries", 3, "how many backends to try before giving up on a request")
	flag.Parse()

	if *backendList == "" {
		log.Fatal("need at least one backend, e.g. -backends=http://localhost:9001,http://localhost:9002")
	}

	backends, err := buildBackends(strings.Split(*backendList, ","))
	if err != nil {
		log.Fatalf("bad backend list: %v", err)
	}

	strategy, err := buildStrategy(*strategyName)
	if err != nil {
		log.Fatal(err)
	}

	pool := balancer.NewPool(backends, strategy)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker := healthcheck.NewChecker(pool, *healthInterval, *healthTimeout)
	go checker.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", proxyhandler.NewHandler(pool, *maxRetries))
	mux.HandleFunc("/lb/stats", statsHandler(pool))

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (strategy=%s, %d backend(s))", *addr, *strategyName, len(backends))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server died: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown didn't finish cleanly: %v", err)
	}
}

func buildBackends(rawURLs []string) ([]*balancer.Backend, error) {
	var backends []*balancer.Backend
	for _, raw := range rawURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}

		backend := &balancer.Backend{URL: u}
		backend.ReverseProxy = proxyhandler.NewReverseProxy(u, backend)
		backend.SetAlive(true)
		backends = append(backends, backend)
	}
	return backends, nil
}

func buildStrategy(name string) (balancer.Strategy, error) {
	switch name {
	case "round-robin", "":
		return balancer.NewRoundRobin(), nil
	case "least-conn":
		return balancer.NewLeastConnections(), nil
	default:
		return nil, &unknownStrategyError{name}
	}
}

type unknownStrategyError struct{ name string }

func (e *unknownStrategyError) Error() string {
	return "unknown strategy: " + e.name
}

func statsHandler(pool *balancer.Pool) http.HandlerFunc {
	type backendStat struct {
		URL         string `json:"url"`
		Alive       bool   `json:"alive"`
		ActiveConns int64  `json:"active_conns"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		stats := make([]backendStat, 0, len(pool.Backends()))
		for _, b := range pool.Backends() {
			stats = append(stats, backendStat{
				URL:         b.URL.String(),
				Alive:       b.IsAlive(),
				ActiveConns: b.ActiveConns(),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}
