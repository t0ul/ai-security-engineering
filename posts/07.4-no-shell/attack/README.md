# the attack

No input file; the attack is the arguments a tool is called with. A tool that greps
a file takes a pattern and a filename, both chosen by a model that read an email.
Two ways to abuse that: put shell metacharacters in the pattern so a command runs
(arg injection), and put ../ in the filename so a file outside the allowed folder is
read (path traversal). The injected command here is harmless, a marker, never
destructive.
