package server

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Question string `json:"question"`
		Unsafe   bool   `json:"unsafe"` // demo: run with RAG controls OFF (raw concat) to show the attack land
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Question == "" {
		http.Error(w, "question required", http.StatusBadRequest)
		return
	}
	answer, sources, err := s.Chat(req.Question, req.Unsafe, s.weekScheduleBlock(time.Now()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"answer": answer, "sources": sources})
}

// weekScheduleBlock renders the accepted events whose date falls in the current
// week (Mon–Sun) as a compact block for the chat. It reads the agent's own signed
// .ics outbox — the trusted derived calendar — so the chat can answer scheduling
// questions from real state. Returns "" when nothing is scheduled this week.
func (s *Server) weekScheduleBlock(now time.Time) string {
	mon := now.AddDate(0, 0, -int((now.Weekday()+6)%7))
	lo := mon.Format("2006-01-02")
	hi := mon.AddDate(0, 0, 6).Format("2006-01-02")
	var lines []string
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
		lines = append(lines, "- "+day+": "+title)
	}
	if len(lines) == 0 {
		return ""
	}
	return fmt.Sprintf("<schedule source=\"extracted-calendar\">\n%s\n</schedule>", strings.Join(lines, "\n"))
}
