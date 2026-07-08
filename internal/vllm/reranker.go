package vllm

import (
	"context"
	"sort"
	"strings"
)

type Reranker struct {
	http  *HTTP
	base  string
	model string
}

func NewReranker(http *HTTP, base, model string) *Reranker {
	return &Reranker{http: http, base: strings.TrimRight(base, "/"), model: model}
}

func (c *Reranker) Enabled() bool {
	return c.http.Enabled() && c.model != ""
}

type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

type scoreRequest struct {
	Model string   `json:"model"`
	Text1 string   `json:"text_1"`
	Text2 []string `json:"text_2"`
}

type scoreResponse struct {
	Data []struct {
		Index int     `json:"index"`
		Score float64 `json:"score"`
	} `json:"data"`
}

type RankedDoc struct {
	Index int
	Score float64
}

func (c *Reranker) Rerank(ctx context.Context, query string, documents []string) ([]RankedDoc, error) {
	if !c.Enabled() || len(documents) == 0 {
		ranked := make([]RankedDoc, len(documents))
		for i := range documents {
			ranked[i] = RankedDoc{Index: i, Score: 0}
		}
		return ranked, nil
	}

	ranked, err := c.rerankViaRerank(ctx, query, documents)
	if err == nil {
		return ranked, nil
	}
	return c.rerankViaScore(ctx, query, documents)
}

func (c *Reranker) rerankViaRerank(ctx context.Context, query string, documents []string) ([]RankedDoc, error) {
	payload := rerankRequest{
		Model:     c.model,
		Query:     query,
		Documents: documents,
	}

	var parsed rerankResponse
	if err := c.http.PostJSON(ctx, join(c.base, "/rerank"), payload, &parsed); err != nil {
		return nil, err
	}

	ranked := make([]RankedDoc, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		ranked = append(ranked, RankedDoc{
			Index: item.Index,
			Score: item.RelevanceScore,
		})
	}
	sortRanked(ranked)
	return ranked, nil
}

func (c *Reranker) rerankViaScore(ctx context.Context, query string, documents []string) ([]RankedDoc, error) {
	payload := scoreRequest{
		Model: c.model,
		Text1: query,
		Text2: documents,
	}

	var parsed scoreResponse
	if err := c.http.PostJSON(ctx, join(c.base, "/score"), payload, &parsed); err != nil {
		return nil, err
	}

	ranked := make([]RankedDoc, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		ranked = append(ranked, RankedDoc{
			Index: item.Index,
			Score: item.Score,
		})
	}
	sortRanked(ranked)
	return ranked, nil
}

func sortRanked(ranked []RankedDoc) {
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})
}
