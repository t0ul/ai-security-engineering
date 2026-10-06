# Product & UX design plan — Secure Email→Calendar

A local web app (one Go binary serving a single-page UI; no cloud). Two users in
one person: the **owner** who turns messy emails into trustworthy calendar
events, and the **operator** who governs the agent. The product's edge is *trust
you can see* — this is a security tool, so the UX job is to make safety legible,
never to hide it.

## North star
Drop an email → see the events the agent found → glance at what's flagged →
accept. Nothing lands on your calendar without your click; everything is
explainable and replayable.

## Core loops
1. **Owner:** drop/paste email → event cards appear (date, time, place,
   confidence) → low-confidence/conflicting ones are chipped "review" → Accept
   downloads a signed `.ics` (with a reminder) → open in Calendar.
2. **Operator:** watch the live trace, approve config changes (evidence-first),
   flip the kill switch, review incidents, run the security scorecard.

## Screens
- **Drop.** Big drop zone + paste box. On ingest: a progress line of the real
  pipeline steps (scrub → guard → extract → sanitize) so the security work is
  visible, not hidden.
- **Events / Calendar.** Event cards + a month view. Each card: title, when,
  where, a **confidence badge**, a **"needs review"** chip when `NeedsReview()`
  fires (low confidence or a weekday/date warning), and a **🔔 reminder** toggle.
  Accept → `.ics`. Multi-select → one `.ics`. Conflicts (same event, two dates)
  surface as a merge prompt.
- **Security.** A **Run scorecard** button runs the red-team suite in-process and
  renders the ASR matrix (every technique 100%→0%, green). This is "run tests"
  as a first-class product feature — the owner can prove the agent is safe.
- **Incidents.** List of `trace_id`s (newest first); click one → the **replay
  timeline** (scrub → plan → policy gate → … ) on chain-verified evidence.
- **Governance.** Approvals (four-eyes, evidence + nonce confirm — *not* one
  click), kill-switch levels, versioned config/prompt/pin/eval inventory.
- **Models.** Registry + promotion gates, gated on the latest persisted eval.

## Reminders — yes, and cheap
`.ics` has native `VALARM`. We emit one per event (30 min before a timed event,
9am for an all-day), rendered as a per-event 🔔 toggle. No background daemon, no
push infra — Apple/Google Calendar fire the alert. (If we ever want *our own*
reminders, that's a scheduler + notifications — much heavier; `.ics` VALARM is
the right call for now.)

## Design principles
- **Evidence-first, no dark patterns.** Approvals show what you're approving and
  require an explicit confirm (clickjack-resistant). Low-confidence is flagged,
  never silently trusted.
- **Legible safety.** Show the pipeline working; show what the sanitizer
  stripped; show the audit chain is intact.
- **Calm + fast.** Keyboard-driven, instant feedback, dark/light, 16px gutters,
  phone-width friendly. Subtle motion only.
- **Local & private.** One binary, one SQLite file, loopback only.

## Tech
Reuse the Go HTTP server (`controlplane.ConsoleServer`) + a served SPA. A `webapp`
leaf package mounts: governance console + `/api/scorecard` (run tests) +
`/api/incidents` (replay). This *is* also the test/ops console — one app, reused.

## AISBOM — needed?
Not as full M17 SLSA/attestation (overkill for a single-user laptop binary). But
a **lean build manifest** is cheap and fits the inventory story: which models
(registry), which Go deps (build info), which prompt versions+hashes (cpstore).
Optional `cmd/sbom` emitting that JSON — enough to answer "what's in this build?"
without the enterprise supply-chain machinery.

## Roadmap (UX)
v1 Drop + Events/Calendar + Accept(.ics+reminder). v2 Security + Incidents tabs.
v3 Governance + Models. v4 polish: month-view drag, conflict merge, search over
the corpus.
