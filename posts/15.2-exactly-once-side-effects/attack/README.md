# the attack

No attacker and no input file; the hazard is a retry. Agents retry constantly: a
request times out, a process resumes after a crash, a client never saw the
response and sends the same thing again. For a read, a retry is free. For a
dangerous side effect, a refund, a payment, an outbound message, a retry that
re-fires is a double charge.

duplicate-side-effect is that re-fire. The question is whether a dangerous action
happens once per intent or once per attempt. The fix is an idempotency key: route
the action through a durable ledger so a retry of a completed action replays its
result instead of doing it again, while a genuinely failed attempt, where the
effect never happened, is still free to be retried until it completes.
