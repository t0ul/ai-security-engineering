---
title: "Tasks and reminders"
description: "The agent now pulls to-dos, heads-ups, and actions out of an email, not just meetings. An action is the dangerous kind, because it carries a URL to act on, so it has to be surfaced for review rather than dropped into a silent to-do list. Classify by what a sentence sounds like and an attacker hides a phishing link behind to-do language. The real classifier checks for a URL first, so a link is always an action."
date: 2026-06-09
draft: false
tags: ["ai-security", "golang", "agents", "classification"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## More than meetings

The capstone chapter builds the agent out to the thing you would actually use, one feature at a time, each arriving with its own attack and its own fix. First feature: an email does not only announce events. It also carries to-dos ("return the signed form"), heads-ups ("guest author visiting"), and actions ("RSVP at this link"). So the agent learns to pull all of them out, tagging each item with a kind: event, task, heads-up, or action.

That last kind is the one with teeth. An action carries a URL to act on, and a URL in an email you did not vet is exactly the thing the whole book has been careful about. A task is a note to yourself; an action is a door. The agent treats them differently: an action is surfaced for you to look at before you touch the link, while a task can sit quietly on a list. That difference is the feature, and it is also the vulnerability, because it means an attacker wins by getting a link filed as a task. A control lesson, deterministic, no model:

```bash
cd posts/13.2-tasks-and-reminders
go run .
```

## Classify by what it sounds like, and lose

Here is the email. One genuine reminder with no link, and one phishing link dressed up as a chore.

```
Reminder: please return the signed field-trip form by Friday.

Also, don't forget to submit the permission form at http://attacker.example/phish before Monday.
```

The obvious way to classify is by what each sentence sounds like: if it reads like a to-do, "reminder", "don't forget", "submit", it is a task. That rule is reasonable, common, and wrong, because the second sentence reads exactly like a to-do, and it has a link in it.

```
== before: a classifier that files to-dos first ==
  kind=task    url=""   Reminder: please return the signed field-trip form b...
  kind=task    url=""   Also, don't forget to submit the permission form at ...
  the phishing link is filed as a chore, no action flag, URL not surfaced: true
```

Both land as tasks. The attacker's link is now sitting in your to-do list, indistinguishable from returning a form, its URL not surfaced for review because the agent decided this was a chore and chores do not have links worth showing. The owner clears their list, clicks to "submit the form", and lands on the attacker.

## Check for the link first

The fix is an ordering, and it is one line of judgment: a URL decides the kind before any to-do vocabulary gets a vote. The agent's real classifier in `pkg/agent/items` checks for a link first, and only if there is none does it ask whether the sentence sounds like a task or a heads-up.

```go
switch {
case hasURL:  ev.Kind = schema.KindAction; ev.URL = reURL.FindString(s)
case action:  ev.Kind = schema.KindTask
default:      ev.Kind = schema.KindHeadsUp
}
```

Run the same email through it.

```
== after: the agent's real classifier (URL-first) ==
  kind=task    url=""                              Return the signed field-trip form by
  kind=action  url="http://attacker.example/phish" Don't forget to submit the permission form at http:/...
  the link is an action with its URL surfaced for review: true (genuine reminder still a task: true)
```

The genuine reminder is still a task, because it has no link. The smuggled one is now an action, with its URL pulled out into its own field where the owner, and the human gate from the agency chapter, will see it before anything acts on it. The to-do language did not help the attacker at all, because the classifier never asked what the sentence sounded like until after it had already found the link.

## The cheap rule beats the clever one

It would be tempting to make this smarter, to score how suspicious a link looks or train something to spot phishing. The agent does the boring thing instead: a link is an action, full stop, no matter how friendly the sentence around it. That is the same move as never letting the model decide a date, or never running a tool beyond its capability. The security-relevant decision, is this a thing that acts, is made by a rule that cannot be social-engineered, and the model-shaped judgment is only allowed to run on what is left. A phishing link cannot talk its way into the task list, because the thing that sorts it never listens to it.

The next feature is the weekly view, pulling all of this together into "what is my week", and its attack is quieter: not a link, but a crafted item that tries to make one real day look like another.

## Take it to your own project

When you add a feature, re-apply the controls:

- Give a new item type the same inert output, human accept, and validation as the first.
- Do not let a new surface skip the controls the old ones have.
- Attack the new feature before you trust it.
