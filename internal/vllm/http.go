package vllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTP struct {
	APIKey string
	Client *http.Client
}

func NewHTTP(apiKey string) *HTTP {
	return &HTTP{
		APIKey: apiKey,
		Client: &http.Client{Timeout: 180 * time.Second},
	}
}

func (h *HTTP) Enabled() bool {
	return h.APIKey != ""
}

func (h *HTTP) PostJSON(ctx context.Context, url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("api %s status %d: %s", url, resp.StatusCode, string(respBody))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func (h *HTTP) GetJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.APIKey)

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("api %s status %d: %s", url, resp.StatusCode, string(respBody))
	}
	return json.Unmarshal(respBody, out)
}

func join(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (h *HTTP) FirstModel(ctx context.Context, modelsURL string) (string, error) {
	var resp modelsResponse
	if err := h.GetJSON(ctx, modelsURL, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no models at %s", modelsURL)
	}
	return resp.Data[0].ID, nil
}
