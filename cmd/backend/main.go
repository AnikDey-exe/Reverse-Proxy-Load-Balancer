// simple upstream server for testing the load balancer against. prints
// which instance answered so you can watch requests spread out, and can
// fake latency/failures for testing failover.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	port := flag.Int("port", 9001, "port to listen on")
	name := flag.String("name", "", "identifier returned in responses (defaults to :port)")
	latency := flag.Duration("latency", 0, "artificial latency added to every response")
	failRate := flag.Float64("fail-rate", 0, "fraction of requests (0-1) to fail with a 500")
	flag.Parse()

	id := *name
	if id == "" {
		id = fmt.Sprintf(":%d", *port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *latency > 0 {
			time.Sleep(*latency)
		}
		if *failRate > 0 && rand.Float64() < *failRate {
			http.Error(w, fmt.Sprintf("backend %s: induced failure", id), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Backend-Id", id)
		fmt.Fprintf(w, "hello from backend %s\n", id)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("backend %s up on %s", id, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
