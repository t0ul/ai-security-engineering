#!/usr/bin/env bash
# Scaffold posts/<id>-<slug>/ from _TEMPLATE and write its manifest row to SPEC.md,
# so a lesson always starts from the same shape and the same brief.
set -eu
post="$1"
id="${post%%-*}"
dst="posts/$post"
[ -e "$dst" ] && { echo "exists: $dst"; exit 1; }

row=$(awk -F'|' -v id="$id" 'NR>2 { k=$2; gsub(/^ +| +$/,"",k); if (k==id) print }' posts/MANIFEST.md)
[ -z "$row" ] && { echo "no manifest row for id $id (add it to posts/MANIFEST.md first)"; exit 1; }

cp -R posts/_TEMPLATE "$dst"
perl -pi -e "s#posts/_TEMPLATE#posts/$post#" "$dst/go.mod"
{
  echo "# SPEC $post"
  echo
  echo "$row" | awk -F'|' '{printf "- part: %s\n- title: %s\n- spec: %s\n", $4, $5, $6}' | sed -e 's/  */ /g' -e 's/ *$//'
  echo
  echo "Done when: go test passes (attack lands undefended, blocked defended),"
  echo "receipt.txt is captured from a real run, and index.md passes make lint."
} > "$dst/SPEC.md"
echo "scaffolded $dst (read SPEC.md first)"
