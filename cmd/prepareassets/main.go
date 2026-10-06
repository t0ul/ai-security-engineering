// Command prepareassets provisions the MicroVM assets: the PUI PUI Linux kernel,
// the Alpine mini root filesystem, and the two GGUF models (Llama-3.2-3B planner,
// Qwen-1.5B coder). Models are fetched through the verified-download path
// (extension policy + GGUF magic check), the M10 supply-chain control.
//
//	prepareassets [-dir set-up/vm-assets]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/t0ul/ai-security-engineering/internal/assets"
)

const (
	kernelURL = "https://github.com/Code-Hex/puipui-linux/releases/download/v1.0.3/puipui_linux_v1.0.3_aarch64.tar.gz"
	rootfsURL = "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/aarch64/alpine-minirootfs-3.20.3-aarch64.tar.gz"
	qwenURL   = "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
	llamaURL  = "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf"
)

func main() {
	dir := flag.String("dir", "set-up/vm-assets", "asset output directory")
	flag.Parse()

	if runtime.GOARCH != "arm64" {
		log.Fatal("prepareassets: this lab requires Apple Silicon (arm64)")
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatalf("prepareassets: %v", err)
	}

	steps := []func(string) error{prepareKernel, prepareRootFS, prepareModels, buildDetonationd}
	for _, step := range steps {
		if err := step(*dir); err != nil {
			log.Fatalf("prepareassets: %v", err)
		}
	}
	fmt.Println("all MicroVM assets prepared.")
}

func prepareKernel(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "Image")); err == nil {
		fmt.Println("kernel: already present")
		return nil
	}
	fmt.Println("kernel: downloading PUI PUI Linux...")
	tarPath := filepath.Join(dir, "puipui.tar.gz")
	if err := assets.DownloadFile(kernelURL, tarPath); err != nil {
		return err
	}
	defer os.Remove(tarPath)
	if err := assets.ExtractTarGz(tarPath, dir); err != nil {
		return err
	}
	gz := filepath.Join(dir, "Image.gz")
	if _, err := os.Stat(gz); err == nil {
		if err := assets.GunzipFile(gz, filepath.Join(dir, "Image")); err != nil {
			return err
		}
		os.Remove(gz)
	}
	return nil
}

func prepareRootFS(dir string) error {
	alpineDir := filepath.Join(dir, "alpine-root")
	if _, err := os.Stat(alpineDir); err == nil {
		fmt.Println("rootfs: already present")
		return nil
	}
	fmt.Println("rootfs: downloading Alpine mini root filesystem...")
	if err := os.MkdirAll(alpineDir, 0o755); err != nil {
		return err
	}
	tarPath := filepath.Join(dir, "alpine-rootfs.tar.gz")
	if err := assets.DownloadFile(rootfsURL, tarPath); err != nil {
		return err
	}
	defer os.Remove(tarPath)
	// Alpine's rootfs tar carries device nodes and ownership the Go tar reader
	// cannot recreate without root, so shell out to the system tar here.
	cmd := exec.Command("tar", "-xzf", tarPath, "-C", alpineDir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// buildDetonationd cross-compiles the in-guest detonation daemon into the shared
// assets dir. The VM is Alpine on Apple Silicon (linux/arm64); the daemon is
// pure Go (CGO off), so it runs as a static binary with no runtime deps.
func buildDetonationd(dir string) error {
	dest := filepath.Join(dir, "detonationd")
	fmt.Println("daemon: cross-compiling detonationd (linux/arm64, static)...")
	cmd := exec.Command("go", "build", "-o", dest, "./cmd/detonationd")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func prepareModels(dir string) error {
	models := []struct {
		name, url, file string
	}{
		{"Qwen-1.5B coder", qwenURL, "qwen1.5b.gguf"},
		{"Llama-3.2-3B planner", llamaURL, "llama-3.2-3b.gguf"},
	}
	for _, m := range models {
		dest := filepath.Join(dir, m.file)
		if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
			fmt.Printf("model: %s already present\n", m.name)
			continue
		}
		fmt.Printf("model: downloading %s (verified)...\n", m.name)
		if err := assets.DownloadAndVerify(m.url, dest, ""); err != nil {
			return err
		}
	}
	return nil
}
