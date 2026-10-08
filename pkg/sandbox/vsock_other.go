//go:build !linux

package sandbox

import (
	"errors"
	"net"
)

// Listen is unavailable off Linux: AF_VSOCK exists only inside the MicroVM's
// Linux guest. The daemon still builds and unit-tests on other platforms via
// ServeConn / ServeListener (e.g. over TCP with DAEMON_TCP), so the host can run
// the command runner and HTTP framing without a VM.
func Listen(port uint32) (net.Listener, error) {
	return nil, errors.New("sandbox: vsock listener is only supported on linux (run inside the MicroVM, or set DAEMON_TCP)")
}

// DialHost is unavailable off Linux: AF_VSOCK exists only inside the MicroVM's
// Linux guest. Host-side development reaches the egress broker over TCP instead
// (set BROKER_TCP), so the in-VM tool and its tests build and run anywhere.
func DialHost(port uint32) (net.Conn, error) {
	return nil, errors.New("sandbox: vsock dial is only supported on linux (run inside the MicroVM, or set BROKER_TCP)")
}
