package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// GGUF Magic Header bytes in ASCII ("GGUF")
var GGUFMagic = []byte{'G', 'G', 'U', 'F'}

// DownloadAndVerify fetches a model file, computes SHA256 on the fly, and validates its magic bytes.
func DownloadAndVerify(url string, destPath string, expectedSHA256 string) error {
	// 1. Enforce file extension security rule
	if !strings.HasSuffix(url, ".gguf") {
		return errors.New("security policy violation: only .gguf models are permitted")
	}

	fmt.Printf("📥 Streaming model from: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	// 2. Prepare local file destination
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	// 3. Set up dual writer to write to disk AND compute SHA-256 simultaneously
	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("error during file streaming: %w", err)
	}

	// 4. Validate SHA-256 Hash
	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	fmt.Printf("🔒 Computed SHA-256: %s\n", actualSHA256)

	if expectedSHA256 != "" && !strings.EqualFold(actualSHA256, expectedSHA256) {
		_ = os.Remove(destPath) // Delete malicious/corrupted artifact
		return fmt.Errorf("HASH MISMATCH! Expected: %s, Got: %s", expectedSHA256, actualSHA256)
	}

	// 5. Scan binary headers for GGUF Magic Bytes
	if err := verifyGGUFHeader(destPath); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("binary header scan failed: %w", err)
	}

	fmt.Println(" File passed all security verification checks!")
	return nil
}

func verifyGGUFHeader(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 4)
	_, err = io.ReadFull(file, header)
	if err != nil {
		return fmt.Errorf("unable to read header bytes: %w", err)
	}

	for i := 0; i < 4; i++ {
		if header[i] != GGUFMagic[i] {
			return fmt.Errorf("invalid magic header. File is not a valid GGUF binary (got %v)", header)
		}
	}
	return nil
}

func main() {
	// Official Qwen 2.5 Coder 1.5B Instruct GGUF from HuggingFace
	modelURL := "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
	targetPath := "./vm-assets/qwen1.5b.gguf"
	
	// Pass expected SHA256 (leave empty string if verifying only magic bytes during initial test)
	expectedHash := "" 

	if err := os.MkdirAll("./vm-assets", 0755); err != nil {
		log.Fatal(err)
	}

	if err := DownloadAndVerify(modelURL, targetPath, expectedHash); err != nil {
		log.Fatalf("❌ Security Check Failed: %v", err)
	}
}