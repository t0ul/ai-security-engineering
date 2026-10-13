---
title: "Scrubbing secrets and PII at ingest"
description: "The agent indexes every email into a searchable corpus so it can answer questions later. That corpus is on disk, so whatever secret an email carries ends up stored in the clear, waiting for a leak. This post indexes a secret-laden email as-is and finds everything on disk, then scrubs at ingest with goflage, and is honest about the one token format the scrubber does not know."
date: 2025-11-25
draft: true
tags: ["ai-security", "golang", "agents", "pii"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## What the agent keeps

Chapter 4 was about what the model is told. Chapter 5 is about what the agent keeps. To answer a question like "what is on the calendar this month," the agent indexes every email it reads into a searchable corpus, so it has something to search later. That corpus is a file on disk, and it is broadly queryable by design, which makes it the quietest privacy problem in the whole project: anything secret that an email happens to carry gets copied into it, in the clear, and then just sits there. A later prompt-leak like the one from [the last post](/blog/system-prompt-leakage/), or a careless query, and it is out.

So the rule for private data is the boring, effective one: do not let it reach the corpus in the first place. Scrub it at the door. This is a control lesson, deterministic, no model:

```bash
cd posts/05.1-scrubbing-secrets-and-pii
go run .
```

## I. The corpus remembers everything

Here is the kind of email that causes the problem. It is a mundane directory update, and like a lot of real mail it carries things that should never be stored: a parent's address and SSN, and, pasted into an admin note the way these things actually leak, a cloud key, a code-hosting token, and a billing key. Everything in it is fake, but the shapes are real.

```
New parent contact: parent.contact@example.com.
For the records office, the enrollment ID carries SSN 123-45-6789.
(Admin note: the sync job uses aws key AKIAIOSFODNN7EXAMPLE and the GitHub
token ghp_abcdefghijklmnopqrstuvwxyz0123456789. The new billing key is
sk-live-abc123DEF456ghi789.)
```

Index it the way the agent does, and then look at what landed on disk:

```
== before: index the email as-is ==
  parent email  on disk: true
  SSN           on disk: true
  AWS key       on disk: true
  GitHub token  on disk: true
  every secret the email carried is now in the searchable corpus, in the clear
```

Every one of them, written into the searchable store verbatim. The agent did nothing wrong by its own lights; it indexed an email so it could answer questions about it later. But it has now made a durable, queryable copy of a stranger's SSN and a live-looking set of credentials, and every email it ever reads adds to the pile.

## II. Scrub at the door

The fix is to clean the text before it is ever indexed. I added `goflage`, a small dependency-free Go library that finds and redacts secrets and personal data. It is deliberately not a model: it uses regular expressions and checksums to recognize the high-precision cases, emails, US SSNs, AWS keys, API tokens, JWTs, Luhn-valid credit cards, IP addresses, and replaces each with a labelled placeholder. What it defends against is exactly the leak above: run the email through it at ingest and the corpus never sees the real values.

```
== after: scrub at ingest with goflage ==
  goflage findings: [{API_TOKEN 1} {AWS_ACCESS_KEY 1} {EMAIL_ADDRESS 1} {US_SSN 1}]
  parent email  on disk: false
  SSN           on disk: false
  AWS key       on disk: false
  GitHub token  on disk: false
```

It reported what it found, four kinds of secret, and after scrubbing, not one of the four is on disk. The corpus holds the readable parts of the email with `<EMAIL_ADDRESS>`, `<US_SSN>`, `<AWS_ACCESS_KEY>`, and `<API_TOKEN>` where the secrets were. The agent can still search and summarize the email; it just can no longer leak what it redacted, because it never stored it.

## III. What it does not catch

Now the honest part, because a scrubber you trust blindly is its own hazard. Look again at that email: there was a fifth secret, the billing key, and it is not in the list of findings. Check the corpus for it:

```
  but the unrecognized billing key is still on disk: true
```

It slipped straight through. `goflage` is a deterministic, high-precision tool: it redacts the shapes it has a recognizer for, and no others. The billing key's format is not one it knows, so to the scrubber it is just a word, and it sailed into the corpus with everything else benign. This is the real shape of pattern-based scrubbing, and it is why the library draws its own line honestly: open-ended detection, the "is this a name, is this a secret I have never seen" judgment, belongs behind a model, which is a different and heavier tool. A regex scrubber is fast, dependency-free, and exact on what it knows, and silent on what it does not.

So two things follow. The missing recognizer is a bug with a fix, a pattern to add, and the lesson's test pins the gap so that the day `goflage` learns that format, the test fails and reminds me to update this page. And more durably, a scrubber is one layer, not a guarantee: it belongs in front of the corpus, but the corpus should also assume something slipped, which is why the posts after this one keep tightening what can leave the agent even when something private did get in.

## Conclusion

The agent now cleans private data out of the text before it is ever stored, so the searchable memory it builds is not also a searchable copy of everyone's secrets. The recognized shapes, emails, SSNs, keys, and tokens, never reach disk, and the one shape the scrubber does not know is a named, honest gap rather than a silent one.

That handles a secret coming in. The next posts handle a secret going out, through the parts of the agent that reach the world: the calendar file it writes, and the links it renders. The first is a quiet exfiltration channel hiding in a place no one looks, the fields of a calendar event.

## Take it to your own project

Catch secrets and personal data at the door:

- Scrub secrets and PII at ingest, before anything reaches disk or the model.
- Maintain recognizers for the key and token formats you actually handle.
- Verify nothing sensitive lands in the corpus, with a test.
- Prefer dropping sensitive data to masking it where you can.
