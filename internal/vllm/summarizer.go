package vllm

import (
	"context"
	"fmt"
	"strings"
)

type Summarizer struct {
	http  *HTTP
	base  string
	model string
}

func NewSummarizer(http *HTTP, base, model string) *Summarizer {
	return &Summarizer{http: http, base: strings.TrimRight(base, "/"), model: model}
}

func (c *Summarizer) Enabled() bool {
	return c.http.Enabled() && c.model != ""
}

func (c *Summarizer) Summarize(ctx context.Context, question, contextText string) (string, error) {
	if !c.Enabled() {
		return contextText, nil
	}

	systemPrompt := "Ты сжимаешь фрагменты переписки из чата Tarantool. " +
		"Сохраняй факты, имена, версии, команды, ссылки и id сообщений [msg_id=...]. " +
		"Убирай повторы и шум. Пиши на русском."

	userPrompt := fmt.Sprintf(
		"Вопрос пользователя:\n%s\n\nСообщения чата для сжатия:\n%s\n\nСделай компактное summary, пригодное для ответа на вопрос.",
		question,
		contextText,
	)

	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
	}

	var parsed chatResponse
	if err := c.http.PostJSON(ctx, join(c.base, "/chat/completions"), payload, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty summarizer response")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
