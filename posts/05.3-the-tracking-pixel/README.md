# 05.3-the-tracking-pixel

A markdown image in model output is a tracking pixel: a browser fetches it on page
load, no click, and its URL can carry data out. Handed to the page raw, the beacon
reaches the browser; run through the agent's SanitizeMarkdown first and the image
and auto-fetching link are stripped.

Run it:

```bash
go run .
```
