# Book tooling. Every lesson lives in posts/<id>-<slug>/ as its own Go module.
#
#   make status                  what exists, what has code, what has a receipt
#   make new POST=03.1-gledger   scaffold a lesson (prose-only or control, per its manifest spec)
#   make check [POST=...]        the gate: prose lint + vet + every contract test (all, or one)
#   make board                   the whole book's attack board: every lesson re-attacked, red/green
#   make capture POST=...        run the lesson and save its real output to receipt.txt
#   make listings                print each lesson's code into its chapter (listing markers in index.md)
#   make site-sync | site-check  copy every post body over its site copy (t0ul.com), or report drift
#   make lint                    prose lint only
#   make hooks                   install the pre-push hook so `make check` runs before every push
#   make pr POST=...             branch, check, commit only that folder, push, open PR
#
# The gate tests reality. Lessons that involve the model talk to the local model
# server (planner on 127.0.0.1:11435, see post 00.2); a contract test must fail
# loudly if the model is down, never skip. Nothing in the gate is stubbed.

SHELL := /bin/bash
MODS  := $(shell ls posts/[0-9]*/go.mod 2>/dev/null | xargs -n1 dirname)

.PHONY: status new check board capture lint listings site-sync site-check hooks pr

status:
	@scripts/status.sh

new:
	@test -n "$(POST)" || { echo "usage: make new POST=<id>-<slug>"; exit 2; }
	@scripts/new-post.sh "$(POST)"

check: lint
	@scripts/listings.py check --all
	@set -e; for d in $(if $(POST),posts/$(POST),$(MODS)); do \
	  test -f "$$d/go.mod" || continue; \
	  echo "== $$d"; (cd "$$d" && go vet ./... && go test -count=1 ./...); \
	done

board:
	@scripts/board.sh

capture:
	@test -n "$(POST)" || { echo "usage: make capture POST=<id>-<slug>"; exit 2; }
	@cd "posts/$(POST)" && go run . 2>&1 | tee receipt.txt

lint:
	@scripts/lint-prose.sh

listings:
	@scripts/listings.py write --all

site-sync:
	@scripts/site-sync.py write

site-check:
	@scripts/site-sync.py check

hooks:
	@git config core.hooksPath .githooks && echo "pre-push hook installed: make check runs before every push"

pr:
	@test -n "$(POST)" || { echo "usage: make pr POST=<id>-<slug>"; exit 2; }
	@scripts/pr.sh "$(POST)"
