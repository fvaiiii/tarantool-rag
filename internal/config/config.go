package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	APIKey string

	LLMAPIBase        string
	LLMModel          string
	EmbeddingAPIBase  string
	EmbeddingModel    string
	RerankerAPIBase   string
	RerankerModel     string
	SummarizerAPIBase string
	SummarizerModel   string

	TarantoolJSON   string
	DatasetJSON     string
	IndexDir        string
	TopK            int
	RetrieveCandidates int
	ContextMessages    int
	SummarizeMinChars  int
	WindowSize         int
	WindowStep         int
	NeighborRadius     int
	HTTPAddr           string
}

func Load() Config {
	loadDotEnv(".env")

	return Config{
		APIKey: env("API_KEY", ""),

		LLMAPIBase:        env("LLM_API_BASE", "https://cute.hacode.ru/vllm/llm/v1"),
		LLMModel:          env("LLM_MODEL", "lovedheart/Qwen3.5-9B-FP8"),
		EmbeddingAPIBase:  env("EMBEDDING_API_BASE", "https://cute.hacode.ru/vllm/embedding/v1"),
		EmbeddingModel:    env("EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-0.6B"),
		RerankerAPIBase:   env("RERANKER_API_BASE", "https://cute.hacode.ru/vllm/reranker/v1"),
		RerankerModel:     env("RERANKER_MODEL", "nvidia/llama-nemotron-rerank-1b-v2"),
		SummarizerAPIBase: env("SUMMARIZER_API_BASE", "https://cute.hacode.ru/vllm/summarizer/v1"),
		SummarizerModel:   env("SUMMARIZER_MODEL", "cute-team/teams-summarizator-granite-2B"),

		TarantoolJSON:      env("TARANTOOL_JSON", "data/tarantool.json"),
		DatasetJSON:          env("DATASET_JSON", "data/dataset.json"),
		IndexDir:             env("INDEX_DIR", "data/index"),
		TopK:                 envInt("TOP_K", 30),
		RetrieveCandidates: envInt("RETRIEVE_CANDIDATES", 100),
		ContextMessages:    envInt("CONTEXT_MESSAGES", 20),
		SummarizeMinChars:  envInt("SUMMARIZE_MIN_CHARS", 12000),
		WindowSize:         envInt("WINDOW_SIZE", 40),
		WindowStep:         envInt("WINDOW_STEP", 5),
		NeighborRadius:     envInt("NEIGHBOR_RADIUS", 25),
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
