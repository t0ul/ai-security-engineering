#!/usr/bin/env bash
# Prose lint for posts/*/index.md: the mechanical half of the style rules.
# Fails on em dashes, scaffolding labels surfaced in the prose, a few banned
# phrases, and missing front matter. Tone is still a human's call.
set -u
fail=0
files=$(ls posts/[0-9]*/index.md 2>/dev/null)
[ -z "$files" ] && { echo "lint: no posts yet"; exit 0; }

if grep -n $'\xe2\x80\x94' $files; then
  echo "lint: em dash found (use a comma, colon, period, or parentheses)"; fail=1
fi
if grep -nE '^(#+ |\*\*)(Get it working|Show the problem|Make it secure|Prove it|What it is|What it defends)' $files; then
  echo "lint: scaffolding label surfaced in prose"; fail=1
fi
if grep -niE 'first blood|weaponiz|owns the box|walks right in|the fleet' $files; then
  echo "lint: banned phrase"; fail=1
fi
for f in $files; do
  head -1 "$f" | grep -q '^---$' || { echo "lint: $f missing front matter"; fail=1; }
done

[ $fail -eq 0 ] && echo "lint: clean"
exit $fail
