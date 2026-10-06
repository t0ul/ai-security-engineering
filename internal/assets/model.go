package assets

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrModelFormat rejects any model artifact that is not a permitted, verifiable
// format. Pickle-based formats (.pkl/.pt/.bin) can execute arbitrary code on
// load (M10 / ASI04), so only .gguf and .safetensors are allowed.
var ErrModelFormat = errors.New("assets: only .gguf and .safetensors model formats are permitted")

// CheckModelFormat enforces the artifact-format allowlist by extension.
func CheckModelFormat(nameOrURL string) error {
	low := strings.ToLower(nameOrURL)
	if strings.HasSuffix(low, ".gguf") || strings.HasSuffix(low, ".safetensors") {
		return nil
	}
	return fmt.Errorf("%w: %q", ErrModelFormat, nameOrURL)
}

// VerifySafetensorsHeader validates the safetensors structure: an 8-byte
// little-endian header length followed by that many bytes of valid JSON, within
// the file. safetensors is pure tensor data with a JSON header — no executable
// code path, unlike pickle.
func VerifySafetensorsHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var n uint64
	if err := binary.Read(f, binary.LittleEndian, &n); err != nil {
		return fmt.Errorf("%w: unreadable header length", ErrModelFormat)
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if n == 0 || 8+int64(n) > fi.Size() {
		return fmt.Errorf("%w: header length out of range", ErrModelFormat)
	}
	hdr := make([]byte, n)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return fmt.Errorf("%w: short header", ErrModelFormat)
	}
	if !json.Valid(hdr) {
		return fmt.Errorf("%w: header is not valid JSON", ErrModelFormat)
	}
	return nil
}
