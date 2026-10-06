package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

// Embedder turns text into a vector. In the fleet it calls llama.cpp's
// /v1/embeddings through gouncer; tests inject a deterministic fake.
type Embedder func(ctx context.Context, text string) ([]float32, error)

// addVector stores an embedding for id (no-op when no embedder is configured).
func (s *Store) addVector(id, text string) error {
	if s.Embed == nil {
		return nil
	}
	v, err := s.Embed(context.Background(), text)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(v)
	_, err = s.db.Exec(`INSERT OR REPLACE INTO vectors(id, vec) VALUES(?,?)`, id, string(b))
	return err
}

// SemanticQuery ranks tenant-visible documents by cosine similarity to the
// query embedding. The tenant ACL is in the SQL (authorization-first), the same
// as lexical Query; vectors never leave the store (embedding-inversion defense).
func (s *Store) SemanticQuery(ctx context.Context, tenant, query string, k int) ([]Chunk, error) {
	if s.Embed == nil {
		return nil, errors.New("rag: no embedder configured")
	}
	qv, err := s.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT d.id, d.prov, d.text, v.vec FROM docs d JOIN vectors v ON d.id = v.id WHERE d.tenant = ? OR d.tenant = ''`,
		tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		c   Chunk
		sim float64
	}
	var hits []scored
	for rows.Next() {
		var c Chunk
		var prov, vecJSON string
		if err := rows.Scan(&c.DocID, &prov, &c.Text, &vecJSON); err != nil {
			return nil, err
		}
		var vec []float32
		if json.Unmarshal([]byte(vecJSON), &vec) != nil {
			continue
		}
		c.Prov = Provenance(prov)
		hits = append(hits, scored{c, cosine(qv, vec)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })
	out := make([]Chunk, 0, k)
	for i := 0; i < len(hits) && i < k; i++ {
		out = append(out, hits[i].c)
	}
	return out, nil
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// HTTPEmbedder returns an Embedder that calls an OpenAI-style /v1/embeddings
// endpoint (e.g. the gouncer gateway fronting llama.cpp).
func HTTPEmbedder(url, model string) Embedder {
	client := &http.Client{Timeout: 30 * time.Second}
	return func(ctx context.Context, text string) ([]float32, error) {
		body, _ := json.Marshal(map[string]any{"model": model, "input": text})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var out struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		if len(out.Data) == 0 {
			return nil, errors.New("rag: empty embeddings response")
		}
		return out.Data[0].Embedding, nil
	}
}
