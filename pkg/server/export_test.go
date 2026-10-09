package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/pkg/agent/ics"
	"github.com/t0ul/ai-security-engineering/pkg/agent/schema"
	"github.com/t0ul/ai-security-engineering/pkg/datastore"
)

// TestCalendarExportCombinesEventsAndReminders: the one-click export returns a single .ics
// with events as VEVENTs and reminders (tasks) as all-day VEVENTs prefixed "Reminder:" —
// never VTODO, which Google Calendar ignores. Real outbox .ics, real handler, no mocks.
func TestCalendarExportCombinesEventsAndReminders(t *testing.T) {
	dir := t.TempDir()
	src := []schema.Event{
		{Title: "Picture Day", Start: "2026-10-13", AllDay: true, Kind: schema.KindEvent},
		{Title: "Bring a dish", Due: "2026-10-20", AllDay: true, Kind: schema.KindTask},
	}
	body, _, err := ics.Write(src, "seed")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "7.items.ics"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(New(Config{OutboxDir: dir}).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/calendar.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("want a calendar content-type, got %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "family-calendar.ics") {
		t.Errorf("want an attachment filename, got %q", cd)
	}
	b, _ := io.ReadAll(resp.Body)
	out := string(b)
	if !strings.Contains(out, "Picture Day") {
		t.Error("the event is missing from the export")
	}
	if !strings.Contains(out, "Reminder: Bring a dish") {
		t.Error("the reminder is missing or not prefixed in the export")
	}
	if strings.Contains(out, "BEGIN:VTODO") {
		t.Error("reminders must be exported as VEVENTs, not VTODO (Google Calendar ignores VTODO)")
	}
}

// TestCalendarIncrementalExport: the first "download new" export includes everything and marks
// it; a second export returns nothing new; resetting makes everything new again. Real SQLite.
func TestCalendarIncrementalExport(t *testing.T) {
	dir := t.TempDir()
	src := []schema.Event{
		{Title: "Picture Day", Start: "2026-10-13", AllDay: true, Kind: schema.KindEvent},
		{Title: "Book Fair", Start: "2026-10-20", AllDay: true, Kind: schema.KindEvent},
	}
	body, _, err := ics.Write(src, "seed")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.events.ics"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	inv, err := datastore.Open(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer inv.Close()

	srv := httptest.NewServer(New(Config{OutboxDir: dir, Exported: inv}).Handler())
	defer srv.Close()
	count := func(path string) string {
		r, err := http.Post(srv.URL+path, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		_, _ = io.ReadAll(r.Body)
		return r.Header.Get("X-New-Count")
	}
	if c := count("/api/calendar/export"); c != "2" {
		t.Fatalf("first export should have 2 new events, got %q", c)
	}
	if c := count("/api/calendar/export"); c != "0" {
		t.Fatalf("second export should have 0 new (already exported), got %q", c)
	}
	count("/api/calendar/reset-exported")
	if c := count("/api/calendar/export"); c != "2" {
		t.Fatalf("after reset, export should have 2 new again, got %q", c)
	}
}
