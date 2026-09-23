package proxyhandler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"loadbalancer/internal/balancer"
)

func newTestBackend(t *testing.T, id string) (*balancer.Backend, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Id", id)
		w.WriteHeader(http.StatusOK)
	}))

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}
	b := &balancer.Backend{URL: u}
	b.ReverseProxy = NewReverseProxy(u, b)
	b.SetAlive(true)
	return b, srv
}

func TestHandler_RoutesToAliveBackend(t *testing.T) {
	backend, srv := newTestBackend(t, "only")
	defer srv.Close()

	pool := balancer.NewPool([]*balancer.Backend{backend}, balancer.NewRoundRobin())
	h := NewHandler(pool, 3)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Backend-Id"); got != "only" {
		t.Fatalf("X-Backend-Id = %q, want %q", got, "only")
	}
}

func TestHandler_FailsOverToNextBackend(t *testing.T) {
	// nothing listens here, so this fails at dial time
	deadURL, _ := url.Parse("http://127.0.0.1:1")
	dead := &balancer.Backend{URL: deadURL}
	dead.ReverseProxy = NewReverseProxy(deadURL, dead)
	dead.SetAlive(true)

	alive, srv := newTestBackend(t, "good")
	defer srv.Close()

	// RoundRobin's cursor starts by advancing to index 1, so dead needs
	// to go first in a 2-backend pool for it to actually get hit first
	pool := balancer.NewPool([]*balancer.Backend{alive, dead}, balancer.NewRoundRobin())
	h := NewHandler(pool, 3)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (should have failed over)", rec.Code)
	}
	if got := rec.Header().Get("X-Backend-Id"); got != "good" {
		t.Fatalf("X-Backend-Id = %q, want %q", got, "good")
	}
	if dead.IsAlive() {
		t.Fatal("dead backend should have been marked not alive after the failed request")
	}
}

func TestHandler_AllBackendsDownReturns503(t *testing.T) {
	deadURL, _ := url.Parse("http://127.0.0.1:1")
	dead := &balancer.Backend{URL: deadURL}
	dead.ReverseProxy = NewReverseProxy(deadURL, dead)
	dead.SetAlive(true)

	pool := balancer.NewPool([]*balancer.Backend{dead}, balancer.NewRoundRobin())
	h := NewHandler(pool, 2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandler_NoBackendsReturns503(t *testing.T) {
	pool := balancer.NewPool(nil, balancer.NewRoundRobin())
	h := NewHandler(pool, 3)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
