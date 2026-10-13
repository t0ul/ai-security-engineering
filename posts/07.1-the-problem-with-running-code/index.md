---
title: "The problem with running code"
description: "Part 4 changes the kind of problem. So far the agent has read mail, written inert files, and fetched through a gate. Now it gets a tool that runs code, and an agent that executes a command a model produced from a hostile email is a different class of danger. This post shows the unguarded version, with nothing between the model and a shell, so the walls the rest of Part 4 builds have something real to defend against."
date: 2026-01-13
draft: false
tags: ["ai-security", "golang", "agents", "isolation"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## A different class of danger

Everything in the first three parts was, in the end, survivable with ordinary care. The agent read untrusted mail, wrote a file nobody auto-imports, and when it reached the network it did so through a gate that checked the address. The worst a single failure could do was calendar a wrong event or leak one request, and each of those we closed.

Part 4 is different, because here the agent gains a tool that runs code. The honest reason an email-to-calendar agent would ever need that is real enough, summarizing an attached script, extracting data from a format only a program can parse, and the industry is busily handing agents shells and interpreters as fast as it can. But an agent that executes a command, driven by a model that just read an email from someone you do not control, is not a step up in risk from a fetcher. It is a different kind of thing. This post does only one job: show how bad the unguarded version is, with nothing between the model and a shell, so that the walls the rest of this part builds are walls against something real. There is no fix in this post. The fixes are the next four.

## The one line that owns the machine

Here is the dangerous version, and it is dangerous precisely because it is so reasonable-looking. The model produced a command, so the agent runs the command:

```go
// the model produced a command from the email; the agent runs it
modelProduced := theModelsCommand()
exec.Command("sh", "-c", modelProduced).Run()
```

That is the whole vulnerability: two lines, no villainy, just a tool doing its job. To show it without harming anything, I had the model "produce" a command that is completely harmless, one that writes a marker file so we can prove execution happened and nothing more. I want to be very clear that this is the safe stand-in: in a real attack the command on that line is `rm -rf`, or a reverse shell, or a loop that reads every file you own and posts it to an attacker. I am not going to run any of those, and neither should you. The marker is enough to make the point:

```
model produced:  echo OWNED-BY-THE-EMAIL > /tmp/agent_pwned.txt
agent ran it, and the host now has a file saying: OWNED-BY-THE-EMAIL
proof: a command the agent never vetted executed on the machine
```

A command that came, through the model, from an email, ran on my machine and left its mark. It ran as me, with my permissions, my files, my credentials, my network. The agent did not decide to do anything wrong; it ran what it was told, which is the entire job of an executor, and that is exactly why handing an executor to a model that reads hostile text is handing the machine to whoever wrote the email.

## Why a filter is not a wall

It is tempting to think you can make this safe by inspecting the command: block `rm`, block `curl`, refuse anything that looks dangerous. That road does not end anywhere good. A shell is a programming language with endless ways to say the same thing, encodings, variable expansion, obscure builtins, and a determined attacker has more spellings of a bad command than you have patterns to block. Every allow-list of strings over a shell is a list of the attacks you thought of, and the attack that matters is the one you did not.

And notice what has quietly happened to the whole threat model. Back in [the plan](/blog/the-plan/) I named the lethal trifecta: private data, untrusted input, and a way to reach the outside world, all in one process. Until now the agent never had all three at full strength. With a real executor it does, and more, because code can do anything the three could and then persist to do it again. A regular expression over a URL, which was a perfectly good control for [a fetch tool](/blog/a-search-tool-hits-a-local-server/), is not a control against a process that can rewrite the regular expression.

## What a wall actually looks like

So the answer is not to inspect the command harder. It is to run it somewhere it can do no harm no matter what it is, and to make sure the part of the agent that decides what to run is never the part that touched the hostile text. That is the shape of the rest of Part 4, and it is worth seeing the whole plan before the first piece:

- An isolated machine with no network card, a small virtual machine the code runs inside, with no path to the host's files and no way to reach the internet. Whatever the command is, it executes in a box that contains nothing and connects to nothing.
- A two-model design, so the model that reads the untrusted email and the model that is allowed to act are different, and the output of the first is only ever data to the second, never instructions.
- A single audited broker for the rare, legitimate moment the code genuinely needs the network, the one door out, logged and allow-listed.
- Arguments as arrays against an allow-list instead of a shell string, so there is no shell to be clever with in the first place.

None of these is a filter on the command. Each one removes a capability the attack needs, so that running the code, even code written by an attacker, simply cannot reach anything that matters.

## Conclusion

That is the problem, shown plainly: an agent that runs a model's command owns nothing less than the machine, and no amount of squinting at the command text changes that. It is the most dangerous thing in the whole series, which is why it gets a whole part and why that part is built out of isolation and separation rather than inspection.

The next post builds the first and most important wall: the detonation chamber, a virtual machine with no network and no access to the host, where a command can run and touch nothing. We take the exact line above, the one that owned the machine, and move it inside a box, and watch the same command run and reach nothing at all.
