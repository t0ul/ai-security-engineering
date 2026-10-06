package cpstore_test

import (
	"testing"

	"github.com/t0ul/ai-security-engineering/cpstore"
)

func open(t *testing.T) *cpstore.Store {
	t.Helper()
	s, err := cpstore.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLatestEvalFeedsGate(t *testing.T) {
	s := open(t)
	if _, ok, _ := s.LatestEval("1.txt"); ok {
		t.Fatal("expected no score yet")
	}
	if err := s.RecordEval("1.txt", 0.47); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordEval("1.txt", 1.00); err != nil {
		t.Fatal(err)
	}
	e, ok, err := s.LatestEval("1.txt")
	if err != nil || !ok || e.F1 != 1.00 {
		t.Fatalf("latest score = %+v ok=%v err=%v, want F1 1.00", e, ok, err)
	}
}

func TestApprovalsPersistAndList(t *testing.T) {
	s := open(t)
	if err := s.RecordApproval("p1", "alice", "bob", "config.write", "upgrade planner", 1); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListApprovals(10)
	if err != nil || len(got) != 1 {
		t.Fatalf("approvals = %v err=%v", got, err)
	}
	if got[0].Proposer != "alice" || got[0].Approver != "bob" || got[0].Version != 1 {
		t.Fatalf("unexpected approval row: %+v", got[0])
	}
}

func TestRecordersDoNotError(t *testing.T) {
	s := open(t)
	if err := s.RecordPrompt("planner", "v2", "deadbeef", "You are..."); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPin("search", "abc123", "carol"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAdmin("carol", "killswitch", "level=block_tools"); err != nil {
		t.Fatal(err)
	}
}
