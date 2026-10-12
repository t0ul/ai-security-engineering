---
title: "Model artifacts"
description: "The agent downloads its model as a file from somewhere and runs it. One model format, the pickle, executes code the moment you load it, so a swapped file is remote code execution before the model says a word. This post runs the agent's real supply-chain gate, which refuses anything that is not a magic-verified GGUF pinned to the hash you approved, and deletes it before it is ever loaded."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "supply-chain", "models"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The model is a download you trust

The last post pinned the tool catalog so the agent only calls tools it approved. But the model is itself a file the agent downloaded from somewhere and runs, and so are its weights and adapters. The same question applies, and it is sharper here, because of how model files load.

The common interchange format in the Python world is the pickle, and a pickle is not data, it is a program. Loading one runs whatever its author put in it, by design. So a model distributed as a pickle, or a checkpoint that smuggles one, is code execution the instant you load it, before the model produces a single token. You do not get to inspect it first, because inspecting it, in the naive way, means loading it. An attacker who can swap the file you download, a poisoned mirror, a typosquatted repo, a man in the middle, owns the machine the agent runs on. This is a supply-chain attack, OWASP's LLM03, and the model file is the supply.

This is the one lesson in the chapter where the control is not an importable library but a gate the agent runs as a build step, so I am going to run the real gate here and show you its output, the same way I did with [the detonation chamber](/blog/the-detonation-chamber/).

## The agent only loads GGUF, pinned

The agent does not use pickles. It runs its models as GGUF, which is a plain data format with no code in it, and it refuses to fetch anything else. The gate is a small program called `verifymodel`, and before any file is allowed near the runtime it checks three things, in order: the URL ends in `.gguf` and nothing else, the file's SHA-256 matches the hash the agent was pinned to, and the file actually begins with the GGUF magic bytes. Anything that fails is deleted on the spot, so a rejected artifact is not sitting on disk waiting to be loaded by mistake.

Run the gate against a pickle offered as a model, and it does not get far. Named `.pkl`, it is refused outright: the agent will not even fetch a URL that does not end in `.gguf`, so a pickle never reaches the machine in the first place. An attacker will of course just rename their file, so the refusal that matters is the next one, against the same pickle renamed to look like a GGUF:

```
the pickle renamed .gguf: the magic-byte check refuses it before any load
verifymodel: security check failed: assets: file is not a valid GGUF binary (got [128 4 149 76])
```

The bytes `[128 4 149 76]` are the start of a pickle, `\x80\x04`, its protocol header. They are not `GGUF`, so the file is rejected and deleted, and the code inside it never runs, because the gate decided on the first four bytes and never handed the file to a loader. The pickle I used is inert, a plain dictionary, standing in safely for a malicious one, and the point is that it never gets the chance to be anything.

## A real model, pinned to a hash

The magic check stops a pickle wearing a GGUF name. It does not stop a different GGUF, a subtly altered model, a backdoored fine-tune, that is still a valid GGUF. That is what the hash pin is for. The agent is approved against one specific SHA-256, and a file that does not match it is refused even though it is a perfectly well-formed GGUF.

```
3. a real GGUF pinned to the WRONG hash: refused
   verifymodel: security check failed: assets: SHA-256 mismatch:
   want 0000...0000, got c50d4b51e037e14f777536fc58aeda2d52d48ad9ae0d60596ff8c7c7598da46c

4. the same GGUF pinned to the CORRECT hash: verified
   verified: <server>/model.gguf -> .../dest/m2
```

Only the fourth file survived on disk. The three refused artifacts were each deleted by the gate as it rejected them, so nothing that failed verification is left lying around to be picked up later. The model that runs is the exact file whose hash you approved, in a format that cannot carry code, and nothing else gets through.

## Reproduce it

The gate lives inside the product, in `internal/assets`, which Go keeps unimportable from outside the module on purpose, so this lesson runs it as the tool it is rather than wiring it into a test. From an [emaildrop](https://github.com/t0ul/emaildrop) clone:

```bash
go build -o /tmp/verifymodel ./cmd/verifymodel

# an inert pickle, offered as a model, is refused before it can load
printf '\x80\x04\x95' > /tmp/evil && mv /tmp/evil /tmp/evil.gguf
python3 -m http.server 8099 &   # serve /tmp
/tmp/verifymodel -url http://127.0.0.1:8099/evil.gguf -dest /tmp/out.gguf

# a GGUF pinned to its real hash verifies; change one byte of the hash and it does not
printf 'GGUF\x03\x00\x00\x00' > /tmp/model.gguf
SHA=$(shasum -a 256 /tmp/model.gguf | cut -d' ' -f1)
/tmp/verifymodel -url http://127.0.0.1:8099/model.gguf -dest /tmp/ok.gguf -sha "$SHA"
```

You will see the pickle rejected on its magic bytes and the GGUF accepted only against the right hash, exactly as above. Pin the hash the same way you would pin a dependency, and treat the model as what it is: the largest and most trusted download the agent makes.

## The trifecta, named

Two chapters of Part 4 are now in place. Code the agent runs is boxed away from the host, the hand-off between its brains is authenticated, the tools it calls are pinned, and the model it loads is verified. Each of those closed off one leg of what people have started calling the lethal trifecta: access to untrusted content, access to private data, and the ability to act or communicate outward. Any one leg alone is survivable. All three at once, in a single agent that reads your email, holds your calendar, and can reach the network, is the shape every attack in this book has been circling. The next post puts the three legs on the table together and shows how the controls so far keep them from lining up into a working exfiltration.
