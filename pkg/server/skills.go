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
	Authored      bool   `json:"authored"` // operator-authored (deletable) vs a shipped fixture
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
	// Author signs + persists an operator-authored skill and adds it to the catalog
	// (trusted-but-unapproved); AllowedTools offers the choices; Delete removes an
	// authored skill (never a fixture).
	Author(name, instructions string, tools []string) (SkillCatalogRow, error)
	Delete(name string) error
	AllowedTools() []string
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
	var allowed []string
	if s.SkillCatalog != nil {
		catalog = s.SkillCatalog.Catalog()
		allowed = s.SkillCatalog.AllowedTools()
	}
	writeJSON(w, map[string]any{"approvals": approvals, "catalog": catalog, "allowed_tools": allowed})
}

// skillsAuthor signs + persists an operator-authored skill and adds it to the catalog as a
// trusted-but-unapproved entry (the operator still Approves it before it reaches the agent).
// CSRF + authz(write/skill). Authoring under the trusted anchor is the operator vouching for
// their own instructions; approval remains the activation step.
func (s *Server) skillsAuthor(w http.ResponseWriter, r *http.Request) {
	if s.SkillCatalog == nil {
		http.Error(w, "no skill catalog", http.StatusBadRequest)
		return
	}
	var req struct {
		Name         string   `json:"name"`
		Instructions string   `json:"instructions"`
		Tools        []string `json:"tools"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	row, err := s.SkillCatalog.Author(req.Name, req.Instructions, req.Tools)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "skill": row})
}

// skillsDelete removes an operator-authored skill (never a shipped fixture).
// CSRF + authz(write/skill).
func (s *Server) skillsDelete(w http.ResponseWriter, r *http.Request) {
	if s.SkillCatalog == nil {
		http.Error(w, "no skill catalog", http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := s.SkillCatalog.Delete(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "name": req.Name})
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
