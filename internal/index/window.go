package index

import (
	"math"
	"sort"
	"strings"

	"github.com/tarantool-rag/internal/loader"
)

type Window struct {
	MessageDocIdxs []int
	Text           string
}

func buildWindows(messages []loader.Message, size, step int) []Window {
	if size <= 0 {
		size = 20
	}
	if step <= 0 {
		step = 5
	}
	if len(messages) == 0 {
		return nil
	}

	windows := make([]Window, 0, len(messages)/step+1)
	for start := 0; start < len(messages); start += step {
		end := start + size
		if end > len(messages) {
			end = len(messages)
		}

		idxs := make([]int, 0, end-start)
		parts := make([]string, 0, end-start)
		for docIdx := start; docIdx < end; docIdx++ {
			idxs = append(idxs, docIdx)
			parts = append(parts, loader.MessageDocument(messages[docIdx]))
		}

		windows = append(windows, Window{
			MessageDocIdxs: idxs,
			Text:           strings.Join(parts, "\n"),
		})

		if end == len(messages) {
			break
		}
	}
	return windows
}

func buildWindowInverted(windows []Window) (map[string][]Posting, []int, float64) {
	inverted := make(map[string][]Posting)
	lengths := make([]int, len(windows))
	var totalLen int

	for windowIdx, window := range windows {
		tokens := loader.Tokenize(window.Text)
		lengths[windowIdx] = len(tokens)
		totalLen += len(tokens)

		tf := make(map[string]int)
		for _, token := range tokens {
			tf[token]++
		}
		for token, freq := range tf {
			inverted[token] = append(inverted[token], Posting{
				DocIdx: windowIdx,
				TF:     freq,
			})
		}
	}

	avg := 0.0
	if len(windows) > 0 {
		avg = float64(totalLen) / float64(len(windows))
	}
	return inverted, lengths, avg
}

func (idx *Index) SearchWindows(query string, topK int) []SearchHit {
	if len(idx.Windows) == 0 {
		return nil
	}

	tokens := loader.Tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	scores := make(map[int]float64)
	seen := make(map[string]struct{})

	for _, term := range tokens {
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}

		postings, ok := idx.WindowInverted[term]
		if !ok {
			continue
		}

		n := float64(len(idx.Windows))
		df := float64(len(postings))
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))

		for _, posting := range postings {
			docLen := float64(idx.WindowDocLengths[posting.DocIdx])
			tf := float64(posting.TF)
			denom := tf + k1*(1-b+b*docLen/idx.WindowAvgDocLength)
			scores[posting.DocIdx] += idf * (tf * (k1 + 1)) / denom
		}
	}

	ranked := make([]SearchHit, 0)
	for windowIdx, score := range scores {
		if score <= 0 || windowIdx >= len(idx.Windows) {
			continue
		}
		for _, docIdx := range idx.Windows[windowIdx].MessageDocIdxs {
			msg := idx.Messages[docIdx]
			ranked = append(ranked, SearchHit{
				MessageID: msg.ID,
				Score:     score,
				Message:   msg,
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})
	return topUniqueMessages(ranked, topK)
}

func topUniqueMessages(hits []SearchHit, topK int) []SearchHit {
	if topK <= 0 {
		return hits
	}
	seen := make(map[int]struct{})
	out := make([]SearchHit, 0, topK)
	for _, hit := range hits {
		if _, ok := seen[hit.MessageID]; ok {
			continue
		}
		seen[hit.MessageID] = struct{}{}
		out = append(out, hit)
		if len(out) >= topK {
			break
		}
	}
	return out
}

func ReciprocalRankFusion(lists [][]SearchHit, topK int, k int) []SearchHit {
	if k <= 0 {
		k = 60
	}
	scores := make(map[int]float64)
	best := make(map[int]SearchHit)

	for _, list := range lists {
		for rank, hit := range list {
			scores[hit.MessageID] += 1.0 / (float64(k) + float64(rank+1))
			if prev, ok := best[hit.MessageID]; !ok || hit.Score > prev.Score {
				best[hit.MessageID] = hit
			}
		}
	}

	ranked := make([]SearchHit, 0, len(scores))
	for id, score := range scores {
		hit := best[id]
		hit.Score = score
		ranked = append(ranked, hit)
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}
	return ranked
}

func (idx *Index) NeighborHits(anchor []SearchHit, radius int) []SearchHit {
	if radius <= 0 || len(anchor) == 0 {
		return nil
	}

	seen := make(map[int]struct{})
	out := make([]SearchHit, 0)

	for _, hit := range anchor {
		anchorDoc, ok := idx.ByID[hit.MessageID]
		if !ok {
			continue
		}
		start := anchorDoc - radius
		if start < 0 {
			start = 0
		}
		end := anchorDoc + radius
		if end >= len(idx.Messages) {
			end = len(idx.Messages) - 1
		}

		for docIdx := start; docIdx <= end; docIdx++ {
			msg := idx.Messages[docIdx]
			if _, ok := seen[msg.ID]; ok {
				continue
			}
			seen[msg.ID] = struct{}{}
			out = append(out, SearchHit{
				MessageID: msg.ID,
				Score:     hit.Score * 0.5,
				Message:   msg,
			})
		}
	}
	return out
}
