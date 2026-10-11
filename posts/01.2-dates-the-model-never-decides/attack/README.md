# attack inputs

Each file is one event the way it appears in an email: a title line, a date line,
a place line. The demo parses the date line.

- `back-to-school-night.txt`: a real line from the public sample newsletter
  `testdata/emaildrop/samples/6.txt` in the product repo. The email says Thursday;
  September 29, 2026 is a Tuesday. Nobody did this on purpose, which is the point.
- `february-30.txt`: a date that does not exist. Made up for this lesson; a typo
  like it is only a matter of time.
