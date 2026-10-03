package main

import (
	"io"
	"log"
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
						
						// Injects HTTP repos to bypass the SSL clock/certificate errors
						setupCmd := "ip link set eth0 up && udhcpc -i eth0 && mkdir -p /mnt/assets && mount -t virtiofs assets /mnt/assets && cp -a /mnt/assets/alpine-root /alpine && mkdir -p /alpine/mnt/assets && mount -o bind /mnt/assets /alpine/mnt/assets && mount -t proc none /alpine/proc && mount -t sysfs none /alpine/sys && mount -o bind /dev /alpine/dev && rm -f /alpine/etc/resolv.conf && cp /etc/resolv.conf /alpine/etc/resolv.conf && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.20/main' > /alpine/etc/apk/repositories && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.20/community' >> /alpine/etc/apk/repositories && chroot /alpine /bin/sh -c 'apk update && apk add python3 py3-pip' && clear && echo '🚀 Python Sandbox Ready.' && chroot /alpine /bin/sh\n"
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