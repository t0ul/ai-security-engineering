# the attack

No input file; the target is a shared blackboard that several agents read and write.
Two ways it goes wrong.

Forged attribution: a rogue agent writes a fact and labels it "from: planner", so a
reader acting on the trusted planner's facts obeys the rogue. Here the forged fact
is an instruction to wire money to an attacker. If the board trusts the label, the
rogue speaks with the planner's authority.

Cross-scope recall: an agent's private workspace, here a finance account number,
must not surface in another agent's recall. A board that returns every scope to any
caller leaks one agent's memory to the rest. The markers are benign. The question is
whether shared memory is attributed by signature, not by label, and recalled per
scope, not wholesale.
