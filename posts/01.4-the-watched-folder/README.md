# 01.4-the-watched-folder

The inbox folder is an untrusted boundary. A naive scanner reads every file right
now, fully, with no limits, so an oversized file, a half-written file, or a flood
of files each bites it. The product's watcher applies a size cap, a settle delay,
and a per-cycle cap.

Run it:

```bash
go run .
```

You should see the naive scan read a too-big file whole, read a half-written file,
and process a whole flood, then the same inputs refused, skipped, and bounded.
