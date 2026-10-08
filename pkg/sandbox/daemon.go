// Package sandbox is the detonation daemon that runs INSIDE the ephemeral
// MicroVM. The trusted control plane on the host validates a command
// (controlplane.Interpreter) and hands it to this daemon over virtio-vsock,
// which launch_vm.go bridges to the host loopback at 127.0.0.1:5000. The sandbox
// needs no IP route back to the host, so a compromised command can only run here
// and return bytes — it cannot reach the governance layer.
//
// Protocol (matches controlplane.Interpreter): the hardened path sends an argv
// array run with no shell; a legacy command string (`sh -c`) is kept for
// host-side development. Each detonation runs in a fresh temp dir removed
// afterwards (ephemeral isolation).
//
//	POST / {"argv":["uname","-a"],"trace_id":"<id>"} -> 200 {"output":"<stdout>","trace_id":"<id>"}
//	POST / {"command":"<sh>","trace_id":"<id>"}      -> 200 {"output":"<stdout>","trace_id":"<id>"}
//
// The command runner and the hand-rolled HTTP framing are OS-independent and
// unit-tested on any platform; only the AF_VSOCK listener is Linux-only (see
// vsock_linux.go). The daemon depends only on the Go stdlib, so it is available
// the instant the VM boots.
package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultPort is the vsock port the daemon listens on (bridged to host loopback).
const DefaultPort = 5000

// DefaultTimeout bounds one command so a runaway cannot hang the chamber.
const DefaultTimeout = 20 * time.Second

// Request is the inbound detonation command. The hardened control plane sends
// Argv (an argument array run with NO shell); Command (a legacy `sh -c` string)
// is kept for host-side development and manual testing. Argv takes precedence.
type Request struct {
	Command string   `json:"command,omitempty"`
	Argv    []string `json:"argv,omitempty"`
	TraceID string   `json:"trace_id"`
}

// Response is the detonation result.
type Response struct {
	Output  string `json:"output"`
	TraceID string `json:"trace_id"`
}

// Daemon executes commands and serves the detonation protocol over a connection.
type Daemon struct {
	Timeout time.Duration
	// Log, if set, receives one structured line per exec (trace_id, command).
	// It must never be given the command output — only what was requested.
	Log func(traceID, command string)
}

// NewDaemon returns a daemon with the default timeout.
func NewDaemon() *Daemon { return &Daemon{Timeout: DefaultTimeout} }

func (d *Daemon) timeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return DefaultTimeout
}

// Execute logs the request and runs the command, returning the protocol
// response. Argv (no-shell array) is preferred; Command (`sh -c`) is the legacy
// fallback.
func (d *Daemon) Execute(req Request) Response {
	if d.Log != nil {
		logged := req.Command
		if len(req.Argv) > 0 {
			logged = strings.Join(req.Argv, " ")
		}
		if len(logged) > 500 {
			logged = logged[:500]
		}
		d.Log(req.TraceID, logged)
	}
	switch {
	case len(req.Argv) > 0:
		return Response{Output: d.RunArgv(req.Argv), TraceID: req.TraceID}
	case req.Command != "":
		return Response{Output: d.RunCommand(req.Command), TraceID: req.TraceID}
	default:
		return Response{Output: "[detonation-daemon] no command provided", TraceID: req.TraceID}
	}
}

// RunArgv runs an argument array directly — exec(argv[0], argv[1:]) with NO
// shell — so an argument can never be re-interpreted as a command. This is the
// hardened detonation path. Output handling matches RunCommand.
func (d *Daemon) RunArgv(argv []string) string {
	if len(argv) == 0 {
		return "[detonation-daemon] no command provided"
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout())
	defer cancel()
	return d.capture(ctx, exec.CommandContext(ctx, argv[0], argv[1:]...))
}

// RunCommand runs command via `sh -c` (legacy host-dev path), capturing stdout
// and stderr, bounded by the timeout.
func (d *Daemon) RunCommand(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout())
	defer cancel()
	return d.capture(ctx, exec.CommandContext(ctx, "sh", "-c", command))
}

// capture runs cmd in a fresh, private temp working directory that is removed
// afterwards, so one detonation cannot observe another's files (ephemeral
// per-detonation isolation). It captures stdout+stderr, bounded by the timeout.
// A non-zero exit is normal and returns whatever was captured; a timeout or a
// failure to start is reported as a daemon message.
func (d *Daemon) capture(ctx context.Context, cmd *exec.Cmd) string {
	if dir, err := os.MkdirTemp("", "detonation-"); err == nil {
		cmd.Dir = dir
		defer os.RemoveAll(dir)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("[detonation-daemon] command timed out after %ds", int(d.timeout().Seconds()))
	}
	// A non-zero exit (ExitError) is expected; anything else means the command
	// could not be started (e.g. no such binary) and is surfaced to the caller.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Sprintf("[detonation-daemon] execution error: %v", err)
	}

	out := stdout.String()
	if stderr.Len() > 0 {
		out += "\n[stderr]\n" + stderr.String()
	}
	return strings.TrimSpace(out)
}

// ServeConn reads one HTTP/1.1 request from conn, executes the command, writes
// the JSON response, and closes the connection. A malformed request yields an
// empty command (the daemon reports "no command provided"), never a crash.
func (d *Daemon) ServeConn(conn io.ReadWriteCloser) {
	defer conn.Close()
	body, err := readHTTPBody(conn)
	if err != nil && len(body) == 0 {
		return
	}
	var req Request
	_ = json.Unmarshal(body, &req) // empty/invalid -> zero Request
	respBody, _ := json.Marshal(d.Execute(req))
	_ = writeHTTPResponse(conn, respBody)
}

// ServeListener accepts connections and serves each on its own goroutine until
// the listener is closed.
func (d *Daemon) ServeListener(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go d.ServeConn(conn)
	}
}

// readHTTPBody reads one HTTP/1.1 request (headers then Content-Length body).
func readHTTPBody(r io.Reader) ([]byte, error) {
	br := bufio.NewReader(r)
	length := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break // end of headers
		}
		if k, v, ok := strings.Cut(trimmed, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "content-length") {
			if n, perr := strconv.Atoi(strings.TrimSpace(v)); perr == nil && n >= 0 {
				length = n
			}
		}
	}
	if length == 0 {
		return []byte{}, nil
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(br, body); err != nil {
		return body, err
	}
	return body, nil
}

// writeHTTPResponse writes a minimal HTTP/1.1 200 with the JSON body.
func writeHTTPResponse(w io.Writer, body []byte) error {
	head := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(body))
	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}
