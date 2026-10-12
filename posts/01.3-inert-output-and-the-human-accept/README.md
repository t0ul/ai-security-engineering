# 01.3-inert-output-and-the-human-accept

Auto-adding extracted events puts a flagged parse straight on a real calendar with
no human in the loop. Writing an inert .ics file and adding only what a person
accepts keeps the flagged event off until someone chooses to import it.

Run it:

```bash
go run .
```

You should see the flagged event land on the calendar with no defense, then be held
out of the safe default once the agent only writes a file and a human accepts.
