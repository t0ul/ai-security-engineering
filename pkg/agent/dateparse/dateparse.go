// Package dateparse is deterministic date/time parsing + integrity checking
// (Module 9). School emails bury dates in prose, omit the year, and sometimes
// state the WRONG weekday (the real corpus lists Back-to-School Night as both
// Tuesday and Thursday, Sept 29). We never trust an LLM's date arithmetic: a
// model may propose the text span, but normalization and the weekday-vs-date
// consistency check happen here, deterministically, in the standard library.
package dateparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultYear is assumed when a phrase omits the year.
const DefaultYear = 2026

var months = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
}

// weekdays is Monday-indexed to mirror Python's date.weekday() (Monday == 0).
var weekdays = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

var wdIndex = func() map[string]int {
	m := make(map[string]int, 7)
	for i, w := range weekdays {
		m[strings.ToLower(w)] = i
	}
	return m
}()

const (
	monthsAlt   = `January|February|March|April|May|June|July|August|September|October|November|December`
	weekdaysAlt = `Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday`
)

var (
	reWeekday  = regexp.MustCompile(`(?i)\b(` + weekdaysAlt + `)\b`)
	reMonthDay = regexp.MustCompile(`(?i)\b(` + monthsAlt + `)\s+(\d{1,2})(?:st|nd|rd|th)?\b`)
	reYear     = regexp.MustCompile(`\b(20\d{2})\b`)
	// A time token: hour, optional :minutes, optional am/pm. A token counts as a
	// TIME only if it has minutes (a colon) or an am/pm marker, so bare integers
	// ("Room 205", "Grades 2-5") are never mistaken for times.
	reTimeTok = regexp.MustCompile(`(?i)\b(\d{1,2})(?::(\d{2}))?\s*([ap]\.?m\.?)?`)
)

// HM is an hour (0..23) and minute (0..59).
type HM struct {
	Hour   int
	Minute int
}

func normAMPM(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(s), "p") {
		return "pm"
	}
	return "am"
}

// ParseTimes returns the time tokens in order. It handles ranges where am/pm is
// written once, e.g. "5:30-8:00 PM" -> 17:30 and 20:00 (the earlier time
// inherits the later token's am/pm).
func ParseTimes(text string) []HM {
	type tok struct {
		h, mi int
		ap    string
	}
	var toks []tok
	for _, m := range reTimeTok.FindAllStringSubmatch(text, -1) {
		hStr, miStr, apStr := m[1], m[2], m[3]
		if miStr == "" && apStr == "" {
			continue // bare integer, not a time
		}
		h, _ := strconv.Atoi(hStr)
		mi := 0
		if miStr != "" {
			mi, _ = strconv.Atoi(miStr)
		}
		ap := normAMPM(apStr)
		// Reject impossible clock values rather than letting time.Date silently roll
		// them over (e.g. "25:99" would normalize to the next day +1h39m). Minute 0..59;
		// with am/pm the hour is 1..12, otherwise 0..23.
		if mi > 59 || h > 23 || (ap != "" && (h < 1 || h > 12)) {
			continue
		}
		toks = append(toks, tok{h: h, mi: mi, ap: ap})
	}
	// Back-fill a missing am/pm from the next token that has one.
	next := ""
	for i := len(toks) - 1; i >= 0; i-- {
		if toks[i].ap == "" {
			toks[i].ap = next
		} else {
			next = toks[i].ap
		}
	}
	out := make([]HM, 0, len(toks))
	for _, t := range toks {
		hour := t.h
		switch {
		case t.ap == "pm" && t.h < 12:
			hour = t.h + 12
		case t.ap == "am" && t.h == 12:
			hour = 0
		}
		out = append(out, HM{Hour: hour, Minute: t.mi})
	}
	return out
}

// DatePhrase is the resolved date for a phrase plus its integrity flags.
type DatePhrase struct {
	Date          time.Time
	StatedWeekday string
	ActualWeekday string
	Mismatch      bool
	Warning       string
	AssumedYear   bool
}

func canonWeekday(s string) string {
	if i, ok := wdIndex[strings.ToLower(s)]; ok {
		return weekdays[i]
	}
	return ""
}

// ParseDatePhrase pulls month/day (+ optional stated weekday, optional explicit
// year) from a phrase, returning the resolved date and an integrity flag, or nil
// if no date is present (or the date is impossible, e.g. February 30).
func ParseDatePhrase(text string, defaultYear int) *DatePhrase {
	md := reMonthDay.FindStringSubmatch(text)
	if md == nil {
		return nil
	}
	month := months[strings.ToLower(md[1])]
	day, _ := strconv.Atoi(md[2])

	year := defaultYear
	assumedYear := true
	if ym := reYear.FindStringSubmatch(text); ym != nil {
		year, _ = strconv.Atoi(ym[1])
		assumedYear = false
	}

	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Year() != year || int(d.Month()) != month || d.Day() != day {
		return nil // time.Date normalizes; reject what Python's date() would refuse
	}

	statedWd := ""
	if w := reWeekday.FindStringSubmatch(text); w != nil {
		statedWd = canonWeekday(w[1])
	}
	actualWd := weekdays[(int(d.Weekday())+6)%7] // Go Sunday==0 -> Monday-indexed
	mismatch := statedWd != "" && statedWd != actualWd
	warning := ""
	if mismatch {
		warning = fmt.Sprintf("weekday_mismatch: stated %s, but %s is %s",
			statedWd, d.Format("2006-01-02"), actualWd)
	}
	return &DatePhrase{
		Date: d, StatedWeekday: statedWd, ActualWeekday: actualWd,
		Mismatch: mismatch, Warning: warning, AssumedYear: assumedYear,
	}
}

// Result is phrase -> start/end ISO strings + all-day flag + warnings.
type Result struct {
	Start    string
	End      string
	AllDay   bool
	Warnings []string
}

// ExtractDatetime resolves a phrase into a Result, or nil when it has no date.
func ExtractDatetime(phrase string, defaultYear int) *Result {
	dp := ParseDatePhrase(phrase, defaultYear)
	if dp == nil {
		return nil
	}
	d := dp.Date
	times := ParseTimes(phrase)
	warnings := []string{}
	if dp.Warning != "" {
		warnings = append(warnings, dp.Warning)
	}
	if dp.AssumedYear {
		warnings = append(warnings, fmt.Sprintf("assumed_year:%d", d.Year()))
	}

	if len(times) == 0 {
		return &Result{Start: d.Format("2006-01-02"), End: "", AllDay: true, Warnings: warnings}
	}

	start := time.Date(d.Year(), d.Month(), d.Day(), times[0].Hour, times[0].Minute, 0, 0, time.UTC)
	res := &Result{Start: start.Format("2006-01-02T15:04:05"), AllDay: false, Warnings: warnings}
	if len(times) > 1 {
		end := time.Date(d.Year(), d.Month(), d.Day(), times[1].Hour, times[1].Minute, 0, 0, time.UTC)
		res.End = end.Format("2006-01-02T15:04:05")
	}
	return res
}

// MonthDayIndex returns the [start,end] byte offsets of the first "Month Day"
// phrase in s, or nil if none. Byte offsets land on rune boundaries, so slicing
// s at them yields the same text Python's codepoint indices would.
func MonthDayIndex(s string) []int { return reMonthDay.FindStringIndex(s) }

// WeekdayIndex returns the [start,end] byte offsets of the first weekday name, or nil.
func WeekdayIndex(s string) []int { return reWeekday.FindStringIndex(s) }
