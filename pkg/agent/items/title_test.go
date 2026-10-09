package items

import (
	"strings"
	"testing"
)

// TestCleanTitle covers the real sentence-fragment titles from the UX review: the
// deterministic cleaner should produce a short noun/verb phrase, cut at the clause
// boundary, and drop leading filler and dependent clauses.
func TestCleanTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"PTA Meeting: Thursday, September 24th following drop-off", "PTA Meeting"},
		{"Our first PTA Meeting of the school year is coming up next Thursday", "first PTA Meeting of the school year is coming up"},
		{"We do not accept changes or cancellations via email", "do not accept changes or cancellations via email"}, // negation must survive
		{"You can donate at the link above, pay via Zelle, or deposit a check", "donate at the link above"},
		{"If you do not have time, we would appreciate any donation to buy food", "appreciate any donation to buy food"},
		{"Plus, please check to see if your employer will match your donation - double your impact", "check to see if your employer will match your donation"},
		{"2026-2027 Lunch/Recess Volunteer Interest Survey", "2026-2027 Lunch/Recess Volunteer Interest Survey"},
	}
	for _, c := range cases {
		if got := CleanTitle(c.in); got != c.want {
			t.Errorf("CleanTitle(%q)\n  got  %q\n  want %q", c.in, got, c.want)
		}
	}
}

// TestCleanTitleCapsLength holds the ~word cap so a title never runs on.
func TestCleanTitleCapsLength(t *testing.T) {
	long := "Parents and guardians will drop off their children at their classroom doors every single day this year"
	if n := len(strings.Fields(CleanTitle(long))); n > maxTitleWords {
		t.Errorf("CleanTitle must cap at %d words, got %d (%q)", maxTitleWords, n, CleanTitle(long))
	}
}

// TestScrubTitle locks PII removal (the Zelle gmail in a task title) via goflage —
// the first live wiring of the scrubber. The value is gone entirely, not a placeholder.
func TestScrubTitle(t *testing.T) {
	got := ScrubTitle("pay via Zelle @ps51ptamoney@gmail.com before Friday")
	if strings.Contains(got, "gmail") || strings.Contains(got, "ps51ptamoney") {
		t.Errorf("email must be scrubbed from the title, got %q", got)
	}
	if !strings.Contains(got, "Zelle") || !strings.Contains(got, "Friday") {
		t.Errorf("scrub must keep the surrounding words, got %q", got)
	}
	if got := ScrubTitle("Back to School Night"); got != "Back to School Night" {
		t.Errorf("a PII-free title must pass through unchanged, got %q", got)
	}
}

// TestTidyItemNoPII is the end-to-end guarantee for a displayed task title: no raw
// email, no stray "@", capitalized, short.
func TestTidyItemNoPII(t *testing.T) {
	got := TidyItem("You can donate at the link above, pay via Zelle @ps51ptamoney@gmail.com, or deposit a check")
	if strings.Contains(got, "@") || strings.Contains(got, "gmail") {
		t.Errorf("PII/stray @ must not reach a task title, got %q", got)
	}
	if got == "" || !(got[0] >= 'A' && got[0] <= 'Z') {
		t.Errorf("a task title should be capitalized, got %q", got)
	}
}

// TestTidyIdempotent: tidying a tidy title is a no-op (safe to apply at both extraction
// and projection).
func TestTidyIdempotent(t *testing.T) {
	for _, s := range []string{
		"PTA Meeting: Thursday, September 24th",
		"If you do not have time, we would appreciate any donation to buy food",
		"pay via Zelle @ps51ptamoney@gmail.com",
	} {
		once := Tidy(s)
		if twice := Tidy(once); twice != once {
			t.Errorf("Tidy not idempotent for %q: %q -> %q", s, once, twice)
		}
	}
}

// TestTidyEventKeepsFragmentLowercase: an event title that cleans down to a mid-sentence
// fragment stays lowercase so the event projection's dropFragments can cull it (Tidy,
// unlike TidyItem, must NOT capitalize).
func TestTidyEventKeepsFragmentLowercase(t *testing.T) {
	got := Tidy("We would love to have parent/guardian volunteers to support our students")
	if got == "" || (got[0] >= 'A' && got[0] <= 'Z') {
		t.Errorf("a cleaned event fragment must stay lowercase for culling, got %q", got)
	}
}
