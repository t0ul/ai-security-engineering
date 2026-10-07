package webapp

import "strings"

// dedupEvents collapses near-duplicate events that the calendar and timeline
// aggregate across multiple .ics files (the event extractor and the item
// extractor can both emit the same real event — e.g. "Science Fair" and the
// sentence fragment "The Science Fair is"). Conservative: two events merge only
// when they share a time slot AND one normalized title contains the other, so
// genuinely distinct same-day events are never merged. The higher-quality title
// (cleaner, Title-Case, not a fragment) wins.
func dedupEvents(in []Event) []Event {
	keep := make([]Event, 0, len(in))
	for _, e := range in {
		ne := normTitle(e.Title)
		merged := false
		for j := range keep {
			if !sameSlot(e, keep[j]) {
				continue
			}
			nk := normTitle(keep[j].Title)
			if ne == "" || nk == "" || ne == nk || strings.Contains(nk, ne) || strings.Contains(ne, nk) {
				if titleScore(e) > titleScore(keep[j]) {
					keep[j] = e // the cleaner title wins the slot
				}
				merged = true
				break
			}
		}
		if !merged {
			keep = append(keep, e)
		}
	}
	return keep
}

// sameSlot reports whether two events occupy the same calendar slot: same kind and
// the same Start (date+time), or the same Due for dated tasks/actions.
func sameSlot(a, b Event) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Start != "" && a.Start == b.Start {
		return true
	}
	return a.Due != "" && a.Due == b.Due
}

// normTitle lowercases a title and strips leading articles and trailing linking
// words so a clean title and the sentence fragment around it compare equal.
func normTitle(t string) string {
	s := strings.ToLower(strings.TrimSpace(t))
	s = strings.TrimPrefix(s, "the ")
	for _, suf := range []string{" is", " are", " on", " at", " will be", " begins", " starts"} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimSpace(s)
}

// titleScore rates a title's quality: a clean, Title-Case noun phrase beats a
// sentence fragment.
func titleScore(e Event) int {
	t := strings.TrimSpace(e.Title)
	s := 0
	if len(t) >= 4 {
		s++
	}
	if t != "" && t[0] >= 'A' && t[0] <= 'Z' {
		s++
	}
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "the ") {
		s--
	}
	for _, bad := range []string{" is", " on ", " at ", " are "} {
		if strings.HasSuffix(low, strings.TrimRight(bad, " ")) || strings.Contains(low, bad) {
			s--
		}
	}
	return s
}
