package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClosestActionName(t *testing.T) {
	actions := []string{"repo.Get", "repo.Find", "mapping.Assign"}
	got, ok := closestActionName("repo.Gett", actions)
	if !ok {
		t.Fatalf("expected closest action match")
	}
	if got != "repo.Get" {
		t.Fatalf("got %q want %q", got, "repo.Get")
	}
}

func TestInsertAssignBeforeRepoSave(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "op.cue")
	src := `package api

Op: {
	flow: [
		{action: "repo.Save", source: "Order", input: "newOrder"},
	]
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	changed, err := insertAssignBeforeRepoSave(path, 5, "ID", "uuid.NewString()")
	if err != nil {
		t.Fatalf("insertAssignBeforeRepoSave: %v", err)
	}
	if !changed {
		t.Fatalf("expected change")
	}
	out, _ := os.ReadFile(path)
	txt := string(out)
	if !strings.Contains(txt, `to: "newOrder.ID"`) {
		t.Fatalf("missing inserted ID assignment:\n%s", txt)
	}
}

func TestReplaceActionNearLine(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "op.cue")
	src := `package api

Op: {
	flow: [
		{action: "event.Publish", name: "OrderCreated", payloadMap: {}},
	]
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	changed, err := replaceActionNearLine(path, 5, "event.Publish", "event.Outbox")
	if err != nil {
		t.Fatalf("replaceActionNearLine: %v", err)
	}
	if !changed {
		t.Fatalf("expected change")
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), `"event.Outbox"`) {
		t.Fatalf("expected action replacement:\n%s", string(out))
	}
}
