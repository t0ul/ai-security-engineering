package webapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/t0ul/ai-security-engineering/pkg/agent/ics"
	"github.com/t0ul/ai-security-engineering/pkg/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/pkg/agent/schema"
)

// namedSummary pairs a sidecar with its file name (needed so the UI can accept
// an individual item by file+index).
type namedSummary struct {
	Name string
	S    pipeline.EmailSummary
}

// loadSummaries reads every <stem>.summary.json sidecar the pipeline wrote.
func (s *Server) loadSummaries() []namedSummary {
	var out []namedSummary
	if s.OutboxDir == "" {
		return out
	}
	entries, _ := os.ReadDir(s.OutboxDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".summary.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.OutboxDir, e.Name()))
		if err != nil {
			continue
		}
		var es pipeline.EmailSummary
		if json.Unmarshal(raw, &es) == nil {
			out = append(out, namedSummary{Name: e.Name(), S: es})
		}
	}
	return out
}

// itemRow is an item plus where it came from, so the UI can accept it.
type itemRow struct {
	File  string `json:"file"`
	Index int    `json:"index"`
	schema.Event
}

// dateKey is the date an item sorts by: due date for tasks/actions, else start.
func dateKey(e schema.Event) string {
	if e.Due != "" {
		return e.Due
	}
	return e.Start
}

// items returns every extracted item across all emails, optionally filtered by
// ?kind= (event|task|heads_up|action), sorted by date (dateless last).
func (s *Server) items(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	var all []itemRow
	for _, ns := range s.loadSummaries() {
		for i, it := range ns.S.Items {
			if kind == "" || it.ResolvedKind() == kind {
				all = append(all, itemRow{File: ns.Name, Index: i, Event: it})
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		di, dj := dateKey(all[i].Event), dateKey(all[j].Event)
		if (di == "") != (dj == "") {
			return di != "" // dated items first
		}
		return di < dj
	})
	writeJSON(w, map[string]any{"items": all})
}

// review returns the needs-review queue (low-confidence / warned items).
func (s *Server) review(w http.ResponseWriter, _ *http.Request) {
	var all []schema.Event
	for _, ns := range s.loadSummaries() {
		all = append(all, ns.S.NeedsReview...)
	}
	writeJSON(w, map[string]any{"items": all})
}

// summary returns one email's sidecar (digest + contacts + action-items + items)
// by file name. The name is confined to a bare sidecar basename in the outbox —
// no path traversal.
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("file")
	if !safeSidecar(name) {
		http.Error(w, "bad file", http.StatusBadRequest)
		return
	}
	raw, err := os.ReadFile(filepath.Join(s.OutboxDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var es pipeline.EmailSummary
	if json.Unmarshal(raw, &es) != nil {
		http.Error(w, "unreadable summary", http.StatusInternalServerError)
		return
	}
	writeJSON(w, es)
}

// accept writes a single reviewed item to an inert .ics the user can import.
// Body: {"file":"<stem>.summary.json","index":N}. The URL/field sanitizers in
// ics.Write still apply; an action's click is egress-gated later (A6).
func (s *Server) accept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		File    string `json:"file"`
		Index   int    `json:"index"`
		Nonce   string `json:"nonce"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || !safeSidecar(req.File) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.toolsBlocked() {
		http.Error(w, "blocked by the kill switch", http.StatusServiceUnavailable)
		return
	}
	item, ok := s.loadItem(req.File, req.Index)
	if !ok {
		http.Error(w, "no such item", http.StatusBadRequest)
		return
	}
	// HITL (ASI09): evidence-first, single-use nonce confirm before any write.
	if !s.hitlGate(w, "accept", req.File, req.Index, req.Nonce, req.Confirm,
		"Add to your calendar: "+item.Title, "when: "+itemWhen(item), "kind: "+item.ResolvedKind()) {
		return
	}
	s.doAccept(w, req.File, item)
}

func (s *Server) doAccept(w http.ResponseWriter, file string, item schema.Event) {
	text, _, err := ics.Write([]schema.Event{item}, "Accepted")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stem := strings.TrimSuffix(file, ".summary.json")
	outName := fmt.Sprintf("%s.accept-%d.ics", stem, time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(s.OutboxDir, outName), []byte(text), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.Feedback != nil { // data-flywheel: an accept is ground-truth that this extraction was right
		s.Feedback("accept", file, item.Title)
	}
	writeJSON(w, map[string]any{"ok": true, "file": outName})
}

// reject records that an extracted item was wrong — the negative half of the
// data-flywheel. No side effect beyond the durable feedback signal.
func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		File  string `json:"file"`
		Index int    `json:"index"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || !safeSidecar(req.File) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, ok := s.loadItem(req.File, req.Index)
	if !ok {
		http.Error(w, "no such item", http.StatusBadRequest)
		return
	}
	if s.Feedback != nil {
		s.Feedback("reject", req.File, item.Title)
	}
	writeJSON(w, map[string]any{"ok": true})
}

// flywheel reports the accept/reject tallies accumulated from real use.
func (s *Server) flywheel(w http.ResponseWriter, _ *http.Request) {
	accepts, rejects := s.FlywheelStats()
	rate := 0.0
	if total := accepts + rejects; total > 0 {
		rate = float64(accepts) / float64(total)
	}
	writeJSON(w, map[string]any{"accepts": accepts, "rejects": rejects, "accept_rate": rate})
}

func safeSidecar(n string) bool {
	return n != "" && filepath.Base(n) == n && strings.HasSuffix(n, ".summary.json")
}
