package server

import (
	"encoding/json"
	"fmt"
	"log"
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

// SummaryStore is the DB-backed projection of the per-email summary sidecars. The
// .summary.json files stay the source of truth; this projection is rebuilt from them
// whenever they change and serves the Tasks/digest/Directory/Review views without a
// per-request file scan. Each row carries the EmailSummary as JSON.
type SummaryStore interface {
	ReplaceSummaries(fingerprint string, rows []SummaryRow) error
	LoadSummaries() (fingerprint string, rows []SummaryRow, err error)
}

// SummaryRow is one projected sidecar: its file name and the EmailSummary JSON.
type SummaryRow struct {
	File string
	JSON string
}

// loadSummaries returns the parsed summary sidecars, served in three tiers: an
// in-memory cache, then the DB projection (persists across restarts), then a rebuild
// by scanning the .summary.json files. The summaries fingerprint keys all three, so
// any sidecar add/remove/edit invalidates them and nothing is served stale.
func (s *Server) loadSummaries() []namedSummary {
	if s.OutboxDir == "" {
		return nil
	}
	fp := s.summariesFingerprint()
	s.smMu.Lock()
	if s.smCache != nil && fp == s.smFP {
		cached := s.smCache
		s.smMu.Unlock()
		return cached
	}
	s.smMu.Unlock()
	// DB projection: serve from it when it was built from the current sidecars.
	if s.Summaries != nil && fp != "" {
		if dbFP, rows, err := s.Summaries.LoadSummaries(); err == nil && dbFP == fp {
			out := decodeSummaries(rows)
			s.smMu.Lock()
			s.smCache, s.smFP = out, fp
			s.smMu.Unlock()
			return out
		}
	}
	// Rebuild from the JSON sidecars, then persist the projection.
	out := s.readAllSummaries()
	if s.Summaries != nil {
		rows := make([]SummaryRow, 0, len(out))
		for _, ns := range out {
			if b, err := json.Marshal(ns.S); err == nil {
				rows = append(rows, SummaryRow{File: ns.Name, JSON: string(b)})
			}
		}
		if err := s.Summaries.ReplaceSummaries(fp, rows); err == nil {
			log.Printf("summaries: rebuilt DB projection from .summary.json (%d sidecars)", len(rows))
		}
	}
	s.smMu.Lock()
	s.smCache, s.smFP = out, fp
	s.smMu.Unlock()
	return out
}

// readAllSummaries scans and parses every <stem>.summary.json sidecar in the outbox.
func (s *Server) readAllSummaries() []namedSummary {
	var out []namedSummary
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

// decodeSummaries turns persisted projection rows back into parsed sidecars.
func decodeSummaries(rows []SummaryRow) []namedSummary {
	out := make([]namedSummary, 0, len(rows))
	for _, r := range rows {
		var es pipeline.EmailSummary
		if json.Unmarshal([]byte(r.JSON), &es) == nil {
			out = append(out, namedSummary{Name: r.File, S: es})
		}
	}
	return out
}

// summariesFingerprint is a cheap key over the .summary.json sidecars (name+size+
// mtime) — any add/remove/edit changes it, invalidating the caches and the DB
// projection so a reader never serves stale.
func (s *Server) summariesFingerprint() string {
	entries, err := os.ReadDir(s.OutboxDir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(projectionVersion + "|") // invalidate on a dedup/derivation logic change
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".summary.json") {
			continue
		}
		if info, ierr := e.Info(); ierr == nil {
			fmt.Fprintf(&b, "%s:%d:%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}

// itemRow is an item plus where it came from, so the UI can accept it.
type itemRow struct {
	File  string `json:"file"`
	Index int    `json:"index"`
	schema.Event
}

// dedupItems collapses duplicate items the extractor emitted for the same real thing
// (same normalized title on the same day, regardless of kind). The cleaner title wins.
// Keyed by (normalized title, day), so distinct same-day items and the same item on
// different days are kept.
func dedupItems(rows []itemRow) []itemRow {
	seen := map[string]int{}
	var out []itemRow
	for _, r := range rows {
		if strings.TrimSpace(r.Title) == "" {
			continue
		}
		day := dateKey(r.Event) // dedup per DAY, not per timestamp (a fabricated time must not dodge it)
		if len(day) >= 10 {
			day = day[:10]
		}
		key := normTitle(r.Title) + "|" + day
		if idx, ok := seen[key]; ok {
			if itemCleaner(r, out[idx]) {
				out[idx] = r
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, r)
	}
	return out
}

// itemCleaner reports whether a has a cleaner title than b (starts with a capital,
// i.e. not a sentence fragment, and is longer).
func itemCleaner(a, b itemRow) bool {
	ac := a.Title != "" && a.Title[0] >= 'A' && a.Title[0] <= 'Z'
	bc := b.Title != "" && b.Title[0] >= 'A' && b.Title[0] <= 'Z'
	if ac != bc {
		return ac
	}
	return len(a.Title) > len(b.Title)
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
// ItemStore is the DB-backed DEDUPED item projection. The extractor emits the same
// real item several times (across emails, as different kinds, with I/1 spelling
// variants); the dupes are removed ONCE at build time and the result is stored here,
// rather than re-deduping on every request. Rebuilt from the summary sidecars when
// they change.
type ItemStore interface {
	ReplaceItems(fingerprint string, rows []PersistedItem) error
	LoadItems() (fingerprint string, rows []PersistedItem, err error)
}

// PersistedItem is one projected item: its source file + index and the item JSON.
type PersistedItem struct {
	File  string
	Index int
	JSON  string
}

// ItemStatusStore is the mutable status overlay for projected items (done/dismiss/
// snooze). Keyed by a content fingerprint stable across projection rebuilds, so a
// completed task stays completed when the derived items are rebuilt. Nil = the Tasks tab
// is read-only (no complete/dismiss/snooze).
type ItemStatusStore interface {
	SetItemStatus(key, status, snoozeUntil string) error
	LoadItemStatuses() (map[string]ItemStatus, error)
}

// ItemStatus mirrors datastore.ItemStatus at the server boundary.
type ItemStatus struct {
	Status      string
	SnoozeUntil string
}

// itemKey is the stable content fingerprint for an item's status overlay: the same
// identity the dedup collapses on (normalized title | day | resolved kind), so it
// survives a projection rebuild where File/Index would shift.
func itemKey(e schema.Event) string {
	day := dateKey(e)
	if len(day) >= 10 {
		day = day[:10]
	}
	return normTitle(e.Title) + "|" + day + "|" + e.ResolvedKind()
}

// allItems returns the deduped items, served in three tiers (memory → DB projection →
// rebuild). The rebuild flattens the summary items, removes duplicates ONCE, and
// persists the result, so the dupes are gone from the store, not just hidden at read.
func (s *Server) allItems() []itemRow {
	if s.OutboxDir == "" {
		return nil
	}
	fp := s.summariesFingerprint()
	s.itMu.Lock()
	if s.itCache != nil && fp == s.itFP {
		cached := s.itCache
		s.itMu.Unlock()
		return cached
	}
	s.itMu.Unlock()
	if s.Items != nil && fp != "" {
		if dbFP, rows, err := s.Items.LoadItems(); err == nil && dbFP == fp {
			out := decodeItems(rows)
			s.itMu.Lock()
			s.itCache, s.itFP = out, fp
			s.itMu.Unlock()
			return out
		}
	}
	var raw []itemRow
	for _, ns := range s.loadSummaries() {
		for i, it := range ns.S.Items {
			raw = append(raw, itemRow{File: ns.Name, Index: i, Event: it})
		}
	}
	ded := dedupItems(raw)
	if s.Items != nil {
		persist := make([]PersistedItem, 0, len(ded))
		for _, r := range ded {
			if b, err := json.Marshal(r.Event); err == nil {
				persist = append(persist, PersistedItem{File: r.File, Index: r.Index, JSON: string(b)})
			}
		}
		if err := s.Items.ReplaceItems(fp, persist); err == nil {
			log.Printf("items: rebuilt deduped DB projection (%d → %d items)", len(raw), len(ded))
		}
	}
	s.itMu.Lock()
	s.itCache, s.itFP = ded, fp
	s.itMu.Unlock()
	return ded
}

func decodeItems(rows []PersistedItem) []itemRow {
	out := make([]itemRow, 0, len(rows))
	for _, r := range rows {
		var ev schema.Event
		if json.Unmarshal([]byte(r.JSON), &ev) == nil {
			out = append(out, itemRow{File: r.File, Index: r.Index, Event: ev})
		}
	}
	return out
}

// statusItemRow is an item annotated with its stable key and mutable status, so the
// Tasks tab can complete/dismiss/snooze it (C2).
type statusItemRow struct {
	itemRow
	Key         string `json:"key"`
	Status      string `json:"status,omitempty"`
	SnoozeUntil string `json:"snooze_until,omitempty"`
}

func (s *Server) items(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	showAll := r.URL.Query().Get("all") == "1" // include done/dismissed/snoozed
	all := s.allItems()
	if kind != "" {
		filtered := make([]itemRow, 0, len(all))
		for _, it := range all {
			if it.Event.ResolvedKind() == kind {
				filtered = append(filtered, it)
			}
		}
		all = filtered
	}
	sort.SliceStable(all, func(i, j int) bool {
		di, dj := dateKey(all[i].Event), dateKey(all[j].Event)
		if (di == "") != (dj == "") {
			return di != "" // dated items first
		}
		return di < dj
	})
	var statuses map[string]ItemStatus
	if s.ItemStatus != nil {
		statuses, _ = s.ItemStatus.LoadItemStatuses()
	}
	today := time.Now().Format("2006-01-02")
	out := make([]statusItemRow, 0, len(all))
	for _, it := range all {
		key := itemKey(it.Event)
		st := statuses[key]
		// Default list hides completed/dismissed work and items snoozed to a future
		// date; ?all=1 shows everything with its status so the UI can offer un-done.
		if !showAll {
			if st.Status == "done" || st.Status == "dismissed" {
				continue
			}
			if st.Status == "snoozed" && st.SnoozeUntil > today {
				continue
			}
		}
		out = append(out, statusItemRow{itemRow: it, Key: key, Status: st.Status, SnoozeUntil: st.SnoozeUntil})
	}
	writeJSON(w, map[string]any{"items": out})
}

// itemStatus sets an item's mutable status (done/dismissed/snoozed/active). Body:
// {"key":"…","status":"done"} or {"key":"…","status":"snoozed","snooze_until":"2026-10-20"}.
// A consumer action (the App token holds write/calendar), gated + CSRF-protected.
func (s *Server) itemStatus(w http.ResponseWriter, r *http.Request) {
	if s.ItemStatus == nil {
		http.Error(w, "item status not configured", http.StatusNotImplemented)
		return
	}
	var req struct {
		Key         string `json:"key"`
		Status      string `json:"status"`
		SnoozeUntil string `json:"snooze_until"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Key) == "" {
		http.Error(w, "key and status required", http.StatusBadRequest)
		return
	}
	switch req.Status {
	case "active", "done", "dismissed", "snoozed":
	default:
		http.Error(w, "status must be active|done|dismissed|snoozed", http.StatusBadRequest)
		return
	}
	if req.Status != "snoozed" {
		req.SnoozeUntil = ""
	}
	if err := s.ItemStatus.SetItemStatus(req.Key, req.Status, req.SnoozeUntil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "key": req.Key, "status": req.Status})
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
	if s.Flywheel != nil { // data-flywheel: an accept is ground-truth that this extraction was right
		s.Flywheel.Record("accept", file, item.Title)
	}
	writeJSON(w, map[string]any{"ok": true, "file": outName})
}

// reject records that an extracted item was wrong — the negative half of the
// data-flywheel. No side effect beyond the durable feedback signal.
func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
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
	if s.Flywheel != nil {
		s.Flywheel.Record("reject", req.File, item.Title)
	}
	writeJSON(w, map[string]any{"ok": true})
}

// flywheel reports the accept/reject tallies accumulated from real use.
func (s *Server) flywheel(w http.ResponseWriter, _ *http.Request) {
	accepts, rejects := s.Flywheel.Stats()
	rate := 0.0
	if total := accepts + rejects; total > 0 {
		rate = float64(accepts) / float64(total)
	}
	writeJSON(w, map[string]any{"accepts": accepts, "rejects": rejects, "accept_rate": rate})
}

func safeSidecar(n string) bool {
	return n != "" && filepath.Base(n) == n && strings.HasSuffix(n, ".summary.json")
}
