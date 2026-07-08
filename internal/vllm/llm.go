package vllm

import (
	"context"
	"fmt"
	"strings"
)

type LLM struct {
	http   *HTTP
	base   string
	model  string
}

func NewLLM(http *HTTP, base, model string) *LLM {
	return &LLM{http: http, base: strings.TrimRight(base, "/"), model: model}
}

func (c *LLM) Enabled() bool {
	return c.http.Enabled() && c.model != ""
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *LLM) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("llm is not configured")
	}

	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
	}

	var parsed chatResponse
	if err := c.http.PostJSON(ctx, join(c.base, "/chat/completions"), payload, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *LLM) AnswerFromContext(ctx context.Context, question, contextText string) (string, error) {
	systemPrompt := "Ты ассистент по истории чата Tarantool в Telegram. " +
		"Отвечай только на основе предоставленных сообщений. " +
		"Если информации недостаточно — честно скажи об этом. " +
		"Указывай id сообщений, на которые опираешься, в формате [msg_id=...]. " +
		"Отвечай на русском языке, структурированно и по делу."

	userPrompt := fmt.Sprintf(
		"Вопрос пользователя:\n%s\n\nРелевантные сообщения из чата:\n%s\n\nСформируй полный ответ на вопрос.",
		question,
		contextText,
	)
	return c.Generate(ctx, systemPrompt, userPrompt)
}
