package normalizer

import (
	"strings"
	"testing"
)

func TestValidateValueExpr(t *testing.T) {
	t.Parallel()

	if err := validateValueExpr("uuid.NewString()"); err != "" {
		t.Fatalf("expected valid expression, got error: %s", err)
	}

	if err := validateValueExpr("domain.Tender{ID: uuid.NewString()}"); err != "" {
		t.Fatalf("expected valid composite literal, got error: %s", err)
	}

	if err := validateValueExpr("{{ .Value }}"); err != "" {
		t.Fatalf("expected template expression to be skipped, got error: %s", err)
	}

	err := validateValueExpr("uuid.NewString(")
	if err == "" {
		t.Fatalf("expected syntax error for invalid expression")
	}
	if !strings.Contains(err, "Invalid Go expression") {
		t.Fatalf("expected Invalid Go expression message, got: %s", err)
	}
}

func TestValidateFlowSteps_MappingAssignReportsExpressionSyntax(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "mapping.Assign",
		Args: map[string]any{
			"to":    "resp.ID",
			"value": "uuid.NewString(",
		},
	}}

	warnings := validateFlowSteps("CreateItem", "tender", steps, nil, nil, nil, "strict", nil)
	found := false
	for _, w := range warnings {
		if w.Code == "GO_SYNTAX_ERROR" {
			found = true
			if !strings.Contains(w.Message, "Invalid Go expression") {
				t.Fatalf("expected expression error message, got: %s", w.Message)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected GO_SYNTAX_ERROR warning")
	}
}

func TestValidateFlowSteps_MappingAssignRejectsFunctionCall(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "mapping.Assign",
		Args: map[string]any{
			"to":    "resp.ID",
			"value": "strings.TrimSpace(req.Name)",
		},
	}}

	warnings := validateFlowSteps("CreateItem", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MAPPING_ASSIGN_UNSAFE_EXPR", "Unsafe mapping.Assign value") {
		t.Fatalf("expected MAPPING_ASSIGN_UNSAFE_EXPR warning, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_MappingAssignAllowsSafeValues(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.UserID",
				"value": "req.UserID",
			},
		},
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.Status",
				"value": "\"draft\"",
			},
		},
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.Ok",
				"value": "true",
			},
		},
	}

	warnings := validateFlowSteps("CreateItem", "tender", steps, nil, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "MAPPING_ASSIGN_UNSAFE_EXPR" || w.Code == "GO_SYNTAX_ERROR" {
			t.Fatalf("did not expect mapping.Assign validation error for safe values, got: %+v", warnings)
		}
	}
}

func TestValidateFlowSteps_EventPublishRejectsRawPayload(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "event.Publish",
		Args: map[string]any{
			"name":    "TenderClosed",
			"payload": "domain.TenderClosed{TenderID: req.TenderID}",
		},
	}}

	warnings := validateFlowSteps("CloseTender", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "EVENT_PUBLISH_RAW_PAYLOAD_FORBIDDEN", "raw 'payload' is not allowed") {
		t.Fatalf("expected EVENT_PUBLISH_RAW_PAYLOAD_FORBIDDEN, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_EventPublishPayloadMapAllowsSafeValues(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "event.Publish",
		Args: map[string]any{
			"name": "TenderClosed",
			"payloadMap": map[string]any{
				"TenderID": "req.TenderID",
				"Status":   "\"closed\"",
				"Ok":       "true",
			},
		},
	}}

	warnings := validateFlowSteps("CloseTender", "tender", steps, nil, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "UNSAFE_EVENT_PAYLOAD_EXPR" || w.Code == "EVENT_PUBLISH_RAW_PAYLOAD_FORBIDDEN" || w.Code == "MISSING_PAYLOAD_MAP" {
			t.Fatalf("did not expect payloadMap validation errors, got: %+v", warnings)
		}
	}
}

func TestValidateFlowSteps_EventPublishPayloadMapRejectsUnsafeValue(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "event.Publish",
		Args: map[string]any{
			"name": "TenderClosed",
			"payloadMap": map[string]any{
				"TenderID": "uuid.NewString()",
			},
		},
	}}

	warnings := validateFlowSteps("CloseTender", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_EVENT_PAYLOAD_EXPR", "payloadMap") {
		t.Fatalf("expected UNSAFE_EVENT_PAYLOAD_EXPR, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_LogicCallArgsReportExpressionSyntax(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "logic.Call",
		Args: map[string]any{
			"func": "s.Helper.Do",
			"args": []any{"ctx", "port.CreateReq{ID: uuid.NewString("},
		},
	}}

	warnings := validateFlowSteps("CreateItem", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_CALL_ARG_EXPR", "logic.Call args[1]:") {
		t.Fatalf("expected UNSAFE_CALL_ARG_EXPR for logic.Call args, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_LogicCallWarnsOnBareFmtErrorf(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "logic.Call",
		Args: map[string]any{
			"func": "fmt.Errorf(\"tender not found\")",
		},
	}}

	warnings := validateFlowSteps("CreateItem", "tender", steps, nil, nil, nil, "strict", nil)
	found := false
	for _, w := range warnings {
		if w.Code != "LAMBDA_BARE_ERROR" {
			continue
		}
		found = true
		if w.Severity != "warn" {
			t.Fatalf("expected LAMBDA_BARE_ERROR severity=warn, got %q", w.Severity)
		}
		if !strings.Contains(w.Message, "fmt.Errorf") {
			t.Fatalf("expected fmt.Errorf hint in warning message, got: %s", w.Message)
		}
	}
	if !found {
		t.Fatalf("expected LAMBDA_BARE_ERROR warning, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ServiceCallArgsReportExpressionSyntax(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "service.Call",
		Args: map[string]any{
			"service": "Auth",
			"method":  "RefreshToken",
			"args":    []any{"ctx", "port.RefreshReq{Token: req.Token"},
		},
	}}

	warnings := validateFlowSteps("Refresh", "tender", steps, nil, []string{"Auth"}, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_CALL_ARG_EXPR", "service.Call args[1]:") {
		t.Fatalf("expected UNSAFE_CALL_ARG_EXPR for service.Call args, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoQueryInputReportsExpressionSyntax(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Application", Owner: "application", BoundedContext: "application"}}
	steps := []FlowStep{{
		Action: "repo.Query",
		Args: map[string]any{
			"source": "Application",
			"method": "GetByTenderAndCompany",
			"input":  "req.TenderID(",
			"args":   []any{"req.TenderID", "req.CompanyID"},
			"output": "app",
		},
	}}

	warnings := validateFlowSteps("GetApplication", "application", steps, entities, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_QUERY_ARG_EXPR", "repo.Query input:") {
		t.Fatalf("expected UNSAFE_QUERY_ARG_EXPR for repo.Query input, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoGetRequiresErrorGuard(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{
		Action: "repo.Get",
		Args: map[string]any{
			"source": "Tender",
			"input":  "req.ID",
			"output": "tender",
		},
	}}

	warnings := validateFlowSteps("GetTender", "tender", steps, entities, nil, nil, "strict", nil)
	found := false
	for _, w := range warnings {
		if w.Code == "REPO_GET_MISSING_ERROR" {
			found = true
			if w.Severity != "error" {
				t.Fatalf("expected error severity for REPO_GET_MISSING_ERROR, got %q", w.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected REPO_GET_MISSING_ERROR, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoFindWithoutErrorWarns(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{
		Action: "repo.Find",
		Args: map[string]any{
			"source": "Tender",
			"input":  "req.ID",
			"output": "tender",
		},
	}}

	warnings := validateFlowSteps("FindTender", "tender", steps, entities, nil, nil, "strict", nil)
	found := false
	for _, w := range warnings {
		if w.Code == "REPO_FIND_WITHOUT_ERROR" {
			found = true
			if w.Severity != "warn" {
				t.Fatalf("expected warn severity for REPO_FIND_WITHOUT_ERROR, got %q", w.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected REPO_FIND_WITHOUT_ERROR warning, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoQueryArgsReportExpressionSyntax(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Application", Owner: "application", BoundedContext: "application"}}
	steps := []FlowStep{{
		Action: "repo.Query",
		Args: map[string]any{
			"source": "Application",
			"method": "GetByTenderAndCompany",
			"input":  "req.TenderID",
			"args":   []any{"req.TenderID", "req.CompanyID("},
			"output": "app",
		},
	}}

	warnings := validateFlowSteps("GetApplication", "application", steps, entities, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_QUERY_ARG_EXPR", "repo.Query args[1]:") {
		t.Fatalf("expected UNSAFE_QUERY_ARG_EXPR for repo.Query args[1], got: %+v", warnings)
	}
}

func TestValidateFlowSteps_LogicCheckRejectsFunctionCall(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "logic.Check",
		Args: map[string]any{
			"condition": "strings.Contains(req.Name, \"x\")",
			"throw":     "invalid name",
		},
	}}

	warnings := validateFlowSteps("CheckName", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_CONDITION_EXPR", "logic.Check condition:") {
		t.Fatalf("expected UNSAFE_CONDITION_EXPR, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowCallArgsReportExpressionSyntax(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "flow.Call",
		Args: map[string]any{
			"op": "Tender.ValidateTenderForBid",
			"args": map[string]any{
				"tenderID":  "req.TenderID",
				"companyID": "uuid.NewString()",
			},
			"output": "validated",
		},
	}}

	warnings := validateFlowSteps("PlaceBid", "bids", steps, nil, []string{"Tender"}, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNSAFE_CALL_ARG_EXPR", "flow.Call args.companyID:") {
		t.Fatalf("expected UNSAFE_CALL_ARG_EXPR for flow.Call args, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowCallRequiresDeclaredDependency(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "flow.Call",
		Args: map[string]any{
			"op": "Tender.ValidateTenderForBid",
		},
	}}

	warnings := validateFlowSteps("PlaceBid", "bids", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_SERVICE_DEP", "flow.Call targets 'Tender'") {
		t.Fatalf("expected MISSING_SERVICE_DEP for flow.Call dependency, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoFindMissingRequiredFields(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{
		Action: "repo.Find",
		Args: map[string]any{
			"source": "Tender",
		},
	}}
	warnings := validateFlowSteps("GetTender", "tender", steps, entities, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_INPUT", "repo.Find missing 'input'") {
		t.Fatalf("expected MISSING_INPUT for repo.Find, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "repo.Find missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT for repo.Find, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoSaveMissingInput(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{
		Action: "repo.Save",
		Args: map[string]any{
			"source": "Tender",
		},
	}}
	warnings := validateFlowSteps("SaveTender", "tender", steps, entities, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_INPUT", "repo.Save missing 'input'") {
		t.Fatalf("expected MISSING_INPUT for repo.Save, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_LogicCheckMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "logic.Check",
		Args:   map[string]any{},
	}}
	warnings := validateFlowSteps("Check", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_CONDITION", "logic.Check missing 'condition'") {
		t.Fatalf("expected MISSING_CONDITION, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_THROW", "logic.Check missing 'throw'") {
		t.Fatalf("expected MISSING_THROW, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_MathExprMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "math.Expr",
		Args:   map[string]any{},
	}}
	warnings := validateFlowSteps("Calc", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_EXPR", "math.Expr missing 'expr'") {
		t.Fatalf("expected MISSING_EXPR, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "math.Expr missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_TimeParseMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{
		Action: "time.Parse",
		Args:   map[string]any{},
	}}
	warnings := validateFlowSteps("Parse", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_VALUE", "time.Parse missing 'value'") {
		t.Fatalf("expected MISSING_VALUE, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "time.Parse missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT, got: %+v", warnings)
	}
}

func hasWarningWithText(warnings []FlowWarning, code string, contains string) bool {
	for _, w := range warnings {
		if w.Code == code && strings.Contains(w.Message, contains) {
			return true
		}
	}
	return false
}
