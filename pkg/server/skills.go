package server

import (
	"encoding/json"
	"net/http"
)

// SkillCatalogRow is one entry of the skills supply as shown in the Studio: whether
// its signature is from a trusted key and whether its current content is approved.
type SkillCatalogRow struct {
	Name          string `json:"name"`
	Version       int    `json:"version"`
	SignerTrusted bool   `json:"signer_trusted"`
	Approved      bool   `json:"approved"`
	Hash          string `json:"hash"`
}

// SkillLoadResult is the outcome of loading a skill through (or around) the gate.
type SkillLoadResult struct {
	Loaded       bool   `json:"loaded"`
	Unsafe       bool   `json:"unsafe"`
	Instructions string `json:"instructions,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// SkillLoader is the live consumer of the Skills approval plane: it loads a named
// skill through the governed sign+pin+scope gate, exposes the catalog for the
// Studio, and resolves the content hash an operator pins on approval.
type SkillLoader interface {
	Catalog() []SkillCatalogRow
	FullHash(name string) (string, bool)
	Load(name string, unsafe bool) SkillLoadResult
}

// skillsList returns the governed approvals (plane) alongside the catalog supply.
func (s *Server) skillsList(w http.ResponseWriter, _ *http.Request) {
	type approvalRow struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
		Hash    string `json:"hash"`
	}
	var approvals []approvalRow
	for _, a := range s.Skills.List() {
		approvals = append(approvals, approvalRow{Name: a.Name, Version: a.Version, Hash: short12(a.Hash)})
	}
	var catalog []SkillCatalogRow
	if s.SkillCatalog != nil {
		catalog = s.SkillCatalog.Catalog()
	}
	writeJSON(w, map[string]any{"approvals": approvals, "catalog": catalog})
}

// skillsApprove pins the catalog skill's CURRENT content hash as operator-approved.
// CSRF + authz(write/skill). Approving what you can see is the HITL trust decision.
func (s *Server) skillsApprove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if s.SkillCatalog == nil {
		http.Error(w, "no skill catalog", http.StatusBadRequest)
		return
	}
	hash, ok := s.SkillCatalog.FullHash(req.Name)
	if !ok {
		http.Error(w, "no such skill", http.StatusBadRequest)
		return
	}
	a := s.Skills.Approve(req.Name, hash)
	writeJSON(w, map[string]any{"ok": true, "name": a.Name, "version": a.Version, "hash": short12(a.Hash)})
}

// skillsReset revokes an approval, so the skill is unapproved and fails closed again.
func (s *Server) skillsReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	a := s.Skills.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": a.Name, "version": a.Version})
}

// skillsLoad runs a skill through the governed gate, or (unsafe) bypasses it for the
// controls-off demo — the skills twin of the chat "controls off" toggle.
func (s *Server) skillsLoad(w http.ResponseWriter, r *http.Request) {
	if s.SkillCatalog == nil {
		http.Error(w, "no skill catalog", http.StatusBadRequest)
		return
	}
	var req struct {
		Name   string `json:"name"`
		Unsafe bool   `json:"unsafe"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.SkillCatalog.Load(req.Name, req.Unsafe))
}
