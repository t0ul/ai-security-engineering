package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var reSafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// drop accepts an uploaded .txt (form field "email") or pasted text (field
// "text") and writes it into the watched inbox, where the watcher picks it up.
// The content is untrusted — the pipeline guards/sanitizes it on ingest; here we
// only harden the filename (basename, safe chars, .txt).
func (s *Server) drop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if s.paused() {
		http.Error(w, "processing paused by the kill switch", http.StatusServiceUnavailable)
		return
	}
	var data []byte
	name := fmt.Sprintf("drop-%d.txt", time.Now().UnixNano())

	if file, hdr, err := r.FormFile("email"); err == nil {
		defer file.Close()
		data, _ = io.ReadAll(io.LimitReader(file, 256*1024))
		if hdr != nil && hdr.Filename != "" {
			name = safeName(hdr.Filename)
		}
	} else if text := r.FormValue("text"); strings.TrimSpace(text) != "" {
		data = []byte(text)
	} else {
		http.Error(w, "no email file or text provided", http.StatusBadRequest)
		return
	}

	dest := filepath.Join(s.InboxPath, name)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "file": name})
}

func safeName(n string) string {
	n = reSafeName.ReplaceAllString(filepath.Base(n), "_")
	if !strings.HasSuffix(strings.ToLower(n), ".txt") {
		n += ".txt"
	}
	return n
}

// Event is a calendar item rendered in the owner view (meeting, task, heads-up,
// or action — see Kind).
type Event struct {
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Location    string `json:"location"`
	AllDay      bool   `json:"all_day"`
	HasReminder bool   `json:"has_reminder"`
	Signed      bool   `json:"signed"` // the .ics has a valid agent content credential (M20)
	File        string `json:"file"`
	Kind        string `json:"kind"`          // event|task|heads_up|action (from X-KIND)
	Due         string `json:"due,omitempty"` // task/action due date
	URL         string `json:"url,omitempty"` // action target
}

// events reads the accepted .ics artifacts in OutboxDir and returns their events
// for the calendar view. The .ics is the source of truth the user accepts.
// allEvents returns the parsed outbox events, cached and keyed by the outbox
// directory's modification time so repeated requests (Calendar, Items, Summary,
// and now the chat's week schedule) don't re-read and re-parse every .ics each
// time. accept/reject and new drops add or remove files, which bumps the dir
// mtime and invalidates the cache. The returned slice is read-only — callers
// render it, they must not mutate it in place.
func (s *Server) allEvents() []Event {
	if s.OutboxDir == "" {
		return nil
	}
	// Fingerprint the .ics/.sig entries (name+size+mtime), not just the dir mtime:
	// an in-place content edit (e.g. a tampered .ics) changes a file's size/mtime
	// but not necessarily the parent dir's, and must invalidate so the signature
	// re-verifies — serving a stale signed=true would be a security regression.
	fp := s.outboxFingerprint()
	s.evMu.Lock()
	if s.evCache != nil && fp == s.evFP {
		cached := s.evCache
		s.evMu.Unlock()
		return cached
	}
	s.evMu.Unlock()
	out := s.readAllEvents()
	s.evMu.Lock()
	s.evCache, s.evFP = out, fp
	s.evMu.Unlock()
	return out
}

// outboxFingerprint is a cheap key over the outbox's .ics/.sig entries: it stats
// (via ReadDir) but never reads or parses, so a cache hit skips all I/O + parse +
// signature verification. Any add/remove/edit changes it.
func (s *Server) outboxFingerprint() string {
	entries, err := os.ReadDir(s.OutboxDir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if !strings.HasSuffix(n, ".ics") && !strings.HasSuffix(n, ".sig") {
			continue
		}
		if info, ierr := e.Info(); ierr == nil {
			fmt.Fprintf(&b, "%s:%d:%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}

// readAllEvents reads every .ics in the outbox into a sorted, de-duplicated event
// list. allEvents caches the result.
func (s *Server) readAllEvents() []Event {
	var out []Event
	if s.OutboxDir != "" {
		entries, _ := os.ReadDir(s.OutboxDir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".ics") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(s.OutboxDir, e.Name()))
			if err != nil {
				continue
			}
			signed := s.verifySig(e.Name(), raw)
			evs := parseICS(string(raw), e.Name())
			for i := range evs {
				evs[i].Signed = signed
			}
			out = append(out, evs...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return dedupEvents(out)
}

func (s *Server) events(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"events": s.allEvents(), "inbox": s.InboxPath})
}

// parseICS extracts events from an .ics body (minimal VEVENT reader).
func parseICS(body, file string) []Event {
	var out []Event
	var cur *Event
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "BEGIN:VEVENT":
			cur = &Event{File: file, Kind: "event"}
		case line == "BEGIN:VTODO":
			cur = &Event{File: file, Kind: "task"}
		case line == "END:VEVENT", line == "END:VTODO":
			if cur != nil {
				out = append(out, *cur)
				cur = nil
			}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "X-KIND:"):
			cur.Kind = strings.TrimPrefix(line, "X-KIND:")
		case strings.HasPrefix(line, "SUMMARY:"):
			cur.Title = unescapeICS(strings.TrimPrefix(line, "SUMMARY:"))
		case strings.HasPrefix(line, "LOCATION:"):
			cur.Location = unescapeICS(strings.TrimPrefix(line, "LOCATION:"))
		case strings.HasPrefix(line, "URL:"):
			cur.URL = unescapeICS(strings.TrimPrefix(line, "URL:"))
		case strings.HasPrefix(line, "DTSTART;VALUE=DATE:"):
			cur.Start = fmtDate(strings.TrimPrefix(line, "DTSTART;VALUE=DATE:"))
			cur.AllDay = true
		case strings.HasPrefix(line, "DTSTART:"):
			cur.Start = fmtDateTime(strings.TrimPrefix(line, "DTSTART:"))
		case strings.HasPrefix(line, "DTEND:"):
			cur.End = fmtDateTime(strings.TrimPrefix(line, "DTEND:"))
		case strings.HasPrefix(line, "DUE;VALUE=DATE:"):
			cur.Due = fmtDate(strings.TrimPrefix(line, "DUE;VALUE=DATE:"))
			cur.Start = cur.Due // VTODO has no DTSTART; use Due for sort/display
			cur.AllDay = true
		case strings.HasPrefix(line, "DUE:"):
			cur.Due = fmtDateTime(strings.TrimPrefix(line, "DUE:"))
			cur.Start = cur.Due
		case line == "BEGIN:VALARM":
			cur.HasReminder = true
		}
	}
	return out
}

func fmtDate(s string) string { // 20260924 -> 2026-09-24
	if len(s) >= 8 {
		return s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	return s
}

func fmtDateTime(s string) string { // 20260924T083000 -> 2026-09-24 08:30
	if len(s) >= 15 {
		return fmtDate(s[:8]) + " " + s[9:11] + ":" + s[11:13]
	}
	return fmtDate(s)
}

func unescapeICS(s string) string {
	r := strings.NewReplacer(`\,`, ",", `\;`, ";", `\n`, " ", `\\`, `\`)
	return r.Replace(s)
}
