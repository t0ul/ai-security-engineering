---
title: "Cross-tenant leak"
description: "The last leak was about what a document said. This one is about whose documents come back at all. One retrieval store holds more than one household's email, and a search that is not scoped to the asker hands one household's private line to another. The fix is to put the tenant check inside the search itself, so authorization is not a step the retrieval path can forget."
date: 2026-03-31
draft: false
tags: ["ai-security", "golang", "rag", "authorization"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Whose documents come back

The last post was about a poisoned document steering an answer: the danger was in what a retrieved passage said. This one is simpler and in some ways worse. The danger is which passages come back at all. A retrieval store does not have to serve one person. The moment it holds email for more than one household, every query is also an authorization decision, because the honest-looking question "what appointments do I have" has a wrong answer that is somebody else's private life.

The attack does not need a crafted prompt or a planted document. It needs a retrieval that searches the whole index and forgets to ask whose results these are. This is a control lesson, deterministic, no model, because the leak is in the query, not the model:

```bash
cd posts/10.3-cross-tenant-leak
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init tenants`, then `go get` the two modules on the first `require` block below at the versions shown, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/10.3-cross-tenant-leak

go 1.27.1

require (
	// The agent's real tenant-scoped retrieval store. No overrides.
	github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
	modernc.org/sqlite v1.60.1
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/t0ul/GoRag v0.0.0-20261008195127-bc203f269e1c // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
```
<!-- /listing -->

## The spine of the program

`main` builds the index once, then asks the same question two ways. The receipt needs the unscoped search to leak and the scoped one not to, while Bob still sees his own and public documents and Alice still sees hers.

<!-- listing: main.go#head -->
```go
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
```
<!-- /listing -->

## Two households in one index

The index holds three documents in the agent's real retrieval store. Alice has a private reminder, a therapy appointment, under her tenant. Bob has a PTA note under his. And there is one public school notice under no tenant at all, which everyone is allowed to see. The therapy line is a benign marker standing in for whatever genuinely private thing a household's email holds.

<!-- listing: main.go#the index with two tenants -->
```go
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
```
<!-- /listing -->

Now Bob asks about his appointments. The retrieval I would write first just searches the index for the query and returns what matches, ranked by relevance. It is the same full-text search the real store runs, minus one clause:

<!-- listing: main.go#before: retrieval that searches the whole index -->
```go
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
```
<!-- /listing -->

It works, in the sense that it returns appointments. It returns all of them.

```
== before: retrieval that searches the whole index ==
  Bob asks "what appointments do I have?" -> 3 hits
  Alice's private line is in Bob's results: true
  rag-tenant-leak ASR: 100% (1/1)
```

Three hits for a question that should have had two, and the extra one is Alice's therapy appointment, now sitting in Bob's results ready to be read aloud by the assistant. Nobody attacked anything. The query was built without a notion of who was asking, so it answered for everybody.

## Authorization inside the search

The fix is to make the tenant check part of the query rather than a thing you remember to do after. The agent retrieves through `rag.Store`, whose `Query` takes the asker's tenant and carries it into the SQL itself:

```go
`SELECT id, prov, text FROM docs WHERE docs MATCH ? AND (tenant = ? OR tenant = '') ORDER BY rank LIMIT ?`
```

The only difference from the naive version is the `AND (tenant = ? OR tenant = '')`, and it is the whole lesson. A document comes back only if it belongs to the asker or is public, the empty tenant. There is no second step where application code filters the results and could be skipped, no place to forget: a row that is not yours is never selected in the first place. This is authorization-first retrieval, the ACL living in the same statement that does the search. Here is the lesson's call into it, opening the store read-only, asking as Bob and then as Alice:

<!-- listing: main.go#after: the agent's real Query, ACL in the search -->
```go
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
```
<!-- /listing -->

Run Bob's question through it:

```
== after: the agent's Query, with the tenant ACL in the search ==
  Bob's query -> 2 hits; Alice's private line present: false
  Bob still sees his own + public: own=true public=true
  rag-tenant-leak ASR: 0% (0/1)
  Alice's own query still returns her private line: true
```

Two hits now, Bob's own note and the public notice, and Alice's appointment is gone from his results. The control is not a blunt instrument, though: the public notice still reaches Bob, and Alice asking the same question still gets her own private line back. Scoping to the asker is exactly that, scoping, not hiding everything.

The agent goes one belt tighter, which is worth noticing in the code above. The search path opens the store read-only, so the connection that serves a query physically cannot write the index. Tenant scoping decides what a reader may see; the read-only connection makes sure a reader is only ever a reader, so a bug or an injection on the retrieval path can neither leak across tenants nor poison the corpus for the next one.

The three helpers search a list for a phrase, turn a boolean into a count, and panic on an error while building the index:

<!-- listing: main.go#helpers -->
```go
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
```
<!-- /listing -->

That is the whole program. Its two tests build a fresh index each and run the two halves; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// Deterministic, no model. An unscoped search over the real index returns another
// household's private email; the agent's real tenant-scoped Query never does,
// while still returning the asker's own and public documents, and the rightful
// owner still retrieves her own. The leaked line is a benign marker.

func TestLeakLandsUndefended(t *testing.T) {
	path, cleanup := buildIndex()
	defer cleanup()
	if !leakLandsUndefended(path) {
		t.Fatal("an unscoped retrieval must leak another tenant's document; otherwise there is nothing to defend")
	}
}

func TestACLScopesToAsker(t *testing.T) {
	path, cleanup := buildIndex()
	defer cleanup()
	if !aclScopesToAsker(path) {
		t.Fatal("the tenant-scoped Query must hide other tenants, keep the asker's own and public docs, and still serve the rightful owner")
	}
}
```
<!-- /listing -->

## Make the database keep the rule

The reason this belongs in the query and not in a wrapper is that wrappers are optional and `WHERE` clauses are not. Any retrieval the agent does, through any path, goes through the same `Query`, so the tenant rule is enforced once, by the database, for everyone, instead of being re-implemented and occasionally forgotten at each call site. That is the general shape of a good authorization control: push it down to the layer that cannot be bypassed, and let every caller inherit it.

That is leaking across households closed. The last leak in this chapter is across time. Retrieval pulls from a store that outlives any single email, and a document poisoned or wrongly scoped today keeps coming back tomorrow. The next post gives the memory a lifecycle, a scope and an expiry and a sanitized recall, so that what the agent remembers cannot quietly become what the agent is told.

## Take it to your own project

Scope every lookup to who is asking:

- Check ownership before returning any retrieved record.
- Scope retrieval and queries to the current tenant or user.
- Default to returning nothing when scope is unclear.
- Test a cross-tenant query and confirm it returns none of the other tenant's data.
