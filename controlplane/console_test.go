package controlplane

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t0ul/goverlord"
)

func consoleFixture(t *testing.T) *httptest.Server {
	t.Helper()
	audit, _ := tempAudit(t)
	gov := NewGovernance(audit,
		map[string]any{"planner_model": "planner"},
		goverlord.Operator{ID: "alice", Roles: []string{RoleOperator}},
		goverlord.Operator{ID: "bob", Roles: []string{RoleApprover}},
		goverlord.Operator{ID: "carol", Roles: []string{RoleSRE}},
	)
	srv := httptest.NewServer((&ConsoleServer{Gov: gov}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, base, path string, body any) (int, map[string]any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp, err := http.Post(base+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func getState(t *testing.T, base string) StateView {
	t.Helper()
	resp, err := http.Get(base + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var s StateView
	json.NewDecoder(resp.Body).Decode(&s)
	return s
}

func TestConsoleFourEyesFlow(t *testing.T) {
	base := consoleFixture(t).URL

	// Propose under dual control -> pending, not applied.
	code, out := post(t, base, "/api/propose", map[string]any{
		"op": "alice", "note": "upgrade", "set": map[string]any{"planner_model": "planner-v2"},
	})
	if code != http.StatusOK || out["applied"] != false {
		t.Fatalf("propose: code=%d out=%v", code, out)
	}
	id, _ := out["proposal_id"].(string)
	if id == "" {
		t.Fatal("no proposal id returned")
	}

	// Proposer approving own change -> 422.
	if code, _ := post(t, base, "/api/approve", map[string]any{"op": "alice", "id": id}); code != http.StatusUnprocessableEntity {
		t.Fatalf("four-eyes self-approve should be 422, got %d", code)
	}

	// Different approver -> applied; state reflects it.
	if code, _ := post(t, base, "/api/approve", map[string]any{"op": "bob", "id": id}); code != http.StatusOK {
		t.Fatalf("approve by bob should be 200, got %d", code)
	}
	if s := getState(t, base); s.Version != 1 || s.Config["planner_model"] != "planner-v2" {
		t.Fatalf("state not updated: %+v", s)
	}
}

func TestConsoleRBACForbidden(t *testing.T) {
	base := consoleFixture(t).URL
	// alice (operator) cannot roll back -> 403.
	if code, _ := post(t, base, "/api/rollback", map[string]any{"op": "alice", "to": 0}); code != http.StatusForbidden {
		t.Fatalf("operator rollback should be 403, got %d", code)
	}
}

func TestConsoleKillSwitch(t *testing.T) {
	base := consoleFixture(t).URL
	if code, out := post(t, base, "/api/killswitch", map[string]any{"op": "carol", "engage": true}); code != http.StatusOK || out["killed"] != true {
		t.Fatalf("killswitch engage: code=%d out=%v", code, out)
	}
	// Mutations refused while killed -> 409.
	if code, _ := post(t, base, "/api/propose", map[string]any{"op": "alice", "set": map[string]any{"x": 1}}); code != http.StatusConflict {
		t.Fatalf("propose while killed should be 409, got %d", code)
	}
	if !getState(t, base).Killed {
		t.Fatal("state should report killed")
	}
}

func TestConsoleIndexServesPage(t *testing.T) {
	base := consoleFixture(t).URL
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Governed Control Plane") {
		t.Fatal("index did not serve the console page")
	}
}
