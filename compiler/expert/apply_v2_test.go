package expert

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleProviderCUE = `package provider

import "example.com/schema"

provider: schema.#Provider & schema.ProfileIncasPayout & {
	package_name: "n692"
	sid:          "n692"
}
`

func TestApplyChangeV2MergeOperationsInitPayout(t *testing.T) {
	change := ChangeV2{
		Op: ChangeMerge,
		Target: IntentTarget{
			Kind:         IntentTargetProjectCUERoot,
			RelativePath: "provider.cue",
		},
		CUEPath:   "operations",
		Value:     json.RawMessage(`[{"kind":"init_payout","transport":{"endpoint":"payout"}}]`),
		Rationale: "declare payout operation",
	}
	out, err := ApplyChangeV2([]byte(sampleProviderCUE), change, false)
	if err != nil {
		t.Fatalf("ApplyChangeV2: %v", err)
	}
	text := string(out)
	for _, part := range []string{"operations:", "kind:", "init_payout", "endpoint:"} {
		if !strings.Contains(text, part) {
			t.Fatalf("expected %q in output, got:\n%s", part, text)
		}
	}
}

func TestApplyChangeV2BeforeHashFixture(t *testing.T) {
	content := []byte(sampleProviderCUE)
	hash := ContentHash(content)
	change := ChangeV2{
		Op: ChangeMerge,
		Target: IntentTarget{
			Kind:         IntentTargetProjectCUERoot,
			RelativePath: "provider.cue",
		},
		CUEPath:    "operations.init_payout",
		Value:      json.RawMessage(`{"enabled":true}`),
		BeforeHash: hash,
		Rationale:  "declare payout operation",
	}
	out, err := ApplyChangeV2(content, change, false)
	if err != nil {
		t.Fatalf("ApplyChangeV2: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestContentHashDeterministic(t *testing.T) {
	a := ContentHash([]byte("alpha"))
	b := ContentHash([]byte("alpha"))
	if a != b || len(a) != 64 {
		t.Fatalf("unexpected hash: %q", a)
	}
}

func TestVerifyChangeBeforeHash(t *testing.T) {
	content := []byte(sampleProviderCUE)
	if err := VerifyChangeBeforeHash(content, ChangeV2{BeforeHash: ContentHash(content)}); err != nil {
		t.Fatalf("VerifyChangeBeforeHash: %v", err)
	}
	if err := VerifyChangeBeforeHash(content, ChangeV2{BeforeHash: "deadbeef"}); err == nil {
		t.Fatal("expected stale hash error")
	}
}
