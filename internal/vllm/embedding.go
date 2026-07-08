package vllm

import (
	"context"
	"strings"
)

type Embedding struct {
	http  *HTTP
	base  string
	model string
}

func NewEmbedding(http *HTTP, base, model string) *Embedding {
	return &Embedding{http: http, base: strings.TrimRight(base, "/"), model: model}
}

func (c *Embedding) Enabled() bool {
	return c.http.Enabled() && c.model != ""
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (c *Embedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if len(texts) == 0 {
		return nil, nil
	}

	payload := embeddingRequest{
		Model: c.model,
		Input: texts,
	}

	var parsed embeddingResponse
	if err := c.http.PostJSON(ctx, join(c.base, "/embeddings"), payload, &parsed); err != nil {
		return nil, err
	}

	out := make([][]float32, len(texts))
	for _, item := range parsed.Data {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Embedding
		}
	}
	return out, nil
}

func (c *Embedding) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	vectors, err := c.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}
	return vectors[0], nil
}
