# the attack

The same injected newsletter as the previous lesson: one real event, then a line
that tells the model to add an event that is nowhere in the email. This lesson
does not strip that line. It leaves it in and changes only how the email is
presented to the model, to isolate the instruction-hierarchy layer from the
sanitizer. `hostile.txt` is the email.
