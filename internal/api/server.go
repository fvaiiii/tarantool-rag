package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/tarantool-rag/internal/rag"
)

//go:embed demo.html
var demoHTML []byte

type Server struct {
	Pipeline *rag.Pipeline
}

func NewServer(pipeline *rag.Pipeline) *Server {
	return &Server{Pipeline: pipeline}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleDemo)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /search", s.handleSearch)
	mux.HandleFunc("POST /ask", s.handleAsk)
	return mux
}

func (s *Server) handleDemo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(demoHTML)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"indexed_messages":   len(s.Pipeline.Index.Messages),
		"llm_enabled":        s.Pipeline.VLLM.LLM.Enabled(),
		"embedding_enabled":  s.Pipeline.VLLM.Embedding.Enabled(),
		"reranker_enabled":   s.Pipeline.VLLM.Reranker.Enabled(),
		"summarizer_enabled": s.Pipeline.VLLM.Summarizer.Enabled(),
	})
}

type searchRequest struct {
	Query string `json:"query"`
	TopK  *int   `json:"top_k"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	topK := s.Pipeline.TopK
	if req.TopK != nil && *req.TopK > 0 {
		topK = *req.TopK
	}

	hits, err := s.Pipeline.Retrieve(r.Context(), req.Query, topK)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	result := map[string]any{
		"query":       req.Query,
		"message_ids": make([]int, 0, len(hits)),
		"messages":    make([]rag.MessageDTO, 0, len(hits)),
	}
	for _, hit := range hits {
		msg := hit.Message
		author := msg.FromName
		if author == "" {
			author = msg.FromID
		}
		result["message_ids"] = append(result["message_ids"].([]int), hit.MessageID)
		result["messages"] = append(result["messages"].([]rag.MessageDTO), rag.MessageDTO{
			ID:    hit.MessageID,
			Score: hit.Score,
			Date:  msg.Date,
			From:  author,
			Text:  msg.Text,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

type askRequest struct {
	Question       string `json:"question"`
	TopK           *int   `json:"top_k"`
	GenerateAnswer *bool  `json:"generate_answer"`
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}

	topK := s.Pipeline.TopK
	if req.TopK != nil && *req.TopK > 0 {
		topK = *req.TopK
	}
	generate := true
	if req.GenerateAnswer != nil {
		generate = *req.GenerateAnswer
	}

	result, err := s.Pipeline.Ask(r.Context(), req.Question, topK, generate)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func ParseKS(raw string, fallback []int) []int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	ks := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, err := strconv.Atoi(part)
		if err == nil && k > 0 {
			ks = append(ks, k)
		}
	}
	if len(ks) == 0 {
		return fallback
	}
	return ks
}
