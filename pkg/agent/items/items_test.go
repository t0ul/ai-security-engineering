package items_test

import (
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/pkg/agent/items"
	"github.com/t0ul/ai-security-engineering/pkg/agent/schema"
	"github.com/t0ul/ai-security-engineering/pkg/agent/tool"
)

const sample = `PS 123 Weekly Update
Please buy popcorn to support the school fundraiser by October 2.
Guest author visiting on October 13 for the whole class.
Return the signed permission slip no later than September 30.
Accept the classroom photo-sharing invite here: https://class.example/join?code=abc
Back to School Night is October 1 at 6:30 PM in the gym.
`

func find(evs []schema.Event, substr string) (schema.Event, bool) {
	for _, e := range evs {
		if strings.Contains(strings.ToLower(e.Title), strings.ToLower(substr)) {
			return e, true
		}
	}
	return schema.Event{}, false
}

func TestClassifyMixedKinds(t *testing.T) {
	evs := items.Classify(sample, 2026)

	if e, ok := find(evs, "popcorn"); !ok || e.Kind != schema.KindTask || e.Due != "2026-10-02" {
		t.Fatalf("popcorn not a task with due 2026-10-02: %+v", e)
	}
	if e, ok := find(evs, "guest author"); !ok || e.Kind != schema.KindHeadsUp || e.Start != "2026-10-13" {
		t.Fatalf("guest author not a dated heads-up: %+v", e)
	}
	if e, ok := find(evs, "permission slip"); !ok || e.Kind != schema.KindTask || e.Due != "2026-09-30" {
		t.Fatalf("permission slip not a task with due 2026-09-30: %+v", e)
	}
	if e, ok := find(evs, "invite"); !ok || e.Kind != schema.KindAction || e.URL != "https://class.example/join?code=abc" {
		t.Fatalf("invite not an action with URL: %+v", e)
	}
}

func TestClassifyEmitsMeetingAsEvent(t *testing.T) {
	// "Back to School Night ... 6:30 PM" is a meeting: emitted as an event (safety
	// net; deduped against the event extractor in the pipeline), never a task.
	e, ok := find(items.Classify(sample, 2026), "back to school night")
	if !ok || e.Kind != schema.KindEvent || e.AllDay {
		t.Fatalf("meeting should be a timed event, got %+v", e)
	}
}

func TestClassifyPropagatesEventDateToAsks(t *testing.T) {
	email := "PS 51 PTA Multicultural Potluck\n\n" +
		"Multicultural Potluck will be on Friday, October 23rd from 5:30 PM! Join us for a night of food.\n\n" +
		"We ask that you bring a dish to share. If you do not have time, we would appreciate any donation to buy supplemental food!\n"
	evs := items.Classify(email, 2026)

	for _, e := range evs {
		if strings.Contains(strings.ToLower(e.Title), "join us") {
			t.Fatalf("event flavor became an item: %+v", e)
		}
	}
	// The meeting is captured as an event, titled from the PTA subject line.
	if p, ok := find(evs, "multicultural potluck"); !ok || p.Kind != schema.KindEvent || p.Start != "2026-10-23T17:30:00" || p.Title != "Multicultural Potluck" {
		t.Fatalf("potluck not captured as a cleanly-titled event: %+v", p)
	}
	if b, ok := find(evs, "bring a dish"); !ok || b.Kind != schema.KindTask || b.Due != "2026-10-23" {
		t.Fatalf("bring-a-dish not a task dated to the event: %+v", b)
	}
	if d, ok := find(evs, "donation"); !ok || d.Kind != schema.KindTask || d.Due != "2026-10-23" {
		t.Fatalf("donation ask not dated to the event: %+v", d)
	}
}

func TestClassifyAssemblesMultiLineEvent(t *testing.T) {
	// Back-to-School Night: title, date, and time are on separate lines.
	email := "BACK TO SCHOOL NIGHT\n\nTuesday September 29th, 2026\n\nThe event will run from\n5:30pm to 8:00pm\nPlease select one session.\n"
	e, ok := find(items.Classify(email, 2026), "school night")
	if !ok || e.Kind != schema.KindEvent {
		t.Fatalf("multi-line event not assembled: %+v", e)
	}
	if e.Start != "2026-09-29T17:30:00" || e.End != "2026-09-29T20:00:00" {
		t.Fatalf("date+time not assembled from separate lines: %+v", e)
	}
}

func TestClassifyTitlesDatedListFromHeader(t *testing.T) {
	email := "Upcoming Evacuation Drills\n\nThursday, October 1st at 10:08 AM\nWednesday, October 14th at 1:51 PM\n"
	evs := items.Classify(email, 2026)
	n := 0
	for _, e := range evs {
		if e.Title == "Evacuation Drills" && e.Kind == schema.KindEvent {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 header-titled drill events, got %d: %+v", n, evs)
	}
}

func TestReferenceDocSuppressed(t *testing.T) {
	email := "P.S. 51 Family and Student Handbook 2026-2027\nStudents must arrive on time. Visitors must check in and show ID.\n"
	if evs := items.Classify(email, 2026); evs != nil {
		t.Fatalf("a handbook should be suppressed (corpus/Q&A), got %d items: %+v", len(evs), evs)
	}
}

func TestClassifyDedupsRepeatedAsk(t *testing.T) {
	email := "Please return the permission slip by September 30.\nPlease return the permission slip by September 30.\n"
	if n := len(items.Classify(email, 2026)); n != 1 {
		t.Fatalf("expected 1 deduped task, got %d", n)
	}
}

func TestItemExtractorWritesICS(t *testing.T) {
	res := items.ItemExtractor{}.Run(sample, tool.Ctx{Source: "wk.txt", DefaultYear: 2026})
	if res.Capability != tool.WriteICS {
		t.Fatalf("capability = %q, want write_ics", res.Capability)
	}
	ics, ok := res.Artifacts["items.ics"]
	if !ok || !strings.Contains(ics, "BEGIN:VTODO") || !strings.Contains(ics, "X-KIND:action") {
		t.Fatalf("items.ics missing or incomplete:\n%s", ics)
	}
	if len(res.Events) == 0 || res.Events[0].SourceEmail != "wk.txt" {
		t.Fatalf("source email not stamped: %+v", res.Events)
	}
}

// TestClassifyStripsExplicitYearFromTitle locks the title fix: a dated line carrying an
// explicit year (needed for multi-year calendars) must not leak the year into the event
// title — "…, 2026 — First day of school" yields "First day of school", not "2026 — …".
func TestClassifyStripsExplicitYearFromTitle(t *testing.T) {
	email := "Thursday, September 10, 2026 - First day of school.\n"
	e, ok := find(items.Classify(email, 2026), "first day of school")
	if !ok {
		t.Fatal("expected a 'first day of school' event")
	}
	if strings.Contains(e.Title, "2026") || strings.HasPrefix(e.Title, "-") {
		t.Errorf("title must not leak the year or separator, got %q", e.Title)
	}
	if e.Start[:10] != "2026-09-10" {
		t.Errorf("date must still be correct, got %q", e.Start)
	}
}
