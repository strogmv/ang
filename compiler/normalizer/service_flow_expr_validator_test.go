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
		t.Fatalf("expected REPO_FIND_WITHOUT_ERROR, got: %+v", warnings)
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

func TestValidateFlowSteps_ListAppendMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "list.Append", Args: map[string]any{}}}
	warnings := validateFlowSteps("Append", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_TO", "list.Append missing 'to'") {
		t.Fatalf("expected MISSING_TO, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_ITEM", "list.Append missing 'item'") {
		t.Fatalf("expected MISSING_ITEM, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ListSortMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "list.Sort", Args: map[string]any{}}}
	warnings := validateFlowSteps("Sort", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_ITEMS", "list.Sort missing 'items'") {
		t.Fatalf("expected MISSING_ITEMS, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_BY", "list.Sort missing 'by'") {
		t.Fatalf("expected MISSING_BY, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ListFilterMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "list.Filter", Args: map[string]any{}}}
	warnings := validateFlowSteps("Filter", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_FROM", "list.Filter missing 'from'") {
		t.Fatalf("expected MISSING_FROM, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_CONDITION", "list.Filter missing 'condition'") {
		t.Fatalf("expected MISSING_CONDITION, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "list.Filter missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ListPaginateMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "list.Paginate", Args: map[string]any{}}}
	warnings := validateFlowSteps("Paginate", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_INPUT", "list.Paginate missing 'input'") {
		t.Fatalf("expected MISSING_INPUT, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OFFSET", "list.Paginate missing 'offset'") {
		t.Fatalf("expected MISSING_OFFSET, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_LIMIT", "list.Paginate missing 'limit'") {
		t.Fatalf("expected MISSING_LIMIT, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "list.Paginate missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ListEnrichMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "list.Enrich", Args: map[string]any{}}}
	warnings := validateFlowSteps("Enrich", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_ITEMS", "list.Enrich missing 'items'") {
		t.Fatalf("expected MISSING_ITEMS, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_LOOKUP_SOURCE", "list.Enrich missing 'lookupSource'") {
		t.Fatalf("expected MISSING_LOOKUP_SOURCE, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_LOOKUP_INPUT", "list.Enrich missing 'lookupInput'") {
		t.Fatalf("expected MISSING_LOOKUP_INPUT, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_SET", "list.Enrich missing 'set'") {
		t.Fatalf("expected MISSING_SET, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowIfMissingConditionAndThen(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "flow.If", Args: map[string]any{}}}
	warnings := validateFlowSteps("IfOp", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_CONDITION", "flow.If missing 'condition'") {
		t.Fatalf("expected MISSING_CONDITION, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_THEN", "flow.If requires non-empty 'then' block") {
		t.Fatalf("expected MISSING_THEN, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowForMissingEachAsDo(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "flow.For", Args: map[string]any{}}}
	warnings := validateFlowSteps("ForOp", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_EACH", "flow.For missing 'each'") {
		t.Fatalf("expected MISSING_EACH, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_AS", "flow.For missing 'as'") {
		t.Fatalf("expected MISSING_AS, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_DO", "flow.For requires non-empty 'do' block") {
		t.Fatalf("expected MISSING_DO for flow.For, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowWhileMissingConditionAndDo(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "flow.While", Args: map[string]any{}}}
	warnings := validateFlowSteps("WhileOp", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_CONDITION", "flow.While missing 'condition'") {
		t.Fatalf("expected MISSING_CONDITION, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_DO", "flow.While requires non-empty 'do' block") {
		t.Fatalf("expected MISSING_DO for flow.While, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_FlowSwitchMissingValueAndCases(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{{Action: "flow.Switch", Args: map[string]any{}}}
	warnings := validateFlowSteps("SwitchOp", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_VALUE", "flow.Switch missing 'value'") {
		t.Fatalf("expected MISSING_VALUE, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_CASES", "flow.Switch requires at least one case") {
		t.Fatalf("expected MISSING_CASES, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_BlocksMissingDo(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "tx.Block", Args: map[string]any{}},
		{Action: "flow.Block", Args: map[string]any{}},
	}
	warnings := validateFlowSteps("BlockOp", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "MISSING_DO", "tx.Block requires non-empty 'do' block") {
		t.Fatalf("expected MISSING_DO for tx.Block, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_DO", "flow.Block requires non-empty 'do' block") {
		t.Fatalf("expected MISSING_DO for flow.Block, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_DomainActionsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "audit.Log", Args: map[string]any{}},
		{Action: "auth.RequireRole", Args: map[string]any{}},
		{Action: "auth.CheckRole", Args: map[string]any{}},
		{Action: "rbac.CheckPermission", Args: map[string]any{}},
		{Action: "entity.PatchNonZero", Args: map[string]any{}},
		{Action: "entity.PatchValidated", Args: map[string]any{}},
		{Action: "enum.Validate", Args: map[string]any{}},
		{Action: "fsm.Transition", Args: map[string]any{}},
	}

	warnings := validateFlowSteps("DomainActions", "tender", steps, nil, nil, nil, "strict", nil)

	if !hasWarningWithText(warnings, "MISSING_ACTOR", "audit.Log missing 'actor'") {
		t.Fatalf("expected MISSING_ACTOR for audit.Log, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_COMPANY", "audit.Log missing 'company'") {
		t.Fatalf("expected MISSING_COMPANY for audit.Log, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_EVENT", "audit.Log missing 'event'") {
		t.Fatalf("expected MISSING_EVENT for audit.Log, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_USER_ID", "auth.RequireRole missing 'userID'") {
		t.Fatalf("expected MISSING_USER_ID for auth.RequireRole, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_COMPANY_ID", "auth.RequireRole missing 'companyID'") {
		t.Fatalf("expected MISSING_COMPANY_ID for auth.RequireRole, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_ROLES", "auth.RequireRole missing 'roles'") {
		t.Fatalf("expected MISSING_ROLES for auth.RequireRole, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_USER", "auth.CheckRole missing 'user'") {
		t.Fatalf("expected MISSING_USER for auth.CheckRole, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_ROLES", "auth.CheckRole missing 'roles'") {
		t.Fatalf("expected MISSING_ROLES for auth.CheckRole, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_USER", "rbac.CheckPermission missing 'user'") {
		t.Fatalf("expected MISSING_USER for rbac.CheckPermission, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_PERMISSION", "rbac.CheckPermission missing 'permission'") {
		t.Fatalf("expected MISSING_PERMISSION for rbac.CheckPermission, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_TARGET", "entity.PatchNonZero missing 'target'") {
		t.Fatalf("expected MISSING_TARGET for entity.PatchNonZero, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_FROM", "entity.PatchNonZero missing 'from'") {
		t.Fatalf("expected MISSING_FROM for entity.PatchNonZero, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_FIELDS", "entity.PatchNonZero missing 'fields'") {
		t.Fatalf("expected MISSING_FIELDS for entity.PatchNonZero, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_TARGET", "entity.PatchValidated missing 'target'") {
		t.Fatalf("expected MISSING_TARGET for entity.PatchValidated, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_FROM", "entity.PatchValidated missing 'from'") {
		t.Fatalf("expected MISSING_FROM for entity.PatchValidated, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_FIELDS", "entity.PatchValidated missing 'fields'") {
		t.Fatalf("expected MISSING_FIELDS for entity.PatchValidated, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_VALUE", "enum.Validate missing 'value'") {
		t.Fatalf("expected MISSING_VALUE for enum.Validate, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_ALLOWED", "enum.Validate missing 'allowed'") {
		t.Fatalf("expected MISSING_ALLOWED for enum.Validate, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_THROW", "enum.Validate missing 'throw'") {
		t.Fatalf("expected MISSING_THROW for enum.Validate, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_ENTITY", "fsm.Transition missing 'entity'") {
		t.Fatalf("expected MISSING_ENTITY for fsm.Transition, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_TO", "fsm.Transition missing 'to'") {
		t.Fatalf("expected MISSING_TO for fsm.Transition, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_RepoUpsertMissingRequiredFields(t *testing.T) {
	t.Parallel()

	entities := []Entity{{Name: "Tender", Owner: "tender", BoundedContext: "tender"}}
	steps := []FlowStep{{Action: "repo.Upsert", Args: map[string]any{}}}

	warnings := validateFlowSteps("Upsert", "tender", steps, entities, nil, nil, "strict", nil)

	if !hasWarningWithText(warnings, "MISSING_SOURCE", "repo.Upsert missing 'source'") {
		t.Fatalf("expected MISSING_SOURCE, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_FIND", "repo.Upsert missing 'find'") {
		t.Fatalf("expected MISSING_FIND, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_INPUT", "repo.Upsert missing 'input'") {
		t.Fatalf("expected MISSING_INPUT, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "repo.Upsert missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_BRANCHES", "repo.Upsert requires at least one branch") {
		t.Fatalf("expected MISSING_BRANCHES, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_InfrastructureActionsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{Action: "cache.Get", Args: map[string]any{}},
		{Action: "cache.Set", Args: map[string]any{}},
		{Action: "jwt.Sign", Args: map[string]any{}},
		{Action: "jwt.Verify", Args: map[string]any{}},
		{Action: "storage.Upload", Args: map[string]any{}},
		{Action: "crypto.Hash", Args: map[string]any{}},
		{Action: "http.Call", Args: map[string]any{}},
	}

	warnings := validateFlowSteps("Infra", "tender", steps, nil, nil, nil, "strict", nil)

	if !hasWarningWithText(warnings, "MISSING_KEY", "cache.Get missing 'key'") {
		t.Fatalf("expected MISSING_KEY for cache.Get, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "cache.Get missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT for cache.Get, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_KEY", "cache.Set missing 'key'") {
		t.Fatalf("expected MISSING_KEY for cache.Set, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_VALUE", "cache.Set missing 'value'") {
		t.Fatalf("expected MISSING_VALUE for cache.Set, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_CLAIMS", "jwt.Sign missing 'claims'") {
		t.Fatalf("expected MISSING_CLAIMS for jwt.Sign, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "jwt.Sign missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT for jwt.Sign, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_TOKEN", "jwt.Verify missing 'token'") {
		t.Fatalf("expected MISSING_TOKEN for jwt.Verify, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "jwt.Verify missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT for jwt.Verify, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_KEY", "storage.Upload missing 'key'") {
		t.Fatalf("expected MISSING_KEY for storage.Upload, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_DATA", "storage.Upload missing 'data'") {
		t.Fatalf("expected MISSING_DATA for storage.Upload, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_INPUT", "crypto.Hash missing 'input'") {
		t.Fatalf("expected MISSING_INPUT for crypto.Hash, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_OUTPUT", "crypto.Hash missing 'output'") {
		t.Fatalf("expected MISSING_OUTPUT for crypto.Hash, got: %+v", warnings)
	}

	if !hasWarningWithText(warnings, "MISSING_METHOD", "http.Call missing 'method'") {
		t.Fatalf("expected MISSING_METHOD for http.Call, got: %+v", warnings)
	}
	if !hasWarningWithText(warnings, "MISSING_URL", "http.Call missing 'url'") {
		t.Fatalf("expected MISSING_URL for http.Call, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_VarFromIfBranchCannotEscape(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "flow.If",
			Args: map[string]any{
				"condition": "req.IsAdmin",
				"_then": []FlowStep{
					{
						Action: "repo.Find",
						Args: map[string]any{
							"source": "Policy",
							"input":  "req.PolicyID",
							"output": "policy",
							"error":  "Not found",
						},
					},
				},
			},
		},
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.Name",
				"value": "policy.Name",
			},
		},
	}

	warnings := validateFlowSteps("IfScope", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNDECLARED_FLOW_VAR", "undefined flow variable 'policy'") {
		t.Fatalf("expected UNDECLARED_FLOW_VAR for policy leak from flow.If branch, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_ForAliasCannotEscapeLoop(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "flow.For",
			Args: map[string]any{
				"each": "items",
				"as":   "item",
				"_do": []FlowStep{
					{
						Action: "mapping.Assign",
						Args: map[string]any{
							"to":    "resp.LastName",
							"value": "item.Name",
						},
					},
				},
			},
		},
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.Name",
				"value": "item.Name",
			},
		},
	}

	warnings := validateFlowSteps("ForScope", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNDECLARED_FLOW_VAR", "undefined flow variable 'item'") {
		t.Fatalf("expected UNDECLARED_FLOW_VAR for loop alias leak, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_OuterVarVisibleInsideIfBranch(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "repo.Find",
			Args: map[string]any{
				"source": "Policy",
				"input":  "req.PolicyID",
				"output": "policy",
				"error":  "Not found",
			},
		},
		{
			Action: "flow.If",
			Args: map[string]any{
				"condition": "req.IsAdmin",
				"_then": []FlowStep{
					{
						Action: "mapping.Assign",
						Args: map[string]any{
							"to":    "resp.Name",
							"value": "policy.Name",
						},
					},
				},
			},
		},
	}

	warnings := validateFlowSteps("IfScopePositive", "tender", steps, nil, nil, nil, "strict", nil)
	for _, w := range warnings {
		if w.Code == "UNDECLARED_FLOW_VAR" {
			t.Fatalf("did not expect UNDECLARED_FLOW_VAR, got: %+v", warnings)
		}
	}
}

func TestValidateFlowSteps_AuditLogUndeclaredVars(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "audit.Log",
			Args: map[string]any{
				"actor":   "actorCtx.UserID",
				"company": "tenant.CompanyID",
				"event":   "\"TenderUpdated\"",
			},
		},
	}

	warnings := validateFlowSteps("AuditScope", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNDECLARED_FLOW_VAR", "undefined flow variable 'actorCtx'") {
		t.Fatalf("expected UNDECLARED_FLOW_VAR for actorCtx in audit.Log, got: %+v", warnings)
	}
}

func TestValidateFlowSteps_QueueEnqueueUndeclaredPayload(t *testing.T) {
	t.Parallel()

	steps := []FlowStep{
		{
			Action: "queue.Enqueue",
			Args: map[string]any{
				"subject": "\"events.tender\"",
				"payload": "queuedPayload",
			},
		},
	}

	warnings := validateFlowSteps("QueueScope", "tender", steps, nil, nil, nil, "strict", nil)
	if !hasWarningWithText(warnings, "UNDECLARED_FLOW_VAR", "undefined flow variable 'queuedPayload'") {
		t.Fatalf("expected UNDECLARED_FLOW_VAR for queue.Enqueue payload, got: %+v", warnings)
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
