# 01.2-dates-the-model-never-decides

A naive date parser calendars a Thursday that is really a Tuesday and a February 30
that does not exist, with no warning. The agent's real parser flags the first for
review and rejects the second, and still calendars a correct date.

Run it:

```bash
go run .
```

You should see both wrong dates land on the calendar with no defense, then be held
and rejected once the control is in, and a clean date still go through.
