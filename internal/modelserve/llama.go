package modelserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// LlamaServer builds a Server spec for one llama.cpp `serve` instance, with a
// completion-based readiness probe: readiness means the model loaded and
// generated a token, not merely that the socket is open.
func LlamaServer(name, bin, host, modelPath string, port, ctxSize int) Server {
	return Server{
		Name: name,
		Bin:  bin,
		Args: []string{
			"serve",
			"-m", modelPath,
			"--host", host,
			"--port", strconv.Itoa(port),
			"--ctx-size", strconv.Itoa(ctxSize),
		},
		HealthURL: fmt.Sprintf("http://%s:%d/health", host, port),
		Ready:     CompletionReady(host, port),
	}
}

// CompletionReady returns a probe that succeeds only when the server answers a
// minimal chat completion. llama.cpp returns 503 while the model loads, so this
// gates on genuine generation readiness.
func CompletionReady(host string, port int) func(context.Context) bool {
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", host, port)
	payload := []byte(`{"model":"probe","messages":[{"role":"user","content":"ping"}],"max_tokens":1,"temperature":0}`)
	client := &http.Client{Timeout: 15 * time.Second}
	return func(ctx context.Context) bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode == http.StatusOK
	}
}
