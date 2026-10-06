// Package rag is the M8 retrieval layer over an embedded SQLite index (pure-Go,
// no CGO). Retrieval is a real FTS5 full-text query, and the per-tenant access
// control is a WHERE clause on that query — authorization-first retrieval, so a
// tenant physically cannot match another tenant's rows. Documents carry a
// provenance tag; untrusted chunks are injection-neutralized and XML-encapsulated
// by Assemble before they can reach a model, so a poisoned document is data,
// never instructions.
//
// A vector/semantic path (embeddings via gouncer→llama.cpp, cosine rank) slots
// in as an additional column + ORDER BY; FTS5 lexical search is the baseline.
package rag

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/t0ul/ai-security-engineering/agent/guard"
	_ "modernc.org/sqlite"
)

// Provenance marks whether a document's text is trusted or attacker-influenced.
type Provenance string

const (
	Trusted   Provenance = "trusted"
	Untrusted Provenance = "untrusted"
)

// Doc is one indexed document. Tenant "" is public (visible to all tenants).
type Doc struct {
	ID     string
	Tenant string
	Text   string
	Prov   Provenance
}

// Chunk is a retrieval hit.
type Chunk struct {
	DocID string
	Prov  Provenance
	Text  string
}

// Store is a SQLite-backed FTS5 index. Open with a file path to persist, or
// ":memory:" for an ephemeral index. Set Embed to also index vectors for
// semantic retrieval (SemanticQuery).
type Store struct {
	db    *sql.DB
	Embed Embedder // optional; when set, Add also stores an embedding
}

// Open creates/opens the index at path (":memory:" for ephemeral).
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// SQLite is single-writer; one connection also keeps a :memory: DB coherent.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS docs USING fts5(id UNINDEXED, tenant UNINDEXED, prov UNINDEXED, text)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vectors(id TEXT PRIMARY KEY, vec TEXT)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// Add indexes (or replaces) a document by id.
func (s *Store) Add(d Doc) error {
	if _, err := s.db.Exec(`DELETE FROM docs WHERE id = ?`, d.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT INTO docs(id, tenant, prov, text) VALUES(?,?,?,?)`,
		d.ID, d.Tenant, string(d.Prov), d.Text); err != nil {
		return err
	}
	return s.addVector(d.ID, d.Text)
}

var reWord = regexp.MustCompile(`[a-zA-Z0-9]+`)

// matchExpr turns a free-text query into a safe FTS5 MATCH expression: each word
// quoted and OR-ed, so untrusted query text cannot inject FTS5 operators.
func matchExpr(query string) string {
	words := reWord.FindAllString(query, -1)
	for i, w := range words {
		words[i] = `"` + w + `"`
	}
	return strings.Join(words, " OR ")
}

// Query returns up to k chunks the tenant may see, ranked by FTS5 relevance. The
// tenant ACL is part of the query (authorization-first retrieval).
func (s *Store) Query(tenant, query string, k int) ([]Chunk, error) {
	expr := matchExpr(query)
	if expr == "" {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT id, prov, text FROM docs WHERE docs MATCH ? AND (tenant = ? OR tenant = '') ORDER BY rank LIMIT ?`,
		expr, tenant, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chunk
	for rows.Next() {
		var c Chunk
		var prov string
		if err := rows.Scan(&c.DocID, &prov, &c.Text); err != nil {
			return nil, err
		}
		c.Prov = Provenance(prov)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Assemble renders chunks for a prompt. Every chunk is XML-encapsulated with its
// provenance; untrusted chunks are injection-neutralized first, so hidden
// instructions in a poisoned document never reach the model as instructions.
func Assemble(chunks []Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		text := c.Text
		if c.Prov != Trusted {
			text, _ = guard.Sanitize(text)
		}
		b.WriteString("<retrieved_context doc=\"" + c.DocID + "\" provenance=\"" + string(c.Prov) + "\">\n" + text + "\n</retrieved_context>\n")
	}
	return b.String()
}
