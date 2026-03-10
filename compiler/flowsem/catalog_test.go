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

func TestActionCatalog_IncludesSemanticsForAllActions(t *testing.T) {
	t.Parallel()

	entries := ActionCatalog()
	if len(entries) == 0 {
		t.Fatalf("expected non-empty action catalog")
	}
	for _, entry := range entries {
		if entry.KnownBy == "" {
			t.Fatalf("action %q missing KnownBy semantics", entry.Name)
		}
		if entry.Name == "" {
			t.Fatalf("encountered action with empty name")
		}
	}

	var openai *ActionCatalogEntry
	var mapping *ActionCatalogEntry
	for i := range entries {
		switch entries[i].Name {
		case "openai.Chat":
			openai = &entries[i]
		case "mapping.Assign":
			mapping = &entries[i]
		}
	}
	if openai == nil || mapping == nil {
		t.Fatalf("expected openai.Chat and mapping.Assign in catalog")
	}
	if openai.Effect != "ai" {
		t.Fatalf("openai.Chat effect=%q want ai", openai.Effect)
	}
	if openai.TxCompatible {
		t.Fatalf("openai.Chat must not be tx-compatible")
	}
	if !contains(openai.RequiresTags, "quota.checked") || !contains(openai.RequiresTags, "budget.checked") {
		t.Fatalf("openai.Chat requires_tags=%v want quota.checked+budget.checked", openai.RequiresTags)
	}
	if !contains(openai.RequiresVars, "user_message") || !contains(openai.RequiresVars, "history") {
		t.Fatalf("openai.Chat requires_vars=%v want user_message+history", openai.RequiresVars)
	}
	if openai.KnownBy != "explicit" {
		t.Fatalf("openai.Chat known_by=%q want explicit", openai.KnownBy)
	}
	if mapping.Effect != "" {
		t.Fatalf("mapping.Assign effect=%q want pure/empty", mapping.Effect)
	}
	if !mapping.TxCompatible {
		t.Fatalf("mapping.Assign must be tx-compatible")
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
