package main

import (
	"log"
	"net/http"

	"github.com/tarantool-rag/internal/api"
	"github.com/tarantool-rag/internal/config"
	"github.com/tarantool-rag/internal/index"
	"github.com/tarantool-rag/internal/rag"
)

func main() {
	cfg := config.Load()

	idx, err := index.Load(cfg.IndexDir)
	if err != nil {
		log.Fatalf("load index from %s: %v (run: go run ./cmd/build-index)", cfg.IndexDir, err)
	}

	pipeline := rag.NewPipeline(idx, cfg)
	server := api.NewServer(pipeline)

	log.Printf("starting server on %s (%d messages indexed)", cfg.HTTPAddr, len(idx.Messages))
	if err := http.ListenAndServe(cfg.HTTPAddr, server.Handler()); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
