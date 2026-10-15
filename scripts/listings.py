#!/usr/bin/env python3
"""listings.py: keep the code printed in a chapter identical to the code in its lesson folder.

A chapter marks where a listing goes:

    <!-- listing: main.go#before: a plain log, and a tamper it cannot see -->
    ```go
    ...
    ```
    <!-- /listing -->

The name after # is a section header in the Go file, written as

    // ---- name ----

and the listing runs from that header to the line before the next header. Two names
are special: #head is everything before the first header (the doc comment, package,
imports and main), and #all is the whole file, for files without headers such as the
test and go.mod.

    scripts/listings.py write --all            rewrite every listing in every chapter that has one
    scripts/listings.py check --all            exit 1 and name any listing that no longer matches its file
    scripts/listings.py write --src posts/03.1-a-log-you-can-trust path/to/index.md   one chapter, any copy

Nothing in a chapter's code is typed by hand: write it in the lesson folder, run the
lesson, then let this script print it. The check runs in `make check`, so a chapter
cannot drift from its code and still pass the gate.
"""
import re
import sys
from pathlib import Path

MARK = re.compile(r"<!-- listing: (?P<file>[^#\s]+)#(?P<name>[^>]+?) -->\n(?P<body>.*?)<!-- /listing -->", re.S)
HEADER = re.compile(r"^// ---- (.+?) ----$")


def section(src: Path, name: str) -> str:
    noheader = name.endswith("!noheader")
    name = name.removesuffix("!noheader").strip()
    lines = src.read_text().split("\n")
    headers = [i for i, l in enumerate(lines) if HEADER.match(l)]
    if name == "all":
        chunk = lines
    elif name == "head":
        chunk = lines[: headers[0]] if headers else lines
    else:
        starts = [i for i in headers if HEADER.match(lines[i]).group(1) == name]
        if not starts:
            raise SystemExit(f"{src}: no section named {name!r}")
        i = starts[0]
        nxt = [j for j in headers if j > i]
        chunk = lines[i : nxt[0]] if nxt else lines[i:]
        if noheader:
            chunk = chunk[1:]
            while chunk and chunk[0].strip() == "":
                chunk = chunk[1:]
    while chunk and chunk[-1].strip() == "":
        chunk = chunk[:-1]
    return "\n".join(chunk)


def render(srcdir: Path, text: str):
    drift = []

    def rep(m):
        f, name = m.group("file"), m.group("name").strip()
        lang = "go" if f.endswith(".go") else ""
        code = section(srcdir / f, name)
        body = f"```{lang}\n{code}\n```\n"
        if m.group("body") != body:
            drift.append(f"{f}#{name}")
        return f"<!-- listing: {f}#{name} -->\n{body}<!-- /listing -->"

    return MARK.sub(rep, text), drift


def main(argv):
    if len(argv) < 2 or argv[0] not in ("write", "check"):
        raise SystemExit(__doc__)
    mode = argv[0]
    jobs = []
    if argv[1] == "--all":
        for idx in sorted(Path("posts").glob("[0-9]*/index.md")):
            if "<!-- listing:" in idx.read_text():
                jobs.append((idx.parent, idx))
    elif argv[1] == "--src":
        jobs = [(Path(argv[2]), Path(p)) for p in argv[3:]]
    else:
        raise SystemExit(__doc__)
    bad = 0
    for srcdir, idx in jobs:
        text = idx.read_text()
        new, drift = render(srcdir, text)
        if mode == "write":
            if new != text:
                idx.write_text(new)
                print(f"wrote {idx}: {', '.join(drift)}")
            else:
                print(f"{idx}: up to date")
        elif drift:
            bad += 1
            print(f"drift: {idx}: {', '.join(drift)}")
    if mode == "check":
        print("listings: clean" if not bad else f"listings: {bad} chapter(s) drifted; run make listings")
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main(sys.argv[1:])
