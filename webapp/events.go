package webapp

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

// Event is a calendar event rendered in the owner view.
type Event struct {
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Location    string `json:"location"`
	AllDay      bool   `json:"all_day"`
	HasReminder bool   `json:"has_reminder"`
	File        string `json:"file"`
}

// events reads the accepted .ics artifacts in OutboxDir and returns their events
// for the calendar view. The .ics is the source of truth the user accepts.
func (s *Server) events(w http.ResponseWriter, _ *http.Request) {
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
			out = append(out, parseICS(string(raw), e.Name())...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	writeJSON(w, map[string]any{"events": out, "inbox": s.InboxPath})
}

// parseICS extracts events from an .ics body (minimal VEVENT reader).
func parseICS(body, file string) []Event {
	var out []Event
	var cur *Event
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "BEGIN:VEVENT":
			cur = &Event{File: file}
		case line == "END:VEVENT":
			if cur != nil {
				out = append(out, *cur)
				cur = nil
			}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "SUMMARY:"):
			cur.Title = unescapeICS(strings.TrimPrefix(line, "SUMMARY:"))
		case strings.HasPrefix(line, "LOCATION:"):
			cur.Location = unescapeICS(strings.TrimPrefix(line, "LOCATION:"))
		case strings.HasPrefix(line, "DTSTART;VALUE=DATE:"):
			cur.Start = fmtDate(strings.TrimPrefix(line, "DTSTART;VALUE=DATE:"))
			cur.AllDay = true
		case strings.HasPrefix(line, "DTSTART:"):
			cur.Start = fmtDateTime(strings.TrimPrefix(line, "DTSTART:"))
		case strings.HasPrefix(line, "DTEND:"):
			cur.End = fmtDateTime(strings.TrimPrefix(line, "DTEND:"))
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
