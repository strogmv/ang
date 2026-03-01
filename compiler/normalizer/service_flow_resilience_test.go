package normalizer

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestParseFlowSteps_NewResilienceChildren(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
steps: [
		{
			action: "flow.Try"
			do: [{action: "logic.Check", condition: "true", throw: "ok"}]
			catch: [{action: "flow.SuggestNext", options: ["retry"]}]
		},
		{
			action: "flow.Fallback"
			do: [{action: "logic.Check", condition: "true", throw: "ok"}]
			fallback: [{action: "flow.SuggestNext", options: ["open editor"]}]
		},
		{
			action: "flow.Timeout"
			duration: "2 * time.Second"
			do: [{action: "logic.Check", condition: "true", throw: "ok"}]
			onTimeout: [{action: "flow.ExplainError", output: "msg"}]
		},
		{
			action: "flow.Resume"
			name: "after-build"
			onMissing: [{action: "flow.SuggestNext", options: ["create checkpoint"]}]
		}
]
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	listVal := val.LookupPath(cue.ParsePath("steps"))
	n := New()
	steps, err := n.parseFlowSteps(listVal)
	if err != nil {
		t.Fatalf("parseFlowSteps failed: %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}
	if _, ok := steps[0].Args["_catch"].([]FlowStep); !ok {
		t.Fatalf("expected flow.Try to parse catch child")
	}
	if _, ok := steps[1].Args["_fallback"].([]FlowStep); !ok {
		t.Fatalf("expected flow.Fallback to parse fallback child")
	}
	if _, ok := steps[2].Args["_onTimeout"].([]FlowStep); !ok {
		t.Fatalf("expected flow.Timeout to parse onTimeout child")
	}
	if _, ok := steps[3].Args["_onMissing"].([]FlowStep); !ok {
		t.Fatalf("expected flow.Resume to parse onMissing child")
	}
}

func TestValidateFlowSteps_NewResilienceActions(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "flow.Timeout", Args: map[string]any{}},
		{Action: "flow.Try", Args: map[string]any{}},
		{Action: "flow.SuggestNext", Args: map[string]any{}},
	}
	warnings := validateFlowSteps("Build", "Sandbox", steps, nil)

	wantCodes := map[string]bool{
		"MISSING_DURATION": true,
		"MISSING_DO":       true,
		"MISSING_OPTIONS":  true,
	}
	for _, w := range warnings {
		delete(wantCodes, w.Code)
	}
	for code := range wantCodes {
		t.Fatalf("expected warning code %s in %+v", code, warnings)
	}
}

func TestParseFlowSteps_PerformanceDefaults(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
steps: [
	{ action: "http.Call", method: "GET", url: "https://api.test" },
	{
		action: "parallel.Run"
		branches: {
			a: [{action: "logic.Check", condition: "true", throw: "ok"}]
		}
	},
	{ action: "queue.Enqueue", subject: "events.core", payload: "req" },
]
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	listVal := val.LookupPath(cue.ParsePath("steps"))
	n := New()
	steps, err := n.parseFlowSteps(listVal)
	if err != nil {
		t.Fatalf("parseFlowSteps failed: %v", err)
	}
	if got, _ := steps[0].Args["attempts"].(int); got != 2 {
		t.Fatalf("expected http.Call attempts=2 by default, got %#v", steps[0].Args["attempts"])
	}
	if got, _ := steps[0].Args["backoffMs"].(int); got != 150 {
		t.Fatalf("expected http.Call backoffMs=150 by default, got %#v", steps[0].Args["backoffMs"])
	}
	if got, _ := steps[0].Args["timeout"].(string); got != "5*time.Second" {
		t.Fatalf("expected http.Call timeout default, got %#v", steps[0].Args["timeout"])
	}
	if got, _ := steps[1].Args["maxConcurrency"].(int); got != 8 {
		t.Fatalf("expected parallel.Run maxConcurrency=8 by default, got %#v", steps[1].Args["maxConcurrency"])
	}
	if got, _ := steps[2].Args["timeout"].(string); got != "3*time.Second" {
		t.Fatalf("expected queue.Enqueue timeout default, got %#v", steps[2].Args["timeout"])
	}
}

func TestParseFlowSteps_NumericArgsParsed(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
steps: [
	{
		action: "http.Call"
		method: "GET"
		url: "https://api.test"
		attempts: 4
		backoffMs: 25
		timeoutMs: 1200
	},
	{
		action: "parallel.Run"
		maxConcurrency: 3
		branches: {
			a: [{action: "logic.Check", condition: "true", throw: "ok"}]
		}
	},
]
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	listVal := val.LookupPath(cue.ParsePath("steps"))
	n := New()
	steps, err := n.parseFlowSteps(listVal)
	if err != nil {
		t.Fatalf("parseFlowSteps failed: %v", err)
	}
	if got, _ := steps[0].Args["attempts"].(int); got != 4 {
		t.Fatalf("expected attempts=4, got %#v", steps[0].Args["attempts"])
	}
	if got, _ := steps[0].Args["backoffMs"].(int); got != 25 {
		t.Fatalf("expected backoffMs=25, got %#v", steps[0].Args["backoffMs"])
	}
	if got, _ := steps[0].Args["timeoutMs"].(int); got != 1200 {
		t.Fatalf("expected timeoutMs=1200, got %#v", steps[0].Args["timeoutMs"])
	}
	if got, _ := steps[1].Args["maxConcurrency"].(int); got != 3 {
		t.Fatalf("expected maxConcurrency=3, got %#v", steps[1].Args["maxConcurrency"])
	}
}
