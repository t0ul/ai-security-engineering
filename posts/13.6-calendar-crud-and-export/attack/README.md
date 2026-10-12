# the attack

No attacker, no input file. The hazard is the agent doing a correct thing more than
once: the folder watcher re-reads an email after a restart, a request is retried
after a timeout, an export runs a second time. Every one of these is a legitimate
replay, and every one of them, on a naive write, adds the same event to the calendar
again.

duplicate-side-effect is that replay firing its write twice. The fix is idempotency:
key each write by the event's content so a replay maps to the same key, and route it
through a durable ledger that returns the first result instead of writing again. The
question is whether "add this event" happens once per event or once per attempt.
