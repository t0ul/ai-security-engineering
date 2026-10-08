package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// chat answers a natural-language question grounded in the (PII-scrubbed) email
// corpus — the conversational front-end to Ask-School. The retrieved context is
// XML-encapsulated + injection-neutralized before it reaches any model (M8), and
// the answer is scoped to what the corpus actually says. When no live model is
// reachable it falls back to the most relevant passage, clearly labelled.
//
// The handler also hands the chat function this week's schedule, read from the
// agent's OWN derived calendar (the accepted .ics outbox). That is an authorized
// Read of the household's own events — not a contacts/PII harvest — so "what's on
// this week?" can be answered from the app's real state, not just email snippets.
func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
		Unsafe   bool   `json:"unsafe"` // demo: run with RAG controls OFF (raw concat) to show the attack land
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Question == "" {
		http.Error(w, "question required", http.StatusBadRequest)
		return
	}
	answer, sources, err := s.Chat.Answer(req.Question, req.Unsafe, s.weekScheduleBlock(time.Now()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"answer": answer, "sources": sources})
}

// weekScheduleBlock renders the accepted events in the near-term window (from the
// start of this week through ~6 weeks out) as a compact block for the chat, read
// from the agent's own DEDUPED events projection (the DB, derived from the signed
// .ics outbox). A wide window lets the chat answer "what's this week?" AND "what's on
// October 15?" from real calendar state, not an email snippet. One line per event,
// sorted by date, so questions like "how many events on the 15th?" are answerable.
// Returns "" when the calendar is empty in the window.
func (s *Server) weekScheduleBlock(now time.Time) string {
	mon := now.AddDate(0, 0, -int((now.Weekday()+6)%7)) // start of this week
	lo := mon.Format("2006-01-02")
	hi := now.AddDate(0, 0, 45).Format("2006-01-02") // ~6 weeks out
	// Group events UNDER each date so a small model can count per day unambiguously
	// ("how many on the 15th?" = count the lines under that date) instead of scanning
	// a flat list and mis-attributing events to the wrong day.
	byDay := map[string][]string{}
	var days []string
	for _, e := range s.allEvents() {
		d := e.Start
		if d == "" {
			d = e.Due
		}
		if len(d) < 10 {
			continue
		}
		day := d[:10]
		if day < lo || day > hi {
			continue
		}
		title := strings.TrimSpace(e.Title)
		if title == "" {
			continue
		}
		if _, ok := byDay[day]; !ok {
			days = append(days, day)
		}
		byDay[day] = append(byDay[day], title)
	}
	if len(days) == 0 {
		return ""
	}
	sort.Strings(days)
	var b strings.Builder
	for _, day := range days {
		fmt.Fprintf(&b, "%s (%d):\n", day, len(byDay[day]))
		for _, t := range byDay[day] {
			b.WriteString("  - " + t + "\n")
		}
	}
	return fmt.Sprintf("<calendar source=\"extracted-calendar\" note=\"the household's own accepted upcoming events, deduped; each date shows its event count\">\n%s</calendar>", b.String())
}
