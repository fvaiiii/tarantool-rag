package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var tokenRe = regexp.MustCompile(`(?i)[\p{L}\p{N}_]+`)

type Message struct {
	ID        int    `json:"id"`
	Date      string `json:"date"`
	FromName  string `json:"from"`
	FromID    string `json:"from_id"`
	Text      string `json:"text"`
	ReplyToID *int   `json:"reply_to_message_id,omitempty"`
}

type telegramExport struct {
	Messages []rawMessage `json:"messages"`
}

type rawMessage struct {
	ID       int             `json:"id"`
	Type     string          `json:"type"`
	Date     string          `json:"date"`
	From     *string         `json:"from"`
	FromID   *string         `json:"from_id"`
	Text     json.RawMessage `json:"text"`
	ReplyTo  *int            `json:"reply_to_message_id"`
}

func LoadMessages(path string) ([]Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var export telegramExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	messages := make([]Message, 0, len(export.Messages))
	for _, raw := range export.Messages {
		if raw.Type != "message" {
			continue
		}

		text := flattenText(raw.Text)
		if strings.TrimSpace(text) == "" {
			continue
		}

		msg := Message{
			ID:       raw.ID,
			Date:     raw.Date,
			Text:     text,
			ReplyToID: raw.ReplyTo,
		}
		if raw.From != nil {
			msg.FromName = *raw.From
		}
		if raw.FromID != nil {
			msg.FromID = *raw.FromID
		}
		messages = append(messages, msg)
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].ID < messages[j].ID
	})

	return messages, nil
}

func flattenText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return string(raw)
	}

	var b strings.Builder
	for _, part := range parts {
		var text string
		if err := json.Unmarshal(part, &text); err == nil {
			b.WriteString(text)
			continue
		}

		var obj struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(part, &obj); err == nil {
			b.WriteString(obj.Text)
		}
	}
	return b.String()
}

func Tokenize(text string) []string {
	text = strings.ToLower(text)
	matches := tokenRe.FindAllString(text, -1)
	tokens := make([]string, 0, len(matches))
	for _, t := range matches {
		if len([]rune(t)) > 1 {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func MessageDocument(msg Message) string {
	author := msg.FromName
	if author == "" {
		author = msg.FromID
	}
	if author == "" {
		author = "unknown"
	}
	return author + ": " + msg.Text
}
