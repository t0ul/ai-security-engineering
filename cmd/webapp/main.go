// Command webapp is the all-in-one local agent app: it creates a drop folder
// (on your Desktop by default), watches it for dropped .txt emails and turns them
// into signed .ics events, and serves the operator console — Calendar, Security
// (run the scorecard), and Incidents (replay). One binary, loopback only.
//
//	webapp [-addr 127.0.0.1:8788] [-drop ~/Desktop/Email-to-Calendar]
//
// Drop a .txt email into <drop>/inbox and it appears on the Calendar tab.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/agent/roster"
	"github.com/t0ul/ai-security-engineering/agent/tool"
	"github.com/t0ul/ai-security-engineering/agent/watcher"
	"github.com/t0ul/ai-security-engineering/controlplane"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"github.com/t0ul/ai-security-engineering/provenance"
	"github.com/t0ul/ai-security-engineering/rag"
	"github.com/t0ul/ai-security-engineering/webapp"
	"github.com/t0ul/gledger"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8789", "listen address (loopback)")
	drop := flag.String("drop", defaultDrop(), "drop folder (inbox/outbox/processed/logs)")
	allow := flag.String("allow", "schools.nyc.gov,nyc.gov,ps51eliashowe.org,schoolsaccount.nyc", "comma-separated egress allowlist for action/handbook links (parent domains cover subdomains); set empty to deny all")
	vmURL := flag.String("microvm", "http://127.0.0.1:5000", "MicroVM vsock bridge for in-sandbox fetches")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := watcher.NewConfig(*drop)
	auditPath := filepath.Join(cfg.Logs, "audit.jsonl")
	audit, err := gledger.Open(auditPath, "watcher")
	if err != nil {
		log.Fatalf("webapp: %v", err)
	}

	reg := tool.NewRegistry()
	roster.Register(reg)
	pipe := &pipeline.Pipeline{Registry: reg, Audit: audit, OutboxDir: cfg.Outbox, DefaultYear: time.Now().Year(), ToolNames: roster.Names()}

	// Provenance (M20): a persistent agent identity signs every emitted .ics, so
	// the UI can prove the agent produced it unaltered. The ed25519 seed lives in
	// the drop dir (0600); generated once, reused across restarts.
	signer, verifier := agentIdentity(filepath.Join(*drop, "agent.key"))
	pipe.Signer = signer

	// Kill switch (M18): Pause halts processing of new drops; the console toggles it.
	safety := controlplane.NewSafety(audit)
	pipe.Halted = func() bool { return !safety.AllowRequest() }
	if corpus, cerr := rag.Open(filepath.Join(*drop, "corpus.db")); cerr == nil {
		defer corpus.Close()
		pipe.Index = func(traceID, source, rawText string) error {
			return corpus.Add(rag.Doc{ID: traceID, Text: rawText, Prov: rag.Untrusted})
		}
	}

	w := &watcher.Watcher{Pipe: pipe, Cfg: cfg}
	if err := w.EnsureDirs(); err != nil {
		log.Fatalf("webapp: %v", err)
	}
	go func() {
		if err := w.Watch(ctx, func(s string) { log.Println("[watcher]", s) }); err != nil {
			log.Println("[watcher] stopped:", err)
		}
	}()

	// Action-link egress: deny-by-default allowlist, and an in-MicroVM fetcher so
	// a link from an untrusted email/handbook executes in the sandbox, not on the
	// host. Without -allow, action links are refused.
	var allowHosts []string
	for _, h := range strings.Split(*allow, ",") {
		if h = strings.TrimSpace(h); h != "" {
			allowHosts = append(allowHosts, h)
		}
	}
	egress := netpolicy.Policy{Allow: allowHosts}
	interp := controlplane.NewInterpreter(*vmURL, audit)
	fetchTool := controlplane.SandboxFetchTool(interp)
	fetch := func(ctx context.Context, url string) (string, error) {
		out, err := fetchTool.Handler(ctx, map[string]any{"url": url, "trace_id": gledger.NewTraceID()})
		if err != nil {
			return "", err
		}
		s, _ := out.(map[string]any)["output"].(string)
		return s, nil
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: (&webapp.Server{
			AuditPath: auditPath, OutboxDir: cfg.Outbox, InboxPath: cfg.Inbox,
			Egress: egress, Fetch: fetch, Verifier: verifier, Safety: safety,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { <-ctx.Done(); srv.Close() }()

	fmt.Printf("agent console: http://%s\n", *addr)
	fmt.Printf("drop .txt emails into: %s\n", cfg.Inbox)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("webapp: %v", err)
	}
}

// agentIdentity loads (or creates) the persistent ed25519 seed at path and
// returns the signer + a verifier trusting that key. On any error it falls back
// to an ephemeral key so the app still runs (signatures just won't verify across
// restarts).
func agentIdentity(path string) (*provenance.Signer, *provenance.Verifier) {
	const keyID = "email-agent"
	seed, err := os.ReadFile(path)
	if err != nil || len(seed) != ed25519.SeedSize {
		seed = make([]byte, ed25519.SeedSize)
		if _, rerr := rand.Read(seed); rerr == nil {
			_ = os.WriteFile(path, seed, 0o600)
		}
	}
	signer, pub, serr := provenance.SignerFromSeed(keyID, seed)
	if serr != nil {
		signer, pub, _ = provenance.NewSigner(keyID) // ephemeral fallback
	}
	return signer, provenance.NewVerifier().Trust(keyID, pub)
}

// defaultDrop is ~/Desktop/Email-to-Calendar (Mac-friendly), falling back to the
// working directory if the home/Desktop can't be resolved.
func defaultDrop() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Email-to-Calendar"
	}
	return filepath.Join(home, "Desktop", "Email-to-Calendar")
}
