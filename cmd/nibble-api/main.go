package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/bluesky585/nibble/internal/httpapi"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	log.Printf("nibble-api listening on %s", *addr)
	if err := http.ListenAndServe(*addr, httpapi.Handler()); err != nil {
		log.Fatal(err)
	}
}
