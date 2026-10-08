package controlplane

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

// LLMClient calls an OpenAI-style /v1/chat/completions endpoint. In the fleet
// that endpoint is the gouncer gateway (replacing the Python LiteLLM proxy):
// gouncer allowlists the logical model names ("planner", "coder"), scrubs and
// audits, then forwards to the llama.cpp upstreams. The client is deliberately
// thin — all policy lives in the gateway it points at.
type LLMClient struct {
	BaseURL string // gateway chat endpoint; default http://127.0.0.1:4000/v1/chat/completions
	HTTP    *http.Client
}

// NewLLMClient returns a client for the gouncer gateway at baseURL. An empty
// baseURL uses the default local gateway endpoint.
func NewLLMClient(baseURL string) *LLMClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:4000/v1/chat/completions"
	}
	return &LLMClient{BaseURL: baseURL, HTTP: &http.Client{Timeout: 120 * time.Second}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stop        []string      `json:"stop,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// CompleteOpts are the per-call generation knobs (defaults match the Python
// orchestrator: temperature 0.1).
type CompleteOpts struct {
	Temperature float64
	MaxTokens   int
	Stop        []string
}

// Complete sends a system+user turn to the named logical model and returns the
// assistant text. traceID is forwarded as X-Trace-Id so the gateway correlates
// its audit spans with the control plane's.
func (c *LLMClient) Complete(ctx context.Context, traceID, model, system, user string, opts CompleteOpts) (string, error) {
	if opts.Temperature == 0 {
		opts.Temperature = 0.1
	}
	reqBody := chatRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stop:        opts.Stop,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway %s: status %d: %s", model, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode %s response: %w", model, err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("gateway %s: empty choices", model)
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
