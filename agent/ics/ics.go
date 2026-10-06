// Package ics generates an inert iCalendar (.ics) artifact and sanitizes every
// field (Module 5: improper output handling). The .ics executes nothing — the
// user accepts it by double-click — but event fields are an exfiltration
// surface (a URL in NOTES/LOCATION can beacon when a client renders it), so we
// strip links and active schemes from every field, and escape per RFC 5545 so
// content can't break the line structure (the calendar analog of the
// markdown-image sanitizer).
package ics

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/t0ul/ai-security-engineering/agent/schema"
)

var (
	reURL    = regexp.MustCompile(`(?i)\b(?:https?|ftp)://\S+`)
	reScheme = regexp.MustCompile(`(?i)\b(?:javascript|data|file|vbscript):\S+`)
)

// SanitizeField strips exfil vectors from a field and returns the cleaned value
// plus how many links/schemes were removed.
func SanitizeField(value string) (string, int) {
	if value == "" {
		return "", 0
	}
	removed := len(reURL.FindAllString(value, -1)) + len(reScheme.FindAllString(value, -1))
	value = reURL.ReplaceAllString(value, "[link removed]")
	value = reScheme.ReplaceAllString(value, "[blocked]")
	return value, removed
}

// escape applies RFC 5545 text escaping. Order matters: backslash first, then
// the structural characters, then any newline folded to a literal \n so a field
// can never inject a new content line.
func escape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, ";", `\;`)
	value = strings.ReplaceAll(value, ",", `\,`)
	value = strings.ReplaceAll(value, "\r\n", `\n`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, "\r", `\n`)
	return value
}

func formatDT(value string, allDay bool) (string, error) {
	if allDay {
		if len(value) < 10 {
			return "", fmt.Errorf("ics: bad all-day date %q", value)
		}
		d, err := time.Parse("2006-01-02", value[:10])
		if err != nil {
			return "", err
		}
		return d.Format("20060102"), nil
	}
	t, err := time.Parse("2006-01-02T15:04:05", value)
	if err != nil {
		return "", err
	}
	return t.Format("20060102T150405"), nil
}

func uid() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + "@email-to-calendar"
}

// Write renders events to an inert .ics. It returns the text (CRLF line
// endings, per RFC 5545) and the total number of links/schemes removed.
func Write(events []schema.Event, calname string) (string, int, error) {
	now := time.Now().UTC().Format("20060102T150405Z")
	removedTotal := 0
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//ai-security-engineering//email-to-calendar//EN",
		"X-WR-CALNAME:" + escape(calname),
	}
	for _, ev := range events {
		titleSrc := ev.Title
		if titleSrc == "" {
			titleSrc = "Untitled"
		}
		title, r1 := SanitizeField(titleSrc)
		location, r2 := SanitizeField(ev.Location)
		removedTotal += r1 + r2

		lines = append(lines, "BEGIN:VEVENT", "UID:"+uid(), "DTSTAMP:"+now)
		if ev.AllDay {
			dt, err := formatDT(ev.Start, true)
			if err != nil {
				return "", removedTotal, err
			}
			lines = append(lines, "DTSTART;VALUE=DATE:"+dt)
		} else {
			dt, err := formatDT(ev.Start, false)
			if err != nil {
				return "", removedTotal, err
			}
			lines = append(lines, "DTSTART:"+dt)
			if ev.End != "" {
				de, err := formatDT(ev.End, false)
				if err != nil {
					return "", removedTotal, err
				}
				lines = append(lines, "DTEND:"+de)
			}
		}
		lines = append(lines, "SUMMARY:"+escape(title))
		if location != "" {
			lines = append(lines, "LOCATION:"+escape(location))
		}
		if len(ev.Warnings) > 0 {
			desc, r3 := SanitizeField(strings.Join(ev.Warnings, " | "))
			removedTotal += r3
			lines = append(lines, "DESCRIPTION:"+escape("⚠ "+desc))
		}
		// Reminder (inert DISPLAY alarm): 30 min before a timed event, or 9am on
		// the day for an all-day event. iCalendar-native — no extra infrastructure.
		trigger := "-PT30M"
		if ev.AllDay {
			trigger = "PT9H" // 9 hours after 00:00 = 9am
		}
		lines = append(lines,
			"BEGIN:VALARM",
			"ACTION:DISPLAY",
			"DESCRIPTION:"+escape(title),
			"TRIGGER:"+trigger,
			"END:VALARM")
		lines = append(lines, "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")
	return strings.Join(lines, "\r\n") + "\r\n", removedTotal, nil
}
