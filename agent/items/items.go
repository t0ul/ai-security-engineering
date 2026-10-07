// Package items is the deterministic (regex/offline) extractor of calendar items
// from an email: meetings/events, tasks ("buy popcorn"), heads-ups ("guest
// author visiting"), and actions (an invite/RSVP link). It is a WriteICS tool
// emitting one inert .ics the user accepts.
//
// It is line-aware so it survives real-world layout: a section header becomes
// the title of the dated lines beneath it ("Upcoming Evacuation Drills"), and a
// date line with the time on a nearby line is assembled into one timed event
// (Back-to-School Night). Long reference documents (a family handbook) are
// detected and left for the corpus/Q&A rather than shredded into fake tasks.
//
// This is the robust offline FLOOR and the deterministic half of the planned
// LLM ensemble: the LLM handles format variance, this verifies dates (M9) and
// catches hallucinations. See docs/APP-FEATURES-PLAN.md.
package items

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/t0ul/ai-security-engineering/agent/dateparse"
	"github.com/t0ul/ai-security-engineering/agent/ics"
	"github.com/t0ul/ai-security-engineering/agent/schema"
	"github.com/t0ul/ai-security-engineering/agent/tool"
)

var (
	// reAction: an actionable ask — the sentence wants the reader to DO something.
	// ("join" is deliberately absent — "join us for the potluck" is event flavor,
	// not a task; real joins carry a link and are caught as actions.)
	reAction = regexp.MustCompile(`(?i)\b(buy|purchase|bring|donate|donation|drop[- ]?off|rsvp|accept|confirm|sign up|sign-up|sign and return|return the|return your|submit|register|complete the|pay|volunteer|no later than|don't forget|do not forget|reminder to|must|deadline|due by|due)\b`)
	// reHeadsUp: informational, dated — something is happening, no action needed.
	reHeadsUp = regexp.MustCompile(`(?i)\b(visiting|will visit|guest (author|speaker|reader)|coming to|assembly|book fair|spirit week|picture day|no school|early dismissal|field trip|week of)\b`)
	reURL     = regexp.MustCompile(`(?i)\bhttps?://\S+`)
	reBullet  = regexp.MustCompile(`^\s*[-*•●\x{25cf}\x{25cb}\x{2022}]+\s*`)
	reSpace   = regexp.MustCompile(`\s+`)
	reSentence = regexp.MustCompile(`[.!?]+\s+`)
	// reOrgPrefix strips leading org/sender tags from a subject/header so the
	// event name is left ("PS 51 PTA Multicultural Potluck" -> "Multicultural
	// Potluck"; "Upcoming Evacuation Drills" -> "Evacuation Drills").
	reOrgPrefix = regexp.MustCompile(`(?i)^(p\.?\s?s\.?\s*\d+|pta|pto|ps\d+|upcoming|our)\b[\s:–—-]*`)
	// rePolicyModal marks reference/policy prose (handbooks), not a parent's asks.
	rePolicyModal = regexp.MustCompile(`(?i)\b(must|may not|are required|is required|are trained to|will be permitted|are not able|shall|prohibit)\b`)
)

var weekdaySet = map[string]bool{"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
	"friday": true, "saturday": true, "sunday": true}
var monthSet = map[string]bool{"january": true, "february": true, "march": true, "april": true, "may": true,
	"june": true, "july": true, "august": true, "september": true, "october": true, "november": true, "december": true}
var trailWords = map[string]bool{"on": true, "from": true, "will": true, "be": true, "is": true, "are": true,
	"at": true, "the": true, "a": true, "an": true, "our": true, "in": true, "for": true, "of": true, "to": true, "and": true}

func normTitle(s string) string {
	return strings.Join(strings.Fields(nonAlnum.ReplaceAllString(strings.ToLower(s), " ")), " ")
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// keepWarnings drops noise that shouldn't force human review (assumed_year is
// normal — emails rarely state the year); real integrity flags stay.
func keepWarnings(ws []string) []string {
	var out []string
	for _, w := range ws {
		if !strings.HasPrefix(w, "assumed_year") {
			out = append(out, w)
		}
	}
	return out
}

func cleanSubject(s string) string {
	for {
		n := reOrgPrefix.ReplaceAllString(s, "")
		if n == s {
			break
		}
		s = n
	}
	s = strings.TrimSpace(strings.Trim(s, " :–—-"))
	if s == strings.ToUpper(s) { // de-SHOUT an all-caps header
		s = strings.Title(strings.ToLower(s))
	}
	return s
}

// sharesWord reports whether a and b share a significant (>4 rune) word.
func sharesWord(a, b string) bool {
	bl := strings.ToLower(b)
	for _, w := range strings.Fields(strings.ToLower(a)) {
		w = strings.Trim(w, ",.!?:;")
		if len([]rune(w)) > 4 && strings.Contains(bl, w) {
			return true
		}
	}
	return false
}

var reLetters = regexp.MustCompile(`[a-z]{4,}`)

// significant reports whether a fragment has a real word: a run of 4+ letters
// that is not a weekday or month name (so "10:08 AM" / "Thursday" do not count).
func significant(frag string) bool {
	for _, w := range reLetters.FindAllString(strings.ToLower(frag), -1) {
		if !weekdaySet[w] && !monthSet[w] {
			return true
		}
	}
	return false
}

func trimEnds(c string) string {
	strip := func(w string) bool { return trailWords[w] || weekdaySet[w] }
	words := strings.Fields(c)
	for len(words) > 0 {
		if strip(strings.ToLower(strings.Trim(words[len(words)-1], ",.-"))) {
			words = words[:len(words)-1]
			continue
		}
		break
	}
	for len(words) > 0 {
		if strip(strings.ToLower(strings.Trim(words[0], ",.-"))) {
			words = words[1:]
			continue
		}
		break
	}
	return strings.Join(words, " ")
}

// eventTitleOrEmpty derives a short event name from a line by taking the text
// around the date phrase (a title can sit before OR after the date — "Multicultural
// Potluck will be on Oct 23" vs "October 12th - Italian Heritage Day") and
// stripping connector/weekday words. Returns "" for a bare date line
// ("Thursday, October 1st at 10:08 AM"), so the caller falls back to the header.
func eventTitleOrEmpty(s string) string {
	before, after := s, ""
	if idx := dateparse.MonthDayIndex(s); idx != nil {
		before, after = s[:idx[0]], s[idx[1]:]
	}
	b := trimEnds(strings.TrimSpace(strings.Trim(before, " ,.-–—:")))
	a := trimEnds(strings.TrimSpace(strings.Trim(after, " ,.-–—:")))
	// Prefer whichever side carries a real name (the longer, if both do).
	switch {
	case significant(a) && (!significant(b) || len(a) >= len(b)):
		return a
	case significant(b):
		return b
	default:
		return ""
	}
}

func isHeader(line string) bool {
	if line == "" || len(strings.Fields(line)) > 8 {
		return false
	}
	t := strings.TrimRight(line, " ")
	if strings.HasSuffix(t, ".") || strings.Contains(t, ": ") {
		return false // a sentence, not a heading
	}
	return reSentence.FindStringIndex(line) == nil
}

// referenceLike detects a long policy/reference document (a handbook): it should
// feed the corpus for Q&A, not be shredded into fake tasks.
func referenceLike(email string) bool {
	lower := strings.ToLower(email)
	head := lower
	if len(head) > 400 {
		head = head[:400]
	}
	if strings.Contains(head, "handbook") || strings.Contains(head, "family and student") || strings.Contains(head, "policy manual") {
		return true
	}
	lines := strings.Count(email, "\n")
	modals := len(rePolicyModal.FindAllString(email, -1))
	return lines > 150 && modals >= 12
}

// combineDT attaches a time to an all-day date (2026-09-29 -> 2026-09-29T17:30:00).
func combineDT(dateISO string, hm dateparse.HM) string {
	return fmt.Sprintf("%sT%02d:%02d:00", dateISO[:10], hm.Hour, hm.Minute)
}

// nearbyTimes returns the times from the closest date-less line around index i
// (a time or range on its own line), so "Tuesday Sept 29" + "5:30pm to 8:00pm"
// one line apart become a single timed event.
func nearbyTimes(lines []string, i, year int) []dateparse.HM {
	for _, j := range []int{i + 1, i + 2, i + 3, i - 1, i - 2} {
		if j < 0 || j >= len(lines) || j == i || lines[j] == "" {
			continue
		}
		if dateparse.ParseDatePhrase(lines[j], year) != nil {
			continue // that line is itself dated; don't borrow its time
		}
		if hm := dateparse.ParseTimes(lines[j]); len(hm) > 0 {
			return hm
		}
	}
	return nil
}

// Classify extracts the calendar items an email asks for.
func Classify(email string, defaultYear int) []schema.Event {
	if defaultYear == 0 {
		defaultYear = dateparse.DefaultYear
	}
	if referenceLike(email) {
		return nil // reference/handbook -> corpus & Q&A (Track 2 R2), not tasks
	}

	var lines []string
	for _, ln := range strings.Split(email, "\n") {
		lines = append(lines, strings.TrimSpace(reBullet.ReplaceAllString(ln, "")))
	}
	subject := ""
	for _, ln := range lines {
		if ln != "" {
			subject = ln
			break
		}
	}

	var out []schema.Event
	seen := map[string]bool{}
	add := func(ev schema.Event) {
		key := normTitle(ev.Title) + "|" + dateOf(ev)
		if strings.TrimSpace(ev.Title) == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, ev)
	}

	// --- events pass (line-aware): holidays, meetings, drills, closures ---
	header := ""
	eventDates := map[string]bool{}
	for i, ln := range lines {
		if ln == "" {
			continue
		}
		dt := dateparse.ExtractDatetime(ln, defaultYear)
		if dt == nil {
			if isHeader(ln) {
				header = ln
			}
			continue
		}
		// A dated ASK stays a task/heads-up (handled in the asks pass).
		if dt.AllDay && (reAction.MatchString(ln) || reHeadsUp.MatchString(ln)) {
			continue
		}
		start, end, allDay := dt.Start, dt.End, dt.AllDay
		if allDay {
			if hm := nearbyTimes(lines, i, defaultYear); len(hm) > 0 {
				start, allDay = combineDT(dt.Start, hm[0]), false
				if len(hm) > 1 {
					end = combineDT(dt.Start, hm[1])
				}
			}
		}
		title := bestTitle(ln, header, subject)
		if title == "" {
			continue // a bare date with no name is a date reference, not an event
		}
		eventDates[start[:10]] = true
		add(schema.Event{Title: title, Start: start, End: end, AllDay: allDay,
			Kind: schema.KindEvent, Confidence: 0.75, Warnings: keepWarnings(dt.Warnings)})
	}

	// Propagate a date to date-less asks ONLY for a single-event email (so a
	// handbook/newsletter with many dates never stamps every ask with one date).
	primary := ""
	if len(eventDates) == 1 {
		for d := range eventDates {
			primary = d
		}
	}

	// --- asks pass (sentence-based): tasks, heads-ups, actions ---
	for _, s := range sentences(email) {
		dt := dateparse.ExtractDatetime(s, defaultYear)
		if dt != nil && !dt.AllDay {
			continue // a timed, dated sentence is a meeting — the events pass owns it
		}
		hasURL := reURL.MatchString(s)
		action := reAction.MatchString(s)
		headsUp := reHeadsUp.MatchString(s)
		if !action && !headsUp && !hasURL {
			continue
		}

		ev := schema.Event{Title: s, Confidence: 0.9, Warnings: []string{}, AllDay: true}
		switch {
		case hasURL:
			ev.Kind = schema.KindAction
			ev.URL = reURL.FindString(s)
		case action:
			ev.Kind = schema.KindTask
		default:
			ev.Kind = schema.KindHeadsUp
		}
		switch {
		case dt != nil:
			ev.AllDay = dt.AllDay
			ev.Warnings = keepWarnings(dt.Warnings)
			if ev.Kind == schema.KindHeadsUp {
				ev.Start = dt.Start
			} else {
				ev.Due = dt.Start
			}
		case primary != "":
			if ev.Kind == schema.KindHeadsUp {
				ev.Start = primary
			} else {
				ev.Due = primary
			}
			ev.Notes = "date inferred from the email"
		case ev.Kind == schema.KindHeadsUp:
			continue // undated, un-anchored heads-up isn't a calendar item
		}
		add(ev)
	}
	return out
}

// bestTitle picks the cleanest title for a dated event line: a subject that
// clearly names it, else the line's own text, else the section header. Returns
// "" when none is a real name (a bare date line), so the event is dropped.
func bestTitle(line, header, subject string) string {
	if subject != "" && len(strings.Fields(subject)) <= 8 && sharesWord(subject, line) {
		if cs := cleanSubject(subject); len([]rune(cs)) >= 3 {
			return cs
		}
	}
	if t := eventTitleOrEmpty(line); t != "" {
		return t
	}
	if h := cleanSubject(header); significant(h) {
		return h
	}
	return ""
}

func sentences(email string) []string {
	var out []string
	for _, line := range strings.Split(email, "\n") {
		line = strings.TrimSpace(reBullet.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		for _, s := range reSentence.Split(line, -1) {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, reSpace.ReplaceAllString(s, " "))
			}
		}
	}
	return out
}

func dateOf(e schema.Event) string {
	if e.Due != "" {
		return e.Due
	}
	return e.Start
}

// ItemExtractor is the WriteICS tool wrapping Classify. Its artifact is
// "items.ics" (distinct from the event extractor's), so both tools stay
// least-privilege and independent and the web app merges their outputs.
type ItemExtractor struct{}

func (ItemExtractor) Name() string                { return "item_extractor" }
func (ItemExtractor) Capability() tool.Capability { return tool.WriteICS }

func (ItemExtractor) Run(email string, ctx tool.Ctx) tool.Result {
	items := Classify(email, ctx.DefaultYear)
	for i := range items {
		items[i].SourceEmail = ctx.Source
	}
	res := tool.Result{Tool: "item_extractor", Capability: tool.WriteICS, Events: items}
	if len(items) > 0 {
		text, removed, err := ics.Write(items, "Email items")
		if err != nil {
			res.Warnings = append(res.Warnings, "items ics write failed: "+err.Error())
		} else {
			res.Artifacts = map[string]string{"items.ics": text}
			if removed > 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%d link(s) sanitized", removed))
			}
		}
	}
	res.Warnings = append(res.Warnings, fmt.Sprintf("%d item(s)", len(items)))
	return res
}
