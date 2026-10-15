// Command main is the runnable demo for lesson 13.3, my week at a glance.
//
// Run it with:
//
//	go run .
//
// The weekly view groups the household's items by day. Which day an item lands on
// is a placement decision, and an email can lie about it: state one weekday while
// the date says another ("Monday ... October 8th" when the 8th is a Thursday). A
// view that places items by the weekday written in the text puts the item on the
// wrong day. The agent places by the date it parsed, derives the weekday from that
// date, and flags the mismatch. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
)

func main() {
	sort.Strings(items) // stable output
	undef := misplacedUndefended()
	def := placedByDateDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a stated weekday moves the item to the wrong day on the naive view; the agent places by the date it parsed, so the item lands on its real day and the lie is flagged")
	} else {
		fmt.Println("the view did not place as described")
		os.Exit(1)
	}
}

// ---- the items, and the crafted one ----

// The week's items. One genuine, and one crafted to claim a weekday its date does
// not fall on, so a naive view will file it on the wrong day.
var items = []string{
	"Book Fair on Friday, October 2nd",
	"PTA Meeting on Thursday, October 8th",
	"Mandatory all-staff sync on Monday, October 8th", // the lie: Oct 8 is a Thursday
}

const crafted = "Mandatory all-staff sync on Monday, October 8th"

// ---- before: place by the weekday the email says ----

// naiveWeek files each item under the weekday written in its text. The crafted
// item says Monday, so it lands on Monday, nowhere near the Thursday it is on.
func naiveWeek() map[string][]string {
	week := map[string][]string{}
	for _, it := range items {
		dp := dateparse.ParseDatePhrase(it, 2026)
		if dp == nil || dp.StatedWeekday == "" {
			continue
		}
		week[dp.StatedWeekday] = append(week[dp.StatedWeekday], it)
	}
	return week
}

// ---- after: place by the date, derive the day from it ----

// realWeek files each item under the weekday of the date it actually falls on, the
// way the agent's My Week does (it groups by the parsed date), and flags any item
// whose stated weekday disagrees.
func realWeek() (map[string][]string, []string) {
	week := map[string][]string{}
	var flags []string
	for _, it := range items {
		dp := dateparse.ParseDatePhrase(it, 2026)
		if dp == nil {
			continue
		}
		week[dp.ActualWeekday] = append(week[dp.ActualWeekday], it)
		if dp.Mismatch {
			flags = append(flags, dp.Warning)
		}
	}
	return week, flags
}

func on(week map[string][]string, day string) []string { return week[day] }

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func printWeek(week map[string][]string) {
	order := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for _, d := range order {
		if len(week[d]) == 0 {
			continue
		}
		short := make([]string, len(week[d]))
		for i, t := range week[d] {
			short[i] = strings.SplitN(t, " on ", 2)[0]
		}
		fmt.Printf("  %-9s %s\n", d+":", strings.Join(short, "; "))
	}
}

// ---- the attack, against both placements ----

func misplacedUndefended() bool {
	fmt.Println("== before: place items by the weekday the email states ==")
	w := naiveWeek()
	printWeek(w)
	// The crafted item shows up on Monday, and is missing from its real day, Thursday.
	onMonday := contains(on(w, "Monday"), crafted)
	onThursday := contains(on(w, "Thursday"), crafted)
	fmt.Printf("  crafted item on Monday: %v | on its real day (Thursday): %v\n", onMonday, onThursday)
	return onMonday && !onThursday
}

func placedByDateDefended() bool {
	fmt.Println("\n== after: place items by the date, weekday derived from it ==")
	w, flags := realWeek()
	printWeek(w)
	onThursday := contains(on(w, "Thursday"), crafted)
	onMonday := contains(on(w, "Monday"), crafted)
	flagged := false
	for _, f := range flags {
		if strings.Contains(f, "stated Monday") {
			flagged = true
		}
	}
	fmt.Printf("  crafted item on its real day (Thursday): %v | on Monday: %v\n", onThursday, onMonday)
	for _, f := range flags {
		fmt.Printf("  flagged: %s\n", f)
	}
	return onThursday && !onMonday && flagged
}
