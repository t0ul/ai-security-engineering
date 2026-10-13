# the attack

No input file; the hazard is a bad change already live, and a recovery that only
looks like a recovery. An operator makes a prompt change that breaks a safety rule,
notices, and rolls back the obvious way: pastes the old prompt back in. But a
hand-restored prompt is a brand-new version, and a stray space or a missed line
means it is not the version that was known good, only one that resembles it.

The question is whether a rollback can be proven. A clean rollback re-activates a
specific, content-hashed version and checks the live prompt against that hash, so
"we rolled back" is a verifiable fact, not a hope, and the rollback is recorded with
who did it and to which version.
