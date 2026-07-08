package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/tarantool-rag/internal/rag"
)

type Dataset struct {
	Meta      map[string]any `json:"meta"`
	Questions []Question     `json:"questions"`
}

type Question struct {
	ID                 int    `json:"id"`
	Question           string `json:"question"`
	QType              string `json:"qtype"`
	IdealAnswer        string `json:"ideal_answer"`
	RelevantMessageIDs []int  `json:"relevant_message_ids"`
}

type QuestionMetrics struct {
	ID        int                `json:"id"`
	QType     string             `json:"qtype,omitempty"`
	Recall    map[string]float64 `json:"recall"`
	Hit       map[string]float64 `json:"hit"`
	MRR       float64            `json:"mrr"`
	FoundIDs  []int              `json:"found_ids"`
	MissedIDs []int              `json:"missed_ids"`
}

type Result struct {
	Meta        map[string]any     `json:"meta"`
	TopK        int                `json:"top_k"`
	NQuestions  int                `json:"n_questions"`
	Summary     map[string]float64 `json:"summary"`
	PerQuestion []QuestionMetrics  `json:"per_question"`
}

func LoadDataset(path string) (Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Dataset{}, err
	}
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return Dataset{}, err
	}
	return ds, nil
}

func recallAtK(retrieved []int, relevant map[int]struct{}, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	found := 0
	limit := k
	if limit > len(retrieved) {
		limit = len(retrieved)
	}
	for i := 0; i < limit; i++ {
		if _, ok := relevant[retrieved[i]]; ok {
			found++
		}
	}
	return float64(found) / float64(len(relevant))
}

func hitAtK(retrieved []int, relevant map[int]struct{}, k int) float64 {
	limit := k
	if limit > len(retrieved) {
		limit = len(retrieved)
	}
	for i := 0; i < limit; i++ {
		if _, ok := relevant[retrieved[i]]; ok {
			return 1
		}
	}
	return 0
}

func mrr(retrieved []int, relevant map[int]struct{}) float64 {
	for rank, id := range retrieved {
		if _, ok := relevant[id]; ok {
			return 1.0 / float64(rank+1)
		}
	}
	return 0
}

func EvaluateRetrieval(ctx context.Context, pipeline *rag.Pipeline, dataset Dataset, topK int, ks []int) (Result, error) {
	summary := make(map[string]float64)
	for _, k := range ks {
		summary[fmt.Sprintf("recall@%d", k)] = 0
		summary[fmt.Sprintf("hit@%d", k)] = 0
	}
	summary["mrr"] = 0

	perQuestion := make([]QuestionMetrics, 0, len(dataset.Questions))

	for i, q := range dataset.Questions {
		fmt.Printf("evaluating %d/%d (id=%d)\n", i+1, len(dataset.Questions), q.ID)
		relevant := make(map[int]struct{}, len(q.RelevantMessageIDs))
		for _, id := range q.RelevantMessageIDs {
			relevant[id] = struct{}{}
		}

		hits, err := pipeline.Retrieve(ctx, q.Question, topK)
		if err != nil {
			return Result{}, fmt.Errorf("question %d: %w", q.ID, err)
		}

		retrieved := make([]int, 0, len(hits))
		for _, hit := range hits {
			retrieved = append(retrieved, hit.MessageID)
		}

		metrics := QuestionMetrics{
			ID:        q.ID,
			QType:     q.QType,
			Recall:    make(map[string]float64),
			Hit:       make(map[string]float64),
			FoundIDs:  []int{},
			MissedIDs: []int{},
		}

		retrievedSet := make(map[int]struct{}, len(retrieved))
		for _, id := range retrieved {
			retrievedSet[id] = struct{}{}
		}
		for id := range relevant {
			if _, ok := retrievedSet[id]; ok {
				metrics.FoundIDs = append(metrics.FoundIDs, id)
			} else {
				metrics.MissedIDs = append(metrics.MissedIDs, id)
			}
		}
		sort.Ints(metrics.FoundIDs)
		sort.Ints(metrics.MissedIDs)

		for _, k := range ks {
			rk := fmt.Sprintf("recall@%d", k)
			hk := fmt.Sprintf("hit@%d", k)
			metrics.Recall[rk] = recallAtK(retrieved, relevant, k)
			metrics.Hit[hk] = hitAtK(retrieved, relevant, k)
			summary[rk] += metrics.Recall[rk]
			summary[hk] += metrics.Hit[hk]
		}

		metrics.MRR = mrr(retrieved, relevant)
		summary["mrr"] += metrics.MRR
		perQuestion = append(perQuestion, metrics)
	}

	n := float64(len(dataset.Questions))
	if n > 0 {
		for key := range summary {
			summary[key] /= n
		}
	}

	return Result{
		Meta:        dataset.Meta,
		TopK:        topK,
		NQuestions:  len(dataset.Questions),
		Summary:     summary,
		PerQuestion: perQuestion,
	}, nil
}
