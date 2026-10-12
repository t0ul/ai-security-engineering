# attack inputs

This lesson builds its inputs in a temporary inbox at run time rather than shipping
files, because the interesting ones are awkward to keep in a repo:

- an oversized file, just past the product's 256 KiB cap. It is deliberately small
  here (a real denial-of-service file would be gigabytes); crossing the limit is
  what matters, not the size.
- a half-written file, one still being written when the scan runs, simulated by a
  fresh modification time so the settle delay treats it as not yet settled.
- a flood: more small files than the per-cycle cap, dropped at once.
