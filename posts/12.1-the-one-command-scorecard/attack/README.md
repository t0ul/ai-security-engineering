# the attack

No input file. The thing under attack is the scorecard itself, and the attack is
false assurance: a row that is green but proves nothing.

Two ways a row lies. A stubbed row never exposes the attack, so its undefended arm
reports 0% and the control is never actually tested. A not-wired row has a real
control that passes its unit test, but the real pipeline never calls it, so the
integration still leaks. Both show "defended 0%" and look identical to a genuine
row.

The real Attack-Driven Design gate refuses to be fooled: it requires the
undefended arm to breach at 100% (the RED must be reachable) and an optional wired
arm to prove the control is on the path. The question is whether "all green" means
every control is attacked and wired, or just that nobody checked.
