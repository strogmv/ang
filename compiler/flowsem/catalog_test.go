package flowsem

import "testing"

func TestActionCatalog_SortedAndContainsDeterministicActions(t *testing.T) {
	t.Parallel()
	entries := ActionCatalog()
	if len(entries) == 0 {
		t.Fatalf("expected non-empty action catalog")
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Name > entries[i].Name {
			t.Fatalf("catalog must be sorted by name: %s > %s", entries[i-1].Name, entries[i].Name)
		}
	}

	var record *ActionCatalogEntry
	var replay *ActionCatalogEntry
	var historyGet *ActionCatalogEntry
	for i := range entries {
		switch entries[i].Name {
		case "flow.RecordEvent":
			record = &entries[i]
		case "flow.Replay":
			replay = &entries[i]
		case "flow.History.Get":
			historyGet = &entries[i]
		}
	}
	if record == nil || replay == nil || historyGet == nil {
		t.Fatalf("expected flow.RecordEvent/flow.Replay/flow.History.Get in catalog")
	}
	if !hasRequiredArg(*record, "name") {
		t.Fatalf("flow.RecordEvent must require arg 'name'")
	}
	if !hasRequiredArg(*replay, "history") {
		t.Fatalf("flow.Replay must require arg 'history'")
	}
	if !hasRequiredArg(*historyGet, "output") {
		t.Fatalf("flow.History.Get must require arg 'output'")
	}
}

func hasRequiredArg(entry ActionCatalogEntry, name string) bool {
	for _, arg := range entry.Args {
		if arg.Name == name && arg.Required {
			return true
		}
	}
	return false
}
