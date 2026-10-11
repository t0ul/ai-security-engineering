---
title: "Start here: how to use this handbook"
description: "A short welcome to the AI Security Engineering Handbook: what you'll build, the one idea the whole book runs on, and how to work through the repo, one runnable lesson at a time."
date: 2026-09-30
draft: true
tags: ["ai-security", "golang", "agents"]
---

_The front door to my AI Security Engineering series, a local email-to-calendar agent I build, then attack, then harden. The code is [on GitHub](https://github.com/t0ul/ai-security-engineering)._

## Welcome

This is a book about building an AI agent that does not get owned.

We build one real thing together: a small agent that reads my kid's school email and puts the real dates on our family calendar, running entirely on my own machine. It is a boring, useful job, and it happens to sit on top of the most dangerous combination in AI security, a language model holding private data, reading text from people you do not control, with tools that can act on the world. So it is a perfect place to learn how to do this safely.

You do not need to have done much of this before. I explain each term and each tool the first time it shows up, and every step comes with the exact command to run.

## The one idea

Security usually gets bolted on at the end. This book does the opposite. Every feature we build goes through the same small loop:

1. Get a small thing working.
2. Try to break it, with a real attack, and measure how badly it fails.
3. Fix what broke.
4. Run the exact same attack again and show it is gone.

I call that Attack-Driven Design. The point is that nothing here is a claim you have to take on faith. Every defense ends on a number you can reproduce, an attack that went from landing every time to landing never.

## How to work through the repo

This repo is laid out the way the book is. Everything lives under `posts/`, and each folder is one lesson that matches one post:

- Read the post in `posts/<id>-<slug>/index.md`.
- Run that folder. It runs the attack against the undefended code and you watch it land, then against the fixed code and you watch it get blocked.
- The before and after are real output, from code you can read, on your machine.

Work through them in order. Each lesson assumes the one before it, and the whole thing builds, piece by piece, from a plain parser into an agent you could actually trust with your inbox. The `posts/README.md` file explains the folder layout if you want to look under the hood.

## Where to go next

Read [the plan](/blog/the-plan/) for what we are building and why, and the full roadmap. Then [set up your lab](/blog/setting-up-the-lab/), which takes you from a bare Mac to the agent running in about fifteen minutes. After that, the lessons start, and we build the boring core before we ever point an attack at it.

Thanks for being here. Let us build something small, break it on purpose, and learn how to make it hold.
