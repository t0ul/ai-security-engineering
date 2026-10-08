# MVC Go review — anti-patterns & fixes (for a caveman to work)

Consultant pass over the web app after the `refactor/layout` move. Layout is good
now (thin root, `pkg/` libs, `pkg/domain`/`pkg/server`/`ui`). These are the things
left that a Go reviewer flags. Ordered by payoff. Each: **what · where · why ·
fix**. Nothing here is on fire; the top three are the real architecture smells.

## P1 — architecture (worth doing first)

### 1. The composition root does the work. Controller logic lives in `main`.
- **Where:** [cmd/webapp/main.go](cmd/webapp/main.go) — 710 lines, 36 func literals, **14 closures** (`chat`, `evalRun`, `promptTest`, `search`, `profileLoad`, `bundleSave`, …) built in `main` and injected as struct fields.
- **Why:** `main` is the wiring, not the brain. Business logic buried in closures in the composition root can't be unit-tested, can't be read in one place, and leaks the Controller into `cmd/`. `gatewayChatAnswer`, `chatInjectionASR`, `trustedDateBlock` are domain logic sitting in `package main`.
- **Fix:** move each closure's body onto a type in `pkg/server` (or a new `pkg/chat`, `pkg/eval`). `main` should read config, construct dependencies, and call `server.New(...)`. Target: `main` under ~150 lines, zero business logic.

### 2. `Server` is a god-struct of injected closures, not dependencies.
- **Where:** [pkg/server/webapp.go:31](pkg/server/webapp.go) — ~29 fields, ~14 of them `func(...)` function-fields, each guarded by `if s.X != nil` at the route and in the handler.
- **Why:** function-fields-as-DI hides what the server actually needs, makes every capability optional-by-nil (easy to forget a wiring and silently drop a route), and blocks mocking. "Is the Chat tab on?" is answered by a nil check in three places.
- **Fix:** define small interfaces for the real collaborators — `ChatService`, `EvalService`, `PromptStore`, `Clock` — and hold those. Group the governed planes behind one `ControlPlane` dependency. Presence becomes a typed capability, not a nil func. This also kills the nil-guard noise.

### 3. No constructor; raw struct literal + lazy map init.
- **Where:** construction is a bare `&server.Server{...}` literal in [cmd/webapp/main.go](cmd/webapp/main.go); `pend` is lazily `make`-d under a mutex in [pkg/server/confirm.go:25](pkg/server/confirm.go).
- **Why:** nothing validates required fields; invariants (maps initialized, timeouts set) are discovered at runtime. Lazy init is a code smell that exists only because there's no constructor.
- **Fix:** `func New(cfg Config, deps Deps) (*Server, error)` — init `pend`, validate required deps, return an error for a misconfig. One place that knows how to build a valid server.

## P2 — correctness / robustness

### 4. `os.Setenv("EXTRACT_MODE", …)` mutates process-global state inside handlers.
- **Where:** [cmd/webapp/main.go:471](cmd/webapp/main.go) and [:502](cmd/webapp/main.go) (evalRun, promptTest).
- **Why:** process-global, not request-scoped. `evalMu` serializes the two eval paths against each other, but **not** against the live extractor the watcher runs concurrently — a data race on the extractor's mode during an eval. Classic "config via env at runtime" trap.
- **Fix:** pass the mode explicitly (`extractor.ScoreWith(mode, …)` or a field on the extractor). Never `Setenv` to pass an argument.

### 5. Gateway URL hardcoded — violates this project's own no-hardcoding rule.
- **Where:** `"http://localhost:4000/v1/chat/completions"` in [cmd/webapp/main.go:604](cmd/webapp/main.go); `"127.0.0.1:4000"` in `gatewayUp` [:673](cmd/webapp/main.go).
- **Why:** the whole thesis is "config in the DB, consts are only fail-closed defaults" ([config-in-db]). The gateway endpoint is config, pinned in two string literals.
- **Fix:** one `GatewayURL` from a flag/env/cpstore, threaded through; derive the health-check host from it. Single source.

### 6. Unbounded request bodies.
- **Where:** `json.NewDecoder(r.Body).Decode(...)` with no cap in [pkg/server/chat.go:22](pkg/server/chat.go), `profile.go`, `prompts.go`, and the other POST handlers. Only [events.go:35](pkg/server/events.go) limits (`io.LimitReader` 256KB).
- **Why:** a loopback app, but still — a single large POST forces unbounded allocation. Cheap DoS, trivial fix.
- **Fix:** wrap in `http.MaxBytesReader(w, r.Body, 1<<20)` once, in the auth/CSRF middleware or a small `decodeJSON` helper, so every handler inherits it.

### 7. `http.Server` missing read/write/idle timeouts.
- **Where:** [cmd/webapp/main.go](cmd/webapp/main.go) sets only `ReadHeaderTimeout`.
- **Why:** no `ReadTimeout`/`WriteTimeout`/`IdleTimeout` → a slow or stuck client holds a goroutine/conn open. Standard hardening.
- **Fix:** set all four on the `&http.Server{}`.

## P3 — MVC hygiene / view / cleanup

### 8. The View is one 650-line string; no `html/template`, no static split.
- **Where:** [ui/templates/dashboard.html](ui/templates/dashboard.html) is HTML+CSS+JS inline; served by `strings.Replace(ui.Dashboard(), "__CAP_TOKEN__", token, 1)` in [pkg/server/webapp.go:394](pkg/server/webapp.go). No `ui/static/`.
- **Why:** token injection by string replace bypasses `html/template`'s contextual escaping — safe **only** because the token is base64url (no `<`/`"`/`\`); one future non-base64 value and it's XSS. The CSS/JS can't be cached (inlined), and a 650-line blob is hard to edit.
- **Fix:** render via `html/template` (the token becomes a typed field, auto-escaped in the right context). Split CSS/JS into `ui/static/app.css`/`app.js`, embed the dir, serve with `http.FileServer` + cache headers. The `page.go`-extraction plan already called for `ui/static`.

### 9. `allEvents()` re-reads and re-parses the whole outbox on every request.
- **Where:** [pkg/server/events.go:82](pkg/server/events.go), called 4× (events, items, summary path, chat's `weekScheduleBlock`).
- **Why:** O(files) `os.ReadDir` + read + `parseICS` + signature verify per request. Fine at 10 files, a cliff at hundreds, and the chat path now triggers it too.
- **Fix:** cache the parsed list, invalidate on outbox write (the watcher already knows when it writes), or memoize with a short TTL / mtime check.

### 10. Method routing by hand in every handler.
- **Where:** `if r.Method != http.MethodPost { … }` repeated across ~10 handlers.
- **Why:** Go 1.22's `ServeMux` does method+pattern routing (`mux.HandleFunc("POST /api/chat", …)`). The manual check is boilerplate that drifts.
- **Fix:** put the method in the pattern; delete the per-handler guards. Also collapses the "405" handling into the mux.

### 11. Model layer is half-extracted.
- **Where:** 12 DTO structs still defined in `pkg/server` (`PromptRow`, `PolicyRow`, `SamplingRow`, `BudgetRow`, `BundleInfo`, `EvalResult`, `PromptTestResult`, `Anchor`, …).
- **Why:** `pkg/domain` holds the nouns (Profile/Child/SearchHit/MCPServer) but the governed-plane DTOs stayed in the Controller. Defensible (they're presentation shapes) — but decide and document, don't leave it ambiguous.
- **Fix:** either move the response DTOs to `pkg/domain` (full Model) or add a one-line comment in `webapp.go` stating DTOs are Controller-owned presentation types by choice. Pick one.

### 12. Small stuff.
- **`writeJSON` swallows the encode error** silently ([webapp.go:437](pkg/server/webapp.go)) — at least log it; a half-written body with a 200 is a debugging trap.
- **`go vet ./pkg/server/` is not clean** — "using resp before checking for errors" ×4 in [webapp_test.go:34+](pkg/server/webapp_test.go). Fix the tests; keep vet green in CI.
- **`context.Background()` in [webapp.go:351](pkg/server/webapp.go)** (`defendedASR`) — if that ever runs on a request path, thread `r.Context()` so a client disconnect cancels the work.

## Suggested order
1, 2, 3 together (they're one refactor: constructor + interfaces + move logic out of `main`). Then 4, 5, 6, 7 (quick, high-value). Then the P3 cleanups as you touch each file. Keep `go build`/`go test ./...`/`go vet`/`cmd/scorecard` green after each — same discipline as the layout refactor.
