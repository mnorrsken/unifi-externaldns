// Command mockunifi serves (or prints) a fake Unifi /v2 active-leases response
// matching the shape of active-leases.json. Use --print to dump JSON to stdout,
// or run without it to start an HTTP server.
//
//	go run ./cmd/mockunifi --print
//	go run ./cmd/mockunifi --addr :8080
//	UNIFI_API_KEY=k go run ./cmd/unifi-externaldns --api-url=http://localhost:8080 --domain-suffix=lan
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/mnorrsken/unifi-externaldns/internal/mock"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	apiKey := flag.String("api-key", "", "if set, require this X-API-KEY header")
	seed := flag.Uint64("seed", 1, "random seed for generated fields (MAC suffixes, IPs)")
	print := flag.Bool("print", false, "print one JSON response to stdout and exit")
	flag.Parse()

	if *print {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(mock.Generate(*seed)); err != nil {
			log.Fatalf("encode: %v", err)
		}
		return
	}

	log.Printf("mockunifi listening on %s — GET /v2/api/site/<site>/active-leases", *addr)
	if err := http.ListenAndServe(*addr, mock.Handler(*apiKey, *seed)); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
