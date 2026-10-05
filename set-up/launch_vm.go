package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Code-Hex/vz/v3"
	"golang.org/x/term"
)

func main() {
	kernelPath := "./vm-assets/Image"
	initrdPath := "./vm-assets/initramfs.cpio.gz"
	bootCmd := "console=hvc0"

	bootLoader, err := vz.NewLinuxBootLoader(kernelPath, vz.WithCommandLine(bootCmd), vz.WithInitrd(initrdPath))
	if err != nil { log.Fatalf("Failed bootloader init: %v", err) }

	config, err := vz.NewVirtualMachineConfiguration(bootLoader, 4, 2*1024*1024*1024)
	if err != nil { log.Fatalf("Failed VM config: %v", err) }

	vmReader, vmWriter, err := os.Pipe()
	hostReader, hostWriter, err := os.Pipe()
	
	serialAttachment, err := vz.NewFileHandleSerialPortAttachment(vmReader, hostWriter)
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{consoleConfig})

	sharedDir, err := vz.NewSharedDirectory("./vm-assets", true)
	dirShare, err := vz.NewSingleDirectoryShare(sharedDir)
	fsConfig, err := vz.NewVirtioFileSystemDeviceConfiguration("assets")
	fsConfig.SetDirectoryShare(dirShare)
	config.SetDirectorySharingDevicesVirtualMachineConfiguration([]vz.DirectorySharingDeviceConfiguration{fsConfig})

	nat, err := vz.NewNATNetworkDeviceAttachment()
	netConfig, err := vz.NewVirtioNetworkDeviceConfiguration(nat)
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netConfig})

	// virtio-vsock: the ONLY inbound channel into the detonation chamber.
	// The macOS control plane validates a command, then reaches the in-VM
	// daemon over this device (bridged to 127.0.0.1:5000 after boot). The
	// sandbox needs no IP route to the host, so isolation is preserved.
	vsockConfig, err := vz.NewVirtioSocketDeviceConfiguration()
	if err != nil {
		log.Fatalf("Failed vsock config: %v", err)
	}
	config.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{vsockConfig})

	if valid, err := config.Validate(); !valid || err != nil {
		log.Fatalf("Invalid VM config: %v", err)
	}

	vm, err := vz.NewVirtualMachine(config)
	if err != nil { log.Fatalf("Failed to instantiate MicroVM: %v", err) }

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	os.Stdout.Write([]byte("\r\n⚡ Booting PUI PUI Linux MicroVM...\r\n"))
	if err := vm.Start(); err != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		log.Fatalf("\r\nFailed to start MicroVM: %v\r\n", err)
	}

	// Bridge host 127.0.0.1:5000 <-> guest vsock:5000 so the existing
	// control plane (camel_interpreter.py -> http://127.0.0.1:5000) reaches
	// the in-VM detonation daemon with no contract change.
	go startVsockBridge(vm)

	go func() {
		buf := make([]byte, 1024)
		var streamStr string
		state := 0 
		for {
			n, err := hostReader.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				streamStr += string(buf[:n])

				switch state {
				case 0:
					if strings.Contains(streamStr, "login:") {
						time.Sleep(100 * time.Millisecond)
						vmWriter.Write([]byte("root\n"))
						streamStr = ""
						state = 1
					}
				case 1:
					if strings.Contains(strings.ToLower(streamStr), "password:") {
						time.Sleep(100 * time.Millisecond)
						vmWriter.Write([]byte("passwd\n"))
						streamStr = ""
						state = 2
					}
				case 2:
					if strings.Contains(streamStr, "#") {
						time.Sleep(300 * time.Millisecond)
						
						// Appended: sh /mnt/assets/sandbox_init.sh
						setupCmd := "ip link set eth0 up && udhcpc -i eth0 && mkdir -p /mnt/assets && mount -t virtiofs assets /mnt/assets && cp -a /mnt/assets/alpine-root /alpine && mkdir -p /alpine/mnt/assets && mount -o bind /mnt/assets /alpine/mnt/assets && mount -t proc none /alpine/proc && mount -t sysfs none /alpine/sys && mount -o bind /dev /alpine/dev && rm -f /alpine/etc/resolv.conf && cp /etc/resolv.conf /alpine/etc/resolv.conf && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.20/main' > /alpine/etc/apk/repositories && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.20/community' >> /alpine/etc/apk/repositories && chroot /alpine /bin/sh -c 'apk update && apk add python3 py3-pip && sh /mnt/assets/sandbox_init.sh' && echo '\n🚀 Interactive Shell Ready.' && chroot /alpine /bin/sh\n"
						vmWriter.Write([]byte(setupCmd))
						
						go io.Copy(vmWriter, os.Stdin)
						streamStr = ""
						state = 3
					}
				}
			}
			if err != nil { break }
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	os.Stdout.Write([]byte("\r\nTerminating MicroVM...\r\n"))
	_, _ = vm.RequestStop()
}

const (
	vsockPort      = 5000
	hostBridgeAddr = "127.0.0.1:5000"
)

// startVsockBridge accepts TCP on the host loopback and pipes each connection
// to the guest's vsock listener, giving the control plane a plain
// localhost:5000 HTTP endpoint backed entirely by the isolated MicroVM.
func startVsockBridge(vm *vz.VirtualMachine) {
	ln, err := net.Listen("tcp", hostBridgeAddr)
	if err != nil {
		log.Printf("vsock bridge: listen %s failed: %v", hostBridgeAddr, err)
		return
	}
	os.Stdout.Write([]byte("\r\n\U0001f309 vsock bridge up: 127.0.0.1:5000 -> guest vsock:5000\r\n"))
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
	// Open a fresh vsock connection to the guest daemon for this request.
	vsockConn, err := devices[0].Connect(vsockPort)
	if err != nil {
		// Guest daemon not up yet, or port closed; caller sees a dropped conn.
		return
	}
	defer vsockConn.Close()
	done := make(chan struct{}, 2)
	go func() { io.Copy(vsockConn, tcpConn); done <- struct{}{} }()
	go func() { io.Copy(tcpConn, vsockConn); done <- struct{}{} }()
	<-done
}
