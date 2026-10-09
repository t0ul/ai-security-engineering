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
