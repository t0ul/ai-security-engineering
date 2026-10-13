# the attack

No input file; two attacks on what the agent is allowed to trust and ship.

dataset-poisoning: a record that claims to come from a trusted source, so it lands
in the trusted corpus instead of the contained one, but the "signature" is the
attacker's. A naive ingester that trusts the claim plants poisoned data as trusted.
The URL and tuition line are benign markers.

promotion-gate-bypass: a change to the agent, a prompt or model, pushed straight to
production without clearing the gates. Here the change failed the red-team, its ADD
attack-success-rate is not zero, yet a naive promote ships it. The question is
whether trusting data requires a verifiable signature, and whether shipping a change
requires passing eval and the red-team first, both failing closed.
