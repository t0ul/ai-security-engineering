// Package schema is the typed contract every calendar tool produces: a single
// Event shape, stable JSON, and a tiny constructor so confidence defaults to 1.0
// the way the Python dataclass did.
package schema

// Kind values. A single email can yield items of different kinds; empty means
// Event (a timed meeting), the original behavior.
const (
	KindEvent   = "event"    // a timed meeting (default)
	KindTask    = "task"     // a to-do with a due date: "buy popcorn", "return the form"
	KindHeadsUp = "heads_up" // all-day, informational: "guest author visiting"
	KindAction  = "action"   // carries a URL to act on: accept an invite / RSVP link
)

// Event is one extracted calendar item. Start/End/Due are ISO 8601
// ("2026-09-24T08:30:00", or "2026-09-24" when AllDay).
type Event struct {
	Title       string   `json:"title"`
	Start       string   `json:"start"`
	End         string   `json:"end,omitempty"`
	AllDay      bool     `json:"all_day"`
	Location    string   `json:"location,omitempty"`
	SourceEmail string   `json:"source_email,omitempty"`
	Confidence  float64  `json:"confidence"`         // 0..1; low-confidence events get flagged for review
	Warnings    []string `json:"warnings,omitempty"` // e.g. "weekday_mismatch: stated Thu, 2026-09-29 is Tue"

	Kind  string `json:"kind,omitempty"`  // "" = event; see Kind* constants
	Due   string `json:"due,omitempty"`   // task/action due date (distinct from Start)
	Notes string `json:"notes,omitempty"` // the asking line / short free text
	URL   string `json:"url,omitempty"`   // action only; validated + egress-gated before any click
}

// ResolvedKind returns the item kind, defaulting to KindEvent.
func (e Event) ResolvedKind() string {
	if e.Kind == "" {
		return KindEvent
	}
	return e.Kind
}

// New returns an Event with confidence defaulted to 1.0 and a non-nil warnings slice.
func New(title, start string) Event {
	return Event{Title: title, Start: start, Confidence: 1.0, Warnings: []string{}}
}

// ReviewThreshold is the confidence below which an event must be human-verified
// before it is trusted (M9: don't put a low-confidence or flagged event on the
// calendar unattended).
const ReviewThreshold = 0.75

// NeedsReview reports whether a human should verify this event before it is
// accepted — low confidence or any warning (e.g. a weekday/date mismatch).
func (e Event) NeedsReview() bool {
	return e.Confidence < ReviewThreshold || len(e.Warnings) > 0
}
