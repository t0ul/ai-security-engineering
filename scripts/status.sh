#!/usr/bin/env bash
# Progress, computed from the filesystem and posts/MANIFEST.md. Nothing here is
# recorded by hand, so it cannot go stale.
set -u
printf "%-6s %-34s %-7s %-5s %-8s %s\n" id slug folder code receipt title
awk -F'|' 'NF>=6 {
  id=$2; slug=$3; title=$5
  gsub(/^ +| +$/,"",id); gsub(/^ +| +$/,"",slug); gsub(/^ +| +$/,"",title)
  if (id ~ /^[0-9]+\.[0-9]+$/) print id "\t" slug "\t" title
}' posts/MANIFEST.md |
while IFS=$'\t' read -r id slug title; do
  d="posts/$id-$slug"
  folder=$([ -d "$d" ] && echo yes || echo -)
  code=$([ -f "$d/go.mod" ] && echo yes || echo -)
  rec=$([ -f "$d/receipt.txt" ] && echo yes || echo -)
  printf "%-6s %-34s %-7s %-5s %-8s %s\n" "$id" "$slug" "$folder" "$code" "$rec" "$title"
done
