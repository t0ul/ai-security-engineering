---
title: "TODO: calm, descriptive title (thing: plain clarifier)"
description: "TODO: one or two plain sentences; feeds meta description + OG."
date: 2026-MM-DD
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The full source is [on GitHub](https://github.com/t0ul/ai-security-engineering)._

<!--
Post recipe (never print these labels; they are the skeleton, not the prose):
1. Open with the real motivation in t's voice.
2. Get the one feature working: a command and its output.
3. Attack it with the tool and show the findings (the break, measured, ideally found by accident).
4. Resolve: add the library, explaining what it is / why / what it defends, woven into sentences.
5. Re-attack, show it clean (ASR 0%, the 400, chain_ok=false caught). That is the receipt.
Hard rules: no em dashes; no label headers; calm, not hacker-bro; verify every receipt live.
-->

## TODO

This is a control lesson and it runs on its own. The whole program is on this page, in the order it runs, so you can type it in as you read. Or clone the lessons and run the folder:

```bash
cd posts/TODO-id-slug
go run .
```

If you are typing along, make an empty folder, run `go mod init lesson`, then `go get` each module the file below requires, and put everything in one file called `main.go`:

<!-- listing: go.mod#all -->
<!-- /listing -->

## The spine of the program

<!-- listing: main.go#head -->
<!-- /listing -->

## I. TODO the feature, and the attack that lands

<!-- listing: main.go#before: the feature with no control -->
<!-- /listing -->

## II. TODO the control, and the same attack blocked

<!-- listing: main.go#after: the same attack, against the control -->
<!-- /listing -->

## The test is the receipt

<!-- listing: contract_test.go#all -->
<!-- /listing -->
