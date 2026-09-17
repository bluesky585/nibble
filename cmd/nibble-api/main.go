package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/bluesky585/nibble/internal/httpapi"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	indexPath := flag.String("index", "", "optional SQLite file (.db) backing the index; without it the index lives in process memory and is gone on exit")
	flag.Parse()

	var api *httpapi.API
	var err error
	if *indexPath != "" {
		api, err = httpapi.NewPersistent(*indexPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("nibble-api index persisted to %s", *indexPath)
	} else {
		api = httpapi.New()
	}
	defer api.Close()

	log.Printf("nibble-api listening on %s", *addr)
	if err := http.ListenAndServe(*addr, api.Handler()); err != nil {
		log.Fatal(err)
	}
}
