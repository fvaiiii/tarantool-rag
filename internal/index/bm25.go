package index

import (
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/tarantool-rag/internal/loader"
)

const (
	k1 = 1.5
	b  = 0.75
)

type Posting struct {
	DocIdx int
	TF     int
}

type SearchHit struct {
	MessageID int
	Score     float64
	Message   loader.Message
}

type Index struct {
	Messages     []loader.Message
	ByID         map[int]int
	Inverted     map[string][]Posting
	DocLengths   []int
	AvgDocLength float64

	Windows            []Window
	WindowInverted     map[string][]Posting
	WindowDocLengths   []int
	WindowAvgDocLength float64
}

func Build(messages []loader.Message) *Index {
	return BuildWithOptions(messages, 20, 5)
}

func BuildWithOptions(messages []loader.Message, windowSize, windowStep int) *Index {
	idx := &Index{
		Messages:   messages,
		ByID:       make(map[int]int, len(messages)),
		Inverted:   make(map[string][]Posting),
		DocLengths: make([]int, len(messages)),
	}

	var totalLen int
	for docIdx, msg := range messages {
		idx.ByID[msg.ID] = docIdx
		tokens := loader.Tokenize(loader.MessageDocument(msg))
		idx.DocLengths[docIdx] = len(tokens)
		totalLen += len(tokens)

		tf := make(map[string]int)
		for _, token := range tokens {
			tf[token]++
		}
		for token, freq := range tf {
			idx.Inverted[token] = append(idx.Inverted[token], Posting{
				DocIdx: docIdx,
				TF:     freq,
			})
		}
	}

	if len(messages) > 0 {
		idx.AvgDocLength = float64(totalLen) / float64(len(messages))
	}

	idx.Windows = buildWindows(messages, windowSize, windowStep)
	idx.WindowInverted, idx.WindowDocLengths, idx.WindowAvgDocLength = buildWindowInverted(idx.Windows)

	return idx
}

func (idx *Index) idf(term string) float64 {
	postings, ok := idx.Inverted[term]
	if !ok {
		return 0
	}
	n := float64(len(idx.Messages))
	df := float64(len(postings))
	return math.Log(1 + (n-df+0.5)/(df+0.5))
}

func (idx *Index) Search(query string, topK int) []SearchHit {
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

		postings, ok := idx.Inverted[term]
		if !ok {
			continue
		}

		idf := idx.idf(term)
		for _, posting := range postings {
			docLen := float64(idx.DocLengths[posting.DocIdx])
			tf := float64(posting.TF)
			denom := tf + k1*(1-b+b*docLen/idx.AvgDocLength)
			score := idf * (tf * (k1 + 1)) / denom
			scores[posting.DocIdx] += score
		}
	}

	if len(scores) == 0 {
		return nil
	}

	ranked := make([]SearchHit, 0, len(scores))
	for docIdx, score := range scores {
		if score <= 0 {
			continue
		}
		msg := idx.Messages[docIdx]
		ranked = append(ranked, SearchHit{
			MessageID: msg.ID,
			Score:     score,
			Message:   msg,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}
	return ranked
}

func (idx *Index) Get(messageID int) (loader.Message, bool) {
	docIdx, ok := idx.ByID[messageID]
	if !ok {
		return loader.Message{}, false
	}
	return idx.Messages[docIdx], true
}

type savedIndex struct {
	Messages     []loader.Message
	Inverted     map[string][]Posting
	DocLengths   []int
	AvgDocLength float64

	Windows            []Window
	WindowInverted     map[string][]Posting
	WindowDocLengths   []int
	WindowAvgDocLength float64
}

func (idx *Index) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	f, err := os.Create(filepath.Join(dir, "index.gob"))
	if err != nil {
		return fmt.Errorf("create index file: %w", err)
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	if err := enc.Encode(savedIndex{
		Messages:           idx.Messages,
		Inverted:           idx.Inverted,
		DocLengths:         idx.DocLengths,
		AvgDocLength:       idx.AvgDocLength,
		Windows:            idx.Windows,
		WindowInverted:     idx.WindowInverted,
		WindowDocLengths:   idx.WindowDocLengths,
		WindowAvgDocLength: idx.WindowAvgDocLength,
	}); err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	return nil
}

func Load(dir string) (*Index, error) {
	f, err := os.Open(filepath.Join(dir, "index.gob"))
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer f.Close()

	var saved savedIndex
	if err := gob.NewDecoder(f).Decode(&saved); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}

	idx := &Index{
		Messages:           saved.Messages,
		ByID:               make(map[int]int, len(saved.Messages)),
		Inverted:           saved.Inverted,
		DocLengths:         saved.DocLengths,
		AvgDocLength:       saved.AvgDocLength,
		Windows:            saved.Windows,
		WindowInverted:     saved.WindowInverted,
		WindowDocLengths:   saved.WindowDocLengths,
		WindowAvgDocLength: saved.WindowAvgDocLength,
	}
	for docIdx, msg := range saved.Messages {
		idx.ByID[msg.ID] = docIdx
	}
	return idx, nil
}
