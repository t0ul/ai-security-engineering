// Command webapp serves the local operator console for the agent: run the
// security scorecard and replay incidents in the browser. One binary, loopback
// only.
//
//	webapp [-addr :8788] [-log controlplane/logs/audit.jsonl]
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/t0ul/ai-security-engineering/webapp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address (loopback)")
	logPath := flag.String("log", "controlplane/logs/audit.jsonl", "audit log for incidents")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           (&webapp.Server{AuditPath: *logPath}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("agent console on http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}
