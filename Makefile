# Book tooling. Every lesson lives in posts/<id>-<slug>/ as its own Go module.
#
#   make status                  what exists, what has code, what has a receipt
#   make new POST=03.1-gledger   scaffold a lesson from _TEMPLATE + its manifest row
#   make check [POST=...]        prose lint + vet + contract tests (all, or one)
#   make capture POST=...        run the lesson and save its real output to receipt.txt
#   make lint                    prose lint only
#   make pr POST=...             branch, check, commit only that folder, push, open PR

SHELL := /bin/bash
MODS  := $(shell ls posts/[0-9]*/go.mod 2>/dev/null | xargs -n1 dirname)

.PHONY: status new check capture lint pr

status:
	@scripts/status.sh

new:
	@test -n "$(POST)" || { echo "usage: make new POST=<id>-<slug>"; exit 2; }
	@scripts/new-post.sh "$(POST)"

check: lint
	@set -e; for d in $(if $(POST),posts/$(POST),$(MODS)); do \
	  test -f "$$d/go.mod" || continue; \
	  echo "== $$d"; (cd "$$d" && go vet ./... && go test ./...); \
	done

capture:
	@test -n "$(POST)" || { echo "usage: make capture POST=<id>-<slug>"; exit 2; }
	@cd "posts/$(POST)" && go run . 2>&1 | tee receipt.txt

lint:
	@scripts/lint-prose.sh

pr:
	@test -n "$(POST)" || { echo "usage: make pr POST=<id>-<slug>"; exit 2; }
	@scripts/pr.sh "$(POST)"
