//go:build linux

package sandbox

import (
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// Listen opens an AF_VSOCK stream listener on port, accepting from any CID. The
// returned net.Listener yields vsock connections usable by ServeListener. Go's
// net package has no vsock support, so the listener and connection are built on
// the raw syscalls in golang.org/x/sys/unix.
func Listen(port uint32) (net.Listener, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("sandbox: vsock socket: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: port}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("sandbox: vsock bind: %w", err)
	}
	if err := unix.Listen(fd, 8); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("sandbox: vsock listen: %w", err)
	}
	return &vsockListener{fd: fd, port: port}, nil
}

// DialHost opens an AF_VSOCK connection from the guest to the host (CID 2) on
// port. The egress broker runs on the host; this is how an in-VM tool reaches it
// without the guest having any IP network of its own.
func DialHost(port uint32) (net.Conn, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("sandbox: vsock socket: %w", err)
	}
	if err := unix.Connect(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: port}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("sandbox: vsock connect host:%d: %w", port, err)
	}
	return &vsockConn{fd: fd}, nil
}

type vsockAddr struct {
	cid  uint32
	port uint32
}

func (vsockAddr) Network() string  { return "vsock" }
func (a vsockAddr) String() string { return fmt.Sprintf("vsock:%d.%d", a.cid, a.port) }

type vsockListener struct {
	fd   int
	port uint32
}

func (l *vsockListener) Accept() (net.Conn, error) {
	nfd, _, err := unix.Accept(l.fd)
	if err != nil {
		return nil, err
	}
	return &vsockConn{fd: nfd}, nil
}

func (l *vsockListener) Close() error   { return unix.Close(l.fd) }
func (l *vsockListener) Addr() net.Addr { return vsockAddr{cid: unix.VMADDR_CID_ANY, port: l.port} }

// vsockConn adapts a raw vsock fd to net.Conn. ServeConn hand-rolls HTTP framing
// and sets no deadlines, so the deadline methods are no-ops.
type vsockConn struct{ fd int }

func (c *vsockConn) Read(p []byte) (int, error) {
	n, err := unix.Read(c.fd, p)
	if n == 0 && err == nil {
		return 0, io.EOF
	}
	return n, err
}

func (c *vsockConn) Write(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		n, err := unix.Write(c.fd, p[total:])
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
		total += n
	}
	return total, nil
}

func (c *vsockConn) Close() error                     { return unix.Close(c.fd) }
func (c *vsockConn) LocalAddr() net.Addr              { return vsockAddr{} }
func (c *vsockConn) RemoteAddr() net.Addr             { return vsockAddr{} }
func (c *vsockConn) SetDeadline(time.Time) error      { return nil }
func (c *vsockConn) SetReadDeadline(time.Time) error  { return nil }
func (c *vsockConn) SetWriteDeadline(time.Time) error { return nil }
