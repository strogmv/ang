package flowir

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestCheckServiceCallPropagatesTargetOutputType(t *testing.T) {
	program := checkerCallProgram(normalizer.FlowStep{
		Action: "service.Call",
		Args: map[string]any{
			"service": "Profile",
			"method":  "Get",
			"args":    []string{"getRequest"},
			"output":  "profile",
		},
	})

	issues := Check(program)
	requireCheckerIssue(t, issues, "cannot assign bool to resp.Name (string)")
}

func TestCheckFlowCallPropagatesTargetOutputType(t *testing.T) {
	program := checkerCallProgram(normalizer.FlowStep{
		Action: "flow.Call",
		Args: map[string]any{
			"op":     "Profile.Get",
			"args":   map[string]string{"id": "requestID"},
			"output": "profile",
		},
	})

	issues := Check(program)
	requireCheckerIssue(t, issues, "cannot assign bool to resp.Name (string)")
}

func TestCheckLogicCallPropagatesInlineFunctionOutputType(t *testing.T) {
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Output: normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Flow: []normalizer.FlowStep{
			{Action: "logic.Call", Args: map[string]any{
				"func":   "(func(value string) (bool, error) { return value != \"\", nil })",
				"args":   []string{"req.Name"},
				"output": "active",
			}},
			{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Name", "value": "active"}},
		},
	}}}

	issues := Check(Program{Services: []normalizer.Service{service}})
	requireCheckerIssue(t, issues, "cannot assign bool to resp.Name (string)")
}

func TestCheckLogicCallChecksInlineFunctionArgumentTypes(t *testing.T) {
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Output: normalizer.Entity{Name: "RunResponse"},
		Flow: []normalizer.FlowStep{{Action: "logic.Call", Args: map[string]any{
			"func": "(func(value int) (string, error) { return \"\", nil })",
			"args": []string{"req.Name"},
		}}},
	}}}

	issues := Check(Program{Services: []normalizer.Service{service}})
	requireCheckerIssue(t, issues, "logic.Call argument 1 has type string, expected int")
}

func TestCheckRepositoryCallPropagatesBuiltinOutputType(t *testing.T) {
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "id", Type: "string"}}},
		Output: normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Flow: []normalizer.FlowStep{
			{Action: "repo.Get", Args: map[string]any{"source": "Profile", "input": "req.ID", "output": "profile"}},
			{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Name", "value": "profile.Active"}},
		},
	}}}

	issues := Check(Program{
		Services:     []normalizer.Service{service},
		Entities:     []normalizer.Entity{{Name: "Profile", Fields: []normalizer.Field{{Name: "active", Type: "bool"}}}},
		Repositories: []normalizer.Repository{{Entity: "Profile"}},
	})
	requireCheckerIssue(t, issues, "cannot assign bool to resp.Name (string)")
}

func checkerCallProgram(call normalizer.FlowStep) Program {
	caller := normalizer.Service{
		Name: "Caller",
		Uses: []string{"Profile"},
		Methods: []normalizer.Method{{
			Name:   "Run",
			Input:  normalizer.Entity{Name: "RunRequest"},
			Output: normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
			Flow: []normalizer.FlowStep{
				call,
				{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Name", "value": "profile.Active"}},
			},
		}},
	}
	profile := normalizer.Service{
		Name: "Profile",
		Methods: []normalizer.Method{{
			Name:   "Get",
			Input:  normalizer.Entity{Name: "GetProfileRequest", Fields: []normalizer.Field{{Name: "id", Type: "string"}}},
			Output: normalizer.Entity{Name: "ProfileResponse", Fields: []normalizer.Field{{Name: "active", Type: "bool"}}},
		}},
	}
	return Program{Services: []normalizer.Service{caller, profile}}
}

func requireCheckerIssue(t *testing.T, issues []Issue, wanted string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == "FLOW_TYPE_MISMATCH" && strings.Contains(issue.Message, wanted) {
			return
		}
	}
	t.Fatalf("expected issue containing %q, got %#v", wanted, issues)
}
