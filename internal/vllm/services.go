package vllm

import (
	"context"

	"github.com/tarantool-rag/internal/config"
)

type Services struct {
	HTTP       *HTTP
	LLM        *LLM
	Embedding  *Embedding
	Reranker   *Reranker
	Summarizer *Summarizer
}

func NewServices(cfg config.Config) *Services {
	http := NewHTTP(cfg.APIKey)
	return &Services{
		HTTP:       http,
		LLM:        NewLLM(http, cfg.LLMAPIBase, cfg.LLMModel),
		Embedding:  NewEmbedding(http, cfg.EmbeddingAPIBase, cfg.EmbeddingModel),
		Reranker:   NewReranker(http, cfg.RerankerAPIBase, cfg.RerankerModel),
		Summarizer: NewSummarizer(http, cfg.SummarizerAPIBase, cfg.SummarizerModel),
	}
}

func (s *Services) ResolveModels(ctx context.Context) error {
	if s.LLM.model == "" {
		model, err := s.HTTP.FirstModel(ctx, join(s.LLM.base, "/models"))
		if err != nil {
			return err
		}
		s.LLM.model = model
	}
	if s.Embedding.model == "" {
		model, err := s.HTTP.FirstModel(ctx, join(s.Embedding.base, "/models"))
		if err != nil {
			return err
		}
		s.Embedding.model = model
	}
	if s.Reranker.model == "" {
		model, err := s.HTTP.FirstModel(ctx, join(s.Reranker.base, "/models"))
		if err != nil {
			return err
		}
		s.Reranker.model = model
	}
	if s.Summarizer.model == "" {
		model, err := s.HTTP.FirstModel(ctx, join(s.Summarizer.base, "/models"))
		if err != nil {
			return err
		}
		s.Summarizer.model = model
	}
	return nil
}

type ModelsInfo struct {
	LLM        string
	Embedding  string
	Reranker   string
	Summarizer string
}

func (s *Services) Models() ModelsInfo {
	return ModelsInfo{
		LLM:        s.LLM.model,
		Embedding:  s.Embedding.model,
		Reranker:   s.Reranker.model,
		Summarizer: s.Summarizer.model,
	}
}
