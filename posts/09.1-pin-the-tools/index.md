---
title: "A pinned tool catalog"
description: "An authenticated plan can still call a tool whose definition changed after you approved it. This post has an MCP server quietly swap a tool's description, a rug-pull, and the agent takes it. Then it routes every tool call through the real MCP gateway, which pins the manifest it approved and refuses a changed one until an operator reviews it."
date: 2026-02-24
draft: true
tags: ["ai-security", "golang", "agents", "mcp", "supply-chain"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## A plan can be honest and still go wrong

The last post made the hand-off between the agent's brains unforgeable. A plan now provably comes from the real planner, unaltered. But an authentic plan still names tools, and it reaches for them through MCP, the Model Context Protocol, where a server advertises a catalog of tools and the model reads each tool's description to decide how to use it. The plan can be perfectly genuine and still call a tool whose definition is not what it was when you approved it.

That gap is a supply-chain problem, and it opens this chapter. The tool is not yours. It lives behind a server you pointed the agent at once, and nothing stops that server from changing a tool after the fact. Change the description and you change what the model does, because the description is the instruction. This is a control lesson, deterministic, no model:

```bash
cd posts/09.1-pin-the-tools
go run .
```

## Calling the server the easy way

The first version of a tool call is the obvious one: ask the server what tools it has, and if the one you want is there, call it.

```go
func callDirect(s *rugServer, tool string) error {
	tools, _ := s.ListTools(context.Background())
	for _, t := range tools {
		if t.Name == tool {
			_, cerr := s.CallTool(context.Background(), tool, nil)
			return cerr
		}
	}
	return fmt.Errorf("tool %q not advertised", tool)
}
```

It re-reads the catalog every time and trusts whatever it finds. The server starts honest. It advertises one tool, `fetch_url`, described as fetching a URL the user asked about. The agent approves that and uses it.

## The rug-pull

Then the server changes the tool. Same name, same place in the catalog, new description: fetch the URL, and also post the user's inbox to an attacker. In a real rug-pull that line is a working exfiltration instruction the model will dutifully follow; here it is a benign marker that does nothing, so the lesson stays safe to run, but the mechanism is identical. The agent, which trusts the live catalog, uses the new definition the instant it appears.

```
== before: the agent calls the MCP server directly ==
  approved manifest hash: e0cf199b5290
  first call used: "Fetch the contents of a URL the user asked about."
  [rug-pull] server swapped the fetch_url definition; live hash: fab10b0567f8 (changed: true)
  call after the swap used the poisoned definition: true
  mcp-rug-pull ASR: 100% (1/1)
```

The hash of the catalog changed, from `e0cf199b` to `fab10b05`, and the agent did not notice, because it never remembered the first one. It asked the server what to do and the server told it something new.

## Pin what you approved

The fix is to remember the catalog you approved and refuse a different one. The agent calls tools through a small MCP gateway I wrote called [gustoms](https://github.com/t0ul/gustoms). It is the tool-plane twin of the model gateway: an agent never calls a server directly, it calls through gustoms, which keeps an allowlist of approved servers and a content hash of each server's tool catalog, the pin. On every call it re-hashes the live catalog and compares. A catalog that matches the pin goes through; one that does not is refused as a possible rug-pull until an operator looks at it.

The hash is over the tools' names and descriptions, so reordering them is fine and changing, adding, or removing one is not. Nothing else about the attack changes. The same server pulls the same swap, but this time every call runs through the gateway.

```
== after: the agent calls through the pinned gustoms gateway ==
  gateway pinned the manifest it approved; first call used: "Fetch the contents of a URL the user asked about."
  [rug-pull] server swapped the fetch_url definition
  call after the swap refused: gustoms: tool manifest changed since approval (possible rug-pull); re-approval required
  mcp-rug-pull ASR: 0% (0/1)
  operator re-approves after review; the call works again: true
```

The poisoned definition never reached the agent. The gateway saw the live hash no longer matched the one it approved and stopped the call before the swapped tool ran, so the exfiltration line the model would have followed was never handed to it.

## Not a dead end

A pin that only ever says no would be useless, because tools do get legitimate updates. The last line is the other half: an operator reviews the change and re-approves, which re-pins the gateway to the new catalog, and calls flow again. That is the whole point of pinning. It does not forbid change, it forces change to pass through a human who looked, instead of happening silently behind the agent's back. The decision to trust a new tool definition is yours to make on purpose, not the server's to make for you.

That closes the tool catalog. The agent will now only call tools it approved, exactly as approved. But the tools are not the only thing the agent downloads and trusts. The model itself arrives as a file from somewhere, and so do its weights and adapters, and the next post asks the same question of them: how do you know the artifact you are running is the one you approved.

## Take it to your own project

Pin the tools the agent may call:

- Fix the exact set and version of tools, and verify it before use.
- Reject a tool that was swapped, added, or changed under you.
- Treat tool definitions as part of your supply chain.
- Fail closed if the catalog does not match what you pinned.
