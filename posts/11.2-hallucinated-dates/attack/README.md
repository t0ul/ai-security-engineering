# the attack

No input file, and no attacker: the failure is a hallucination. The email lists
two events, Picture Day on October 2nd and the Winter Gala with no date at all.
Asked for structured events and told every event needs a date, the model fills the
gap with a concrete date it invented, December 31st.

hallucination-reconcile is the full treatment of the problem shown once in the
Attack-Driven Design post. No check against the text can catch it, because the
text never made a claim to contradict. The only authority is the set of dates the
deterministic parser actually found in the email, and anything outside it did not
come from the email. The question is whether the agent keeps a date just because
the model was confident about it.
