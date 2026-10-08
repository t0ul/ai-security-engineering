package server

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/t0ul/ai-security-engineering/internal/assets"
)

// ModelRuntimeRow is the live readiness of one catalog model: is its GGUF file
// present and valid on disk, and is its serve port answering. This is the in-console
// equivalent of `cmd/preflight`, so "why is chat dead" has an answer in the UI.
type ModelRuntimeRow struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	Present   bool   `json:"present"`
	ValidGGUF bool   `json:"valid_gguf"`
	Port      int    `json:"port"`
	PortOpen  bool   `json:"port_open"`
}

// runtimeStatus reports the live model runtime: gateway health, asset directory, and
// per-model file/port readiness. Read-only.
func (s *Server) runtimeStatus(w http.ResponseWriter, _ *http.Request) {
	var rows []ModelRuntimeRow
	if s.ModelCatalog != nil {
		entries, _ := s.ModelCatalog.ListModelCatalog()
		for _, e := range entries {
			r := ModelRuntimeRow{Name: e.Name, File: e.File, Port: e.Port}
			path := filepath.Join(s.AssetsDir, e.File)
			if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
				r.Present = true
				r.ValidGGUF = assets.VerifyGGUFHeader(path) == nil
			}
			host := e.Host
			if host == "" {
				host = "127.0.0.1"
			}
			r.PortOpen = dialable(net.JoinHostPort(host, strconv.Itoa(e.Port)))
			rows = append(rows, r)
		}
	}
	gwUp := s.GatewayUp != nil && s.GatewayUp()
	writeJSON(w, map[string]any{
		"gateway":    map[string]any{"url": s.GatewayURL, "up": gwUp},
		"assets_dir": s.AssetsDir,
		"models":     rows,
	})
}

// dialable reports whether a TCP address answers within a short timeout.
func dialable(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
