// Package modelcatalog is the single source of truth for the agent's models: which
// models exist, where to fetch them (verified), what file they land in, and how
// modeld serves them. It replaces three disconnected hardcoded lists (download URLs
// in prepareassets, filenames/ports in modeld, logical bindings in the webapp).
//
// Per config-in-db: the catalog lives in the datastore and is end-user configurable;
// the DefaultSeed here is the FAIL-CLOSED BOOTSTRAP only — what the catalog holds when
// the DB has no rows yet. Nothing downstream should hardcode a model URL/file/port.
package modelcatalog

// Entry is one model in the catalog. Name is the logical model the gouncer gateway
// routes (e.g. "planner", "coder") and the webapp Models plane binds roles to. SHA256
// is the pinned content hash; empty means magic-verify only (the seed default, until a
// pin is recorded) — a UI-triggered download should require it.
type Entry struct {
	Name   string `json:"name"`   // logical model name / gouncer route target
	URL    string `json:"url"`    // download URL (.gguf)
	SHA256 string `json:"sha256"` // pinned content hash; "" = GGUF-magic verify only
	File   string `json:"file"`   // local filename in the asset dir
	Port   int    `json:"port"`   // serve port (modeld)
	Ctx    int    `json:"ctx"`    // context window size
	Host   string `json:"host"`   // bind host (least exposure: loopback)
	// Kind distinguishes how the model is served. "" (default) is a chat/completions
	// model booted into the chat stack (modelstack) and routed by the gateway;
	// "embedding" is a TRAINED embedding model served on demand by the dedicated
	// embeddings server (--embeddings --pooling mean), NOT the chat stack — so semantic
	// retrieval uses a real embedding model instead of a chat model in embeddings mode.
	Kind string `json:"kind,omitempty"`
}

// IsEmbedding reports whether the entry is a trained embedding model (served by the
// dedicated embeddings server, excluded from the chat model stack).
func (e Entry) IsEmbedding() bool { return e.Kind == "embedding" }

// DefaultSeed is the fail-closed bootstrap catalog: the dual-SLM runtime this lab
// ships with. These values were previously hardcoded as consts in cmd/prepareassets
// and flag defaults in cmd/modeld; they now live in ONE place and seed the DB.
func DefaultSeed() []Entry {
	return []Entry{
		{
			Name: "planner",
			URL:  "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf",
			File: "llama-3.2-3b.gguf",
			Port: 11435, Ctx: 8192, Host: "127.0.0.1",
		},
		{
			Name: "coder",
			URL:  "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
			File: "qwen1.5b.gguf",
			Port: 11436, Ctx: 4096, Host: "127.0.0.1",
		},
		{
			// A TRAINED embedding model (not a chat model in embeddings mode): far better
			// semantic-retrieval quality. Served by the dedicated embeddings server on
			// demand, so Kind="embedding" keeps it OUT of the chat stack. Select it as the
			// RAG embedder in the Studio; download it from the catalog (SHA pin optional).
			Name: "nomic-embed-text",
			URL:  "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q4_K_M.gguf",
			File: "nomic-embed-text-v1.5.gguf",
			Port: 11500, Ctx: 2048, Host: "127.0.0.1",
			Kind: "embedding",
		},
	}
}
