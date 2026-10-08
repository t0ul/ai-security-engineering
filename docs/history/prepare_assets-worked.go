//go:build ignore

// This is a retained "worked" scratch snapshot, excluded from the module build
// by the ignore tag. The maintained version is cmd/prepareassets.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

	// --- 1. PUI PUI LINUX ASSETS ---
	arch := runtime.GOARCH
	kernelArch := arch
	if arch == "arm64" {
		kernelArch = "aarch64"
	}

	version := "1.0.3"
	tarName := fmt.Sprintf("puipui_linux_v%s_%s.tar.gz", version, kernelArch)
	downloadURL := fmt.Sprintf("https://github.com/Code-Hex/puipui-linux/releases/download/v%s/%s", version, tarName)
	tarPath := filepath.Join(assetsDir, tarName)

	if _, err := os.Stat(filepath.Join(assetsDir, "Image")); err == nil {
		fmt.Println("✔ PUI PUI Linux kernel already extracted. Skipping download.")
	} else {
		fmt.Printf("\n📥 Downloading PUI PUI Linux from:\n   %s\n", downloadURL)
		if err := downloadFile(downloadURL, tarPath); err != nil {
			log.Fatalf("❌ PUI PUI Linux download failed: %v", err)
		}

		fmt.Println("📦 Extracting archive...")
		if err := extractTarGz(tarPath, assetsDir); err != nil {
			log.Fatalf("❌ Extraction failed: %v", err)
		}
		_ = os.Remove(tarPath) // Clean up tarball

		// Decompress Image.gz for ARM64
		if arch == "arm64" {
			gzPath := filepath.Join(assetsDir, "Image.gz")
			imagePath := filepath.Join(assetsDir, "Image")
			fmt.Println("⚙ Decompressing Image.gz...")
			if err := gunzipFile(gzPath, imagePath); err != nil {
				log.Fatalf("❌ Kernel decompression failed: %v", err)
			}
			_ = os.Remove(gzPath) // Clean up compressed image
		}
	}

	// --- 2. QWEN 1.5B GGUF MODEL ---
	ggufDest := filepath.Join(assetsDir, "qwen1.5b.gguf")
	if fi, err := os.Stat(ggufDest); err == nil && fi.Size() > 0 {
		fmt.Printf("\n✔ Qwen GGUF Model already exists at %s. Skipping download.\n", ggufDest)
	} else {
		ggufURL := "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
		fmt.Printf("\n📥 Downloading Qwen 1.5B GGUF Model...\n   URL: %s\n", ggufURL)
		if err := fetchAndVerifyGGUF(ggufURL, ggufDest); err != nil {
			log.Fatalf("❌ Model preparation failed: %v", err)
		}
	}

	fmt.Println("\n✨ All MicroVM assets prepared successfully!")
}

// downloadFile streams an HTTP GET request to a local file
func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractTarGz extracts files from a tar.gz archive
func extractTarGz(tarGzPath, destDir string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

// gunzipFile decompresses a gzip file
func gunzipFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	gzr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gzr.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, gzr)
	return err
}

// fetchAndVerifyGGUF downloads a file, prints its SHA-256, and verifies the GGUF header
func fetchAndVerifyGGUF(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer out.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	if _, err = io.Copy(writer, resp.Body); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("download streaming failed: %w", err)
	}

	fmt.Printf("   🔒 SHA-256: %s\n", hex.EncodeToString(hasher.Sum(nil)))

	// Verify GGUF Magic Header
	file, err := os.Open(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("unable to read header: %w", err)
	}

	if !bytes.Equal(header, GGUFMagic) {
		_ = os.Remove(destPath)
		return errors.New("invalid header: file is not a valid GGUF binary")
	}
	
	fmt.Println("   ✔ Valid GGUF header verified")
	return nil
}