#!/usr/bin/env python3
"""
Generate anonymized sample emails for the public repo.

The real newsletters (../../e-mails, git-ignored) contain a real school, staff
names, a staff email, and live form links. This script applies a consistent
fictional mapping so the committed demo/eval fixtures carry no real PII while
preserving structure and dates (which the extractor and evals rely on).

Reproducible and on-theme: it is itself a small PII-scrubbing tool.
Usage:  python anonymize_samples.py [SRC_DIR] [OUT_DIR]
"""
import os
import sys

# Ordered: most-specific first so partials don't clobber (e.g. full URL/email
# before the bare domain; "Lower Manhattan..." before "LMC").
REPLACEMENTS = [
    ("https://forms.gle/bt3RZEiu9ZPH43A97", "https://forms.example.org/survey"),
    ("forms.gle/bt3RZEiu9ZPH43A97", "forms.example.org/survey"),
    ("https://www.schools.nyc.gov/about-us/messages-for-families", "https://www.example-schools.org/families"),
    ("omanzano@schools.nyc.gov", "front.office@example-schools.org"),
    ("Lower Manhattan Community Middle School", "Riverside Community Middle School"),
    ("LMC", "RCMS"),
    ("New York City Public Schools", "Metro Public Schools"),
    ("NYC Public Schools", "Metro Public Schools"),
    ("NYCPS", "MPS"),
    ("P.S. 51", "Maplewood Elementary"),
    ("PS 51", "Maplewood Elementary"),
    ("PS51", "Maplewood Elementary"),
    ("Chancellor Kamar Samuels", "Chancellor Jordan Rivers"),
    ("Kamar Samuels", "Jordan Rivers"),
    ("Devin Elle Kurtz", "Jane Rivers"),
    ("Ms. Capasso", "Ms. Carter"),
    ("Ms. Manzano", "Ms. Morgan"),
    ("Ms Gartner", "Ms. Gray"),
    ("Ms. Gartner", "Ms. Gray"),
    ("Ms. Avihay", "Ms. Adams"),
    ("Ms. Levine", "Ms. Lee"),
    ("Ms. Kartez", "Ms. Kane"),
    ("Ms. Miller", "Ms. Mills"),
    ("Ms. Pensebene", "Ms. Park"),
    ("Ms. Murynec", "Ms. Moore"),
    ("Ms. Hughes", "Ms. Hill"),
    ("District 2", "the local district"),
    ("11th Avenue", "Elm Avenue"),
    ("11th Ave", "Elm Avenue"),
    ("44th street", "Oak Street"),
    ("44th Street", "Oak Street"),
    ("Stephanie", "Riley"),
    ("Dana", "Jamie"),
    ("schools.nyc.gov", "example-schools.org"),  # any leftover bare domain
]

# Public, published references intentionally KEPT (not PII): FERPA, PPRA,
# Title I, Chancellor's Regulation A-820, "The Best Day Ever by Marilyn Singer".


def anonymize(text: str) -> str:
    for old, new in REPLACEMENTS:
        text = text.replace(old, new)
    return text


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    src = sys.argv[1] if len(sys.argv) > 1 else os.path.join(here, "..", "..", "e-mails")
    out = sys.argv[2] if len(sys.argv) > 2 else os.path.join(here, "samples")
    os.makedirs(out, exist_ok=True)
    n = 0
    for name in sorted(os.listdir(src)):
        if not name.endswith(".txt"):
            continue
        with open(os.path.join(src, name), encoding="utf-8") as f:
            text = f.read()
        with open(os.path.join(out, name), "w", encoding="utf-8") as f:
            f.write(anonymize(text))
        n += 1
    print(f"anonymized {n} files -> {out}")


if __name__ == "__main__":
    main()
