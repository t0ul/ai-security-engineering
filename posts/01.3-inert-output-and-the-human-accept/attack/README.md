# attack inputs

Two events, written inline in main.go rather than as files, because this lesson is
about what happens AFTER extraction: whether a parsed event reaches a real calendar
on its own.

- Multicultural Potluck, Friday October 23rd: a clean event (the weekday matches
  the date), the kind that should sail through.
- Open House, Thursday September 29th: the flagged event from the date lesson.
  September 29, 2026 is a Tuesday, so the parser marks it for review. It is the
  "bad extraction" that must not reach the calendar without a human looking at it.
