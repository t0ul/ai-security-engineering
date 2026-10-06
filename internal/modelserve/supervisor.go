// Package modelserve supervises the local llama.cpp model servers so the dual
// SLM runtime (Llama-3.2-3B planner + Qwen-1.5B coder) comes up with one command
// instead of two hand-run shells. It refuses to start over a port that is
// already serving, launches each server in its own process group, waits for its
// /health endpoint, detects a child that exits early (so a crash is reported
// rather than masked by a stale instance on the same port), streams prefixed
// logs, and tears the whole set down on context cancellation.
//
// The process launching and the health polling are separated so both are
// testable without a real model: Supervisor runs any command and polls any URL.
package modelserve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Server describes one model server to supervise.
type Server struct {
	Name      string   // short label, e.g. "planner"
	Bin       string   // executable path
	Args      []string // arguments
	HealthURL string   // liveness: GET until 200 (used for the preflight port check)
	// Ready, when set, is the authoritative readiness probe and overrides the
	// HealthURL poll during WaitHealthy — e.g. a real 1-token completion that
	// only succeeds once the model is loaded and able to generate. It must
	// return promptly and be safe to call repeatedly.
	Ready func(ctx context.Context) bool
}

// ready reports the readiness check to use for s during WaitHealthy, and whether
// the server has one at all.
func (srv Server) readyProbe() (func(context.Context) bool, bool) {
	if srv.Ready != nil {
		return srv.Ready, true
	}
	if srv.HealthURL != "" {
		return nil, true // HealthURL poll handled by Supervisor.healthy
	}
	return nil, false
}

// Supervisor launches and tears down a set of Servers.
type Supervisor struct {
	Servers []Server
	// Stdout receives prefixed child output; defaults to os.Stdout.
	Stdout io.Writer
	// HTTPClient is used for health checks; defaults to a 2s-timeout client.
	HTTPClient *http.Client

	mu       sync.Mutex
	children []*child
	stopping bool
	deaths   chan death
}

type child struct {
	name string
	cmd  *exec.Cmd
	done chan struct{} // closed once cmd.Wait returns
	err  error
}

type death struct {
	name string
	err  error
}

func (s *Supervisor) out() io.Writer {
	if s.Stdout != nil {
		return s.Stdout
	}
	return os.Stdout
}

func (s *Supervisor) client() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 2 * time.Second}
}

// Start refuses any server whose health endpoint is already answering (a stale
// instance holds the port), then launches each in its own process group. A
// waiter goroutine reports an early, unexpected exit on the deaths channel. On
// any failure the servers already started are stopped and the error returned.
func (s *Supervisor) Start(ctx context.Context) error {
	s.deaths = make(chan death, len(s.Servers))
	for _, srv := range s.Servers {
		if srv.HealthURL != "" && s.healthy(ctx, srv.HealthURL) {
			s.Shutdown()
			return fmt.Errorf("modelserve: %s: %s is already serving — is modeld already running?", srv.Name, srv.HealthURL)
		}
		cmd := exec.CommandContext(ctx, srv.Bin, srv.Args...)
		// Own process group so Shutdown can signal the whole tree, and so a
		// child cannot outlive the supervisor as an orphan.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdout = prefixWriter(s.out(), srv.Name)
		cmd.Stderr = prefixWriter(s.out(), srv.Name)
		if err := cmd.Start(); err != nil {
			s.Shutdown()
			return fmt.Errorf("modelserve: start %s (%s): %w", srv.Name, srv.Bin, err)
		}
		c := &child{name: srv.Name, cmd: cmd, done: make(chan struct{})}
		s.mu.Lock()
		s.children = append(s.children, c)
		s.mu.Unlock()
		go func(c *child) {
			c.err = c.cmd.Wait()
			close(c.done)
			s.mu.Lock()
			stopping := s.stopping
			s.mu.Unlock()
			if !stopping {
				s.deaths <- death{c.name, c.err}
			}
		}(c)
		fmt.Fprintf(s.out(), "[modelserve] started %s (pid %d)\n", srv.Name, cmd.Process.Pid)
	}
	return nil
}

// WaitHealthy polls each server's HealthURL until it returns 200 or the deadline
// passes, aborting immediately if any child exits meanwhile. Servers with an
// empty HealthURL are treated as ready.
func (s *Supervisor) WaitHealthy(ctx context.Context, perServer time.Duration) error {
	for _, srv := range s.Servers {
		probe, has := srv.readyProbe()
		if !has {
			continue
		}
		if err := s.waitOne(ctx, srv, probe, perServer); err != nil {
			return err
		}
		fmt.Fprintf(s.out(), "[modelserve] %s ready\n", srv.Name)
	}
	return nil
}

// waitOne polls until srv is ready (probe, or the HealthURL 200 check when probe
// is nil), the deadline passes, the context is cancelled, or the child exits.
func (s *Supervisor) waitOne(ctx context.Context, srv Server, probe func(context.Context) bool, timeout time.Duration) error {
	if probe == nil {
		probe = func(c context.Context) bool { return s.healthy(c, srv.HealthURL) }
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		// A dead child beats a ready poll: a stale instance on the same port
		// could answer while our process has already crashed.
		select {
		case d := <-s.deaths:
			return fmt.Errorf("modelserve: %s exited during startup: %v", d.name, d.err)
		default:
		}
		if probe(ctx) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("modelserve: %s not healthy within %s", srv.Name, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d := <-s.deaths:
			return fmt.Errorf("modelserve: %s exited during startup: %v", d.name, d.err)
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) healthy(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// Shutdown signals every supervised process group and waits for it to exit,
// escalating to SIGKILL if a process does not stop promptly. Safe to call once.
func (s *Supervisor) Shutdown() {
	s.mu.Lock()
	s.stopping = true
	children := s.children
	s.children = nil
	s.mu.Unlock()

	for _, c := range children {
		if c.cmd.Process != nil {
			_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGTERM) // negative pid == process group
		}
	}
	for _, c := range children {
		if c.cmd.Process == nil {
			continue
		}
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
			<-c.done
		}
	}
}

// Run starts the servers, waits for readiness, then blocks until ctx is
// cancelled or a server exits unexpectedly, tearing everything down on the way
// out.
func (s *Supervisor) Run(ctx context.Context, readyTimeout time.Duration) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	defer s.Shutdown()
	if err := s.WaitHealthy(ctx, readyTimeout); err != nil {
		return err
	}
	fmt.Fprintf(s.out(), "[modelserve] all servers ready\n")
	select {
	case <-ctx.Done():
		return nil
	case d := <-s.deaths:
		return fmt.Errorf("modelserve: %s exited: %v", d.name, d.err)
	}
}

// prefixWriter returns a writer that tags each line with [name].
func prefixWriter(w io.Writer, name string) io.Writer {
	pr, pw := io.Pipe()
	go func() {
		buf := make([]byte, 4096)
		atLineStart := true
		prefix := []byte("[" + name + "] ")
		for {
			n, err := pr.Read(buf)
			for i := 0; i < n; i++ {
				if atLineStart {
					w.Write(prefix)
					atLineStart = false
				}
				w.Write(buf[i : i+1])
				if buf[i] == '\n' {
					atLineStart = true
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
					fmt.Fprintf(w, "[%s] log pipe error: %v\n", name, err)
				}
				return
			}
		}
	}()
	return pw
}
