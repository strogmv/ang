package main

import "testing"

func TestDocsFingerprintDeterministic(t *testing.T) {
	a := map[string]string{
		"file:///a.cue": "a",
		"file:///b.cue": "b",
	}
	b := map[string]string{
		"file:///b.cue": "b",
		"file:///a.cue": "a",
	}
	ha := docsFingerprint("/tmp/ws", a)
	hb := docsFingerprint("/tmp/ws", b)
	if ha != hb {
		t.Fatalf("fingerprints differ: %s vs %s", ha, hb)
	}
}

func TestDiagnosticsHashStable(t *testing.T) {
	d := []map[string]any{
		{"message": "x", "severity": 1},
	}
	h1 := diagnosticsHash(d)
	h2 := diagnosticsHash(d)
	if h1 != h2 {
		t.Fatalf("hash unstable: %s vs %s", h1, h2)
	}
}
