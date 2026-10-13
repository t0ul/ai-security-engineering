package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/dataset"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

// Deterministic, no model. A forged "trusted" record is rejected at ingestion and
// unsigned data is contained (not rejected); an unsafe change is refused at
// promotion and a reviewed one ships. Both gates fail closed.

func TestDatasetGateHolds(t *testing.T) {
	if !datasetGate() {
		t.Fatal("the dataset gate must reject a forged trusted record, trust a genuine one, and contain unsigned data")
	}
}

func TestPromotionGateHolds(t *testing.T) {
	if !promotionGate() {
		t.Fatal("the promotion gate must refuse an unsafe change and promote a reviewed one")
	}
}

func TestForgedRecordRejectedGenuineTrusted(t *testing.T) {
	s, pub, _ := provenance.NewSigner("src")
	bad, _, _ := provenance.NewSigner("bad")
	v := provenance.NewVerifier().Trust("src", pub)
	txt := "bulletin"
	if ok, err := dataset.Verify(v, dataset.Record{Text: txt, Signed: true, Mark: s.Sign([]byte(txt))}); !ok || err != nil {
		t.Fatalf("a genuinely signed record must be trusted, got ok=%v err=%v", ok, err)
	}
	if ok, err := dataset.Verify(v, dataset.Record{Text: txt, Signed: true, Mark: bad.Sign([]byte(txt))}); ok || !errors.Is(err, dataset.ErrForged) {
		t.Fatalf("a forged record must be rejected as forged, got ok=%v err=%v", ok, err)
	}
}

func TestPromotionGateFailsClosed(t *testing.T) {
	// A gate with no oracles wired must refuse (cannot promote without proof).
	ok, _ := controlplane.PromotionGate{}.Allow()
	if ok {
		t.Fatal("a promotion gate with no eval/ADD oracle must fail closed")
	}
}
