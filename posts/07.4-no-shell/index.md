---
title: "No shell"
description: "The chamber is isolated and its one exit is gated, but there is still a hole in how a command gets in. A command is a string, and a string handed to a shell is a small programming language an attacker can write in. This post closes it: arguments validated against a schema and passed as an array, with no shell to inject into and no path to escape through."
date: 2026-02-03
draft: false
tags: ["ai-security", "golang", "agents", "isolation"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The hole in the way in

This chapter put untrusted code in [a box with no network card](/blog/the-detonation-chamber/) and gave that box [one audited way out](/blog/egress-deny-and-the-broker/). Both are about what the code can reach once it is running. There is a hole left on the other side, in how the command gets into the box in the first place, and it is the oldest bug in computing wearing a new coat.

We have been handing the chamber a command, and a command is a string, and the moment a string built from untrusted input is handed to a shell, the shell treats it as what it is: a small programming language. Then the arguments the model chose are not arguments, they are source code. This post closes that, and it closes the chapter. It is a control lesson, deterministic, no model:

```bash
cd posts/07.4-no-shell
go run .
```

## I. A string is a program

Here is a modest tool: it greps a file for a pattern. It takes two arguments the model filled in from the email, a pattern and a filename, and it runs grep the easy way, by building a command string and handing it to a shell:

```go
cmd := "grep " + pattern + " " + filepath.Join(baseDir, file)
exec.Command("sh", "-c", cmd).Run()
```

Those two `+` are the whole vulnerability, and there are two different attacks waiting in them. The first is in the pattern. Give it a value that closes the grep and starts a command of its own, and the shell runs it:

```
== before: arguments in a shell string ==
  pattern = "x; echo INJECTED-COMMAND-RAN > <marker> #"
  an injected command ran (marker file created): true
```

The semicolon ended the grep, the `echo` ran as its own command, and the trailing `#` commented out the rest. A pattern argument just executed code. The command here only writes a marker, because I am not going to run a destructive one, but the shell does not know or care what the command is, which is exactly the problem.

The second attack is in the filename. The tool means to read only from one folder, but a filename with `../` in it walks right out:

```
  file = "../secret.txt" read a file outside the base folder: true
```

A file the tool was never meant to touch, read because the filename was trusted to stay where it belonged. Two arguments, two holes, and both come from the same mistake: treating attacker-controlled strings as if they were safe to build a command out of.

## II. Arguments as an array, not a sentence

The fix has two parts, and the first is the one that matters most: do not build a shell string at all. Pass the arguments as an array straight to the program, with no shell in between. `exec.Command("grep", pattern, path)` hands grep three separate arguments, and the pattern, whatever it contains, is one literal argument that grep searches for. There is no shell to interpret a semicolon, so a semicolon is just a character in a search. The programming language the attack needed is simply not there.

The second part is to check the arguments before they are used, which is where the agent's `argcheck` comes in. It validates each argument against a small schema: it rejects shell metacharacters in a string, rejects arguments that are not in the schema at all, and confines a path argument under a base directory so `../` and absolute paths are refused. It is defense in depth, the belt to the argv array's suspenders, so that a dangerous argument is turned away up front and never even reaches the program as a literal. Run the same two attacks through it:

```
== after: the validator + an argument array ==
  the injecting pattern: refused: shell metacharacter in argument: "pattern" (no command ran: true)
  the traversing file:   refused: path escapes the confinement base: "../secret.txt"
  a clean call:          found: the word root appears here
```

The injecting pattern is refused for its shell metacharacters, no command runs. The traversing filename is refused for escaping the base, no outside file is read. And the clean call, a real pattern and a real file, still works, running as an argument array with no shell anywhere in it. Both attacks went from landing to refused, and the tool still does its job.

## The two rules

It is worth stating the rules plainly, because they are small and they cover a huge amount of ground. First, never build a shell string out of values you did not write; pass an argument array, so there is no shell and the values are data, not code. Second, validate those values against what the tool actually expects, and confine any path to where it belongs, so a bad argument is rejected before it is used at all. Neither rule tries to guess what an attacker might send. The first removes the shell the attack needs; the second refuses the shapes an argument should never have. Together they mean the command going into the chamber is as constrained as the chamber the command runs in.

## Conclusion

That closes the isolation chapter, and with it the hardest problem in the series. The agent can run code produced from an untrusted email, and every dangerous part of that is now contained: the code executes in a virtual machine with no host and no network, it reaches out only through a broker that dials the allowlist and reads nothing, and the command that goes in is an array of validated arguments, never a sentence an attacker gets to compose. Running untrusted code went from owning the machine to touching nothing.

Part 4 is not done, though, because isolation is only one of its two big ideas. The chamber keeps dangerous code from reaching anything; the next chapter keeps the model that reads dangerous text from being the model that decides to run it in the first place. That is the dual-LLM design, where a privileged planner acts and a quarantined reader touches the untrusted email but can never do anything with it except hand back data. The walls are built; next we split the brain.

## Take it to your own project

Never let a string become a command:

- Pass arguments as an argv array, never by string concatenation.
- Validate every argument against an allowlist.
- Reject shell metacharacters outright.
- Run without a shell, so there is no shell to inject into.
