package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// EvalCaseStore is the promoted-regression watchlist: thumbs-down examples the operator
// promoted (with expected behavior) so a failure becomes a durable, reviewed case. Nil =
// promotion disabled.
type EvalCaseStore interface {
	AddEvalCase(source, label, note string) (int64, error)
	ListEvalCases(limit int) ([]EvalCase, error)
	DeleteEvalCase(id int64) error
}

// EvalCase is one promoted regression case surfaced in the Eval tab.
type EvalCase struct {
	ID     int64  `json:"id"`
	Source string `json:"source"`
	Label  string `json:"label"`
	Note   string `json:"note"`
	At     string `json:"at"`
}

func (s *Server) evalCasesList(w http.ResponseWriter, _ *http.Request) {
	cases, err := s.EvalCases.ListEvalCases(100)
	if err != nil {
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"cases": cases})
}

func (s *Server) evalCasePromote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source string `json:"source"`
		Label  string `json:"label"`
		Note   string `json:"note"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Label) == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}
	id, err := s.EvalCases.AddEvalCase(req.Source, req.Label, req.Note)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) evalCaseDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.EvalCases.DeleteEvalCase(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// EvalResult is one persisted eval score for the Eval card (C7).
type EvalResult struct {
	Label string  `json:"label"`
	F1    float64 `json:"f1"`
	At    string  `json:"at,omitempty"`
}

// PromptTestResult is the shadow-eval verdict for a candidate prompt: its live F1,
// the current baseline, whether the ADD suite still holds (ASR=0), the
// promotion-gate decision, and which mode the eval actually ran in.
type PromptTestResult struct {
	F1       float64 `json:"f1"`
	Baseline float64 `json:"baseline"`
	ASRPass  bool    `json:"asr_pass"`
	GateOK   bool    `json:"gate_ok"`
	Mode     string  `json:"mode"`
	Note     string  `json:"note,omitempty"`
}

func (s *Server) evalList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"history": s.Eval.History()})
}

// evalRunHandler runs the eval live (through the extractor LLM path) and persists
// the scores. Guarded by CSRF + authz(write/eval).
func (s *Server) evalRunHandler(w http.ResponseWriter, r *http.Request) {
	results, mode := s.Eval.Run()
	writeJSON(w, map[string]any{"ok": true, "results": results, "mode": mode})
}

// promptsTest shadow-evals a candidate prompt WITHOUT activating it: it reports
// the candidate's live F1, the ADD-ASR, and the promotion-gate verdict so an
// operator can see the effect before promoting. Guarded by CSRF + authz(write/prompts).
func (s *Server) promptsTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Text == "" {
		http.Error(w, "name and text required", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.Eval.Test(req.Name, req.Text))
}
