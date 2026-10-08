package server

import (
	"encoding/json"
	"net/http"

	"github.com/t0ul/GoRag"
)

// RAGConfig is the operator-tunable retrieval ingestion config (DB-backed). Changing
// any field requires re-running ingestion over the whole corpus (Reindex), because
// chunking and embedding are applied at index time.
type RAGConfig struct {
	Mode         string `json:"mode"`          // "fts" (lexical) | "semantic" (vector)
	Chunker      string `json:"chunker"`       // whole | paragraph | fixed
	ChunkSize    int    `json:"chunk_size"`    // MaxChars (paragraph) / window (fixed)
	ChunkOverlap int    `json:"chunk_overlap"` // fixed-window overlap
	Embedder     string `json:"embedder"`      // "none" | a model name to embed with
}

// RAGStats is the current index shape.
type RAGStats struct {
	Docs   int    `json:"docs"`
	Chunks int    `json:"chunks"`
	Mode   string `json:"mode"`
}

// RAGLab is the retrieval experimentation surface: read/apply the ingestion config,
// re-run ingestion, and report the index shape. Implemented in the composition root
// (it owns the corpus + processed emails).
type RAGLab interface {
	Config() RAGConfig
	Save(RAGConfig) (RAGStats, error) // persist + apply + reindex
	Reindex() (RAGStats, error)
	Stats() RAGStats
	Embedders() []string // selectable embedder names (catalog models + "none")
}

func (s *Server) ragGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"config":    s.RAG.Config(),
		"stats":     s.RAG.Stats(),
		"chunkers":  gorag.ChunkerNames(),
		"modes":     []string{"fts", "semantic"},
		"embedders": s.RAG.Embedders(),
	})
}

// ragSave persists + applies a new ingestion config and reindexes. CSRF + authz(write/rag).
func (s *Server) ragSave(w http.ResponseWriter, r *http.Request) {
	var cfg RAGConfig
	if json.NewDecoder(r.Body).Decode(&cfg) != nil {
		http.Error(w, "bad config", http.StatusBadRequest)
		return
	}
	stats, err := s.RAG.Save(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "stats": stats})
}

// ragReindex re-runs ingestion over the whole corpus with the current config.
func (s *Server) ragReindex(w http.ResponseWriter, _ *http.Request) {
	stats, err := s.RAG.Reindex()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "stats": stats})
}
