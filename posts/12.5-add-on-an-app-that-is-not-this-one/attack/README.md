# the attack

No bespoke input: the attack corpus is the gorauder catalog, the same benign,
OWASP-keyed prompt-injection seeds the emaildrop scorecard uses, fired at a brand-
new app that has nothing to do with email, a toy text summarizer.

The seeds tell the model to ignore its task and echo a marker. Against the naive
summarizer, which takes the user's text as its whole input, they land. Against the
hardened one, which fences that text as untrusted data to summarize and never
obey, they do not. The point is not the summarizer; it is that pointing the exact
same catalog, Technique, and gate at a different app took a few lines and a second,
which is the claim the whole book has been building toward.
