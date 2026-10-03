package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var GGUFMagic = []byte{'G', 'G', 'U', 'F'}

func main() {
	assetsDir := "./vm-assets"
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		log.Fatalf("Failed to create assets directory: %v", err)
	}

	fmt.Println("🚀 Starting MicroVM Asset Preparation...")

	arch := runtime.GOARCH
	if arch != "arm64" {
		log.Fatalf("❌ This lab requires Apple Silicon (arm64).")
	}

	// 1. PUI PUI LINUX KERNEL
	tarName := "puipui_linux_v1.0.3_aarch64.tar.gz"
	downloadURL := fmt.Sprintf("https://github.com/Code-Hex/puipui-linux/releases/download/v1.0.3/%s", tarName)
	tarPath := filepath.Join(assetsDir, tarName)

	if _, err := os.Stat(filepath.Join(assetsDir, "Image")); err == nil {
		fmt.Println("✔ PUI PUI Linux kernel already extracted.")
	} else {
		fmt.Printf("\n📥 Downloading PUI PUI Linux...\n")
		if err := downloadFile(downloadURL, tarPath); err != nil {
			log.Fatalf("❌ Download failed: %v", err)
		}
		if err := extractTarGz(tarPath, assetsDir); err != nil {
			log.Fatalf("❌ Extraction failed: %v", err)
		}
		os.Remove(tarPath)

		gzPath := filepath.Join(assetsDir, "Image.gz")
		imagePath := filepath.Join(assetsDir, "Image")
		if err := gunzipFile(gzPath, imagePath); err != nil {
			log.Fatalf("❌ Kernel decompression failed: %v", err)
		}
		os.Remove(gzPath)
	}

	// 2. ALPINE MINI ROOTFS (Container OS)
	alpineDir := filepath.Join(assetsDir, "alpine-root")
	if _, err := os.Stat(alpineDir); err == nil {
		fmt.Println("✔ Alpine Mini RootFS already exists.")
	} else {
		fmt.Println("\n📥 Downloading Alpine Mini RootFS (Container Space)...")
		os.MkdirAll(alpineDir, 0755)
		rootfsTar := filepath.Join(assetsDir, "alpine-rootfs.tar.gz")
		rootfsURL := "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/aarch64/alpine-minirootfs-3.20.3-aarch64.tar.gz"
		
		curlCmd := exec.Command("curl", "-f", "-L", "-#", "-o", rootfsTar, rootfsURL)
		curlCmd.Stdout = os.Stdout
		curlCmd.Stderr = os.Stderr
		if err := curlCmd.Run(); err != nil {
			log.Fatalf("❌ RootFS download failed: %v", err)
		}
		
		fmt.Println("📦 Extracting Alpine RootFS...")
		tarCmd := exec.Command("tar", "-xzf", rootfsTar, "-C", alpineDir)
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			log.Fatalf("❌ RootFS extraction failed: %v", err)
		}
		os.Remove(rootfsTar)
	}

	// 3. QWEN 1.5B GGUF MODEL
	ggufDest := filepath.Join(assetsDir, "qwen1.5b.gguf")
	if fi, err := os.Stat(ggufDest); err == nil && fi.Size() > 0 {
		fmt.Println("\n✔ Qwen GGUF Model already exists.")
	} else {
		ggufURL := "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
		fmt.Printf("\n📥 Downloading Qwen 1.5B GGUF...\n")
		if err := fetchAndVerifyGGUF(ggufURL, ggufDest); err != nil {
			log.Fatalf("❌ Model download failed: %v", err)
		}
	}

	fmt.Println("\n✨ All MicroVM assets prepared successfully!")
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil { return err }
	defer resp.Body.Close()
	out, err := os.Create(dest)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(tarGzPath, destDir string) error {
	file, err := os.Open(tarGzPath)
	if err != nil { return err }
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil { return err }
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF { break }
		if err != nil { return err }
		target := filepath.Join(destDir, header.Name)
		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0755)
		} else if header.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil { return err }
			io.Copy(out, tr)
			out.Close()
		}
	}
	return nil
}

func gunzipFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	gzr, err := gzip.NewReader(in)
	if err != nil { return err }
	defer gzr.Close()
	out, err := os.Create(dest)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, gzr)
	return err
}

func fetchAndVerifyGGUF(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil { return err }
	defer resp.Body.Close()
	out, err := os.Create(destPath)
	if err != nil { return err }
	defer out.Close()
	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)
	io.Copy(writer, resp.Body)
	file, _ := os.Open(destPath)
	defer file.Close()
	header := make([]byte, 4)
	io.ReadFull(file, header)
	if !bytes.Equal(header, GGUFMagic) {
		os.Remove(destPath)
		return errors.New("invalid GGUF header")
	}
	return nil
}