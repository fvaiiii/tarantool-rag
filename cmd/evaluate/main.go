package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tarantool-rag/internal/api"
	"github.com/tarantool-rag/internal/config"
	"github.com/tarantool-rag/internal/eval"
	"github.com/tarantool-rag/internal/index"
	"github.com/tarantool-rag/internal/rag"
)

func main() {
	cfg := config.Load()

	datasetPath := flag.String("dataset", cfg.DatasetJSON, "path to dataset.json")
	indexDir := flag.String("index-dir", cfg.IndexDir, "index directory")
	topK := flag.Int("top-k", cfg.TopK, "retrieval top-k")
	ksRaw := flag.String("ks", "5,10,20,30", "k values for recall/hit metrics")
	output := flag.String("output", "data/eval_results.json", "output json path")
	flag.Parse()

	idx, err := index.Load(*indexDir)
	if err != nil {
		log.Fatalf("load index: %v", err)
	}

	dataset, err := eval.LoadDataset(*datasetPath)
	if err != nil {
		log.Fatalf("load dataset: %v", err)
	}

	pipeline := rag.NewPipeline(idx, cfg)
	ctx := context.Background()
	ks := api.ParseKS(*ksRaw, []int{5, 10, 20, 30})
	result, err := eval.EvaluateRetrieval(ctx, pipeline, dataset, *topK, ks)
	if err != nil {
		log.Fatalf("evaluate: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		log.Fatalf("mkdir output: %v", err)
	}
	f, err := os.Create(*output)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatalf("write output: %v", err)
	}

	fmt.Println("\n=== Retrieval metrics ===")
	for key, value := range result.Summary {
		fmt.Printf("%s: %.4f\n", key, value)
	}
	fmt.Printf("\nSaved to %s\n", *output)
}
