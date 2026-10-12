---
title: "Answering what is on this month"
description: "Part 4 finished the agent's architecture. Part 5 gives it memory: the ability to answer a question by retrieving across everything it has ever read, not just today's email. This post builds that retrieval with GoRag, shows it answering what is on this month, and does not attack it, because it is the feature the rest of the chapter attacks."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "rag", "retrieval"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## A question the agent could not answer yet

Everything so far has worked one email at a time: a newsletter comes in, the agent extracts its events, and that is the end of the transaction. But the questions you actually want to ask an assistant are not about one email. "What is on this month?" is a question about everything it has ever read, and to answer it the agent needs to look back across a pile of past newsletters and pull out the few passages that are relevant now.

That is retrieval, and it is what Part 5 is about. This post builds it, and like [the eval harness](/blog/the-eval-harness-f1-as-a-receipt/) earlier, there is nothing to attack here. The job is to stand up the feature plainly, because the danger it introduces is the subject of the next three posts. The moment the agent answers from a store of things it read in the past, that store becomes a place an attacker can hide a message, and a poisoned passage retrieved today is just as dangerous as a poisoned email arriving today, with none of the scrutiny. So first, the feature.

## Ingesting with GoRag

Retrieval starts before any question is asked, by turning raw emails into clean, searchable units. I added [GoRag](https://github.com/t0ul/GoRag), a small dependency-free Go library that is the ingestion half of a retrieval pipeline. It does two jobs. First it cleans the text: real emails arrive full of hard-wrapped lines, runs of spaces, and invisible characters, and `gorag.Clean` standardizes all of that without touching the words, so the index is not full of noise. Then it chunks the cleaned text into passages, with a choice of strategy, because the unit you index is the unit you retrieve: index whole emails and a hit drags in the whole thing, index paragraphs and a hit is a focused passage.

Here it is run over one of the sample newsletters from the setup post, cleaned and split into paragraph chunks:

```
ingested newsletter.txt: 8 chunks (paragraph, max 280 chars)
```

The retrieval store, the part that holds those chunks and ranks them against a query, lives in the agent rather than in GoRag, because it is app-specific. In [emaildrop](https://github.com/t0ul/emaildrop) it is `pkg/server/rag.go`, which wires GoRag's cleaner and chunkers to a store with a governed configuration: which chunker, what size and overlap, how many passages to return. GoRag stays generic so any project can reuse it; the policy about how this agent retrieves stays with this agent.

## Answering the question

With the newsletter ingested, a question becomes a lookup. "What is on this month?" resolves to the current month, and the retriever returns the chunks that match, so the agent answers from a handful of relevant passages instead of re-reading the entire inbox.

```
query: "what is on this month?" -> match "october"
  [chunk 1] Multicultural Potluck: Friday, October 23rd from 5:30PM -7:00PM, School Cafeteria...
  [chunk 6] Friday, October 23rd from 5:30PM-7PM, School Cafeteria. Join us for our beloved community event, our Multicultural Potluck...

2 chunk(s) retrieved; the agent answers from these, not the whole inbox
```

Two chunks came back, both the Multicultural Potluck, and the agent composes its answer from those. That is the whole shape of retrieval-augmented generation: find the relevant passages, then let the model answer with them in front of it. The real pipeline ranks by relevance rather than a plain keyword match and caps the result at a governed top-k, but the move is the one you see here, pull the few passages that matter and answer from them.

You can reproduce the ingestion directly against GoRag, and the full in-app version from an emaildrop checkout via `pkg/server/rag.go` and the ask endpoint in `pkg/server/ask.go`.

## What this just opened

Look again at where the answer came from. The agent did not reason about the newsletter in front of it. It reached into a store, pulled out passages it had filed away earlier, and put them in front of the model as context to answer with. Every defense in Part 3 and Part 4 was about the email arriving now: spotlight it, scrub it, confine what it can do. None of them watched the store.

So the attacker's move writes itself. If a passage in the index becomes part of the prompt the moment it is retrieved, then getting a malicious passage into the index is getting an instruction into the agent, delivered later, out of context, with the authority of something the agent chose to recall. The next post plants exactly that, a document that steers the answer, and shows what encapsulating a retrieved chunk as data rather than instructions does to it. After that, two more leaks this same store can spring: one tenant's data surfacing in another's results, and a poisoned memory that outlives the email that planted it.
