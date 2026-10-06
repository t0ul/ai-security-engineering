// Package assets holds the shared MicroVM provisioning primitives: verified
// model download (the M10 supply-chain control) and archive extraction. One
// implementation lives here so the prepareassets and verifymodel commands share
// exactly one verified-download path instead of each carrying its own copy.
package assets

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GGUFMagic is the leading magic of a GGUF model file ("GGUF").
var GGUFMagic = []byte{'G', 'G', 'U', 'F'}

// ErrNotGGUF is returned when a downloaded file fails the magic-byte check.
var ErrNotGGUF = errors.New("assets: file is not a valid GGUF binary")

// DownloadAndVerify streams url to destPath while computing SHA-256, enforces
// the .gguf extension policy, verifies the GGUF magic header, and (when
// expectedSHA256 is non-empty) the content hash. A corrupted or mismatched
// artifact is removed before returning an error, so a failed download never
// leaves a poisoned file in place.
func DownloadAndVerify(url, destPath, expectedSHA256 string) error {
	if !strings.HasSuffix(url, ".gguf") {
		return errors.New("assets: security policy — only .gguf model URLs are permitted")
	}
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("assets: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("assets: download %s: status %s", url, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("assets: create %s: %w", destPath, err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, hasher), resp.Body); err != nil {
		out.Close()
		_ = os.Remove(destPath)
		return fmt.Errorf("assets: stream %s: %w", destPath, err)
	}
	out.Close()

	if expectedSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, expectedSHA256) {
			_ = os.Remove(destPath)
			return fmt.Errorf("assets: SHA-256 mismatch: want %s, got %s", expectedSHA256, got)
		}
	}
	if err := VerifyGGUFHeader(destPath); err != nil {
		_ = os.Remove(destPath)
		return err
	}
	return nil
}

// VerifyGGUFHeader reports whether the file begins with the GGUF magic bytes.
func VerifyGGUFHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, len(GGUFMagic))
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("assets: read header of %s: %w", path, err)
	}
	for i := range GGUFMagic {
		if header[i] != GGUFMagic[i] {
			return fmt.Errorf("%w (got %v)", ErrNotGGUF, header)
		}
	}
	return nil
}

// DownloadFile streams url to dest with no verification (for non-model assets
// such as the kernel archive).
func DownloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("assets: download %s: status %s", url, resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

// ExtractTarGz extracts a .tar.gz into destDir. It rejects entries whose path
// escapes destDir (tar-slip / zip-slip defense).
func ExtractTarGz(srcPath, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("assets: tar entry escapes destination: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec // size-bounded by kernel archive
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// GunzipFile decompresses a gzip file to dest.
func GunzipFile(src, dest string) error {
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
