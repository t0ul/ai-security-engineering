package controlplane

import (
	"context"
	"errors"
	"fmt"

	"github.com/t0ul/ai-security-engineering/mcp"
)

// SandboxExecTool exposes the egress-denied MicroVM detonation chamber as a
// governed MCP tool, so any agent — or the gustoms gateway sitting in front of
// it (allowlist + manifest pin + per-call authorization) — can run untrusted
// commands through the one isolation boundary instead of on the host.
//
// The caller passes an argv array; the interpreter's Policy enforces the
// argv[0] allowlist and rejects shell metacharacters (argcheck), then detonates
// the array with NO shell inside the VM over vsock. Two properties make this
// safe to expose: an argument can never break out into command injection (no
// shell), and a detonated command cannot beacon out (the VM has no egress).
func SandboxExecTool(in *Interpreter) mcp.Tool {
	return mcp.Tool{
		Name:        "sandbox_exec",
		Description: "Run an allow-listed command (argv array) inside the isolated, egress-denied MicroVM and return its output.",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			argv, err := toArgv(args["argv"])
			if err != nil {
				return nil, err
			}
			traceID, _ := args["trace_id"].(string)
			// Policy block vs sandbox output both come back as the output string;
			// a blocked command is prefixed "[Interpreter] BLOCKED".
			return map[string]any{"output": in.ExecuteArgv(ctx, traceID, argv)}, nil
		},
	}
}

// toArgv coerces a JSON array of strings into an argv slice.
func toArgv(v any) ([]string, error) {
	raw, ok := v.([]any)
	if !ok || len(raw) == 0 {
		return nil, errors.New("sandbox_exec: 'argv' must be a non-empty array of strings")
	}
	argv := make([]string, len(raw))
	for i, e := range raw {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("sandbox_exec: argv[%d] is not a string", i)
		}
		argv[i] = s
	}
	return argv, nil
}
