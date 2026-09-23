package proxyhandler

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"loadbalancer/internal/balancer"
)

type ctxKey int

const resultCtxKey ctxKey = iota

// requestResult rides along in the request context so a backend's
// ErrorHandler (which only gets the ResponseWriter/Request, nothing else)
// has a way to tell the retry loop in Handler.serve that this attempt
// failed.
type requestResult struct {
	failed bool
}

func NewReverseProxy(target *url.URL, backend *balancer.Backend) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("%s didn't answer: %v", target, err)
		backend.SetAlive(false)
		if res, ok := r.Context().Value(resultCtxKey).(*requestResult); ok {
			res.failed = true
		}
		// don't write anything to w here - if we got this far nothing
		// has gone out to the client yet, so Handler.serve can still try
		// a different backend on the same ResponseWriter
	}

	return proxy
}

type Handler struct {
	Pool        *balancer.Pool
	MaxAttempts int
}

func NewHandler(pool *balancer.Pool, maxAttempts int) *Handler {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Handler{Pool: pool, MaxAttempts: maxAttempts}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, 0)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, attempt int) {
	if attempt >= h.MaxAttempts {
		http.Error(w, "no healthy backends available", http.StatusServiceUnavailable)
		return
	}

	backend := h.Pool.Next()
	if backend == nil {
		http.Error(w, "no healthy backends available", http.StatusServiceUnavailable)
		return
	}

	result := &requestResult{}
	ctx := context.WithValue(r.Context(), resultCtxKey, result)
	req := r.WithContext(ctx)

	backend.IncConns()
	backend.ReverseProxy.ServeHTTP(w, req)
	backend.DecConns()

	if result.failed {
		h.serve(w, r, attempt+1)
	}
}
