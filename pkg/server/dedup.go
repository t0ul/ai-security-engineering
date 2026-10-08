package server

import "strings"

// dedupEvents collapses duplicate events the calendar and timeline aggregate across
// multiple .ics files (the event extractor and the item extractor can both emit the
// same real event, sometimes with a different or fabricated time — e.g. an all-day
// drill and the same drill mis-timed at 22:27). The rule is per-DAY: two events of
// the SAME KIND on the SAME DAY whose normalized titles match collapse to one. That
// catches same-day/different-time duplicates (the common noise) while keeping events
// and tasks distinct (a dated to-do is not the calendar event) and keeping the same
// event on different days (a recurring drill). The cleaner, better-dated entry wins.
func dedupEvents(in []Event) []Event {
	keep := make([]Event, 0, len(in))
	for _, e := range in {
		ne := normTitle(e.Title)
		merged := false
		for j := range keep {
			if !sameDay(e, keep[j]) {
				continue
			}
			nk := normTitle(keep[j].Title)
			if ne == "" || nk == "" || ne == nk || strings.Contains(nk, ne) || strings.Contains(ne, nk) {
				keep[j] = betterEvent(e, keep[j])
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

// sameDay reports whether two events are the same kind on the same calendar day
// (ignoring the time of day, so a fabricated time doesn't dodge dedup).
func sameDay(a, b Event) bool {
	if a.Kind != b.Kind {
		return false
	}
	da, db := dayOf(a), dayOf(b)
	return da != "" && da == db
}

// dayOf is an event's date (yyyy-mm-dd), from Start or, for dated tasks, Due.
func dayOf(e Event) string {
	d := e.Start
	if d == "" {
		d = e.Due
	}
	if len(d) >= 10 {
		return d[:10]
	}
	return d
}

// betterEvent picks the keeper when two same-day duplicates merge: the cleaner title
// wins, then an all-day entry beats a timed one (a school event mis-stamped with a
// time like 22:27 is noise), then the one carrying a reminder.
func betterEvent(a, b Event) Event {
	if sa, sb := titleScore(a), titleScore(b); sa != sb {
		if sa > sb {
			return a
		}
		return b
	}
	if a.AllDay != b.AllDay {
		if a.AllDay {
			return a
		}
		return b
	}
	if a.HasReminder && !b.HasReminder {
		return a
	}
	return b
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
