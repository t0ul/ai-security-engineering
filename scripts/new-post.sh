#!/usr/bin/env bash
# Scaffold posts/<id>-<slug>/ from the manifest row, so a lesson always starts
# from the same shape and the same brief.
#
# Two kinds, decided by the manifest spec:
#   - prose-only (spec says "prose only" or "no attack"): just index.md + SPEC.md.
#     Front-matter posts and foundation posts have no attack, so no code module.
#   - control (everything else): the full _TEMPLATE (go.mod + main.go +
#     contract_test.go + attack/) for a runnable before/after.
set -eu
post="$1"
id="${post%%-*}"
dst="posts/$post"
[ -e "$dst" ] && { echo "exists: $dst"; exit 1; }

row=$(awk -F'|' -v id="$id" '{ k=$2; gsub(/^ +| +$/,"",k); if (k==id) print }' posts/MANIFEST.md)
[ -z "$row" ] && { echo "no manifest row for id $id (add it to posts/MANIFEST.md first)"; exit 1; }
spec=$(echo "$row" | awk -F'|' '{print $6}')

prose=0
case "$(echo "$spec" | tr 'A-Z' 'a-z')" in
  *"prose only"*|*"no attack"*) prose=1 ;;
esac

mkdir -p "$dst"
cp posts/_TEMPLATE/index.md "$dst/index.md"
if [ "$prose" -eq 0 ]; then
  cp posts/_TEMPLATE/main.go posts/_TEMPLATE/contract_test.go "$dst/"
  mkdir -p "$dst/attack"; cp posts/_TEMPLATE/attack/README.md "$dst/attack/"
  sed 's#posts/_TEMPLATE#posts/'"$post"'#' posts/_TEMPLATE/go.mod > "$dst/go.mod"
fi

{
  echo "# SPEC $post"
  echo
  echo "$row" | awk -F'|' '{printf "- part: %s\n- title: %s\n- spec: %s\n", $4, $5, $6}' | sed -e 's/  */ /g' -e 's/ *$//'
  echo
  if [ "$prose" -eq 1 ]; then
    echo "Kind: PROSE-ONLY (no attack). Deliver index.md only; no code module."
    echo "Done when: index.md passes make lint and renders cleanly for QA."
  else
    echo "Kind: CONTROL. Deliver a runnable before/after."
    echo "Done when: go test passes (attack lands undefended, blocked defended),"
    echo "receipt.txt is captured from a real run, and index.md passes make lint."
  fi
} > "$dst/SPEC.md"

if [ "$prose" -eq 1 ]; then
  echo "scaffolded $dst (PROSE-ONLY; read SPEC.md)"
else
  echo "scaffolded $dst (CONTROL; read SPEC.md)"
fi
