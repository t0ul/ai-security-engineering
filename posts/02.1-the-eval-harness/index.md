---
title: "The eval harness: F1 as a receipt"
description: "Chapter 1 built a small agent. Before we attack it, we need a way to say it works that is a number, not a feeling. This post builds the eval harness: hand-labeled answers, a scorer, and an F1 of 1.00 on the clean sample that becomes the baseline every later attack is measured against."
date: 2025-09-30
draft: true
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## "It works" is a feeling

Chapter 1 built the boring core: one event shape, dates checked by code, inert output, a watched folder with limits. Along the way I kept saying the agent works, and you have no reason to take that on faith, because I have not shown you a single number. "It works" is a feeling, and a feeling is no way to run a security project.

This short post fixes that, and it is the hinge of the book. There is nothing to attack here. The job is to build the ruler, because the next post swings the attacks at the whole agent and I need a way to measure what they break. If you cannot score the agent clean, you cannot show it getting worse under attack, and you cannot show your defense putting it back. So before any of that, a number.

## I. A hand-labeled answer key

The harness is small and old-fashioned, which I like. It is a set of sample emails with the correct answers written down by hand, and a scorer that compares what the agent extracted to those answers. The answers live next to the samples, in the product repo, as plain JSON. Here is the one for the newsletter from the setup post:

```json
{
  "source_email": "samples/3.txt",
  "default_year": 2026,
  "events": [
    {"title": "PTA Meeting", "start": "2026-09-24T08:30:00", "end": null, "all_day": false, "location": "Gymnatorium"},
    {"title": "Back-to-School Night", "start": "2026-09-29T17:30:00", "end": "2026-09-29T20:00:00", "all_day": false, "location": "School Building"},
    {"title": "Multicultural Potluck", "start": "2026-10-23T17:30:00", "end": "2026-10-23T19:00:00", "all_day": false, "location": "School Cafeteria"}
  ]
}
```

These three are the Save the Dates, the events that genuinely must be caught in that email. A human read the newsletter and wrote them down. That is the ground truth, and it is the part no code can produce for you: someone has to decide what the right answer is. The scorer then runs the extractor over `samples/3.txt` and checks its output against this list.

## II. Running it

From your emaildrop checkout, score the deterministic extractor against that answer key:

```bash
EXTRACT_MODE=regex go run ./cmd/eval
```

```
=== eval: samples/3.txt ===
gold=3 pred=3 matched=3
precision=1.00 recall=1.00 f1=1.00
  [ok] PTA Meeting            start 2026-09-24T08:30:00==2026-09-24T08:30:00:true  end -==-:true  loc:true
  [ok] Back-to-School Night   start 2026-09-29T17:30:00==2026-09-29T17:30:00:true  end 2026-09-29T20:00:00==2026-09-29T20:00:00:true  loc:true
  [ok] Multicultural Potluck  start 2026-10-23T17:30:00==2026-10-23T17:30:00:true  end 2026-10-23T19:00:00==2026-10-23T19:00:00:true  loc:true
field accuracy: start 3/3  end 3/3  location 3/3
```

Read the top line first. `gold=3` is how many events are in the answer key. `pred=3` is how many the extractor returned. `matched=3` is how many of its answers lined up with a real one, where lining up means the same day and a matching title. Three labeled, three found, three correct.

The second line turns that into the three numbers worth knowing. Precision is how many of the extractor's answers were real: it returned three and all three were right, so 1.00. Recall is how many of the real events it found: there were three and it caught all three, so 1.00. F1 is the single score that combines them, the harmonic mean of the two, and it is 1.00 only when both are. One number, and it is only kind to you when you have neither invented an event nor missed one.

The per-event lines go one level deeper, because catching an event on the right day is not the same as getting it right. For each match the scorer checks the start time, the end time, and the location, and the `field accuracy` line at the bottom tallies them: start 3 of 3, end 3 of 3, location 3 of 3. Back-to-School Night is not just on September 29, it is on September 29 from 5:30 to 8:00 at the School Building, and every field agrees.

## III. Why a receipt and not a test

That `f1=1.00` is what I mean when I say "it works," and it is different from the Go tests in two ways that matter for the rest of the book.

First, it is a gate, not just a report. The eval command exits zero only when F1 is exactly 1.00, so it fails loudly the moment the extractor drops an event or invents one. It is wired into the product's checks for that reason: a change that quietly makes extraction worse cannot pass. A regression in the thing the whole agent exists to do is caught by a number going below one.

Second, and this is why the post exists here rather than buried later, it is the baseline. Right now, on clean mail with no attacker in the picture, the agent scores a perfect one. That is the "before" the entire rest of the book measures against. When the next post points the red-team tools at the agent and an injected instruction makes it calendar an event that was never in the email, this same kind of measurement is how we will see the damage, as a number falling. And when a defense goes in, it is how we will see it climb back. A defense you cannot measure is a defense you are taking on faith, which is the exact thing this harness exists to refuse.

I will be plain about the scope. This is one labeled sample, the clean calibration email, scored against a human's answer key. It is not a claim that the extractor is flawless on every newsletter ever written; squeezing real-world extraction up is its own thread later in the book. What it is, is an honest, reproducible zero-attack baseline: on this email, the agent is perfect, and now we have the ruler to prove it and the gate to keep it there.

## Conclusion

So the agent finally has a number, and the number is one. That is the end of the quiet part. Chapter 1 built the thing and Chapter 2 opened by measuring it, and the next post is the one the whole book has been walking toward: I point the red-team tools at the entire undefended agent, with no defenses in place at all, and we watch every attack land at once. The clean `f1=1.00` you just ran is the last time the board is green for a while. The next thing you will see is it going red, on purpose, and that red board becomes the to-do list for everything after.
