-- SQLite schema for the agent's durable inventory (pkg/datastore).
--
-- This is the SINGLE SOURCE OF TRUTH for every table. It is embedded into the
-- binary via //go:embed in cpstore.go and applied verbatim on Open(). Nothing
-- creates a table anywhere else — audit this one file to see the whole store.
--
-- Convention: append-only history tables carry an `at` timestamp (an inventory of
-- versioned artifacts and the decisions about them); current-state tables are keyed
-- and replaced in place.

-- === Governed control-plane artifacts (versioned, hashed, rollback-able) ===
-- Each row is one activation: name + version + sha256 hash of the value + the value.
CREATE TABLE IF NOT EXISTS prompts(name TEXT, version TEXT, hash TEXT, text TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS grammars(name TEXT, version TEXT, hash TEXT, text TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS models(name TEXT, version TEXT, hash TEXT, model TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS policies(name TEXT, version TEXT, hash TEXT, items TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS sampling(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS retrieval(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS budgets(name TEXT, version TEXT, hash TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- === Operator decisions & audit (HITL approvals, supply-chain pins, admin log) ===
CREATE TABLE IF NOT EXISTS approvals(id TEXT, proposer TEXT, approver TEXT, perm TEXT, note TEXT, version INTEGER, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS mcp_pins(server TEXT, hash TEXT, approved_by TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS skill_pins(name TEXT, version TEXT, hash TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS admin_audit(actor TEXT, action TEXT, detail TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- === Eval & data flywheel ===
CREATE TABLE IF NOT EXISTS eval_scores(label TEXT, f1 REAL, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS feedback(source TEXT, title TEXT, decision TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- === Config & known-good bundles (current-state) ===
-- config is the generic key/value store (profile, extractor flags, projection
-- fingerprints, the emails-seeded marker, …).
CREATE TABLE IF NOT EXISTS config(key TEXT PRIMARY KEY, value TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS bundles(label TEXT, config TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- === Runtime registry (current-state) ===
-- The model catalog: single source of truth for which models exist, their verified
-- download URL + SHA pin, local file, and serve port/ctx. prepareassets/modeld/webapp
-- all read this.
CREATE TABLE IF NOT EXISTS models_catalog(name TEXT PRIMARY KEY, url TEXT, sha256 TEXT, file TEXT, port INTEGER, ctx INTEGER, host TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- === Domain entities (derived projection) ===
-- events is the calendar/events projection the view serves from. The signed .ics
-- outbox stays the source of truth (it carries the M20 content credential); this
-- table is rebuilt (deduped) from it whenever the outbox changes, keyed by the outbox
-- fingerprint stored in config('events_fingerprint').
CREATE TABLE IF NOT EXISTS events(title TEXT, start_at TEXT, end_at TEXT, location TEXT, all_day INTEGER, has_reminder INTEGER, signed INTEGER, file TEXT, kind TEXT, due_at TEXT, url TEXT);

-- summaries is the projection of the per-email <stem>.summary.json sidecars the
-- pipeline wrote (items/tasks, contacts, digest, needs-review). The Tasks, By-email,
-- Directory, and Review views serve from here instead of re-scanning + re-parsing the
-- JSON files on every request. Rebuilt whenever the outbox summaries change, keyed by
-- the fingerprint stored in config('summaries_fingerprint'). The .summary.json stays
-- the source of truth; this is the queryable projection.
CREATE TABLE IF NOT EXISTS summaries(file TEXT PRIMARY KEY, json TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- items is the DEDUPED item projection (tasks/heads-up/actions/events) the Tasks tab
-- and mini-calendar serve from: duplicates the extractor emitted for the same real
-- item are removed ONCE at build time and the result is stored here, rather than
-- re-deduping on every request. Rebuilt from the summary sidecars when they change,
-- keyed by config('items_fingerprint'). Each row keeps its source file + index so an
-- accept can still resolve the original item.
CREATE TABLE IF NOT EXISTS items(file TEXT, idx INTEGER, json TEXT);

-- item_status is the MUTABLE status overlay for projected items (done/dismissed/
-- snoozed). The items projection itself is derived and rebuilt from the sidecars, so
-- status cannot live there; it is keyed by a CONTENT fingerprint of the item
-- (normalized title | day | kind) that is stable across rebuilds. Current-state,
-- replaced in place. snooze_until is an ISO date the item stays hidden until.
CREATE TABLE IF NOT EXISTS item_status(key TEXT PRIMARY KEY, status TEXT, snooze_until TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);

-- chat_turns is the persistent conversation: one row per user/assistant turn, so the
-- chat survives a reload, is multi-turn (prior turns are replayed to the model), and
-- carries observability (which model answered, token usage) + a per-answer rating
-- (up/down/neutral) that feeds the feedback flywheel.
CREATE TABLE IF NOT EXISTS chat_turns(id INTEGER PRIMARY KEY AUTOINCREMENT, role TEXT, content TEXT, sources TEXT, model TEXT, prompt_tokens INTEGER, completion_tokens INTEGER, rating TEXT, at DATETIME DEFAULT CURRENT_TIMESTAMP);
