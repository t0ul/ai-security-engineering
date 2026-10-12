---
title: "Cross-tenant leak"
description: "The last leak was about what a document said. This one is about whose documents come back at all. One retrieval store holds more than one household's email, and a search that is not scoped to the asker hands one household's private line to another. The fix is to put the tenant check inside the search itself, so authorization is not a step the retrieval path can forget."
date: 2026-10-12
draft: true
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

## Two households in one index

The index holds three documents in the agent's real retrieval store. Alice has a private reminder, a therapy appointment, under her tenant. Bob has a PTA note under his. And there is one public school notice under no tenant at all, which everyone is allowed to see. The therapy line is a benign marker standing in for whatever genuinely private thing a household's email holds.

Now Bob asks about his appointments. The retrieval I would write first just searches the index for the query and returns what matches, ranked by relevance:

```go
db.Query(`SELECT text FROM docs WHERE docs MATCH ? ORDER BY rank LIMIT ?`, expr, k)
```

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

The only difference from the naive version is the `AND (tenant = ? OR tenant = '')`, and it is the whole lesson. A document comes back only if it belongs to the asker or is public, the empty tenant. There is no second step where application code filters the results and could be skipped, no place to forget: a row that is not yours is never selected in the first place. This is authorization-first retrieval, the ACL living in the same statement that does the search. Run Bob's question through it:

```
== after: the agent's Query, with the tenant ACL in the search ==
  Bob's query -> 2 hits; Alice's private line present: false
  Bob still sees his own + public: own=true public=true
  rag-tenant-leak ASR: 0% (0/1)
  Alice's own query still returns her private line: true
```

Two hits now, Bob's own note and the public notice, and Alice's appointment is gone from his results. The control is not a blunt instrument, though: the public notice still reaches Bob, and Alice asking the same question still gets her own private line back. Scoping to the asker is exactly that, scoping, not hiding everything.

The agent goes one belt tighter, which is worth noticing in the code above. The search path opens the store read-only, so the connection that serves a query physically cannot write the index. Tenant scoping decides what a reader may see; the read-only connection makes sure a reader is only ever a reader, so a bug or an injection on the retrieval path can neither leak across tenants nor poison the corpus for the next one.

## Make the database keep the rule

The reason this belongs in the query and not in a wrapper is that wrappers are optional and `WHERE` clauses are not. Any retrieval the agent does, through any path, goes through the same `Query`, so the tenant rule is enforced once, by the database, for everyone, instead of being re-implemented and occasionally forgotten at each call site. That is the general shape of a good authorization control: push it down to the layer that cannot be bypassed, and let every caller inherit it.

That is leaking across households closed. The last leak in this chapter is across time. Retrieval pulls from a store that outlives any single email, and a document poisoned or wrongly scoped today keeps coming back tomorrow. The next post gives the memory a lifecycle, a scope and an expiry and a sanitized recall, so that what the agent remembers cannot quietly become what the agent is told.
