# the attack

No input file. The thing under attack is the test suite's honesty. Two shapes of
dishonest suite both report green while proving nothing: one skips the model when
it is slow or down and calls that a pass; the other swaps in a stub that always
refuses, so the attack can never land and the row is permanently green.

A live red-team does neither. It fires the real attack (here, a plain injection
telling the model to echo a marker) at the real model, reports a measured ASR, and
fails closed when the model is unreachable. The question is whether a green suite
means the attacks were run against reality, or that the suite found a way to avoid
running them.
