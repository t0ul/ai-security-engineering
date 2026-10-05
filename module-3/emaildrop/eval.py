"""
eval.py — score the event extractor against hand-labeled ground truth.
Regression metric for Module 11. Run: python eval.py [labels/3.json]
"""
import os
import re
import sys
import json

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from tools.event_extractor import extract_events


def norm(t: str) -> str:
    return re.sub(r"[^a-z0-9]+", " ", (t or "").lower()).strip()


def titles_match(a: str, b: str) -> bool:
    na, nb = norm(a), norm(b)
    return na == nb or na in nb or nb in na


def score(label_path, use_tool=False):
    label = json.load(open(label_path, encoding="utf-8"))
    src = os.path.join(os.path.dirname(os.path.abspath(__file__)), label["source_email"])
    gold = label["events"]
    text = open(src, encoding="utf-8").read()
    if use_tool:
        # score the actual tool output (honours EXTRACT_MODE: auto|llm|regex)
        from tools.event_extractor import EVENT_EXTRACTOR
        res = EVENT_EXTRACTOR.run(text, {"source": label["source_email"],
                                         "default_year": label.get("default_year", 2026)})
        pred = res.events
    else:
        pred = extract_events(text, source=label["source_email"],
                              default_year=label.get("default_year", 2026))

    matched, used = [], set()
    for g in gold:
        hit = None
        for i, p in enumerate(pred):
            if i in used:
                continue
            if p.start[:10] == g["start"][:10] and titles_match(p.title, g["title"]):
                hit = (g, p, i)
                break
        if hit:
            used.add(hit[2])
            matched.append(hit)

    tp = len(matched)
    fp = len(pred) - tp
    fn = len(gold) - tp
    prec = tp / (tp + fp) if (tp + fp) else 0.0
    rec = tp / (tp + fn) if (tp + fn) else 0.0
    f1 = 2 * prec * rec / (prec + rec) if (prec + rec) else 0.0

    print(f"=== eval: {label['source_email']} ===")
    print(f"gold={len(gold)} pred={len(pred)} matched={tp}")
    print(f"precision={prec:.2f} recall={rec:.2f} f1={f1:.2f}")
    start_ok = end_ok = loc_ok = 0
    for g, p, _ in matched:
        s = (p.start == g["start"])
        e = ((p.end or None) == (g.get("end") or None))
        l = bool(g.get("location")) and bool(p.location) and norm(g["location"]) in norm(p.location)
        start_ok += s; end_ok += e; loc_ok += l
        flag = "ok" if (s and e) else "CHECK"
        print(f"  [{flag}] {g['title']:<22} start {p.start}=={g['start']}:{s}  end {p.end}=={g.get('end')}:{e}  loc:{l}")
    if matched:
        print(f"field accuracy — start {start_ok}/{tp}  end {end_ok}/{tp}  location {loc_ok}/{tp}")
    return f1


if __name__ == "__main__":
    lp = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "labels", "3.json")
    use_tool = "--tool" in sys.argv
    f1 = score(lp, use_tool=use_tool)
    sys.exit(0 if f1 == 1.0 else 1)
