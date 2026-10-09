// Package domain holds the app's shared model types — the nouns the server, the
// UI, and the tools all pass around — in the MVC split (Model). It has no HTTP,
// storage, or agent dependencies: pure data. The calendar event itself is modeled
// upstream as schema.Event (the agent's extracted form); the server renders its
// own presentation DTO over that, so Event is deliberately NOT duplicated here.
package domain

// Child is one kid's profile — the key that personalizes the daily timeline (R1).
// Household config set by the operator, never derived from an untrusted email.
type Child struct {
	Name    string `json:"name"`
	Grade   string `json:"grade,omitempty"`
	Class   string `json:"class,omitempty"`
	Teacher string `json:"teacher,omitempty"`
	School  string `json:"school,omitempty"`
	In      string `json:"in,omitempty"`    // arrival, e.g. "8:15"
	Lunch   string `json:"lunch,omitempty"` // lunch, e.g. "10:55"
	Out     string `json:"out,omitempty"`   // dismissal, e.g. "2:30"
	Notes   string `json:"notes,omitempty"` // allergies, bus, etc.

	HalfDays   []string `json:"half_days,omitempty"`    // ISO dates with early dismissal
	HalfDayOut string   `json:"half_day_out,omitempty"` // early dismissal time on a half day, e.g. "11:30"
}

// Profile is the household's children.
type Profile struct {
	Children []Child `json:"children"`
}

// SearchHit is one Ask-School result: a snippet from a past email/handbook, with
// its source and whether it is untrusted (recalled content is data, not
// instructions — M8). Contacts/secrets were scrubbed at index time (M3).
type SearchHit struct {
	Source    string `json:"source"`
	Snippet   string `json:"snippet"`
	Untrusted bool   `json:"untrusted"`
}

// MCPServer is one row of the MCP governance tab (C6): a server the agent reaches
// through the gustoms gateway, its advertised + allow-listed tools, the approved
// pin vs the live manifest hash, and a plain-language status. Status is one of
// "pinned" (manifest matches the approved pin), "rug-pull" (manifest changed —
// calls refused until re-approval), "unapproved" (no pin yet, strict pinning
// refuses calls), "blocked" (kill switch is refusing all tool calls), or "error".
type MCPServer struct {
	Name       string   `json:"name"`
	Tools      []string `json:"tools"`
	Allowed    []string `json:"allowed"`
	Pinned     string   `json:"pinned"`  // short approved manifest hash
	Current    string   `json:"current"` // short live manifest hash
	Status     string   `json:"status"`
	URL        string   `json:"url,omitempty"`        // operator-registered servers only
	Registered bool     `json:"registered,omitempty"` // true = operator-registered (deletable), false = built-in
}
