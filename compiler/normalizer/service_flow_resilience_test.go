package normalizer

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/strogmv/ang/compiler/flowsem"
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
		},
		{
			action: "flow.Replay"
			history: "req.History"
			onMismatch: [{action: "flow.SuggestNext", options: ["refresh input"]}]
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
	if len(steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(steps))
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
	if _, ok := steps[4].Args["_onMismatch"].([]FlowStep); !ok {
		t.Fatalf("expected flow.Replay to parse onMismatch child")
	}
}

func TestValidateFlowSteps_NewResilienceActions(t *testing.T) {
	t.Parallel()

	steps := []flowsem.Step{
		{Action: "flow.Timeout", Args: map[string]any{}},
		{Action: "flow.Try", Args: map[string]any{}},
		{Action: "flow.SuggestNext", Args: map[string]any{}},
		{Action: "flow.History.Get", Args: map[string]any{}},
		{Action: "flow.Replay", Args: map[string]any{}},
	}
	issues := flowsem.Validate(steps)

	wantCodes := map[string]bool{
		"MISSING_DURATION": true,
		"MISSING_DO":       true,
		"MISSING_OPTIONS":  true,
		"MISSING_OUTPUT":   true,
		"MISSING_HISTORY":  true,
	}
	for _, issue := range issues {
		delete(wantCodes, issue.Code)
	}
	for code := range wantCodes {
		t.Fatalf("expected warning code %s in %+v", code, issues)
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
	{ action: "queue.Dequeue", subject: "events.core", output: "msg" },
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
	if got, _ := steps[3].Args["timeout"].(string); got != "3*time.Second" {
		t.Fatalf("expected queue.Dequeue timeout default, got %#v", steps[3].Args["timeout"])
	}
	if got, _ := steps[3].Args["attempts"].(int); got != 2 {
		t.Fatalf("expected queue.Dequeue attempts default=2, got %#v", steps[3].Args["attempts"])
	}
	if got, _ := steps[3].Args["backoffMs"].(int); got != 150 {
		t.Fatalf("expected queue.Dequeue backoffMs default=150, got %#v", steps[3].Args["backoffMs"])
	}
	if got, _ := steps[3].Args["jitterMs"].(int); got != 50 {
		t.Fatalf("expected queue.Dequeue jitterMs default=50, got %#v", steps[3].Args["jitterMs"])
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
		{
			action: "queue.Dequeue"
			subject: "events.core"
			output: "msg"
			attempts: 4
			backoffMs: 25
			jitterMs: 10
			timeoutMs: 900
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
	if got, _ := steps[2].Args["timeoutMs"].(int); got != 900 {
		t.Fatalf("expected queue.Dequeue timeoutMs=900, got %#v", steps[2].Args["timeoutMs"])
	}
	if got, _ := steps[2].Args["attempts"].(int); got != 4 {
		t.Fatalf("expected queue.Dequeue attempts=4, got %#v", steps[2].Args["attempts"])
	}
	if got, _ := steps[2].Args["backoffMs"].(int); got != 25 {
		t.Fatalf("expected queue.Dequeue backoffMs=25, got %#v", steps[2].Args["backoffMs"])
	}
	if got, _ := steps[2].Args["jitterMs"].(int); got != 10 {
		t.Fatalf("expected queue.Dequeue jitterMs=10, got %#v", steps[2].Args["jitterMs"])
	}
}

func TestParseFlowSteps_ReliabilityAndObservabilityWrappers(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
steps: [
	{
		action: "concurrency.Run"
		key: "\"build\""
		max: 8
		do: [{action: "logic.Check", condition: "true", throw: "ok"}]
	},
	{
		action: "circuit.Breaker"
		name: "\"external-api\""
		threshold: 3
		openTTL: "30*time.Second"
		do: [{action: "logic.Check", condition: "true", throw: "ok"}]
	},
	{
		action: "bulkhead.Run"
		name: "\"s3-upload\""
		max: 12
		do: [{action: "logic.Check", condition: "true", throw: "ok"}]
	},
	{
		action: "trace.Span"
		name: "\"BuildProject\""
		attrs: {
			project_id: "req.ID"
		}
		do: [{action: "logic.Check", condition: "true", throw: "ok"}]
	},
	{
		action: "slo.Budget"
		name: "\"build\""
		duration: "2*time.Second"
		do: [{action: "logic.Check", condition: "true", throw: "ok"}]
	},
	{ action: "log.Emit", level: "\"info\"", message: "\"created\"" },
	{ action: "metric.Emit", name: "\"project.created\"", kind: "\"counter\"", value: "1" },
	{ action: "idempotency.DeriveKey", from: ["req.UserID", "req.OrderID"], output: "idemKey" },
	{ action: "idempotency.Check", key: "idemKey" },
	{ action: "idempotency.SaveResult", key: "idemKey", ttl: "24*time.Hour" },
	{ action: "ratelimit.Limit", key: "req.UserID", rps: 20 },
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
	if len(steps) != 11 {
		t.Fatalf("expected 11 steps, got %d", len(steps))
	}
	for i := 0; i < 5; i++ {
		if _, ok := steps[i].Args["_do"].([]FlowStep); !ok {
			t.Fatalf("expected step[%d]=%s to parse do child", i, steps[i].Action)
		}
	}
	if got, _ := steps[1].Args["threshold"].(int); got != 3 {
		t.Fatalf("expected circuit.Breaker threshold=3, got %#v", steps[1].Args["threshold"])
	}
	if got, _ := steps[10].Args["rps"].(int); got != 20 {
		t.Fatalf("expected ratelimit.Limit rps=20, got %#v", steps[10].Args["rps"])
	}
}
