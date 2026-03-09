package flowsem

import "testing"

// These actions were previously under-covered by unit tests.
// Keep explicit literals to prevent silent catalog regressions.
func TestActionCatalog_RegressionActionsAreRegisteredAndKnown(t *testing.T) {
	t.Parallel()

	actions := []string{
		"archive.ZipDir",
		"cast.ToString",
		"circuit.Check",
		"circuit.RecordFailure",
		"circuit.RecordSuccess",
		"claude.Chat",
		"config.Get",
		"convert.ToFloat",
		"convert.ToInt",
		"db.Delete",
		"db.Get",
		"db.Insert",
		"db.List",
		"db.Lock",
		"db.Query",
		"db.SelectForUpdate",
		"db.Update",
		"db.Upsert",
		"dedupe.Once",
		"event.Match",
		"event.Subscribe",
		"event.Wait",
		"field.CopyNonEmpty",
		"flow.Cron",
		"flow.Delay",
		"flow.Join",
		"flow.Parallel",
		"flow.Return",
		"flow.Schedule",
		"flow.Tag",
		"fs.WriteFile",
		"http.Paginate",
		"http.RetryPolicy",
		"list.Avg",
		"list.Len",
		"list.New",
		"list.Sum",
		"map.Build",
		"map.New",
		"mapping.Map",
		"notification.Dispatch",
		"num.Add",
		"num.Div",
		"num.Mul",
		"num.Sub",
		"openai.Chat",
		"bulkhead.Acquire",
		"concurrency.Limit",
		"ratelimit.Check",
		"secret.Get",
		"session.Get",
		"state.Delete",
		"state.Get",
		"state.Set",
		"str.Concat",
		"str.StripMarkdown",
		"time.CheckExpiry",
	}

	for _, action := range actions {
		action := action
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			if _, ok := specs[action]; !ok {
				t.Fatalf("action %q is not registered in flowsem specs", action)
			}
			issues := Validate([]Step{{Action: action}})
			for _, it := range issues {
				if it.Code == "UNKNOWN_ACTION" {
					t.Fatalf("action %q resolved as UNKNOWN_ACTION: %+v", action, issues)
				}
			}
		})
	}
}
