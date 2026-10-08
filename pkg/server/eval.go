package server

import (
	"encoding/json"
	"net/http"
)

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
	writeJSON(w, map[string]any{"history": s.EvalHistory()})
}

// evalRunHandler runs the eval live (through the extractor LLM path) and persists
// the scores. Guarded by CSRF + authz(write/eval).
func (s *Server) evalRunHandler(w http.ResponseWriter, r *http.Request) {
	results, mode := s.EvalRun()
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
	writeJSON(w, s.PromptTest(req.Name, req.Text))
}
