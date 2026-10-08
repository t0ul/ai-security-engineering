package server

import (
	"net/http"

	"github.com/t0ul/ai-security-engineering/internal/modelcatalog"
)

// ModelCatalogLister reads the DB-backed model catalog (the single source of truth
// for which models exist, their verified download URL/SHA, file, and serve port/ctx).
// *datastore.Store satisfies it. Read-only here; add/edit/download is a later slice.
type ModelCatalogLister interface {
	ListModelCatalog() ([]modelcatalog.Entry, error)
}

// ModelCatalogRow is one catalog entry for the Studio Models tab. SHA is shown as
// set/unset (a pin is integrity, and the full hash is long); the URL is shown so the
// operator can see provenance.
type ModelCatalogRow struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Pinned bool   `json:"pinned"` // whether a SHA-256 pin is recorded
	File   string `json:"file"`
	Port   int    `json:"port"`
	Ctx    int    `json:"ctx"`
	Host   string `json:"host"`
}

func (s *Server) modelCatalogList(w http.ResponseWriter, _ *http.Request) {
	entries, err := s.ModelCatalog.ListModelCatalog()
	if err != nil {
		http.Error(w, "catalog read failed", http.StatusInternalServerError)
		return
	}
	out := make([]ModelCatalogRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, ModelCatalogRow{
			Name: e.Name, URL: e.URL, Pinned: e.SHA256 != "",
			File: e.File, Port: e.Port, Ctx: e.Ctx, Host: e.Host,
		})
	}
	writeJSON(w, map[string]any{"catalog": out})
}
