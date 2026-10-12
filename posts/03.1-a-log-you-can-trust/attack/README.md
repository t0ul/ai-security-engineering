# the attack

There is no input file here; the attack is on the log itself. The demo records a
short run to two logs, a naive plain one and gledger's hash-chained one, then does
what an intruder who reaches the disk would do: edit one recorded line to rewrite
history, flipping the moment the agent BLOCKED an injected event into ALLOWED, so
the audit no longer shows the bad thing happening. The question each log answers is
whether that edit is detectable.
