# 07.4-no-shell

A tool called with model-chosen arguments is safe only if those arguments never
become a shell string. Pasted into `sh -c`, a pattern injects a command and a ../
filename escapes the allowed folder. The agent's real validator checks each argument
against a schema and confines paths, then runs the tool as an argv array, so there
is no shell to inject into. Closes the isolation chapter.

Run it:

```bash
go run .
```
