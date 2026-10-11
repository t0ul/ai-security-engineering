#!/usr/bin/env bash
# Open a PR for exactly one lesson folder. Stages only that folder and the
# manifest, never the whole tree, so unrelated changes cannot ride along.
set -eu
post="$1"
br="post/$post"

if ! git diff --quiet HEAD -- . ":!posts/$post" ":!posts/MANIFEST.md"; then
  echo "unrelated tracked changes present; commit or stash them first:"
  git status -s
  exit 1
fi

make check POST="$post"
git checkout -B "$br"
git add "posts/$post" posts/MANIFEST.md
git commit -m "post($post): add lesson folder"
git push -u origin "$br"
gh pr create --title "Post $post" \
  --body "Lesson folder \`posts/$post\`. Run: \`cd posts/$post && go run .\` and \`go test ./...\`."
