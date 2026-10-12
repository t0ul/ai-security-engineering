---
title: "My week at a glance"
description: "The weekly view groups the household's items by day, and which day an item lands on is a decision an email can lie about: state one weekday while the date says another. A view that places items by the weekday written in the text files them on the wrong day and leaves the real day looking empty. The agent places by the date it parsed, derives the weekday from that, and flags the mismatch."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents", "integrity"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The view that lays out your week

The last feature sorted items by kind. This one lays them out in time: My Week, the seven-day strip that answers "what have we got going on" at a glance. It is the view people actually open, and building it means a new decision the agent has to make for every item, which day does this land on.

That decision looks trivial and is not, because an email gets a vote in it. Dates in school emails are written for humans, with the weekday spelled out next to the number, "Friday, October 2nd", and a weekday is redundant information that can disagree with the date it sits beside. Usually that is an honest typo. It can also be crafted. A control lesson, deterministic, no model:

```bash
cd posts/13.3-my-week
go run .
```

## Place by what the email says, and land on the wrong day

Here is the week. A genuine Book Fair, a genuine PTA meeting, and one crafted item: a sync that says Monday but is dated October 8th, which is a Thursday.

The natural way to build the view is to read the weekday off each item and drop it in that column. It is right there in the text, after all.

```
== before: place items by the weekday the email states ==
  Monday:   Mandatory all-staff sync
  Thursday: PTA Meeting
  Friday:   Book Fair
  crafted item on Monday: true | on its real day (Thursday): false
```

The sync is on Monday now. Not on Thursday, where it actually falls, but on Monday, because that is the word the email used. A parent scanning their week sees a quiet Thursday and a Monday commitment that is not real, plans around both, and is wrong twice from one crafted line. Nobody clicked a link; the attack is just a date that lies about its own weekday, and a view that believed it.

## Place by the date, derive the day

The fix is the rule from the integrity chapter, now doing a job in the UI: the date is the authority, the weekday is decoration. The agent's My Week groups items by the date it parsed and derives the weekday from that date, so the word in the text never decides a column. And because it already knows both, it can say when they disagree.

```
== after: place items by the date, weekday derived from it ==
  Thursday: Mandatory all-staff sync; PTA Meeting
  Friday:   Book Fair
  crafted item on its real day (Thursday): true | on Monday: false
  flagged: weekday_mismatch: stated Monday, but 2026-10-08 is Thursday
```

The sync is on Thursday, next to the PTA meeting it shares the day with, and Monday is empty because nothing is on it. The owner sees the real shape of the week. And the mismatch is not swallowed silently, it is surfaced: stated Monday, but the 8th is a Thursday, the same deterministic check from the conflicts post, reused to label a crafted item instead of just correcting it. Honest placement, and a note about why the email was wrong.

## The same authority, everywhere it matters

There is nothing new in this fix, and that is the point worth making at the capstone. The weekday check was built chapters ago to catch a mangled date in extraction; here it places an item in a calendar view; the same code will decide which day a reminder fires on and which day a question about "this week" is answered for. One deterministic authority on what day a date is, used everywhere a day matters, so there is no second place for a crafted weekday to slip through. The model and the email can suggest; the date, parsed once by code, decides.

The next feature opens the agent up to people, a contact directory built from who appears in your mail, and its attack is about disclosure: surfacing the right contacts to the owner without quietly building a dossier an injected email can ask for.
