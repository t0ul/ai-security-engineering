#!/usr/bin/env bash
# Assemble the handbook from posts/MANIFEST.md and build it with honkit.
#
#   scripts/book.sh              assemble book/src, build book/web and book/ai-security-engineering.epub
#   BOOK=/some/dir scripts/book.sh   build somewhere else
#
# Each chapter is the post's index.md with its front matter turned into a title,
# the series note dropped, and every svg figure rasterized to png for the EPUB.
# Listings are already printed into index.md by `make listings`, so the code
# carries through as fenced blocks. Needs npx (honkit is fetched on first use),
# ebook-convert from calibre for the EPUB, and qlmanage (macOS) for the figures.
set -euo pipefail
shopt -s nullglob
cd "$(dirname "$0")/.."

BOOK=${BOOK:-book}
SRC="$BOOK/src"
rm -rf "$SRC"; mkdir -p "$SRC"

cat > "$SRC/book.json" <<'JSON'
{ "title": "AI Security Engineering", "author": "t", "language": "en" }
JSON
cat > "$SRC/README.md" <<'MD'
# AI Security Engineering

A handbook that builds a local email-to-calendar AI agent and secures it with
Attack-Driven Design: build a small thing, attack it with a real, measured attack,
fix what broke, then run the same attack again and show it is gone.

Every chapter is a runnable lesson, and every lesson's whole program is printed in
its chapter so you can type it in or clone it. Use the sidebar to move through the
book in order; each part assumes the one before it.
MD
{ echo "# Summary"; echo ""; echo "* [Introduction](README.md)"; echo ""; } > "$SRC/SUMMARY.md"

prevpart=""; missing=0; count=0
while IFS='|' read -r _ id slug part title _spec _rest; do
  id=$(echo "${id:-}" | xargs); slug=$(echo "${slug:-}" | xargs)
  part=$(echo "${part:-}" | xargs); title=$(echo "${title:-}" | xargs)
  case "$id" in id|''|----*) continue;; esac
  [ -z "$slug" ] && continue
  folder="posts/${id}-${slug}"
  if [ ! -f "$folder/index.md" ]; then echo "MISSING: $folder"; missing=$((missing+1)); continue; fi
  if [ "$part" != "$prevpart" ]; then printf '\n## %s\n\n' "$part" >> "$SRC/SUMMARY.md"; prevpart="$part"; fi
  dir="${id}-${slug}"; mkdir -p "$SRC/$dir"
  for img in "$folder"/*.png "$folder"/*.jpg "$folder"/*.jpeg "$folder"/*.gif; do cp "$img" "$SRC/$dir/"; done
  for svg in "$folder"/*.svg; do
    base=$(basename "$svg" .svg)
    if command -v qlmanage >/dev/null 2>&1; then
      qlmanage -t -s 1600 -o "$SRC/$dir" "$svg" >/dev/null 2>&1 && mv "$SRC/$dir/$base.svg.png" "$SRC/$dir/$base.png"
      # QuickLook pads the thumbnail to a square; trim the padding back to the figure.
      command -v magick >/dev/null 2>&1 && magick "$SRC/$dir/$base.png" -trim +repage -bordercolor white -border 24 "$SRC/$dir/$base.png"
    else
      cp "$svg" "$SRC/$dir/"
    fi
  done
  python3 - "$folder/index.md" "$SRC/$dir/index.md" "$title" <<'PY'
import re, sys
src, dst, title = sys.argv[1], sys.argv[2], sys.argv[3]
s = open(src).read()
if s.startswith('---\n'):                       # drop the site front matter
    s = s[s.index('\n---\n', 4) + 5:]
s = re.sub(r'^\s*_Part of my \[AI Security Engineering\].*?_\n', '', s, count=1, flags=re.S)  # the series note
s = re.sub(r'\]\(([^)\s]+)\.svg\)', r'](\1.png)', s)  # figures were rasterized for the EPUB
open(dst, 'w').write('# ' + title + '\n\n' + s.lstrip('\n'))
PY
  echo "* [${id}  ${title}](${dir}/index.md)" >> "$SRC/SUMMARY.md"
  count=$((count+1))
done < posts/MANIFEST.md
echo "assembled $count chapter(s) into $SRC, missing: $missing"
[ "$missing" -eq 0 ] || exit 1

# honkit resolves a relative output path against the book root, so pass absolute ones.
OUT=$(cd "$BOOK" && pwd)
rm -rf "$OUT/web"
npx -y honkit build "$SRC" "$OUT/web"
npx -y honkit epub "$SRC" "$OUT/ai-security-engineering.epub"
echo "built $BOOK/web and $BOOK/ai-security-engineering.epub"
