package mcp

import (
	"os"
	"strings"
	"testing"
)

// useTempMemory points the store at a temp dir for the duration of a test by
// switching CWD, since memoryStorePath resolves relative to ".".
func useTempMemory(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestMemoryRoundTrip(t *testing.T) {
	useTempMemory(t)

	// empty store loads cleanly
	facts, err := loadMemory()
	if err != nil {
		t.Fatalf("loadMemory empty: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected empty store, got %d", len(facts))
	}

	f := MemoryFact{ID: newFactID(), Kind: "fact", Text: "session cookie is cross-origin", CreatedAt: "2026-06-18T00:00:00Z"}
	if err := saveMemory([]MemoryFact{f}); err != nil {
		t.Fatalf("saveMemory: %v", err)
	}
	got, err := loadMemory()
	if err != nil || len(got) != 1 || got[0].Text != f.Text {
		t.Fatalf("round-trip mismatch: %+v err=%v", got, err)
	}
}

func TestMemoryDedup(t *testing.T) {
	facts := []MemoryFact{
		{ID: "a", Text: "Refresh   token only when one EXISTS"},
		{ID: "b", Text: "stale fact", Stale: true},
	}
	if dup := findDuplicate(facts, "refresh token only when one exists"); dup == nil || dup.ID != "a" {
		t.Fatalf("expected dedup to match a, got %+v", dup)
	}
	if dup := findDuplicate(facts, "stale fact"); dup != nil {
		t.Fatalf("stale facts must not count as duplicates, got %+v", dup)
	}
	if dup := findDuplicate(facts, "something new"); dup != nil {
		t.Fatalf("unexpected duplicate %+v", dup)
	}
}

func TestMemoryDigest(t *testing.T) {
	useTempMemory(t)
	_ = saveMemory([]MemoryFact{
		{ID: "f-1", Kind: "decision", Text: "store memory under cue/.memory", Rationale: "versioned with intent", CreatedAt: "2026-06-18T10:00:00Z"},
		{ID: "f-2", Kind: "fact", Text: "old fact", Stale: true, CreatedAt: "2026-06-17T10:00:00Z"},
	})
	digest, err := memoryDigest()
	if err != nil {
		t.Fatalf("memoryDigest: %v", err)
	}
	if !strings.Contains(digest, "store memory under cue/.memory") {
		t.Fatalf("digest missing live decision:\n%s", digest)
	}
	if strings.Contains(digest, "old fact") {
		t.Fatalf("digest must exclude stale facts:\n%s", digest)
	}
	if !strings.Contains(digest, "why: versioned with intent") {
		t.Fatalf("digest missing rationale:\n%s", digest)
	}
}
