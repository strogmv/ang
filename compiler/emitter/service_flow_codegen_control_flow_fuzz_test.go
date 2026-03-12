package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

// FuzzRenderFlow_ControlFlow_NoPanic checks that control-flow codegen handles
// arbitrary step payloads without panicking.
func FuzzRenderFlow_ControlFlow_NoPanic(f *testing.F) {
	f.Add("req.Role", "owner", "guest", "item")
	f.Add("req.Status", "", "fallback", "_")
	f.Add("bad expr(", "a", "b", "x")

	f.Fuzz(func(t *testing.T, value, caseA, caseB, alias string) {
		value = trimForFuzz(value, 128)
		caseA = fallback(trimForFuzz(caseA, 64), "a")
		caseB = fallback(trimForFuzz(caseB, 64), "b")
		alias = fallback(safeIdent(alias), "item")

		steps := []normalizer.FlowStep{
			{Action: "flow.If", Args: map[string]any{
				"condition": "true",
				"_then": []normalizer.FlowStep{
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"then"}}},
				},
			}},
			{Action: "flow.For", Args: map[string]any{
				"each": "items",
				"as":   alias,
				"_do": []normalizer.FlowStep{
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"loop"}}},
				},
			}},
			{Action: "flow.While", Args: map[string]any{
				"condition": "false",
				"_do": []normalizer.FlowStep{
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"while"}}},
				},
			}},
			{Action: "flow.Switch", Args: map[string]any{
				"value": value,
				"_cases": map[string][]normalizer.FlowStep{
					caseA: {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"A"}}}},
					caseB: {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"B"}}}},
				},
				"_default": []normalizer.FlowStep{
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"default"}}},
				},
			}},
			{Action: "flow.Checkpoint", Args: map[string]any{"name": "fuzzCP", "data": "req"}},
			{Action: "flow.Resume", Args: map[string]any{"name": "fuzzCP", "output": "restored"}},
			{Action: "flow.RecordEvent", Args: map[string]any{"name": `"fuzz.event"`, "payload": "req"}},
			{Action: "flow.History.Get", Args: map[string]any{"output": "hist"}},
			{Action: "flow.Replay", Args: map[string]any{
				"history": "hist",
				"_do": []normalizer.FlowStep{
					{Action: "flow.RecordEvent", Args: map[string]any{"name": `"replayed.event"`}},
				},
			}},
		}

		_ = renderFlow(steps) // no-panic contract
	})
}

// FuzzRenderFlow_ControlFlow_SyntaxForSafeInputs ensures syntax-valid snippets
// for sanitized control-flow inputs.
func FuzzRenderFlow_ControlFlow_SyntaxForSafeInputs(f *testing.F) {
	f.Add("role", "owner", "guest", "it")
	f.Add("status", "created", "done", "row")

	f.Fuzz(func(t *testing.T, valueSeed, caseASeed, caseBSeed, aliasSeed string) {
		valueExpr := safeSwitchExpr(valueSeed)
		caseA := fallback(trimForFuzz(caseASeed, 32), "a")
		caseB := fallback(trimForFuzz(caseBSeed, 32), "b")
		alias := fallback(safeIdent(aliasSeed), "item")
		condExpr := safeBoolExpr(valueSeed)

		steps := []normalizer.FlowStep{
			{Action: "flow.If", Args: map[string]any{
				"condition": condExpr,
				"_then": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
			{Action: "flow.For", Args: map[string]any{
				"each": "items",
				"as":   alias,
				"_do": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
			{Action: "flow.While", Args: map[string]any{
				"condition": condExpr,
				"_do": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
			{Action: "flow.Switch", Args: map[string]any{
				"value": valueExpr,
				"_cases": map[string][]normalizer.FlowStep{
					caseA: {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
					caseB: {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
				},
				"_default": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
		}

		code := renderFlow(steps)
		if _, err := parseFlowStmtList(code); err != nil {
			t.Fatalf("generated control-flow code must be syntactically valid: %v\n\n%s", err, code)
		}
	})
}

// FuzzRenderFlow_ControlFlow_Deterministic ensures generation is stable for the
// same control-flow input and doesn't depend on map iteration order.
func FuzzRenderFlow_ControlFlow_Deterministic(f *testing.F) {
	f.Add("role", "owner", "guest")
	f.Add("status", "created", "done")

	f.Fuzz(func(t *testing.T, valueSeed, caseASeed, caseBSeed string) {
		valueExpr := safeSwitchExpr(valueSeed)
		caseA := fallback(trimForFuzz(caseASeed, 32), "a")
		caseB := fallback(trimForFuzz(caseBSeed, 32), "b")

		steps := []normalizer.FlowStep{
			{Action: "flow.Switch", Args: map[string]any{
				"value": valueExpr,
				"_cases": map[string][]normalizer.FlowStep{
					caseA: {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"A"}}}},
					caseB: {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"B"}}}},
				},
				"_default": []normalizer.FlowStep{
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"default"}}},
				},
			}},
		}

		code1 := renderFlow(steps)
		code2 := renderFlow(steps)
		if code1 != code2 {
			t.Fatalf("expected deterministic render output\n--- first ---\n%s\n--- second ---\n%s", code1, code2)
		}
	})
}

func safeSwitchExpr(seed string) string {
	switch tinyHash(seed) % 4 {
	case 0:
		return "req.Role"
	case 1:
		return "req.Status"
	case 2:
		return "strings.ToLower(req.Role)"
	default:
		return `"fallback"`
	}
}

func safeBoolExpr(seed string) string {
	switch tinyHash(seed) % 3 {
	case 0:
		return "true"
	case 1:
		return "len(items) >= 0"
	default:
		return "req.Offset < 10"
	}
}

func safeIdent(v string) string {
	v = trimForFuzz(v, 64)
	var b strings.Builder
	for i, r := range v {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if isLetter || r == '_' {
				b.WriteRune(r)
			}
			continue
		}
		if isLetter || isDigit || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func trimForFuzz(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func tinyHash(s string) int {
	h := 17
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}
