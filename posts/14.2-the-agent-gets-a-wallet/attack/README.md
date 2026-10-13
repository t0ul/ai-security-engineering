# the attack

No input file; the target is the agent's own credentials. The agent is a non-human
identity with a wallet of short-lived, signed, scoped grants it presents to act,
to send email, fetch a URL, write the calendar. When one of those leaks, or the
subject is compromised, you revoke it, and revocation has to take effect now.

revoked-token-still-works is the bug where it does not. A bearer token that is only
checked for its signature and expiry keeps being honored after revocation, until it
times out on its own, which might be an hour or a day of an attacker acting as the
agent. The question is whether every check consults the revocation list, so a
revoked grant stops working the instant it is revoked, and whether one switch can
revoke the whole wallet at once.
