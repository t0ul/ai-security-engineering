package server

import (
	"encoding/json"
	"net/http"
)

// chat answers a natural-language question grounded in the (PII-scrubbed) email
// corpus — the conversational front-end to Ask-School. The retrieved context is
// XML-encapsulated + injection-neutralized before it reaches any model (M8), and
// the answer is scoped to what the corpus actually says. When no live model is
// reachable it falls back to the most relevant passage, clearly labelled.
func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Question string `json:"question"`
		Unsafe   bool   `json:"unsafe"` // demo: run with RAG controls OFF (raw concat) to show the attack land
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Question == "" {
		http.Error(w, "question required", http.StatusBadRequest)
		return
	}
	answer, sources, err := s.Chat(req.Question, req.Unsafe)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"answer": answer, "sources": sources})
}
