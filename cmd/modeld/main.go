// Command modeld brings up the local dual-SLM runtime with one command: the
// Llama-3.2-3B planner and the Qwen-1.5B coder, each served by llama.cpp. It
// replaces the two hand-run `llama serve ...` shells — it launches both, waits
// for their /health endpoints, streams prefixed logs, and shuts both down
// cleanly on Ctrl-C.
//
//	modeld                      # defaults: planner :11435, coder :11436
//	modeld -bin /path/to/llama -dir set-up/vm-assets
//
// gouncer routes the logical "planner"/"coder" models to these ports; keep the
// bind host at 127.0.0.1 (least exposure) unless a gateway on another host needs
// them.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/internal/modelserve"
)

func main() {
	defaultBin := filepath.Join(homeDir(), ".llama-app", "llama")

	bin := flag.String("bin", defaultBin, "llama.cpp server binary")
	dir := flag.String("dir", "set-up/vm-assets", "directory holding the .gguf models")
	host := flag.String("host", "127.0.0.1", "bind host for the model servers")
	plannerModel := flag.String("planner-model", "llama-3.2-3b.gguf", "planner GGUF filename")
	plannerPort := flag.Int("planner-port", 11435, "planner port")
	plannerCtx := flag.Int("planner-ctx", 8192, "planner context size")
	coderModel := flag.String("coder-model", "qwen1.5b.gguf", "coder GGUF filename")
	coderPort := flag.Int("coder-port", 11436, "coder port")
	coderCtx := flag.Int("coder-ctx", 4096, "coder context size")
	ready := flag.Duration("ready", 120*time.Second, "per-server readiness timeout")
	flag.Parse()

	sup := &modelserve.Supervisor{
		Servers: []modelserve.Server{
			server("planner", *bin, *host, filepath.Join(*dir, *plannerModel), *plannerPort, *plannerCtx),
			server("coder", *bin, *host, filepath.Join(*dir, *coderModel), *coderPort, *coderCtx),
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("modeld: starting dual-SLM runtime (Ctrl-C to stop)...")
	if err := sup.Run(ctx, *ready); err != nil {
		log.Fatalf("modeld: %v", err)
	}
	fmt.Println("modeld: stopped.")
}

func server(name, bin, host, modelPath string, port, ctxSize int) modelserve.Server {
	return modelserve.Server{
		Name: name,
		Bin:  bin,
		Args: []string{
			"serve",
			"-m", modelPath,
			"--host", host,
			"--port", strconv.Itoa(port),
			"--ctx-size", strconv.Itoa(ctxSize),
		},
		// HealthURL backs the preflight "already serving?" check; Ready is the
		// authoritative gate — a real 1-token completion, so "ready" means the
		// model loaded and actually generated, not just that the socket is open.
		HealthURL: fmt.Sprintf("http://%s:%d/health", host, port),
		Ready:     completionReady(host, port),
	}
}

// completionReady returns a probe that succeeds only when the server answers a
// minimal chat completion. llama.cpp returns 503 while the model loads, so this
// gates on genuine generation readiness.
func completionReady(host string, port int) func(context.Context) bool {
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

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}
