# App features plan — beyond meetings (reminders, tasks, actions, digest, contacts)

The email→calendar app today extracts **meetings** and renders them as `.ics`
with reminders. The owner (t) also needs the non-meeting things an email asks
for — "buy popcorn to support the school", a clothes-donation drop-off, "guest
author visiting Tuesday", "accept the classroom photo-sharing invite" — plus the
read-only tools that already exist but aren't surfaced (digest, contacts,
action-items). This plan turns the app from a meeting extractor into a personal
"what does this email need from me" assistant, without loosening any control.

## Ground truth (what exists today)
- `agent/schema.Event`: `Title, Start, End, AllDay, Location, SourceEmail,
  Confidence, Warnings`. `NeedsReview()` flags low-confidence/warned events.
  **No item kind, notes, or URL field.**
- `agent/ics.Write`: emits `VEVENT` + an inert `VALARM` (−30m timed / 9am
  all-day). `SanitizeField` strips control/injection chars. **No `VTODO`, no
  `URL`.**
- `agent/tools`: `ActionItems`, `Digest`, `Contacts`, conflict detector — all
  `ReadOnly`, all return `.Text`. **Built, unit-tested, but the pipeline's
  default `ToolNames` is just `["event_extractor"]`, and read-only `.Text` is
  run-and-dropped (only audited, never persisted).**
- `pipeline.ProcessEmail`: runs registered tools under one trace; **only a
  `WriteICS` tool may write an artifact** (M6 capability enforcement). Read-only
  output is not written anywhere.
- `webapp`: `/api/events` reads `.ics` from `OutboxDir` → calendar list;
  `/api/scorecard`, `/api/incidents`, `/api/drop`. Tabs: Calendar / Security /
  Incidents. No digest/contacts/action surfaces.

## Target model — four item kinds per email
One email can yield several items of different kinds. Introduce a `Kind` on the
extracted item:

| Kind | Example | Time model | `.ics` shape | UI |
| --- | --- | --- | --- | --- |
| **event** (today) | PTA meeting Thu 6pm | timed, start/end | `VEVENT` + VALARM −30m | Calendar |
| **task** | buy popcorn; return permission slip; donate clothes by Fri | due date, no time block | `VTODO` + `DUE` + VALARM | Tasks/Reminders |
| **heads-up** | guest author visiting Tue; book-fair week | all-day, informational | all-day `VEVENT` + VALARM 9am day-before | Calendar (muted) + Tasks |
| **action** | accept classroom-app invite; RSVP; sign-up link | optional due + a URL to act on | `VTODO` with `URL:` | Actions (HITL + netpolicy) |

"Buy popcorn", "clothes donation", "bring snacks", "RSVP by the 10th" are
**tasks**; "guest author coming" is a **heads-up**; "accept the invite / open the
sign-up" is an **action** (it has a link you click).

## Schema changes (`agent/schema`)
Add to `Event` (keep JSON backward-compatible; all omitempty):
- `Kind string` — `"event"|"task"|"heads_up"|"action"` (empty = `"event"`).
- `Due string` — ISO date, for tasks/actions (distinct from `Start`).
- `Notes string` — short free text (the asking line), sanitized before use.
- `URL string` — for actions only; validated + netpolicy-screened before any
  fetch/click.
Constructor `New` keeps defaulting `Kind="event"`, `Confidence=1.0`.

## Extraction / classification
Add a classifier so one email produces a mix of kinds. Two layers, same as the
event extractor:
1. **Regex/deterministic first** (cheap, offline, testable): reuse
   `tools.reAction` patterns (bring/due/rsvp/return/donate/sign up) to mint
   **task**/**action** items; date-bearing informational lines without an ask →
   **heads-up**. A line with a URL + an accept/rsvp/join verb → **action**.
2. **LLM path** (when the gateway is up): a structured extraction prompt that
   returns typed items; spotlighting + `guard` still applied to the body first.
Low-confidence or warned items set `NeedsReview()` → they land in the review
queue, never auto-accepted.

New tool `agent/extractor` sibling (or extend it): `ItemExtractor` with
capability `WriteICS` so its artifacts persist; emits one `.ics` carrying all
kinds for that email, plus the read-only side outputs described next.

## ICS generation (`agent/ics`)
- **`VTODO`** for task/action: `UID`, `DTSTAMP`, `DUE;VALUE=DATE` (or datetime),
  `SUMMARY`, optional `DESCRIPTION` (Notes, sanitized), `URL:` (action only,
  sanitized + scheme-checked), `STATUS:NEEDS-ACTION`, and a `VALARM` (same inert
  DISPLAY alarm; trigger relative to `DUE`). Apple Calendar/Reminders and Google
  ingest `VTODO`.
- **Heads-up**: all-day `VEVENT`, `VALARM` the morning before.
- Keep `SanitizeField` on every text field; keep VALARM inert (no `ACTION:EMAIL`,
  no auto side effects).
- `webapp.parseICS` must learn `VTODO`/`DUE`/`URL`/`STATUS` so the UI can read
  back what was accepted (the `.ics` stays the source of truth).

## Pipeline — persist read-only outputs as sidecars
Read-only tools (digest/contacts/action-items) must not write artifacts (M6), so
the **pipeline** writes a per-email derived sidecar it owns — not the tool. After
running the roster, `ProcessEmail` writes `<stem>.summary.json` to the outbox:
```json
{ "source": "...", "trace_id": "...",
  "digest": "Subject: ...\n2 dated line(s): ...",
  "action_items": ["- bring popcorn Friday", ...],
  "contacts": ["teacher@school.org"],
  "items": [ {kind,title,due,...}, ... ],
  "needs_review": [ ...low-confidence items... ] }
```
This keeps capability enforcement intact (the tool declared read-only; the
trusted pipeline persists the digest for the owner UI), and every write is still
audited. Contacts remain PII-scoped and flagged in the audit, as today.

Default `ToolNames` for the app becomes
`["item_extractor","action_items","digest","contacts"]`.

## Web app
New endpoints (read the outbox sidecars + `.ics`):
- `GET /api/items` — all items across kinds, filterable by `kind` and `due`,
  sorted by date; replaces/extends `/api/events`.
- `GET /api/summary?file=` — the `<stem>.summary.json` for one email (digest +
  contacts + action-items).
- `GET /api/review` — the needs-review queue (low-confidence/warned items).
- `POST /api/accept` — accept a reviewed item → writes its `.ics`/`VTODO`
  (HITL-gated, clickjack-resistant confirm as in `hitl`).
- `POST /api/action` — perform an **action** item: the URL is first cleared by
  `netpolicy` (allowlist + no-IMDS + dial-pin), then opened/fetched via the
  **in-VM** path (`SandboxFetchTool`/broker) — never a blind host fetch.

UI (extend `webapp/page.go`):
- **Tasks/Reminders tab** — tasks + heads-ups as a due-sorted checklist with the
  🔔 and an Accept `.ics`/add-to-Reminders button; "buy popcorn" shows here.
- **Deadlines strip** — everything with a due date in one sorted "by when" list.
- **Per-card summary** — the digest TL;DR line on each email's card.
- **Contacts panel** — surfaced addresses + vCard export (PII-scoped, labeled).
- **Actions** — links with an explicit review + confirm; netpolicy verdict shown
  before you can click.
- **Needs-review inbox** — low-confidence items wait here instead of dropping.
- **Month view** — calendar grid (the earlier backlog item), events + due dots.

## Security (must hold)
- Body stays untrusted: `guard.Sanitize` on ingest, spotlighting, output
  sanitizer — unchanged; new kinds ride the same path.
- Action URLs: validated (http/https, parse) + `netpolicy` screened + fetched in
  the egress-denied VM via the broker. An "accept invite" link to
  `169.254.169.254` or an intranet host is refused. New ADD case
  `ActionLinkExfil` (undefended opens any URL; defended refused) → ASR 100%→0%.
- Any accept/action is HITL-gated (evidence-first, nonce-echo confirm) — no
  one-click auto-accept of an attacker-planted item.
- Contacts PII-scoped + audited; VTODO/URL fields sanitized (`SanitizeField`).
- Task-injection ADD: a body line "reminder to wire $5000 to acct 123" becomes a
  flagged **needs-review** task, never an auto-action.

## Phases (each a small, testable slice; t runs go test / VM)
- **A1 — schema + ics** ✅: added `Kind/Due/Notes/URL` + `ResolvedKind()`;
  `ics.Write` branches (VEVENT for event/heads-up, `VTODO`+`DUE`+`STATUS` for
  task/action, `X-KIND` tag, action `URL` kept only if valid http(s));
  `webapp.parseICS` reads VTODO/DUE/URL/X-KIND into `Event{Kind,Due,URL}`. Tests:
  task/action/heads-up round-trips + non-http URL dropped + webapp parse.
- **A2 — classifier** ✅: `agent/items` — `Classify` (regex, offline) mints
  task/heads-up/action items; `ItemExtractor` is a WriteICS tool emitting
  `items.ics`. Hardened after real emails exposed "too literal" parsing:
  **sentence splitting** (one paragraph → separate asks), **date propagation**
  (a date-less ask inherits the email's event date — "bring a dish" for the Oct
  23 potluck), **URL→action**, and a **safety-net event** for timed meetings the
  regex event-extractor rejects as prose, titled from the **subject line**
  (PTA clue: "PS 51 PTA Multicultural Potluck" → event "Multicultural Potluck"),
  deduped against the event extractor. `assumed_year` dropped from review noise.
  Tests incl. the potluck case.
- **A3 — pipeline sidecars** ✅: `pipeline.EmailSummary` written as
  `<stem>.summary.json` (read-only tool text + Kind-tagged items + needs-review);
  pipeline-owned, not a tool artifact (M6 intact, audited). New `agent/roster`
  (one roster: event_extractor, item_extractor, action_items, digest, contacts)
  wired into `cmd/emaildrop` + `cmd/webapp`. Tested end-to-end.
- **A4 — endpoints** ✅: `/api/items` (all items, `?kind=` filter, date-sorted),
  `/api/summary?file=` (one email's sidecar, traversal-guarded), `/api/review`
  (needs-review queue), `/api/accept` (POST file+index → writes an inert `.ics`).
  httptest coverage incl. a path-traversal rejection.
- **A5 — UI** ✅: new tabs — Calendar (+ Upcoming deadlines strip), Tasks
  (to-do / heads-up / actions with per-item Accept; action links shown but gated
  "opens only after a safety check"), Review (needs-review queue), Activity
  (plain-language agent-action feed via `/api/activity`), plus per-email digest +
  contacts under Tasks. `/api/items` enriched with file+index for Accept.
  Verified live (dropped a seed email, eyeballed all tabs). Known: "guest author"
  shows twice in Upcoming — event_extractor + item_extractor both emit it
  (cross-tool dedup = v2 #3, tracked).
- **A6 — actions** ✅: `/api/action` performs an action link — the URL is read
  from the stored item (never the request body: anti-SSRF), cleared by a
  deny-by-default `netpolicy` allowlist, then fetched via
  `controlplane.SandboxFetchTool` (in the egress-denied MicroVM). `cmd/webapp`
  gains `-allow`/`-microvm`; no allowlist ⇒ refused. UI "Check & fetch" button
  shows result or refusal. ADD `ActionLinkExfil` (planted IMDS link) 100%→0%.
  Tests: allow-listed fetch uses the item's URL; non-allow-listed refused before
  any fetch. (Reused by Track-2 R6 handbook-link enrichment.)
- **A7 — month view** ✅: a List/Month toggle in Calendar (no new tab); month
  grid over `/api/items`, items color-coded by kind (event/task/heads_up/action),
  today highlighted, prev/next nav, click a day for its items. Verified live.
- **A8 — docs**: syllabus tool-roster + BACKLOG + this plan truthed up.

Order: A1→A2→A3→A4→A5 delivers the owner-visible value (tasks, reminders,
digest, contacts); A6 adds the action/link capability; A7/A8 finish.

## Files to touch
`agent/schema/schema.go` · `agent/ics/ics.go` (+test) · `agent/extractor` or new
`agent/items` (+test) · `agent/pipeline/pipeline.go` (sidecar write) ·
`webapp/{webapp.go,events.go,page.go}` (+test) · `controlplane/sandboxtool.go`
(reuse `SandboxFetchTool` for actions) · `redteam/platform.go` (ActionLinkExfil)
· `scratch/syllabus.md`, `docs/BACKLOG.md`.

## v2 additions (found reviewing this plan — worth folding in)
The first draft creates items but doesn't manage them over time or dedup them.
Gaps:

1. **Item lifecycle state** — a task needs **done / snooze / dismiss**, not just
   "created". Model `Status` (`needs_action|done|dismissed`) + a snooze `Due`
   bump; map to `VTODO STATUS:COMPLETED`/`CANCELLED`. Without this, "buy popcorn"
   never goes away after you buy it.
2. **Recurring items** — the weekly newsletter / monthly PTA repeat. Add an
   optional `RRULE` (detected: "every Tuesday", "monthly") → `.ics RRULE`. Keep
   it conservative; a wrong recurrence is worse than none (→ NeedsReview).
3. **Dedup across emails** — the same event shows up in two newsletters. Wire the
   existing conflict/`normTitle` detector into the items view so duplicates
   merge (keep the highest-confidence, link the sources) instead of double-adding.
4. **In-app reminders** — the `.ics VALARM` only fires if you import it into
   Apple/Google Calendar. The local app should also surface a **due-soon list**
   on load (and optionally a browser/OS notification), so it's useful even before
   you accept anything into a real calendar.
5. **Edit before accept** — fix a wrong title/date/time in the UI before writing
   the `.ics` (re-runs `SanitizeField`; stays HITL). Today it's accept-as-is.
6. **Source & provenance view** — click an item → see the original email, the
   `trace_id`, the guard/sanitizer findings, and (if signed) the provenance mark.
   Makes "why is this on my calendar?" answerable. Reuses the audit + sidecars.
7. **Search over past items/emails** — a box that queries the existing RAG corpus
   ("when was the last book fair?", "what's due this week?"). Authorization-first
   retrieval already enforced; this just surfaces it.
8. **Combined subscribe feed (big UX win, security decision)** — publish one
   `webcal`/`.ics` feed the user subscribes to once, so accepted items appear in
   Apple Calendar automatically. Decision: only **accepted** items go in the feed
   (never raw extractions), feed served loopback-only, no auto-publish of
   unreviewed/needs-review items. Without the decision this becomes an exfil/auto
   -calendar surface.
9. **Categories / labels** — school vs work vs personal, and per-child tags, so
   the calendar/tasks filter. Small `Labels []string` on the item.
10. **Default time & timezone** — tasks with "by Friday" need a default reminder
    time; `.ics` is floating local time today — document it and let the owner set
    a default task-reminder hour.

Fold 1–4 into phases (state→A3/A4, recurring→A1/A2, dedup→A3, in-app reminders→
A5); 5–9 are A5/A6 UI; 10 is a config note. None change the security posture if
the subscribe-feed decision (8) holds.

## Track 2 — reference docs, ask-my-child's-day, safe link enrichment
Surfaced by `e-mails/family-handbook.txt`: a handbook is *standing reference you
query*, not an actionable bulletin. UI holds the parent to two intents —
**My Week** (act) and **Ask School** (know) — with security/profile in
Settings/admin. Reuses pieces already built.

Already built (reused here):
- RAG corpus + retrieval + tenant-ACL + Untrusted provenance (M8) ✅
- `controlplane.SandboxFetchTool` + egress broker + `netpolicy` (A6 / MicroVM) ✅
- contacts tool (`agent/tools.Contacts`) ✅

New phases:
- **R1 — child profile** ✅: `webapp.Profile`/`Child` (name/grade[free-text, e.g.
  "K"]/class/teacher/school/in/lunch/out/notes) persisted **in the cpstore DB**
  (config key "profile"), host-only, never from an email; `/api/profile` +
  `/api/profile/save` (csrf + authz write/profile). Edited in the My Week tab.
- **R2 — document-type routing** ✅: `items.DocumentType(email)` classifies
  bulletin vs reference at ingest; the pipeline records it (`EmailSummary.DocType`
  + gledger `classified` + a console badge) and passes `tool.Ctx.Reference` to
  every tool. A reference doc still feeds the Ask corpus but is extracted
  conservatively — the event extractor keeps only *announced* events
  (`announcedEvent`: a real 2+-word name or a specific time), so handbook prose
  dates ("The school year began…") are not calendared while real ones
  (Parent-Teacher Conferences) are; items already suppresses handbook tasks.
  Live-smoked: handbook → reference, only the conference reached the calendar.
- **R3 — daily timeline** ✅ (the hero of My Week): `/api/timeline?day=today|
  tomorrow|YYYY-MM-DD` aggregates the profile's daily anchors (`8:15 in · lunch
  10:55 · out 2:30`) + every extracted event/task/action landing on that day,
  across all processed emails (`allEvents()`); My Week tab with Today/Tomorrow.
  Live-smoked (profile + a dropped dated email → the day's agenda). **Half-day
  exceptions DONE** (profile `HalfDays`/`HalfDayOut`: on a listed day the timeline
  flags `half_day`, moves dismissal to the early time, drops lunch — live-smoked).
  **RRULE DONE** in the ics writer (`schema.Event.Recur` → sanitized `RRULE:` line;
  recurrence *detection* from prose is a follow-on).
- **R4 — Ask School** 🟡: `/api/ask` lexical search over the PII-scrubbed corpus
  (FTS5, injection-safe) + an Ask tab (box + snippets, readable source, untrusted
  badge). Verified live (nurse / phone policy → handbook). Remaining ⬜: an LLM
  *answer* (not just snippets, needs the model) + passage-level chunking (rag
  stores whole emails as one chunk, so the snippet is the doc start, not the
  matching passage).
- **R5 — structured directory** ✅ (MVP): `/api/directory` + Ask-tab Directory card
  consolidates the contacts the `contacts` tool surfaced across every email (deduped
  emails) + profile teachers (name→role). Host-only, PII-scoped. Full role→ext
  enrichment (NER) is a follow-on.
- **R6 — safe link enrichment** ✅: `/api/enrich` (Ask tab "Enrich from a link")
  — a handbook URL is cleared by the egress allowlist (netpolicy deny-by-default,
  registrable-domain, no loopback/private/IMDS), gated by an evidence-first
  single-use HITL confirm, fetched INSIDE the MicroVM (`SandboxFetchTool` via the
  broker), PII-scrubbed on ingest, and indexed as UNTRUSTED so Ask can use but not
  trust it. Never auto-fetched. Security path live-smoked (401 w/o token;
  non-allowlisted host refused; allow-listed passes). Real page content needs the
  VM up (`launchvm`). Powers "what's for lunch Thursday?" once the menu is fetched.
- **R7 — IA consolidation** 🟡: nav now visually splits the parent view
  (Calendar/My Week/Tasks/Review/Activity/Ask) from the `· operator ·` governance
  group (Security/Prompts/Sampling/Policies/Budgets/Eval/Incidents). Full
  consolidation (collapse to Week + Ask + Settings, one hero per tab) lands with
  the repo refactor's `ui/templates` work (`docs/REFACTOR-PLAN.md`), since that
  rewrites the view layer anyway.

Order: R1→R2→R3→R4 delivers ask-my-day; R6 is the link-enrichment showcase
(Post 16); R5/R7 polish.

Done already (toward R6): `netpolicy` now matches an allowlist entry as a
**parent domain** (one `schools.nyc.gov` covers `www.schools.nyc.gov`, while a
look-alike `evilschools.nyc.gov` is still refused; internal-IP checks unchanged).
`cmd/webapp -allow` defaults to the handbook domains
(`schools.nyc.gov,nyc.gov,ps51eliashowe.org,schoolsaccount.nyc`), overridable;
empty = deny all.

Noted (R2 clue): the **sender/subject** is a routing signal — a PTA email skews
to events/volunteer/fundraiser asks; a school-admin handbook to policy/reference.
The subject line is already used as an event-title hint; sender-based
categorization (Labels, v2 #9) can build on this.

## Agent activity transparency (cross-cutting) 🟡
"The agent is doing things — I should see them." Standard in agent UIs now, and
not yet surfaced as a user-facing feed.

Already built (reused): `gledger` audit already records **every** action under one
`trace_id` — tool start/end, inbox read, `index stored`, `summary_written`,
`artifact_written`/`signed`, policy `gate` allow/block, `vsock detonate`, broker
`egress ALLOW/DENY`. `ir.Load`/`ir.Timeline` + `/api/incidents` read it. So the
data exists; the gap is presentation.

Missing (⬜): a plain-language **Activity feed** distinct from the security
incidents view — render spans as sentences: "Read inbox `wk.txt`", "Extracted 3
items (1 needs review)", "Fetched allow-listed `https://…` in the sandbox",
"Blocked SSRF to 169.254.169.254". New `GET /api/activity` (over `ir` + the
broker/policy spans) + an Activity strip per email and globally. Course mapping:
M2 observability + ASI10 (agent-action monitoring / rogue-action visibility) —
it's the same audit the security console reads, shown in the owner's language.
1. **Tasks as `VTODO` vs all-day `VEVENT`+alarm** — VTODO is semantically right
   and supported by Apple Reminders/Calendar + Google, but some clients hide
   VTODO from the calendar grid. Recommend VTODO, with an option to also drop an
   all-day shadow event for visibility.
2. **Action links** — fetch-and-preview in the VM, or just validate + hand you a
   safe click-through? Recommend validate+preview (never auto-act).
3. **LLM classification now or regex-only first** — recommend regex path first
   (offline, testable), LLM path behind the gateway as A2b.
4. **Fold the ops console (`cmd/gridge`) into this app** or keep separate —
   tracked separately; not required for the above.

## Extraction ensemble (LLM + deterministic cross-check) — next, high-value
A full read of all 10 sample emails showed the deterministic path, even hardened,
stays brittle on free-form newsletters (signature lines like "Stephanie and Dana"
become events; verbose task titles; email 5 has no title to name its event). The
robustness answer is an **ensemble**, also a strong M9/CaMeL teaching piece:

- **LLM extracts, deterministic code verifies.** The LLM (event_extractor's
  llmPropose, extended to tasks/asks + kind) handles messy layout and returns
  {title, raw-date-phrase, kind, url?}. It NEVER writes a date: `dateparse`
  resolves + weekday-validates it (anti-hallucination, M9). Text ->
  `ics.SanitizeField`; URLs -> `netpolicy`.
- **Cross-check = confidence + hallucination catch.** Run both; reconcile by
  (date, fuzzy title): agreed -> high confidence (auto); solo -> needs-review; an
  LLM item whose date is absent from the raw text -> rejected. Agreement rate is
  scored against the eval ground-truth (prove ensemble > either alone).
- **Untrusted-output framing.** The LLM read an untrusted email, so its output is
  untrusted and the deterministic layer constrains it -- literally CaMeL.
- **Graceful degradation.** Model up -> ensemble; model down -> today's
  deterministic floor. Ties to Post 16 / M9 / M11 (eval).

Hardening already landed (deterministic floor + the regex half of the ensemble):
reference-doc suppression (handbook -> corpus, not tasks), multi-line event
assembly, section-header titles for dated lists, two-sided title extraction,
subject-line titling (PTA clue), single-event date propagation, same-datetime
dedup with title-quality, and bare-date / sentence-fragment event filtering.
