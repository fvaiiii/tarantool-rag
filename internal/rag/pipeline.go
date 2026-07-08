package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tarantool-rag/internal/config"
	"github.com/tarantool-rag/internal/index"
	"github.com/tarantool-rag/internal/loader"
	"github.com/tarantool-rag/internal/vllm"
)

type MessageDTO struct {
	ID    int     `json:"id"`
	Score float64 `json:"score"`
	Date  string  `json:"date"`
	From  string  `json:"from"`
	Text  string  `json:"text"`
}

type AskResult struct {
	Question   string       `json:"question"`
	Answer     string       `json:"answer"`
	MessageIDs []int        `json:"message_ids"`
	Messages   []MessageDTO `json:"messages"`
}

type Pipeline struct {
	Index              *index.Index
	VLLM               *vllm.Services
	TopK               int
	RetrieveCandidates int
	ContextMax         int
	SummarizeMinChars  int
	NeighborRadius     int
}

func NewPipeline(idx *index.Index, cfg config.Config) *Pipeline {
	return &Pipeline{
		Index:              idx,
		VLLM:               vllm.NewServices(cfg),
		TopK:               cfg.TopK,
		RetrieveCandidates: cfg.RetrieveCandidates,
		ContextMax:         cfg.ContextMessages,
		SummarizeMinChars:  cfg.SummarizeMinChars,
		NeighborRadius:     cfg.NeighborRadius,
	}
}

func (p *Pipeline) collectCandidates(query string) []index.SearchHit {
	queries := loader.ExpandQuery(query)
	lists := make([][]index.SearchHit, 0, len(queries)*2)

	for _, q := range queries {
		lists = append(lists, p.Index.Search(q, p.RetrieveCandidates))
		lists = append(lists, p.Index.SearchWindows(q, p.RetrieveCandidates))
	}

	return index.ReciprocalRankFusion(lists, p.RetrieveCandidates, 60)
}

func (p *Pipeline) expandWithNeighbors(hits []index.SearchHit, topK int) []index.SearchHit {
	if len(hits) == 0 {
		return hits
	}

	seedCount := min(10, len(hits))
	neighbors := p.Index.NeighborHits(hits[:seedCount], p.NeighborRadius)

	seen := make(map[int]struct{})
	out := make([]index.SearchHit, 0, topK)

	appendHit := func(hit index.SearchHit) {
		if _, ok := seen[hit.MessageID]; ok {
			return
		}
		seen[hit.MessageID] = struct{}{}
		out = append(out, hit)
	}

	for _, hit := range hits {
		appendHit(hit)
		if len(out) >= topK {
			return out
		}
	}
	for _, hit := range neighbors {
		appendHit(hit)
		if len(out) >= topK {
			return out
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Message.ID < out[j].Message.ID
	})
	return out
}

func (p *Pipeline) Retrieve(ctx context.Context, query string, topK int) ([]index.SearchHit, error) {
	if topK <= 0 {
		topK = p.TopK
	}

	candidates := p.collectCandidates(query)
	if len(candidates) == 0 {
		return nil, nil
	}

	reranked := candidates
	if p.VLLM.Reranker.Enabled() {
		rerankLimit := min(60, len(candidates))
		docs := make([]string, rerankLimit)
		for i := 0; i < rerankLimit; i++ {
			docs[i] = loader.MessageDocument(candidates[i].Message)
		}

		ranked, err := p.VLLM.Reranker.Rerank(ctx, query, docs)
		if err == nil && len(ranked) > 0 {
			tmp := make([]index.SearchHit, 0, rerankLimit)
			for _, item := range ranked {
				if item.Index < 0 || item.Index >= rerankLimit {
					continue
				}
				hit := candidates[item.Index]
				hit.Score = item.Score
				tmp = append(tmp, hit)
			}
			if len(tmp) > 0 {
				reranked = tmp
			}
		}
	}

	return p.expandWithNeighbors(reranked, topK), nil
}

func FormatContext(hits []index.SearchHit, limit int) string {
	if limit <= 0 {
		limit = len(hits)
	}

	blocks := make([]string, 0, limit)
	seen := make(map[int]struct{})
	for _, hit := range hits {
		if _, ok := seen[hit.MessageID]; ok {
			continue
		}
		seen[hit.MessageID] = struct{}{}

		msg := hit.Message
		author := msg.FromName
		if author == "" {
			author = msg.FromID
		}
		if author == "" {
			author = "unknown"
		}
		blocks = append(blocks, fmt.Sprintf(
			"[msg_id=%d | %s | %s]\n%s",
			msg.ID,
			msg.Date,
			author,
			msg.Text,
		))
		if len(blocks) >= limit {
			break
		}
	}
	return strings.Join(blocks, "\n\n")
}

func (p *Pipeline) Ask(ctx context.Context, question string, topK int, generate bool) (AskResult, error) {
	hits, err := p.Retrieve(ctx, question, topK)
	if err != nil {
		return AskResult{}, err
	}

	contextText := FormatContext(hits, p.ContextMax)
	answer := ""

	if generate {
		promptContext := contextText
		if p.VLLM.Summarizer.Enabled() && len(promptContext) >= p.SummarizeMinChars {
			summary, sumErr := p.VLLM.Summarizer.Summarize(ctx, question, promptContext)
			if sumErr == nil && strings.TrimSpace(summary) != "" {
				promptContext = summary
			}
		}

		if p.VLLM.LLM.Enabled() {
			answer, err = p.VLLM.LLM.AnswerFromContext(ctx, question, promptContext)
			if err != nil {
				return AskResult{}, err
			}
		} else {
			answer = fallbackAnswer(promptContext)
		}
	}

	result := AskResult{
		Question:   question,
		Answer:     answer,
		MessageIDs: make([]int, 0, len(hits)),
		Messages:   make([]MessageDTO, 0, len(hits)),
	}
	for _, hit := range hits {
		msg := hit.Message
		author := msg.FromName
		if author == "" {
			author = msg.FromID
		}
		result.MessageIDs = append(result.MessageIDs, hit.MessageID)
		result.Messages = append(result.Messages, MessageDTO{
			ID:    hit.MessageID,
			Score: hit.Score,
			Date:  msg.Date,
			From:  author,
			Text:  msg.Text,
		})
	}
	return result, nil
}

func fallbackAnswer(contextText string) string {
	if strings.TrimSpace(contextText) == "" {
		return "Не удалось найти релевантные сообщения в чате."
	}
	return "LLM API не настроен. Ниже найденные релевантные сообщения:\n\n" + contextText
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
