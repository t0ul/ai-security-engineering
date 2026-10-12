# the attack

`hostile.txt` is a school-newsletter-shaped email with one real event and, after a
fake end-of-email marker, an instruction aimed not at the reader but at the model
that will read it: add an event that is nowhere in the email, carrying an attacker
URL. It is indirect prompt injection, the kind you find by accident the first time
a real newsletter happens to contain an imperative. The live demo sends it to the
model with no defense, then through the agent's real injection controls.
