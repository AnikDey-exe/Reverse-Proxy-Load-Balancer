package proxyhandler

import (
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
// has a way to tell the retry loop that this attempt failed.
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
	}

	return proxy
}
