package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tarantool-rag/internal/config"
	"github.com/tarantool-rag/internal/vllm"
)

func main() {
	cfg := config.Load()
	if cfg.APIKey == "" {
		log.Fatal("API_KEY is empty — set it in .env")
	}

	services := vllm.NewServices(cfg)
	ctx := context.Background()

	if err := services.ResolveModels(ctx); err != nil {
		log.Fatalf("resolve models: %v", err)
	}

	fmt.Println("LLM model:", services.Models().LLM)
	fmt.Println("Embedding model:", services.Models().Embedding)
	fmt.Println("Reranker model:", services.Models().Reranker)
	fmt.Println("Summarizer model:", services.Models().Summarizer)

	vectors, err := services.Embedding.Embed(ctx, []string{"tarantool msgpack"})
	if err != nil {
		log.Fatalf("embedding test failed: %v", err)
	}
	fmt.Printf("embedding ok, dim=%d\n", len(vectors[0]))

	ranked, err := services.Reranker.Rerank(ctx, "oracle tarantool", []string{
		"можно подключить oracle через odbc",
		"поставь цивильный пермалинк",
	})
	if err != nil {
		log.Fatalf("reranker test failed: %v", err)
	}
	fmt.Printf("reranker ok, top score=%.4f index=%d\n", ranked[0].Score, ranked[0].Index)

	summary, err := services.Summarizer.Summarize(ctx, "как подключить oracle?", "[msg_id=8172] используйте odbc или jdbc")
	if err != nil {
		log.Fatalf("summarizer test failed: %v", err)
	}
	fmt.Printf("summarizer ok: %q\n", truncate(summary, 120))

	answer, err := services.LLM.AnswerFromContext(ctx, "как подключить oracle?", "[msg_id=8172] используйте odbc или jdbc")
	if err != nil {
		log.Fatalf("llm test failed: %v", err)
	}
	fmt.Printf("llm ok: %q\n", truncate(answer, 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
