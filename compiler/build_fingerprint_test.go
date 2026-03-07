package compiler

import "testing"

func TestBuildFingerprintStableAndNonEmpty(t *testing.T) {
	first := BuildFingerprint()
	second := BuildFingerprint()
	if first == "" {
		t.Fatalf("expected non-empty fingerprint")
	}
	if first != second {
		t.Fatalf("expected stable fingerprint, got %q and %q", first, second)
	}
}
