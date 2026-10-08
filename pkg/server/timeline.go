package server

import (
	"net/http"
	"strings"
	"time"
)

// Anchor is a recurring daily touchpoint from a child's profile (arrival, lunch,
// dismissal) — the backbone of "what is my kid's day."
type Anchor struct {
	Child string `json:"child"`
	Label string `json:"label"`
	Time  string `json:"time"`
}

// timeline answers "what does my kid have on <day>" (R3, the hero of My Week): the
// child's daily anchors from the profile, plus every extracted event/task/action
// whose date falls on that day, across all processed emails. day = today |
// tomorrow | YYYY-MM-DD (default today).
func (s *Server) timeline(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	day := now
	switch q := r.URL.Query().Get("day"); q {
	case "", "today":
	case "tomorrow":
		day = now.AddDate(0, 0, 1)
	default:
		if d, err := time.Parse("2006-01-02", q); err == nil {
			day = d
		}
	}
	key := day.Format("2006-01-02")

	// Anchors from each child's profile — with half-day exceptions: on a listed
	// half day, dismissal moves to HalfDayOut and lunch is dropped.
	var anchors []Anchor
	halfDay := false
	for _, c := range s.loadProfile().Children {
		half := false
		for _, d := range c.HalfDays {
			if d == key {
				half = true
				halfDay = true
			}
		}
		out, outLabel := c.Out, "out"
		if half {
			if c.HalfDayOut != "" {
				out = c.HalfDayOut
			}
			outLabel = "out (half day)"
		}
		for _, a := range []struct{ label, t string }{{"in", c.In}, {"lunch", c.Lunch}, {outLabel, out}} {
			if half && a.label == "lunch" {
				continue // no lunch on an early-dismissal day
			}
			if a.t != "" {
				anchors = append(anchors, Anchor{Child: c.Name, Label: a.label, Time: a.t})
			}
		}
	}

	// Items (events/tasks/actions) landing on this day.
	var items []Event
	for _, e := range s.allEvents() {
		if strings.HasPrefix(e.Start, key) || (e.Due != "" && strings.HasPrefix(e.Due, key)) {
			items = append(items, e)
		}
	}

	writeJSON(w, map[string]any{
		"day":      key,
		"weekday":  day.Weekday().String(),
		"anchors":  anchors,
		"items":    items,
		"half_day": halfDay,
	})
}
