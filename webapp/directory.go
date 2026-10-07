package webapp

import (
	"net/http"
	"sort"
	"strings"
)

// directory consolidates the contact details the agent surfaced across every
// processed email (the PII-scoped `contacts` tool) plus the teachers named in the
// child profile, into one deduped lookup (R5). Host-only, operator-facing — the
// same PII-scoped surface as the contacts tool, aggregated. Role/extension
// enrichment (true role→name→ext) is a follow-on; today it is the email roster +
// known people.
func (s *Server) directory(w http.ResponseWriter, _ *http.Request) {
	emails := map[string]bool{}
	for _, ns := range s.loadSummaries() {
		for _, line := range strings.Split(ns.S.Tools["contacts"], "\n") {
			if e := strings.TrimSpace(line); strings.Contains(e, "@") {
				emails[e] = true
			}
		}
	}
	people := map[string]string{} // name -> role
	for _, c := range s.loadProfile().Children {
		if c.Teacher != "" {
			role := "teacher"
			if c.Name != "" {
				role = "teacher · " + c.Name
			}
			people[c.Teacher] = role
		}
	}

	type person struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	addrs := make([]string, 0, len(emails))
	for e := range emails {
		addrs = append(addrs, e)
	}
	sort.Strings(addrs)
	ppl := make([]person, 0, len(people))
	for n, r := range people {
		ppl = append(ppl, person{Name: n, Role: r})
	}
	sort.Slice(ppl, func(i, j int) bool { return ppl[i].Name < ppl[j].Name })

	writeJSON(w, map[string]any{"emails": addrs, "people": ppl})
}
