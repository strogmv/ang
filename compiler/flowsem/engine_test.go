package flowsem

import "testing"

func TestValidate_RepoGetForUpdateRequiresTx(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "repo.GetForUpdate"}})
	found := false
	for _, it := range issues {
		if it.Code == "TX_REQUIRED" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected TX_REQUIRED issue")
	}
}

func TestValidate_RepoGetRequiresErrorGuard(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "repo.Get",
		Args: map[string]any{
			"source": "Tender",
			"input":  "req.ID",
			"output": "tender",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "REPO_GET_MISSING_ERROR" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected REPO_GET_MISSING_ERROR issue, got %+v", issues)
	}
}

func TestValidate_RepoFindWithoutErrorWarns(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "repo.Find",
		Args: map[string]any{
			"source": "Tender",
			"input":  "req.ID",
			"output": "tender",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "REPO_FIND_WITHOUT_ERROR" {
			if it.Severity != "warn" {
				t.Fatalf("expected warn severity for REPO_FIND_WITHOUT_ERROR, got %q", it.Severity)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected REPO_FIND_WITHOUT_ERROR issue, got %+v", issues)
	}
}

func TestValidate_RepoGetWithErrorPassesGuardCheck(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "repo.Get",
		Args: map[string]any{
			"source": "Tender",
			"input":  "req.ID",
			"output": "tender",
			"error":  "Not found",
		},
	}})
	for _, it := range issues {
		if it.Code == "REPO_GET_MISSING_ERROR" {
			t.Fatalf("did not expect REPO_GET_MISSING_ERROR when error is provided: %+v", issues)
		}
	}
}

func TestValidate_UpsertRequiresBranch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "repo.Upsert",
		Args: map[string]any{
			"source": "User",
			"find":   "FindByEmail",
			"input":  "req.Email",
			"output": "user",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_BRANCHES" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_BRANCHES issue")
	}
}

func TestValidate_UnknownAction(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "foo.Bar"}})
	found := false
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_ACTION issue")
	}
}

func TestValidate_FlowSwitchRequiresCases(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "flow.Switch",
		Args:   map[string]any{"value": "req.Role"},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_CASES" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_CASES issue")
	}
}

func TestValidate_ListEnrichSetFormat(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "list.Enrich",
		Args: map[string]any{
			"items":        "items",
			"lookupSource": "Company",
			"lookupInput":  "item.CompanyID",
			"set":          "AuthorName = Name",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "INVALID_SET_FORMAT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected INVALID_SET_FORMAT issue")
	}
}

func TestValidate_FlowTryRequiresDo(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "flow.Try"}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_DO" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_DO issue")
	}
}

func TestValidate_FlowFallbackRequiresFallbackBranch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action:   "flow.Fallback",
		Children: map[string][]Step{"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "nope"}}}},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_FALLBACK" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_FALLBACK issue")
	}
}

func TestValidate_FlowTimeoutRequiresDuration(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action:   "flow.Timeout",
		Children: map[string][]Step{"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "nope"}}}},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_DURATION" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_DURATION issue")
	}
}

func TestValidate_FlowSuggestNextRequiresOptions(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "flow.SuggestNext", Args: map[string]any{}}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_OPTIONS" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_OPTIONS issue")
	}
}

func TestValidate_DeterministicFlowActionsKnown(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "flow.RecordEvent", Args: map[string]any{"name": `"project.created"`, "payload": "req", "output": "evt"}},
		{Action: "flow.History.Get", Args: map[string]any{"output": "hist"}},
		{Action: "flow.Replay", Args: map[string]any{"history": "hist", "output": "replayed"}},
		{Action: "flow.Call", Args: map[string]any{"op": "Tender.GetTender", "args": map[string]string{"id": "req.ID"}, "output": "tenderResp"}},
	})
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_FlowCallRequiresOp(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "flow.Call", Args: map[string]any{}},
	})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_OP" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_OP issue, got %+v", issues)
	}
}

func TestValidate_DeterministicFlowRequiredArgs(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "flow.RecordEvent", Args: map[string]any{}},
		{Action: "flow.History.Get", Args: map[string]any{}},
		{Action: "flow.Replay", Args: map[string]any{}},
	})
	want := map[string]bool{
		"MISSING_NAME":    true,
		"MISSING_OUTPUT":  true,
		"MISSING_HISTORY": true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue, got %+v", code, issues)
	}
}

func TestValidate_PerformanceArgs(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "http.Call",
			Args:   map[string]any{"method": "GET", "url": "https://api.test", "attempts": 0},
		},
		{
			Action: "parallel.Run",
			Args:   map[string]any{"maxConcurrency": 0},
			Children: map[string][]Step{
				"_branches": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{
			Action: "queue.Enqueue",
			Args:   map[string]any{"subject": "events", "payload": "req", "timeoutMs": 0},
		},
	})

	want := map[string]bool{
		"INVALID_ATTEMPTS":        true,
		"INVALID_MAX_CONCURRENCY": true,
		"INVALID_TIMEOUT_MS":      true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_RequiredArgTypeMismatch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "logic.Check",
		Args: map[string]any{
			"condition": true,
			"throw":     "bad",
		},
	}})
	found := false
	for _, it := range issues {
		if it.Code == "INVALID_CONDITION_TYPE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected INVALID_CONDITION_TYPE issue, got %+v", issues)
	}
}

func TestValidate_InvalidGoExprInRawArgs(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.ID",
				"value": "req..UserID",
			},
		},
		{
			Action: "math.Expr",
			Args: map[string]any{
				"expr":   "req.Price *",
				"output": "price",
			},
		},
		{
			Action: "logic.Check",
			Args: map[string]any{
				"condition": "req.CompanyID ==",
				"throw":     "invalid",
			},
		},
		{
			Action: "logic.Call",
			Args: map[string]any{
				"func": "s.AuthService..Login",
			},
		},
	})

	count := 0
	for _, it := range issues {
		if it.Code == "INVALID_GO_EXPR" {
			count++
		}
	}
	if count != 4 {
		t.Fatalf("expected 4 INVALID_GO_EXPR issues, got %d: %+v", count, issues)
	}
}

func TestValidate_MappingAssignSafeValueWhitelist(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.UserID", "value": "req.UserID"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Status", "value": "\"draft\""}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Count", "value": "42"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Enabled", "value": "true"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.ID", "value": "uuid.NewString()"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Now", "value": "time.Now().UTC()"}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "resp.NowRFC3339", "value": "time.Now().UTC().Format(time.RFC3339)"}},
	})

	for _, it := range issues {
		if it.Code == "INVALID_GO_EXPR" || it.Code == "RAW_GO_EXPR_IN_ASSIGN" {
			t.Fatalf("did not expect mapping.Assign expression issue, got %+v", issues)
		}
	}
}

func TestValidate_MappingAssignRawGoExprWarning(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "mapping.Assign",
			Args: map[string]any{
				"to":    "resp.Name",
				"value": "strings.TrimSpace(req.Name)",
			},
		},
	})

	for _, it := range issues {
		if it.Code == "RAW_GO_EXPR_IN_ASSIGN" {
			if it.Severity != "warn" {
				t.Fatalf("expected warn severity, got %q", it.Severity)
			}
			return
		}
	}
	t.Fatalf("expected RAW_GO_EXPR_IN_ASSIGN issue, got %+v", issues)
}

func TestValidate_OptionalArgTypeMismatch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "http.Call",
			Args: map[string]any{
				"method":      "GET",
				"url":         "https://api.test",
				"attempts":    "2",
				"failOnError": "yes",
			},
		},
		{
			Action: "cache.Get",
			Args: map[string]any{
				"key":      "k",
				"output":   "v",
				"optional": "true",
			},
		},
		{
			Action: "list.Sort",
			Args: map[string]any{
				"items": "items",
				"by":    "Name",
				"order": true,
			},
		},
		{
			Action: "str.Normalize",
			Args: map[string]any{
				"input":  "req.Name",
				"output": "name",
				"mode":   false,
			},
		},
	})
	want := map[string]bool{
		"INVALID_ATTEMPTS_TYPE":    true,
		"INVALID_FAILONERROR_TYPE": true,
		"INVALID_OPTIONAL_TYPE":    true,
		"INVALID_ORDER_TYPE":       true,
		"INVALID_MODE_TYPE":        true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_OptionalArgValueMismatch(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "list.Sort",
			Args: map[string]any{
				"items": "items",
				"by":    "Name",
				"order": "descending",
			},
		},
		{
			Action: "str.Normalize",
			Args: map[string]any{
				"input":  "req.Name",
				"output": "name",
				"mode":   "snake",
			},
		},
	})
	want := map[string]bool{
		"INVALID_ORDER": true,
		"INVALID_MODE":  true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_OptionalArgDynamicExpressionAllowed(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "list.Sort",
			Args: map[string]any{
				"items": "items",
				"by":    "Name",
				"order": "req.SortOrder",
			},
		},
		{
			Action: "str.Normalize",
			Args: map[string]any{
				"input":  "req.Name",
				"output": "name",
				"mode":   "req.NormalizeMode",
			},
		},
	})
	for _, it := range issues {
		if it.Code == "INVALID_ORDER" || it.Code == "INVALID_MODE" {
			t.Fatalf("did not expect enum literal issue for dynamic expression, got %+v", issues)
		}
	}
}

func TestValidate_StorageDeleteRequiresKey(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "storage.Delete"}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_KEY issue, got %+v", issues)
	}
}

func TestValidate_StorageListRequiresOutput(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "storage.List", Args: map[string]any{"prefix": `"projects/p1/"`}}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_OUTPUT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_OUTPUT issue, got %+v", issues)
	}
}

func TestValidate_NewDataTransformActionsKnown(t *testing.T) {
	t.Parallel()
	steps := []Step{
		{Action: "regex.Match", Args: map[string]any{"input": "req.Email", "pattern": `"^[^@]+@[^@]+$"`, "output": "ok"}},
		{Action: "regex.Replace", Args: map[string]any{"input": "req.Name", "pattern": `"\\s+"`, "repl": `"-"`, "output": "slug"}},
		{Action: "base64.Encode", Args: map[string]any{"input": "req.Payload", "output": "b64"}},
		{Action: "base64.Decode", Args: map[string]any{"input": "req.Encoded", "output": "raw"}},
		{Action: "url.Parse", Args: map[string]any{"input": "req.URL", "output": "u"}},
		{Action: "url.Build", Args: map[string]any{"base": `"https://api.test"`, "output": "u"}},
		{Action: "query.Encode", Args: map[string]any{"input": "req.QueryMap", "output": "rawQuery"}},
		{Action: "query.Decode", Args: map[string]any{"input": "req.RawQuery", "output": "vals"}},
		{Action: "hash.Sum", Args: map[string]any{"algorithm": `"sha256"`, "input": "req.Payload", "output": "digest"}},
		{Action: "hash.HMAC", Args: map[string]any{"algorithm": `"sha256"`, "key": "req.Secret", "input": "req.Payload", "output": "sig"}},
		{Action: "uuid.New", Args: map[string]any{"output": "id"}},
		{Action: "time.Now", Args: map[string]any{"output": "nowTs"}},
		{Action: "ulid.New", Args: map[string]any{"output": "ulid"}},
		{Action: "math.Op", Args: map[string]any{"op": `"min"`, "a": "x", "b": "y", "output": "m"}},
		{Action: "jsonpath.Get", Args: map[string]any{"input": "req.Payload", "path": `"$.user.email"`, "output": "email"}},
		{Action: "jsonpath.Set", Args: map[string]any{"input": "req.Payload", "path": `"$.user.role"`, "value": `"admin"`, "output": "patched"}},
	}

	issues := Validate(steps)
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_MathOpRequiredArgsByOp(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{Action: "math.Op", Args: map[string]any{"op": `"clamp"`, "value": "x", "max": "10", "output": "y"}}})
	found := false
	for _, it := range issues {
		if it.Code == "MISSING_MIN" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected MISSING_MIN issue, got %+v", issues)
	}
}

func TestValidate_CollectionsPrimitivesKnown(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "list.Map", Args: map[string]any{"from": "items", "expr": "item.ID", "output": "ids", "as": "item"}},
		{Action: "list.Reduce", Args: map[string]any{"from": "amounts", "expr": "sum + item", "output": "sum", "as": "item"}},
		{Action: "list.GroupBy", Args: map[string]any{"from": "items", "key": "item.Status", "output": "byStatus", "as": "item"}},
		{Action: "list.Distinct", Args: map[string]any{"from": "items", "key": "item.ID", "output": "unique", "as": "item"}},
		{Action: "list.Chunk", Args: map[string]any{"from": "items", "size": 100, "output": "batches"}},
		{
			Action: "batch.Run",
			Args:   map[string]any{"from": "items", "size": 50, "as": "batch"},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "nope"}}},
			},
		},
	})
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_ListChunkSizeRules(t *testing.T) {
	t.Parallel()

	issuesMissing := Validate([]Step{{Action: "list.Chunk", Args: map[string]any{"from": "items", "output": "batches"}}})
	foundMissing := false
	for _, it := range issuesMissing {
		if it.Code == "MISSING_SIZE" {
			foundMissing = true
			break
		}
	}
	if !foundMissing {
		t.Fatalf("expected MISSING_SIZE issue, got %+v", issuesMissing)
	}

	issuesInvalid := Validate([]Step{{Action: "list.Chunk", Args: map[string]any{"from": "items", "size": 0, "output": "batches"}}})
	foundInvalid := false
	for _, it := range issuesInvalid {
		if it.Code == "INVALID_SIZE" {
			foundInvalid = true
			break
		}
	}
	if !foundInvalid {
		t.Fatalf("expected INVALID_SIZE issue, got %+v", issuesInvalid)
	}
}

func TestValidate_BatchRunRequiresDoAndValidSize(t *testing.T) {
	t.Parallel()

	issuesMissingDo := Validate([]Step{{Action: "batch.Run", Args: map[string]any{"from": "items", "size": 10}}})
	foundMissingDo := false
	for _, it := range issuesMissingDo {
		if it.Code == "MISSING_DO" {
			foundMissingDo = true
			break
		}
	}
	if !foundMissingDo {
		t.Fatalf("expected MISSING_DO issue, got %+v", issuesMissingDo)
	}

	issuesInvalidSize := Validate([]Step{{
		Action: "batch.Run",
		Args:   map[string]any{"from": "items", "size": 0},
		Children: map[string][]Step{
			"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "x"}}},
		},
	}})
	foundInvalidSize := false
	for _, it := range issuesInvalidSize {
		if it.Code == "INVALID_SIZE" {
			foundInvalidSize = true
			break
		}
	}
	if !foundInvalidSize {
		t.Fatalf("expected INVALID_SIZE issue, got %+v", issuesInvalidSize)
	}
}

func TestValidate_NewSecurityActionsKnown(t *testing.T) {
	t.Parallel()
	steps := []Step{
		{Action: "jwt.Sign", Args: map[string]any{"claims": "map[string]any{\"sub\": req.UserID}", "output": "token"}},
		{Action: "jwt.Verify", Args: map[string]any{"token": "req.Token", "output": "claims"}},
		{Action: "oauth2.Token", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "output": "tok"}},
		{Action: "oauth2.Refresh", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "refreshToken": "req.RefreshToken", "output": "tok"}},
		{Action: "crypto.Encrypt", Args: map[string]any{"input": "req.Payload", "output": "cipher"}},
		{Action: "crypto.Decrypt", Args: map[string]any{"input": "req.Cipher", "output": "plain"}},
		{Action: "rbac.CheckPermission", Args: map[string]any{"user": "currentUser", "permission": `"project.create"`}},
	}

	issues := Validate(steps)
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_NewSecurityActionsRequiredArgs(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "jwt.Sign", Args: map[string]any{"output": "token"}},
		{Action: "jwt.Verify", Args: map[string]any{"output": "claims"}},
		{Action: "oauth2.Token", Args: map[string]any{"output": "tok"}},
		{Action: "oauth2.Refresh", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "output": "tok"}},
		{Action: "crypto.Encrypt", Args: map[string]any{"output": "cipher"}},
		{Action: "crypto.Decrypt", Args: map[string]any{"output": "plain"}},
		{Action: "rbac.CheckPermission", Args: map[string]any{"user": "currentUser"}},
	})

	want := map[string]bool{
		"MISSING_CLAIMS":       true,
		"MISSING_TOKEN":        true,
		"MISSING_TOKENURL":     true,
		"MISSING_REFRESHTOKEN": true,
		"MISSING_INPUT":        true,
		"MISSING_PERMISSION":   true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_ReliabilityAndObservabilityActionsKnown(t *testing.T) {
	t.Parallel()
	steps := []Step{
		{Action: "idempotency.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}},
		{Action: "idempotency.Check", Args: map[string]any{"key": "idemKey"}},
		{Action: "idempotency.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}},
		{Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID", "rps": 20}},
		{
			Action: "concurrency.Run",
			Args:   map[string]any{"key": `"build"`, "max": 8},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{
			Action: "circuit.Breaker",
			Args:   map[string]any{"name": `"external-api"`, "threshold": 3, "openTTL": "30*time.Second"},
			Children: map[string][]Step{
				"_do": {{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://api.test"`, "output": "body"}}},
			},
		},
		{
			Action: "bulkhead.Run",
			Args:   map[string]any{"name": `"s3-upload"`, "max": 16},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{Action: "log.Emit", Args: map[string]any{"level": `"info"`, "message": `"project created"`}},
		{Action: "metric.Emit", Args: map[string]any{"name": `"project.created"`, "kind": `"counter"`, "value": "1"}},
		{
			Action: "trace.Span",
			Args:   map[string]any{"name": `"BuildProject"`},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{
			Action: "slo.Budget",
			Args:   map[string]any{"name": `"build-flow"`, "duration": "2*time.Second"},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
	}

	issues := Validate(steps)
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_ReliabilityAndObservabilityConstraints(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID"}},
		{
			Action: "concurrency.Run",
			Args:   map[string]any{"key": `"build"`},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{
			Action: "circuit.Breaker",
			Args:   map[string]any{"name": `"external-api"`},
		},
		{
			Action: "bulkhead.Run",
			Args:   map[string]any{"name": `"s3-upload"`},
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
		{Action: "log.Emit", Args: map[string]any{"level": `"info"`}},
		{
			Action: "trace.Span",
			Args:   map[string]any{"name": `"BuildProject"`},
		},
		{
			Action: "slo.Budget",
			Children: map[string][]Step{
				"_do": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		},
	})

	want := map[string]bool{
		"MISSING_RPS":      true,
		"MISSING_MAX":      true,
		"MISSING_DO":       true,
		"MISSING_MESSAGE":  true,
		"MISSING_DURATION": true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_ApprovalAndNotifyActionsKnown(t *testing.T) {
	t.Parallel()
	steps := []Step{
		{
			Action: "approval.Request",
			Args: map[string]any{
				"approvalKey": "req.OrderID",
				"title":       `"Refund approval"`,
				"requestedBy": "req.UserID",
				"approvers":   []string{"finance@company.com"},
				"policy":      `"any"`,
				"payload":     "req",
				"approvalId":  "approvalID",
				"status":      "approvalStatus",
			},
		},
		{
			Action: "approval.Wait",
			Args: map[string]any{
				"approvalId": "approvalID",
				"timeout":    "5*time.Minute",
				"onTimeout":  `"fallback"`,
				"decision":   "decision",
				"status":     "status",
				"decidedBy":  "decidedBy",
				"decidedAt":  "decidedAt",
				"reason":     "reason",
			},
			Children: map[string][]Step{
				"_onTimeout": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"retry"}}}},
			},
		},
		{Action: "approval.Decide", Args: map[string]any{"approvalId": "approvalID", "decision": `"approved"`, "actor": "req.UserID", "status": "approvalStatus"}},
		{Action: "notify.Send", Args: map[string]any{"channel": `"email"`, "to": "req.Email", "text": `"Build completed"`}},
		{
			Action: "policy.Evaluate",
			Args: map[string]any{
				"policyKey": "req.PolicyKey",
				"subject":   "req.UserID",
				"resource":  "req.ProjectID",
				"operation": `"project.create"`,
				"tenant":    "req.TenantID",
				"attrs":     "req.Attrs",
				"context":   "req",
				"decision":  "policyDecision",
				"reason":    "policyReason",
				"effects":   "policyEffects",
				"output":    "policyResult",
			},
		},
		{
			Action: "policy.Require",
			Args: map[string]any{
				"policyKey": "req.PolicyKey",
				"subject":   "req.UserID",
				"resource":  "req.ProjectID",
				"operation": `"project.create"`,
				"tenant":    "req.TenantID",
				"throw":     `"policy denied"`,
				"code":      `"POLICY_DENIED"`,
				"status":    "http.StatusForbidden",
				"decision":  "requireDecision",
			},
		},
		{
			Action: "policy.Decide",
			Args: map[string]any{
				"policyKey": "req.PolicyKey",
				"output":    "finalDecision",
				"decision":  "finalDecisionName",
			},
		},
	}

	issues := Validate(steps)
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_ApprovalAndNotifyConstraints(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{
		{
			Action: "approval.Request",
			Args: map[string]any{
				"approvalKey": "req.OrderID",
				"title":       `"Refund approval"`,
				"requestedBy": "req.UserID",
				"approvers":   []string{},
				"policy":      `"any"`,
				"payload":     "req",
			},
		},
		{
			Action: "approval.Request",
			Args: map[string]any{
				"approvalKey": "req.OrderID",
				"title":       `"Refund approval"`,
				"requestedBy": "req.UserID",
				"approvers":   []string{"manager@acme.io"},
				"policy":      `"invalid"`,
				"payload":     "req",
			},
		},
		{
			Action: "approval.Wait",
			Args: map[string]any{
				"approvalId": "approvalID",
				"onTimeout":  `"later"`,
			},
		},
		{
			Action: "approval.Decide",
			Args: map[string]any{
				"approvalId": "approvalID",
				"decision":   `"maybe"`,
				"actor":      "req.UserID",
			},
		},
		{Action: "notify.Send", Args: map[string]any{"channel": `"email"`, "to": "req.Email"}},
		{Action: "policy.Evaluate", Args: map[string]any{"policyKey": "req.PolicyKey"}},
		{Action: "policy.Decide", Args: map[string]any{"policyKey": "req.PolicyKey"}},
	})

	want := map[string]bool{
		"MISSING_APPROVERS":     true,
		"INVALID_POLICY":        true,
		"INVALID_ON_TIMEOUT":    true,
		"INVALID_DECISION":      true,
		"MISSING_CONTENT":       true,
		"MISSING_POLICY_OUTPUT": true,
		"MISSING_OUTPUT":        true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_MessagingAsyncActionsKnown(t *testing.T) {
	t.Parallel()

	steps := []Step{
		{Action: "webhook.VerifySignature", Args: map[string]any{"payload": "req.Body", "signature": "req.Signature", "output": "sigOK", "strict": false}},
		{Action: "webhook.Ack", Args: map[string]any{"status": 202, "body": `"accepted"`}},
		{Action: "queue.Enqueue", Args: map[string]any{"subject": "events.core", "payload": "req", "timeoutMs": 1000}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "ackToken": "msgID", "timeoutMs": 1000, "attempts": 3, "backoffMs": 100, "jitterMs": 50}},
		{Action: "queue.Ack", Args: map[string]any{"subject": "events.core", "messageID": "msgID"}},
		{Action: "queue.Nack", Args: map[string]any{"subject": "events.core", "messageID": "msgID", "reason": `"decode failed"`}},
		{Action: "dlq.Publish", Args: map[string]any{"subject": "events.core", "payload": "msg", "reason": `"decode failed"`}},
		{
			Action: "tx.Block",
			Children: map[string][]Step{
				"_do": {{
					Action: "event.Outbox",
					Args:   map[string]any{"name": "ProjectCreated", "payload": "domain.ProjectCreated{ID: req.ID}"},
				}},
			},
		},
	}

	issues := Validate(steps)
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_MessagingAsyncActionConstraints(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "timeoutMs": 0}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "attempts": 0}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "retries": -1}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "backoffMs": -1}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": "events.core", "output": "msg", "jitterMs": -1}},
		{Action: "event.Outbox", Args: map[string]any{"name": "ProjectCreated", "payload": "req"}},
	})

	want := map[string]bool{
		"INVALID_TIMEOUT_MS": true,
		"INVALID_ATTEMPTS":   true,
		"INVALID_RETRIES":    true,
		"INVALID_BACKOFF":    true,
		"INVALID_JITTER":     true,
		"TX_REQUIRED":        true,
	}
	for _, it := range issues {
		delete(want, it.Code)
	}
	for code := range want {
		t.Fatalf("expected %s issue in %+v", code, issues)
	}
}

func TestValidate_EventPublishUnknownEvent(t *testing.T) {
	t.Parallel()

	issues := ValidateWithOptions([]Step{{
		Action: "event.Publish",
		Args: map[string]any{
			"name":       "TenderReportReady",
			"payloadMap": map[string]any{"TenderID": "req.TenderID"},
		},
	}}, ValidateOptions{
		Events: []EventDef{
			{Name: "BidPlaced", Fields: []EventField{{Name: "BidID"}}},
		},
	})

	found := false
	for _, it := range issues {
		if it.Code == "UNKNOWN_EVENT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_EVENT, got %+v", issues)
	}
}

func TestValidate_EventPublishPayloadMapAgainstSchema(t *testing.T) {
	t.Parallel()

	issues := ValidateWithOptions([]Step{{
		Action: "event.Publish",
		Args: map[string]any{
			"name": "BidPlaced",
			"payloadMap": map[string]any{
				"BidID":      "bid.ID",
				"NotAField":  "bid.Price",
				"CreatedAt":  "bid.CreatedAt",
				"CompanyID":  "bid.CompanyID",
				"TenderID":   "bid.TenderID",
				"SupplierID": "bid.SupplierID",
			},
		},
	}}, ValidateOptions{
		Events: []EventDef{
			{
				Name: "BidPlaced",
				Fields: []EventField{
					{Name: "BidID"},
					{Name: "CreatedAt"},
					{Name: "CompanyID"},
					{Name: "TenderID"},
					{Name: "SupplierID"},
				},
			},
		},
	})

	found := false
	for _, it := range issues {
		if it.Code == "PAYLOAD_FIELD_NOT_IN_EVENT" && it.Action == "event.Publish" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PAYLOAD_FIELD_NOT_IN_EVENT for event.Publish, got %+v", issues)
	}
}

func TestValidate_EventOutboxAndBroadcastPayloadMapAgainstSchema(t *testing.T) {
	t.Parallel()

	issues := ValidateWithOptions([]Step{
		{
			Action: "event.Outbox",
			Args: map[string]any{
				"name":       "OrderCreated",
				"payload":    "order",
				"payloadMap": map[string]any{"OrderID": "order.ID", "Unknown": "order.Status"},
			},
		},
		{
			Action: "event.Broadcast",
			Args: map[string]any{
				"name":       "OrderCreated",
				"payloadMap": map[string]any{"OrderID": "order.ID", "AnotherUnknown": "order.Status"},
			},
		},
	}, ValidateOptions{
		Events: []EventDef{
			{Name: "OrderCreated", Fields: []EventField{{Name: "OrderID"}}},
		},
	})

	foundOutbox := false
	foundBroadcast := false
	for _, it := range issues {
		if it.Code != "PAYLOAD_FIELD_NOT_IN_EVENT" {
			continue
		}
		if it.Action == "event.Outbox" {
			foundOutbox = true
		}
		if it.Action == "event.Broadcast" {
			foundBroadcast = true
		}
	}
	if !foundOutbox || !foundBroadcast {
		t.Fatalf("expected PAYLOAD_FIELD_NOT_IN_EVENT for outbox+broadcast, got %+v", issues)
	}
}

func TestValidateServiceCallKnown(t *testing.T) {
	t.Parallel()
	issues := Validate([]Step{{
		Action: "service.Call",
		Args: map[string]any{
			"service": "Tender",
			"method":  "GetByID",
			"args":    []string{"ctx", "req.TenderID"},
		},
	}})
	for _, it := range issues {
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
}

func TestValidate_TimeFormatKnownAndConstraints(t *testing.T) {
	t.Parallel()

	issues := Validate([]Step{
		{Action: "time.Format", Args: map[string]any{"input": "item.CreatedAt", "output": "createdAtStr", "format": "time.RFC3339"}},
		{Action: "time.Format", Args: map[string]any{"input": "item.CreatedAt"}},
	})

	foundMissingOutput := false
	for _, it := range issues {
		if it.Code == "MISSING_OUTPUT" {
			foundMissingOutput = true
		}
		if it.Code == "UNKNOWN_ACTION" {
			t.Fatalf("unexpected UNKNOWN_ACTION for %s", it.Action)
		}
	}
	if !foundMissingOutput {
		t.Fatalf("expected MISSING_OUTPUT for invalid time.Format step, got %+v", issues)
	}
}
