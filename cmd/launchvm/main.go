// Command launchvm boots the PUI PUI Linux MicroVM (Apple Virtualization
// Framework) that serves as the detonation chamber, and bridges host
// 127.0.0.1:5000 to the guest's vsock:5000 so the control plane can reach the
// in-VM detonation daemon with no IP route into the host.
//
// The Apple Virtualization Framework requires the running binary to be codesigned
// with the com.apple.security.virtualization entitlement, so `go run` fails. Build
// then sign:
//
//	go build -o launchvm ./cmd/launchvm
//	codesign --entitlements set-up/entitlements.plist -s - --force launchvm
//	./launchvm            # -dir defaults to set-up/vm-assets
//
// The in-guest bring-up runs sandbox_init.sh, which launches the static Go
// cmd/detonationd (cross-built into the shared assets mount by prepareassets) on
// vsock:5000 — no python in the guest.
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/t0ul/ai-security-engineering/broker"
	"github.com/t0ul/ai-security-engineering/internal/modelserve"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"golang.org/x/term"
)

const (
	vsockPort      = 5000
	hostBridgeAddr = "127.0.0.1:5000"
)

func main() {
	dir := flag.String("dir", "set-up/vm-assets", "MicroVM assets directory")
	allow := flag.String("allow", "", "comma-separated egress allowlist for the in-VM broker (default: deny all)")
	flag.Parse()

	var allowHosts []string
	for _, h := range strings.Split(*allow, ",") {
		if h = strings.TrimSpace(h); h != "" {
			allowHosts = append(allowHosts, h)
		}
	}
	egressPolicy := netpolicy.Policy{Allow: allowHosts}

	kernelPath := filepath.Join(*dir, "Image")
	initrdPath := filepath.Join(*dir, "initramfs.cpio.gz")

	bootLoader, err := vz.NewLinuxBootLoader(kernelPath, vz.WithCommandLine("console=hvc0"), vz.WithInitrd(initrdPath))
	if err != nil {
		log.Fatalf("launchvm: bootloader: %v", err)
	}
	config, err := vz.NewVirtualMachineConfiguration(bootLoader, 4, 2*1024*1024*1024)
	if err != nil {
		log.Fatalf("launchvm: vm config: %v", err)
	}

	vmReader, vmWriter, _ := os.Pipe()
	hostReader, hostWriter, _ := os.Pipe()

	serialAttachment, err := vz.NewFileHandleSerialPortAttachment(vmReader, hostWriter)
	if err != nil {
		log.Fatalf("launchvm: serial attachment: %v", err)
	}
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAttachment)
	if err != nil {
		log.Fatalf("launchvm: console config: %v", err)
	}
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{consoleConfig})

	sharedDir, err := vz.NewSharedDirectory(*dir, true)
	if err != nil {
		log.Fatalf("launchvm: shared dir: %v", err)
	}
	dirShare, err := vz.NewSingleDirectoryShare(sharedDir)
	if err != nil {
		log.Fatalf("launchvm: dir share: %v", err)
	}
	fsConfig, err := vz.NewVirtioFileSystemDeviceConfiguration("assets")
	if err != nil {
		log.Fatalf("launchvm: fs config: %v", err)
	}
	fsConfig.SetDirectoryShare(dirShare)
	config.SetDirectorySharingDevicesVirtualMachineConfiguration([]vz.DirectorySharingDeviceConfiguration{fsConfig})

	// NO network device is attached on purpose: the detonation chamber is
	// egress-denied. A command detonated inside the guest has no interface, no
	// route, and no DNS, so it cannot beacon or exfiltrate. Any network a tool
	// legitimately needs goes out through the host netpolicy proxy (allowlist +
	// dial-pinned + no-IMDS), never the guest's own stack. vsock below is the one
	// and only channel in or out.

	// virtio-vsock: the ONLY channel into the detonation chamber.
	vsockConfig, err := vz.NewVirtioSocketDeviceConfiguration()
	if err != nil {
		log.Fatalf("launchvm: vsock config: %v", err)
	}
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{vsockConfig})

	if valid, err := config.Validate(); !valid || err != nil {
		log.Fatalf("launchvm: invalid vm config: %v", err)
	}
	vm, err := vz.NewVirtualMachine(config)
	if err != nil {
		log.Fatalf("launchvm: instantiate: %v", err)
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("launchvm: raw terminal: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	os.Stdout.Write([]byte("\r\nBooting PUI PUI Linux MicroVM...\r\n"))
	if err := vm.Start(); err != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		log.Fatalf("\r\nlaunchvm: start: %v\r\n", err)
	}

	go startVsockBridge(vm)
	go startEgressBroker(vm, egressPolicy)
	go driveConsole(hostReader, vmWriter)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	os.Stdout.Write([]byte("\r\nTerminating MicroVM...\r\n"))
	_, _ = vm.RequestStop()
}

// driveConsole watches the guest console and scripts the login + bring-up.
func driveConsole(hostReader io.Reader, vmWriter io.Writer) {
	buf := make([]byte, 1024)
	var stream string
	state := 0
	for {
		n, err := hostReader.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			stream += string(buf[:n])
			switch state {
			case 0:
				if strings.Contains(stream, "login:") {
					time.Sleep(100 * time.Millisecond)
					vmWriter.Write([]byte("root\n"))
					stream, state = "", 1
				}
			case 1:
				if strings.Contains(strings.ToLower(stream), "password:") {
					time.Sleep(100 * time.Millisecond)
					vmWriter.Write([]byte("passwd\n"))
					stream, state = "", 2
				}
			case 2:
				if strings.Contains(stream, "#") {
					time.Sleep(300 * time.Millisecond)
					vmWriter.Write([]byte(bringUpCmd))
					go io.Copy(vmWriter, os.Stdin)
					stream, state = "", 3
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// bringUpCmd mounts the shared assets, chroots into Alpine, and runs
// sandbox_init.sh — which launches the static Go detonationd and exec's the
// interactive shell. The daemon binary is cross-compiled into the shared assets
// dir by prepareassets. No networking is brought up (no `ip link`/`udhcpc`) and
// no `apk`/DNS is configured: the chamber is egress-denied by construction, and
// since Python is gone the guest needs nothing from the internet.
const bringUpCmd = "mkdir -p /mnt/assets && mount -t virtiofs assets /mnt/assets && cp -a /mnt/assets/alpine-root /alpine && mkdir -p /alpine/mnt/assets && mount -o bind /mnt/assets /alpine/mnt/assets && mount -t proc none /alpine/proc && mount -t sysfs none /alpine/sys && mount -o bind /dev /alpine/dev && chroot /alpine /bin/sh /mnt/assets/sandbox_init.sh\n"

// startVsockBridge accepts TCP on host loopback and pipes each connection to the
// guest's vsock listener, exposing a plain 127.0.0.1:5000 endpoint backed by the
// isolated MicroVM.
func startVsockBridge(vm *vz.VirtualMachine) {
	// Reclaim the fixed bridge port from any leftover launchvm so a re-run never
	// binds a stale VM (same pattern as modeld's model ports).
	modelserve.ReclaimPort(vsockPort, os.Stdout)
	ln, err := net.Listen("tcp", hostBridgeAddr)
	if err != nil {
		log.Printf("vsock bridge: listen %s: %v", hostBridgeAddr, err)
		return
	}
	os.Stdout.Write([]byte("\r\nvsock bridge up: 127.0.0.1:5000 -> guest vsock:5000\r\n"))
	for {
		tcpConn, err := ln.Accept()
		if err != nil {
			continue
		}
		go bridgeConn(vm, tcpConn)
	}
}

// startEgressBroker listens on the guest-initiated vsock port and gates every
// outbound connection the in-VM tools attempt through netpolicy. The guest has
// NO network device (egress-denied), so this host-side broker is its only path
// out — and a single, audited, allow-listed one. Byte tunneling and policy live
// in the broker package; this just wires the vz vsock listener to it.
func startEgressBroker(vm *vz.VirtualMachine, pol netpolicy.Policy) {
	devices := vm.SocketDevices()
	if len(devices) == 0 {
		return
	}
	ln, err := devices[0].Listen(broker.DefaultPort)
	if err != nil {
		log.Printf("egress broker: listen vsock:%d: %v", broker.DefaultPort, err)
		return
	}
	b := &broker.Broker{Policy: pol, Log: func(target string, allowed bool, reason string) {
		if allowed {
			os.Stdout.Write([]byte("\regress ALLOW " + target + "\r\n"))
		} else {
			os.Stdout.Write([]byte("\regress DENY  " + target + " (" + reason + ")\r\n"))
		}
	}}
	os.Stdout.Write([]byte("\r\negress broker up: guest vsock:5001 -> host netpolicy\r\n"))
	_ = b.Serve(ln)
}

func bridgeConn(vm *vz.VirtualMachine, tcpConn net.Conn) {
	defer tcpConn.Close()
	devices := vm.SocketDevices()
	if len(devices) == 0 {
		return
	}
	vsockConn, err := devices[0].Connect(vsockPort)
	if err != nil {
		return
	}
	defer vsockConn.Close()
	done := make(chan struct{}, 2)
	go func() { io.Copy(vsockConn, tcpConn); done <- struct{}{} }()
	go func() { io.Copy(tcpConn, vsockConn); done <- struct{}{} }()
	<-done
}
