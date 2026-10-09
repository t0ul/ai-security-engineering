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

// EmbedServer builds a Server spec for a llama.cpp instance restricted to embeddings
// (--embeddings + mean pooling), with an embeddings-based readiness probe. Use a
// dedicated instance (separate from the chat servers) on its own port.
func EmbedServer(name, bin, host, modelPath string, port, ctxSize int) Server {
	return Server{
		Name: name,
		Bin:  bin,
		Args: []string{
			"serve",
			"-m", modelPath,
			"--host", host,
			"--port", strconv.Itoa(port),
			"--ctx-size", strconv.Itoa(ctxSize),
			"--embeddings",
			"--pooling", "mean",
			// Embeddings are non-causal: the entire input is processed in ONE batch, so
			// the (physical) batch must be at least as large as the biggest chunk we
			// embed. The default is 512, which rejects any passage over ~512 tokens; size
			// both batches to the context so a chunk up to the full window embeds.
			"--batch-size", strconv.Itoa(ctxSize),
			"--ubatch-size", strconv.Itoa(ctxSize),
		},
		HealthURL: fmt.Sprintf("http://%s:%d/health", host, port),
		Ready:     EmbeddingReady(host, port),
	}
}

// EmbeddingReady returns a probe that succeeds only when the server returns a real
// embedding vector (llama.cpp answers 503 while the model loads).
func EmbeddingReady(host string, port int) func(context.Context) bool {
	url := fmt.Sprintf("http://%s:%d/v1/embeddings", host, port)
	payload := []byte(`{"model":"probe","input":"ping"}`)
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
