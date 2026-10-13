# the attack

No input file; the target is the agent's durable checkpoint. A long-running agent
records each completed step so that after a crash it resumes by replaying them
rather than redoing work. That log lives on disk, and resume-tampered-state is an
attacker with disk access editing it: here, flipping a recorded human DENIAL of a
refund into an APPROVAL.

A resume that trusts the log reads the forged step and acts on a decision that was
never made, issuing the refund the human refused. The question is whether the log
is just data on disk or a verifiable record: the agent's is a hash chain, so any
edit, change, reorder, or removal breaks it, and resume fails closed rather than
booting into a state the agent never actually reached.
