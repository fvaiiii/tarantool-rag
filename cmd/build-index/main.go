package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tarantool-rag/internal/config"
	"github.com/tarantool-rag/internal/index"
	"github.com/tarantool-rag/internal/loader"
)

func main() {
	cfg := config.Load()
	input := flag.String("input", cfg.TarantoolJSON, "path to tarantool.json")
	output := flag.String("output", cfg.IndexDir, "directory for index files")
	flag.Parse()

	start := time.Now()
	log.Printf("loading messages from %s", *input)
	messages, err := loader.LoadMessages(*input)
	if err != nil {
		log.Fatalf("load messages: %v", err)
	}
	log.Printf("loaded %d messages in %.1fs", len(messages), time.Since(start).Seconds())

	start = time.Now()
	log.Println("building BM25 index...")
	idx := index.BuildWithOptions(messages, cfg.WindowSize, cfg.WindowStep)
	log.Printf("index built in %.1fs", time.Since(start).Seconds())

	if err := idx.Save(*output); err != nil {
		log.Fatalf("save index: %v", err)
	}
	fmt.Printf("index saved to %s\n", *output)
}
