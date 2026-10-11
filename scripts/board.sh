#!/usr/bin/env bash
# The book's attack board. One row per lesson, in book order:
#   green    control lesson: attack lands undefended, blocked defended (tests pass)
#   RED      a contract test fails: a defense regressed or an attack no longer lands
#   prose    prose-only lesson (no attack): index.md present, nothing to attack
#   not yet  no folder yet
# Run it after every lesson. A row that was green must never turn red again.
set -u
red=0; green=0; prose=0; todo=0
printf "%-6s %-34s %-24s %s\n" id slug part result
awk -F'|' 'NF>=6 {
  id=$2; slug=$3; part=$4
  gsub(/^ +| +$/,"",id); gsub(/^ +| +$/,"",slug); gsub(/^ +| +$/,"",part)
  if (id ~ /^[0-9]+\.[0-9]+$/) print id "\t" slug "\t" part
}' posts/MANIFEST.md > /tmp/board-rows.$$
while IFS=$'\t' read -r id slug part; do
  d="posts/$id-$slug"
  if [ -f "$d/go.mod" ]; then
    if (cd "$d" && go test -count=1 ./... >/dev/null 2>&1); then
      result="green"; green=$((green+1))
    else
      result="RED"; red=$((red+1))
    fi
  elif [ -f "$d/index.md" ]; then
    result="prose"; prose=$((prose+1))
  else
    result="not yet"; todo=$((todo+1))
  fi
  printf "%-6s %-34s %-24s %s\n" "$id" "$slug" "$part" "$result"
done < /tmp/board-rows.$$
rm -f /tmp/board-rows.$$
echo
echo "green $green  red $red  prose $prose  not yet $todo"
[ "$red" -eq 0 ] || { echo "board: a defense regressed"; exit 1; }
