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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/agent/extractor"
	"github.com/t0ul/ai-security-engineering/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/agent/tool"
	"github.com/t0ul/ai-security-engineering/agent/watcher"
	"github.com/t0ul/ai-security-engineering/rag"
	"github.com/t0ul/ai-security-engineering/webapp"
	"github.com/t0ul/gledger"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8789", "listen address (loopback)")
	drop := flag.String("drop", defaultDrop(), "drop folder (inbox/outbox/processed/logs)")
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
	reg.Register(extractor.New())
	pipe := &pipeline.Pipeline{Registry: reg, Audit: audit, OutboxDir: cfg.Outbox, DefaultYear: 2026}
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

	srv := &http.Server{
		Addr:              *addr,
		Handler:           (&webapp.Server{AuditPath: auditPath, OutboxDir: cfg.Outbox, InboxPath: cfg.Inbox}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { <-ctx.Done(); srv.Close() }()

	fmt.Printf("agent console: http://%s\n", *addr)
	fmt.Printf("drop .txt emails into: %s\n", cfg.Inbox)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("webapp: %v", err)
	}
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
