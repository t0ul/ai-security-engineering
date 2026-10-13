#!/usr/bin/env bash
# The book's attack board. One row per lesson, in book order:
#   green    control lesson: attack lands undefended, blocked defended (tests pass)
#   RED      a contract test fails: a defense regressed or an attack no longer lands
#   not yet  no folder yet
# A non-test lesson (no go.mod) shows its editorial kind instead of a bare label:
#   intro    orientation / front matter / lab setup
#   feature  builds a capability that a later post then attacks
#   method   methodology or architecture (the eval harness, ADD, one trace id)
#   problem  the attack shown unpatched, to motivate a chapter's controls
#   live     a real-tool or VM receipt, not an importable unit test
# Run it after every lesson. A row that was green must never turn red again.
set -u

# kind_for maps a non-test lesson id to its editorial kind (default: notes).
kind_for() {
  case "$1" in
    00.0|00.1|00.2) echo intro;;
    01.1|10.1)      echo feature;;
    00.3|02.1|03.3|16.1) echo method;;
    02.2|07.1)      echo problem;;
    07.2|09.2|16.2) echo live;;
    *)              echo notes;;
  esac
}

red=0; green=0; todo=0
intro=0; feature=0; method=0; problem=0; live=0; notes=0
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
    result="$(kind_for "$id")"
    eval "$result=\$(( $result + 1 ))"
  else
    result="not yet"; todo=$((todo+1))
  fi
  printf "%-6s %-34s %-24s %s\n" "$id" "$slug" "$part" "$result"
done < /tmp/board-rows.$$
rm -f /tmp/board-rows.$$
echo
summary="green $green  red $red"
for k in intro feature method problem live notes; do
  n=$(eval "echo \$$k")
  [ "$n" -gt 0 ] && summary="$summary  $k $n"
done
summary="$summary  not yet $todo"
echo "$summary"
[ "$red" -eq 0 ] || { echo "board: a defense regressed"; exit 1; }
