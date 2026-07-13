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

func TestCheckRepositoryFinderChecksArgumentType(t *testing.T) {
	program := repositoryFinderProgram(normalizer.FlowStep{Action: "repo.Query", Args: map[string]any{
		"source": "Profile", "method": "FindByEmail", "args": []string{"req.Active"}, "output": "profiles", "list": true,
	}})

	issues := Check(program)
	requireCheckerIssue(t, issues, "repo.Query.FindByEmail argument 1 has type bool, expected string")
}

func TestCheckRepositoryFinderChecksArity(t *testing.T) {
	program := repositoryFinderProgram(normalizer.FlowStep{Action: "repo.Query", Args: map[string]any{
		"source": "Profile", "method": "FindByEmail", "input": "req.ID", "output": "profiles", "list": true,
	}})
	program.Repositories[0].Finders[0].Where = append(program.Repositories[0].Finders[0].Where, normalizer.FinderWhere{Field: "company_id", Param: "companyID", ParamType: "string"})

	issues := Check(program)
	requireCheckerCode(t, issues, "FLOW_REPOSITORY_SIGNATURE")
}

func TestCheckRepositoryFinderRejectsInputAndArgsTogether(t *testing.T) {
	program := repositoryFinderProgram(normalizer.FlowStep{Action: "repo.Query", Args: map[string]any{
		"source": "Profile", "method": "FindByEmail", "input": "req.ID", "args": []string{"req.ID"}, "output": "profiles", "list": true,
	}})

	issues := Check(program)
	requireCheckerCode(t, issues, "FLOW_REPOSITORY_SIGNATURE")
}

func TestCheckRepositoryFinderPropagatesPrimitiveOutputType(t *testing.T) {
	program := repositoryFinderProgram(normalizer.FlowStep{Action: "repo.Query", Args: map[string]any{
		"source": "Profile", "method": "FindByEmail", "input": "req.ID", "output": "email",
	}})
	program.Repositories[0].Finders[0].ReturnType = "string"
	program.Services[0].Methods[0].Output = normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "email", Type: "string"}}}
	program.Services[0].Methods[0].Flow = append(program.Services[0].Methods[0].Flow, normalizer.FlowStep{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Email", "value": "email"}})

	issues := Check(program)
	for _, issue := range issues {
		if issue.Code == "FLOW_TYPE_MISMATCH" {
			t.Fatalf("primitive finder output should be assignable to string, got %#v", issues)
		}
	}
}

func TestCheckReportsVariableDeclaredOnlyInsideBranch(t *testing.T) {
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Output: normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Flow: []normalizer.FlowStep{
			{Action: "flow.If", Args: map[string]any{"condition": "true", "_then": []normalizer.FlowStep{{Action: "mapping.Assign", Args: map[string]any{"to": "fromBranch", "value": "req.Name", "declare": true}}}}},
			{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Name", "value": "fromBranch"}},
		},
	}}}

	issues := Check(Program{Services: []normalizer.Service{service}})
	requireCheckerCode(t, issues, "FLOW_VARIABLE_UNKNOWN")
}

func TestCheckReportsUnknownVariableInServiceCall(t *testing.T) {
	program := checkerCallProgram(normalizer.FlowStep{Action: "service.Call", Args: map[string]any{
		"service": "Profile", "method": "Get", "args": []string{"missingRequest"}, "output": "profile",
	}})

	issues := Check(program)
	requireCheckerCode(t, issues, "FLOW_VARIABLE_UNKNOWN")
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

func repositoryFinderProgram(call normalizer.FlowStep) Program {
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "id", Type: "string"}, {Name: "active", Type: "bool"}}},
		Output: normalizer.Entity{Name: "RunResponse"},
		Flow:   []normalizer.FlowStep{call},
	}}}
	return Program{
		Services: []normalizer.Service{service},
		Entities: []normalizer.Entity{{Name: "Profile"}},
		Repositories: []normalizer.Repository{{Entity: "Profile", Finders: []normalizer.RepositoryFinder{{
			Name: "FindByEmail", Returns: "many", Where: []normalizer.FinderWhere{{Field: "email", Param: "email", ParamType: "string"}},
		}}}},
	}
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

func requireCheckerCode(t *testing.T, issues []Issue, wanted string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == wanted {
			return
		}
	}
	t.Fatalf("expected issue code %q, got %#v", wanted, issues)
}
