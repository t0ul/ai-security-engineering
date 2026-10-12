# 05.2-the-calendar-field-exfil

An inert .ics is not harmless if a field hides a URL: it beacons when a calendar
client renders it, and can carry data out in its query string. A naive writer
serializes the poisoned fields as-is; the agent's real ics.Write strips every link
and active scheme from every field.

Run it:

```bash
go run .
```
