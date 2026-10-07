package webapp

import (
	"testing"

	"github.com/t0ul/ADD"
)

// FuzzParseICSNoPanic feeds arbitrary bytes to the minimal .ics reader (it parses
// untrusted accepted/emitted calendar files) to prove it never panics/hangs.
func FuzzParseICSNoPanic(f *testing.F) {
	seeds := []string{
		"BEGIN:VEVENT\r\nSUMMARY:x\r\nDTSTART:20261001T080000\r\nEND:VEVENT\r\n",
		"BEGIN:VTODO\r\nDUE;VALUE=DATE:20261002\r\nURL:https://x/y\r\nEND:VTODO\r\n",
		"",
		"BEGIN:VEVENT\nX-KIND:\nDTSTART;VALUE=DATE:\n",
		"SUMMARY:\x00\x01 DTSTART:99",
	}
	add.FuzzInvariant(f, seeds, func(t *testing.T, in string) {
		_ = parseICS(in, "f.ics")
	})
}
