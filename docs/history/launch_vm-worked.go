//go:build ignore

// This is a retained "worked" scratch snapshot, excluded from the module build
// by the ignore tag. The maintained version is cmd/launchvm.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"golang.org/x/term"
)

func main() {
	kernelPath := "./vm-assets/Image"
	initrdPath := "./vm-assets/initramfs.cpio.gz"

	// 1. PUI PUI Linux natively handles console=hvc0 without kernel panics
	bootCmd := "console=hvc0"

	bootLoader, err := vz.NewLinuxBootLoader(
		kernelPath,
		vz.WithCommandLine(bootCmd),
		vz.WithInitrd(initrdPath),
	)
	if err != nil {
		log.Fatalf("Failed bootloader initialization: %v", err)
	}

	config, err := vz.NewVirtualMachineConfiguration(bootLoader, 4, 4*1024*1024*1024)
	if err != nil {
		log.Fatalf("Failed VM config: %v", err)
	}

	// 2. Direct Terminal Serial Attachment
	serialAttachment, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to create serial attachment: %v", err)
	}
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialAttachment)
	if err != nil {
		log.Fatalf("Failed console device config: %v", err)
	}
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{consoleConfig})

	// 3. VirtioFS Shared Directory (Exposing your Qwen model)
	sharedDir, err := vz.NewSharedDirectory("./vm-assets", true)
	if err != nil {
		log.Fatalf("Failed to create shared directory: %v", err)
	}
	dirShare, err := vz.NewSingleDirectoryShare(sharedDir)
	if err != nil {
		log.Fatalf("Failed to create directory share: %v", err)
	}
	fsConfig, err := vz.NewVirtioFileSystemDeviceConfiguration("assets")
	if err != nil {
		log.Fatalf("Failed filesystem device config: %v", err)
	}
	fsConfig.SetDirectoryShare(dirShare)
	config.SetDirectorySharingDevicesVirtualMachineConfiguration([]vz.DirectorySharingDeviceConfiguration{fsConfig})

	// 4. NAT Network Device
	nat, err := vz.NewNATNetworkDeviceAttachment()
	if err != nil {
		log.Fatalf("Failed to create NAT attachment: %v", err)
	}
	netConfig, err := vz.NewVirtioNetworkDeviceConfiguration(nat)
	if err != nil {
		log.Fatalf("Failed network device configuration: %v", err)
	}
	config.SetNetworkDevicesVirtualMachineConfiguration([]*vz.VirtioNetworkDeviceConfiguration{netConfig})

	if valid, err := config.Validate(); !valid || err != nil {
		log.Fatalf("Invalid VM configuration: %v", err)
	}

	vm, err := vz.NewVirtualMachine(config)
	if err != nil {
		log.Fatalf("Failed to instantiate MicroVM: %v", err)
	}

	// 5. Lock macOS terminal into raw mode (Bypassing local buffering)
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to enter raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	os.Stdout.Write([]byte("\r\n⚡ Booting PUI PUI Linux MicroVM...\r\n"))
	if err := vm.Start(); err != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		log.Fatalf("\r\nFailed to start MicroVM: %v\r\n", err)
	}

	os.Stdout.Write([]byte("MicroVM online. Waiting for shell...\r\n"))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	os.Stdout.Write([]byte("\r\nTerminating MicroVM...\r\n"))
	_, _ = vm.RequestStop()
}