// Command main is the runnable demo for lesson 11.1, conflicts and mangled data.
//
// Run it with:
//
//	go run .
//
// This chapter is about integrity, not an attacker. School emails get dates and
// times wrong, contradict themselves, and say "not" where it matters, and a model
// that proposes an answer will cheerfully pass all of it through. The agent never
// trusts a model's arithmetic or its reading: it re-checks every extracted event
// with deterministic, standard-library code. This demo runs four kinds of mangled
// data past a naive pipeline that trusts it, then past the agent's real checks,
// which catch each one. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
	"github.com/t0ul/emaildrop/pkg/agent/items"
	"github.com/t0ul/emaildrop/pkg/agent/tool"
	"github.com/t0ul/emaildrop/pkg/agent/tools"
)

func main() {
	trustedAsIs()
	all := checksCatchAll()
	fmt.Println()
	if all {
		fmt.Println("attack blocked (the receipt): a naive pipeline passes every mangled record through; the deterministic checks catch the wrong weekday, the borrowed time, the inverted negation, and the self-conflicting date")
	} else {
		fmt.Println("a check did not catch its case")
		os.Exit(1)
	}
}

// ---- before: a naive pipeline that trusts the extraction ----

// naiveStrip is the title cleaner I might write to tidy a title: drop little
// filler words. It does not know that "not" is not filler, so it inverts meaning.
var filler = map[string]bool{"we": true, "do": true, "not": true, "the": true, "a": true, "please": true}

func naiveStrip(title string) string {
	var out []string
	for _, w := range strings.Fields(title) {
		if !filler[strings.ToLower(strings.Trim(w, ".,"))] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

func trustedAsIs() {
	fmt.Println("== before: the extraction is trusted as-is ==")
	fmt.Println("  weekday:   'Thursday, September 29th' accepted as Thursday (it is a Tuesday)")
	fmt.Println("  time:      'PTA Meeting following drop-off' accepted at 22:27 (no time in the text)")
	fmt.Printf("  negation:  'We do not accept late permission forms' cleaned to %q (meaning inverted)\n", naiveStrip("We do not accept late permission forms"))
	fmt.Println("  duplicate: 'Picture Day' kept on both Sep 24 and Oct 2")
	fmt.Println("  integrity problems caught: 0/4")
}

// ---- after: the agent's deterministic integrity checks ----

// checkWeekday uses the real date parser's weekday-vs-date consistency check.
func checkWeekday() (bool, string) {
	dp := dateparse.ParseDatePhrase("Thursday, September 29th", 2026)
	return dp.Mismatch, dp.Warning
}

// checkTime uses the real time parser: a time the event's own text does not
// contain is borrowed, and an impossible clock value is rejected outright.
func checkTime() (bool, string) {
	eventText := "PTA Meeting following drop-off"
	supported := dateparse.ParseTimes(eventText) // what the text actually says
	impossible := dateparse.ParseTimes("25:99")  // rejected, not rolled over
	borrowed := len(supported) == 0              // a claimed 22:27 is unsupported
	return borrowed && len(impossible) == 0,
		fmt.Sprintf("22:27 is not supported by the event text (borrowed); 25:99 rejected as impossible (parsed times: %v)", supported)
}

// checkNegation uses the real title normalizer, which keeps negations.
func checkNegation() (bool, string) {
	title := "We do not accept late permission forms"
	tidy := items.Tidy(title)
	kept := strings.Contains(strings.ToLower(tidy), "not")
	return kept, fmt.Sprintf("the title normalizer keeps the negation -> %q", tidy)
}

// checkDuplicate uses the real conflict detector over the extracted events.
func checkDuplicate() (bool, string) {
	email := "Picture Day: Thursday, September 24th.\nPicture Day: Friday, October 2nd."
	r := tools.Conflicts{}.Run(email, tool.Ctx{DefaultYear: 2026})
	return len(r.Warnings) > 0, strings.ReplaceAll(r.Text, "\n", "; ")
}

func checksCatchAll() bool {
	fmt.Println("\n== after: deterministic integrity checks ==")
	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"weekday ", checkWeekday},
		{"time    ", checkTime},
		{"negation", checkNegation},
		{"duplicate", checkDuplicate},
	}
	caught := 0
	for _, c := range checks {
		ok, detail := c.fn()
		if ok {
			caught++
		}
		fmt.Printf("  %s caught=%v: %s\n", c.name, ok, detail)
	}
	fmt.Printf("  integrity problems caught: %d/%d\n", caught, len(checks))
	return caught == len(checks)
}
