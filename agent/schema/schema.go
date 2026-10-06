// Package schema is the typed contract every calendar tool produces: a single
// Event shape, stable JSON, and a tiny constructor so confidence defaults to 1.0
// the way the Python dataclass did.
package schema

// Event is one extracted calendar entry. Start/End are ISO 8601
// ("2026-09-24T08:30:00", or "2026-09-24" when AllDay).
type Event struct {
	Title       string   `json:"title"`
	Start       string   `json:"start"`
	End         string   `json:"end,omitempty"`
	AllDay      bool     `json:"all_day"`
	Location    string   `json:"location,omitempty"`
	SourceEmail string   `json:"source_email,omitempty"`
	Confidence  float64  `json:"confidence"`           // 0..1; low-confidence events get flagged for review
	Warnings    []string `json:"warnings,omitempty"`   // e.g. "weekday_mismatch: stated Thu, 2026-09-29 is Tue"
}

// New returns an Event with confidence defaulted to 1.0 and a non-nil warnings slice.
func New(title, start string) Event {
	return Event{Title: title, Start: start, Confidence: 1.0, Warnings: []string{}}
}
