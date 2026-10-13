# the attack

No input file; the attack is a forged or altered artifact that claims to be the
agent's output. The agent produces things people act on, a calendar file they
import, a summary they trust, and "the agent made this" has to be checkable or it
is worthless.

output-forgery has two shapes. A forged artifact is one the attacker made, signed
with their own key, claiming to come from the agent, here a fake event telling the
reader to pay tuition. A tampered artifact is the agent's real output with its
contents changed after the fact, a real event whose date and location were altered.
The contents here are benign markers. The question is whether a consumer verifies a
content credential before trusting an artifact, so both the forgery and the tamper
are caught while the agent's genuine output still verifies.
