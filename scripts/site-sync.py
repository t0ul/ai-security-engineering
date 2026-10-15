#!/usr/bin/env python3
"""site-sync.py: keep each chapter's copy on the site equal to the post in this repo.

The post (posts/<id>-<slug>/index.md) is the source of truth for the body. The site copy
(content/blog/<site-slug>/index.md in the t0ul.com repo) keeps its own front matter, which
carries the site-only fields such as weight and part. Site folders that differ from the
post slug are listed in scripts/site-slugs.txt.

    scripts/site-sync.py write     copy every post body over its site copy
    scripts/site-sync.py check     exit 1 and name every site copy whose body differs

The site repo is taken from $SITE (the content/blog directory), default
/Users/t/Desktop/peter-lankton/t0ul.com/content/blog.
"""
import os
import re
import sys
from pathlib import Path

SITE = Path(os.environ.get("SITE", "/Users/t/Desktop/peter-lankton/t0ul.com/content/blog"))
FM = re.compile(r"\A---\n.*?\n---\n", re.S)


def split(text):
    m = FM.match(text)
    if not m:
        raise SystemExit("no front matter")
    return m.group(0), text[m.end():]


def pairs():
    overrides = {}
    for line in Path("scripts/site-slugs.txt").read_text().splitlines():
        if line and not line.startswith("#"):
            pid, slug = line.split("\t")
            overrides[pid] = slug
    for idx in sorted(Path("posts").glob("[0-9]*/index.md")):
        folder = idx.parent.name
        pid, slug = folder.split("-", 1)
        yield idx, SITE / overrides.get(pid, slug) / "index.md"


def main(argv):
    if argv[:1] not in (["write"], ["check"]):
        raise SystemExit(__doc__)
    mode = argv[0]
    drift, missing = [], []
    for post, site in pairs():
        if not site.exists():
            missing.append(str(site.parent.name))
            continue
        _, body = split(post.read_text())
        site_fm, site_body = split(site.read_text())
        if body == site_body:
            continue
        if mode == "write":
            site.write_text(site_fm + body)
            print(f"wrote {site}")
        drift.append(site.parent.name)
    if missing:
        print("no site copy for: " + ", ".join(missing))
    if mode == "check":
        print("site: clean" if not drift else "site: drifted: " + ", ".join(drift))
        sys.exit(1 if drift else 0)


if __name__ == "__main__":
    main(sys.argv[1:])
