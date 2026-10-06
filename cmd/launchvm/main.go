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
	"golang.org/x/term"
)

const (
	vsockPort      = 5000
	hostBridgeAddr = "127.0.0.1:5000"
)

func main() {
	dir := flag.String("dir", "set-up/vm-assets", "MicroVM assets directory")
	flag.Parse()

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

	nat, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		log.Fatalf("launchvm: nat: %v", err)
	}
	netConfig, err := vz.NewVirtioNetworkDeviceConfiguration(nat)
	if err != nil {
		log.Fatalf("launchvm: net config: %v", err)
	}
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netConfig})

	// virtio-vsock: the ONLY inbound channel into the detonation chamber.
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

// bringUpCmd configures networking, mounts the shared assets, chroots into
// Alpine, and runs sandbox_init.sh — which launches the static Go detonationd
// (no apk/python needed) and exec's the interactive shell. The daemon binary is
// cross-compiled into the shared assets dir by prepareassets.
const bringUpCmd = "ip link set eth0 up && udhcpc -i eth0 && mkdir -p /mnt/assets && mount -t virtiofs assets /mnt/assets && cp -a /mnt/assets/alpine-root /alpine && mkdir -p /alpine/mnt/assets && mount -o bind /mnt/assets /alpine/mnt/assets && mount -t proc none /alpine/proc && mount -t sysfs none /alpine/sys && mount -o bind /dev /alpine/dev && rm -f /alpine/etc/resolv.conf && cp /etc/resolv.conf /alpine/etc/resolv.conf && chroot /alpine /bin/sh /mnt/assets/sandbox_init.sh\n"

// startVsockBridge accepts TCP on host loopback and pipes each connection to the
// guest's vsock listener, exposing a plain 127.0.0.1:5000 endpoint backed by the
// isolated MicroVM.
func startVsockBridge(vm *vz.VirtualMachine) {
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
