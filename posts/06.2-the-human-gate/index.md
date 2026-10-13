---
title: "The human gate"
description: "Some actions are irreversible, and no automatic rule should ever perform them. They need a person to say yes. But a one-click yes can be forged, overlaid, or rubber-stamped, so this post builds the real gate: an approval is valid only when a human was shown the evidence and echoes back a nonce displayed with it, which a blind or forged click can never produce."
date: 2025-12-30
draft: true
tags: ["ai-security", "golang", "agents", "human-in-the-loop"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The action no rule should take on its own

[The last post](/blog/least-privilege/) scoped what each tool is allowed to do. This one is about a category of action that no scope should ever grant automatically: the irreversible kind. Putting an event on your real calendar, following a link, sending something on your behalf, these are not things you want a confident model deciding by itself, no matter how well its tools are fenced. They need a person to say yes.

I built a simple version of this back in [the inert-output post](/blog/inert-output-and-the-human-accept/): the agent writes a file and a human imports it, so nothing reaches the real calendar on its own. That was enough when the only question was whether to trust an extraction. It is not enough once the yes itself becomes a target, because a one-click approval is its own attack surface. People rubber-stamp confident AI, a dialog can be overlaid or clickjacked so you approve something other than what you see, and a forged request can simply claim approval that nobody ever gave. So this post is the real gate. It is a control lesson, deterministic, no model:

```bash
cd posts/06.2-the-human-gate
go run .
```

## I. One click is not consent

Here is the approval I would write first: if the request says approve, do the thing. The entire check is a single boolean. Now watch what a bare, forged approval does to it, nobody having seen or reviewed anything:

```
== before: a one-click accept ==
  a bare "approve: true" put it on the calendar: true (calendar: [Principal's Gala (Dec 25)])
```

The gala, an event a hostile email invented, went straight onto the calendar because something, somewhere, sent `approve: true`. That boolean could come from a human who carefully read an event and agreed, or from a clickjacked overlay, or from a persuasive email that talked the user into clicking without looking, or from a forged request that skipped the human entirely. The naive accept cannot tell these apart, because a boolean carries no evidence that a specific person saw a specific thing. It just says yes, and yes was all it needed.

## II. Evidence you saw, a nonce you echoed

A real approval has to prove two things the boolean cannot: that a human was shown what they were approving, and that the yes came from that same showing and not from somewhere else. The agent's approval primitive, `hitl`, is built around exactly that. When the agent wants an irreversible action, it does not offer a button, it issues a challenge: a summary, the evidence to review, and a nonce displayed alongside them. The action is performed only when the approval echoes back that nonce. What this defends against is the whole family of cheap yeses: a blind or forged request has no nonce to echo because it never saw the challenge, an overlaid click cannot echo a nonce it was drawn on top of and cannot read, and an approval with no evidence attached cannot be confirmed at all. Consent becomes a specific human, shown a specific thing, echoing a specific token, which is a far higher bar than a bit set to true.

## III. The gate holds, and the real yes still works

Put the same irreversible action behind the gate and aim three approvals at it: a forged one with no nonce, a clickjacked guess at the nonce, and a genuine one from a person who was shown the evidence and the nonce and echoed it back.

```
== after: the evidence-first human gate ==
  forged approval (no nonce):      confirmed=false, calendar: []
  guessed nonce:                   confirmed=false, calendar: []
  genuine approval (real nonce):   confirmed=true, calendar: [Principal's Gala (Dec 25)]
```

The forged approval and the guess are both refused, and the calendar stays empty through both. Only the third, the real echo of the real nonce, is confirmed, and only then does the event land. That last line matters as much as the first two: the gate is deny-by-default and forgery-resistant, but it is still a gate and not a wall, so a genuine human yes goes through exactly as it should. A control that also blocked the real approval would not be a safer agent, just a broken one.

## What it can and cannot do

I want to be honest about the edge of this. The gate makes an approval unforgeable: it cannot be produced without a human being shown the evidence and echoing the nonce. It cannot make that human read the evidence carefully. What it buys is that the careful reader is given the chance, the suspicious event, the gala on a date that is not in the email, is put in front of a person with the reason it is suspect attached, and nothing happens until they actively say yes to that specific thing. Deny-by-default means the failure mode is inaction, not a silent irreversible act, and un-forgeable means a persuasive email cannot route around the human to manufacture the yes. The last line of defense is still a person, but now it is a person who was actually shown the decision, which is the most a gate can honestly promise.

## Conclusion

Irreversible actions now require a specific, evidenced, un-forgeable human approval. The agent cannot be talked into acting on your behalf by a confident model or a clickjacked dialog, and it cannot act on a yes that no person actually gave, while a real person's yes still works the moment they give it.

There is one more way to abuse an agent that does not need any approval at all, because it never tries to finish an action, it just refuses to stop. The next post is circuit breakers, the cap that keeps a runaway loop from spinning forever and draining the machine, and it closes this chapter and this part of the book.

## Take it to your own project

Put a human in front of anything irreversible:

- Deny irreversible actions by default; require an explicit approval to proceed.
- Make the approval specific to the action, not a blanket trust of the agent.
- Design so persuasive text in the input cannot drive the approval on its own.
- Log who approved what, under a trace id.
