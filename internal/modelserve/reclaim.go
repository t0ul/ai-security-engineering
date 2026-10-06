package modelserve

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// ReclaimPort terminates any process listening on the given TCP port. It is
// best-effort (uses lsof; a no-op if lsof is missing or the port is free) and
// is meant for test harnesses that own fixed ports, so a crashed or leftover
// server never blocks the next run. logw, if non-nil, receives a line per kill.
func ReclaimPort(port int, logw io.Writer) {
	out, err := exec.Command("lsof", "-ti", "tcp:"+strconv.Itoa(port), "-sTCP:LISTEN").Output()
	if err != nil {
		return // lsof absent, or no listener on the port
	}
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 1 {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil && logw != nil {
			fmt.Fprintf(logw, "[modelserve] reclaimed port %d (killed pid %d)\n", port, pid)
		}
	}
}
