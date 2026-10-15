// Command main is the runnable demo for lesson 10.3, the cross-tenant leak.
//
// Run it with:
//
//	go run .
//
// The store holds documents for more than one household. A question from one
// household must only ever retrieve that household's documents, plus anything
// public. This demo indexes two households into the agent's real retrieval store,
// then asks the same question two ways: a naive retrieval that searches the whole
// index, and the agent's real Query, which puts the tenant ACL inside the search
// so another household's private email can never come back. Deterministic, no model.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/t0ul/emaildrop/pkg/rag"
)

func main() {
	path, cleanup := buildIndex()
	defer cleanup()

	undef := leakLandsUndefended(path)
	def := aclScopesToAsker(path)
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): an unscoped search hands Bob another household's private email; the agent's Query scopes to the asker, so Bob gets only his own and public documents while Alice still gets hers")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the index with two tenants ----

// secret is the marker that must never cross households: a line in Alice's
// private email. Bob asking about appointments must not see it.
const secret = "therapy with Dr. Reyes"

// buildIndex writes the agent's real retrieval index to a temp file and fills it
// with two households' documents plus one public notice, using the real Store.
func buildIndex() (path string, cleanup func()) {
	dir, err := os.MkdirTemp("", "ragidx")
	if err != nil {
		panic(err)
	}
	path = filepath.Join(dir, "index.db")
	store, err := rag.Open(path)
	if err != nil {
		panic(err)
	}
	defer store.Close()
	must(store.Add(rag.Doc{ID: "alice-med", Tenant: "alice", Prov: rag.Trusted, Text: "Reminder: your " + secret + " appointment is Tuesday at 4pm."}))
	must(store.Add(rag.Doc{ID: "bob-pta", Tenant: "bob", Prov: rag.Trusted, Text: "Bob, the PTA budget appointment is Thursday in the library."}))
	must(store.Add(rag.Doc{ID: "notice", Tenant: "", Prov: rag.Trusted, Text: "Public notice: the school book fair appointment desk opens next week."}))
	return path, func() { os.RemoveAll(dir) }
}

// ---- before: retrieval that searches the whole index ----

var reWord = regexp.MustCompile(`[a-zA-Z0-9]+`)

func matchExpr(query string) string {
	words := reWord.FindAllString(query, -1)
	for i, w := range words {
		words[i] = `"` + w + `"`
	}
	return strings.Join(words, " OR ")
}

// queryNaive is the retrieval a developer writes before thinking about tenancy:
// the same FTS search the real store runs, minus the one clause that scopes it to
// the asker. It opens the real index and returns whatever matches, from anyone.
func queryNaive(path, query string, k int) []string {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT text FROM docs WHERE docs MATCH ? ORDER BY rank LIMIT ?`, matchExpr(query), k)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		_ = rows.Scan(&t)
		out = append(out, t)
	}
	return out
}

func leaks(texts []string) bool {
	for _, t := range texts {
		if strings.Contains(t, secret) {
			return true
		}
	}
	return false
}

func leakLandsUndefended(path string) bool {
	fmt.Println("== before: retrieval that searches the whole index ==")
	hits := queryNaive(path, "appointment", 5)
	leaked := leaks(hits)
	fmt.Printf("  Bob asks \"what appointments do I have?\" -> %d hits\n", len(hits))
	fmt.Printf("  Alice's private line is in Bob's results: %v\n", leaked)
	fmt.Printf("  rag-tenant-leak ASR: %d%% (%d/1)\n", b2i(leaked)*100, b2i(leaked))
	return leaked
}

// ---- after: the agent's real Query, ACL in the search ----

func texts(chunks []rag.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Text
	}
	return out
}

func aclScopesToAsker(path string) bool {
	fmt.Println("\n== after: the agent's Query, with the tenant ACL in the search ==")
	store, err := rag.OpenReadOnly(path)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	bob, _ := store.Query("bob", "appointment", 5)
	bobTexts := texts(bob)
	bobLeak := leaks(bobTexts)
	bobSeesOwn := anyContains(bobTexts, "PTA budget")
	bobSeesPublic := anyContains(bobTexts, "book fair")
	fmt.Printf("  Bob's query -> %d hits; Alice's private line present: %v\n", len(bob), bobLeak)
	fmt.Printf("  Bob still sees his own + public: own=%v public=%v\n", bobSeesOwn, bobSeesPublic)
	fmt.Printf("  rag-tenant-leak ASR: %d%% (%d/1)\n", b2i(bobLeak)*100, b2i(bobLeak))

	// Alice, the rightful owner, still retrieves her own private line.
	alice, _ := store.Query("alice", "appointment", 5)
	aliceSeesOwn := leaks(texts(alice))
	fmt.Printf("  Alice's own query still returns her private line: %v\n", aliceSeesOwn)

	return !bobLeak && bobSeesOwn && bobSeesPublic && aliceSeesOwn
}

// ---- helpers ----

func anyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
