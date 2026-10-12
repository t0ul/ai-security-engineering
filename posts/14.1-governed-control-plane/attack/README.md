# the attack

No input file; the target is the agent's control plane. The agent resolves its
system prompts from a runtime plane an operator can change without a code deploy,
which is a real operability win and a real risk: a prompt is the agent's
instructions, so whoever can change it can tell the agent to ignore every safety
rule, or to send payments on command.

The attack is a prompt swap by someone who should not be allowed to make it: an
app-scoped role, or anyone at all on an ungoverned plane, replacing the governed
chat prompt with one that strips the never-obey rule. The question is whether
changing the agent's instructions requires a signed, scoped operator capability,
and whether every change is recorded with who made it.
