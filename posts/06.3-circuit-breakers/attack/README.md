# the attack

No input file; the attack is a loop that will not stop. An agent that routes from
one step to the next can be pushed into a cycle, by a bug or by an instruction, that
never reaches a finish. Each trip around is work: a model call, a tool run, money and
time. With nothing to stop it, it spins until the machine or the budget gives out.
This is a denial-of-service against the agent itself, OWASP LLM10.
