package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestFlowRenderable(t *testing.T) {
	ok := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{"condition": "req.CompanyID != \"\"", "throw": "companyId is required"}},
		{Action: "repo.List", Args: map[string]any{"source": "Tender", "method": "ListByCompany", "input": "req.CompanyID", "output": "items"}},
		{Action: "flow.For", Args: map[string]any{"each": "items", "as": "item", "_do": []normalizer.FlowStep{
			{Action: "list.Append", Args: map[string]any{"to": "resp.Data", "item": "item"}},
		}}},
	}
	if !flowRenderable(ok) {
		t.Fatal("expected supported flow to be renderable")
	}

	bad := []normalizer.FlowStep{
		{Action: "unknown.Action", Args: map[string]any{"foo": "bar"}},
	}
	if flowRenderable(bad) {
		t.Fatal("expected unsupported flow to be non-renderable")
	}
}

func TestRenderFlow_ListSort(t *testing.T) {
	// static order
	steps := []normalizer.FlowStep{
		{Action: "repo.List", Args: map[string]any{"source": "Project", "method": "ListAll", "output": "items"}},
		{Action: "list.Sort", Args: map[string]any{"items": "items", "by": "Name"}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, `sort.Slice(items, func(i int, j int) bool`) ||
		!strings.Contains(code, `return items[i].Name < items[j].Name`) {
		t.Fatalf("expected static asc sort comparator\n\n%s", code)
	}

	// static desc
	steps2 := []normalizer.FlowStep{
		{Action: "list.Sort", Args: map[string]any{"items": "items", "by": "CreatedAt", "order": "desc"}},
	}
	code2 := renderFlow(steps2)
	if !strings.Contains(code2, `sort.Slice(items, func(i int, j int) bool`) ||
		!strings.Contains(code2, `return items[j].CreatedAt.Before(items[i].CreatedAt)`) {
		t.Fatalf("expected static desc sort comparator\n\n%s", code2)
	}

	// dynamic order from request
	steps3 := []normalizer.FlowStep{
		{Action: "list.Sort", Args: map[string]any{"items": "items", "by": "Name", "order": "req.SortOrder"}},
	}
	code3 := renderFlow(steps3)
	mustContain3 := []string{
		`sort.Slice(items, func(i int, j int) bool`,
		`strings.ToLower(req.SortOrder) == "desc"`,
		`items[i].Name > items[j].Name`,
		`items[i].Name < items[j].Name`,
	}
	for _, part := range mustContain3 {
		if !strings.Contains(code3, part) {
			t.Fatalf("expected dynamic sort to contain %q\n\n%s", part, code3)
		}
	}
}

func TestRenderFlow_FlowCall(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Call", Args: map[string]any{
			"op": "ValidateTenderForBid",
			"args": map[string]string{
				"tenderID":  "req.TenderID",
				"companyID": "req.CompanyID",
			},
			"output": "validated",
		}},
	}
	code := renderFlow(steps)
	mustContain := []string{
		"validated, err := s.ValidateTenderForBid(ctx, port.ValidateTenderForBidRequest{CompanyID: req.CompanyID, TenderID: req.TenderID})",
		"if err != nil {",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected flow.Call generation to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_FlowCallExternalServiceIgnoreErr(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Call", Args: map[string]any{
			"op":        "Tender.ValidateTenderForBid",
			"ignoreErr": true,
			"output":    "validated",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "validated, _ := s.TenderService.ValidateTenderForBid(ctx, port.ValidateTenderForBidRequest{})") {
		t.Fatalf("expected external flow.Call ignoreErr form\n\n%s", code)
	}
}

func TestRenderFlow_CollectionsPrimitives(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "list.Map", Args: map[string]any{"from": "items", "as": "item", "expr": "item.ID", "output": "ids"}},
		{Action: "list.Reduce", Args: map[string]any{"from": "items", "as": "item", "initial": "0", "expr": "total + item.Price", "output": "total"}},
		{Action: "list.GroupBy", Args: map[string]any{"from": "items", "as": "item", "key": "item.Status", "output": "byStatus"}},
		{Action: "list.Distinct", Args: map[string]any{"from": "items", "as": "item", "key": "item.ID", "output": "uniqueItems"}},
		{Action: "list.Chunk", Args: map[string]any{"from": "items", "size": 100, "output": "batches"}},
		{Action: "batch.Run", Args: map[string]any{
			"from": "items",
			"size": 50,
			"as":   "batch",
			"_do": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"next"}}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"ids := make([]any, 0, len(items))",
		"ids = append(ids, item.ID)",
		"total := 0",
		"total = total + item.Price",
		"byStatus := make(map[string][]any)",
		"byStatus[_groupKey_",
		"uniqueItems := items[:0:0]",
		"_seen_",
		"batches := make([][]any, 0)",
		"_chunkSize_",
		"for _batchStart_",
		"batch := items[_batchStart_",
		`slog.Info("flow.suggest_next", "options", []string{"next"})`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_FlowSwitch(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Switch", Args: map[string]any{
			"value": "req.Role",
			"_cases": map[string][]normalizer.FlowStep{
				"owner": {
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"approve"}}},
				},
				"guest": {
					{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"view"}}},
				},
			},
			"_default": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"retry"}}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"switch req.Role {",
		`case "guest":`,
		`case "owner":`,
		"default:",
		`slog.Info("flow.suggest_next", "options", []string{"retry"})`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_FlowBlock(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Block", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"inside"}}},
			},
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, `slog.Info("flow.suggest_next", "options", []string{"inside"})`) {
		t.Fatalf("expected flow.Block body to render\n\n%s", code)
	}
}

func TestRenderFlow_TxBlockUsesTransactionBoundContext(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "tx.Block", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "req.ID != \"\"", "throw": "id is required"}},
				{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "order"}},
			},
		}},
	}

	code := renderFlow(steps)
	for _, want := range []string{
		"s.txManager.WithTx(ctx, func(ctx context.Context) error {",
		"return errors.New(http.StatusBadRequest, \"Validation Error\", \"id is required\")",
		"s.OrderRepo.Save(ctx, &order)",
		"return nil",
		"if err := s.txManager.WithTx",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected generated tx.Block code to contain %q\n\n%s", want, code)
		}
	}
	if strings.Contains(code, "return resp, errors.New(http.StatusBadRequest") {
		t.Fatalf("tx.Block callback must return error, not the outer response:\n%s", code)
	}
}

func TestRenderFlow_TxBlockHoistsInlineLogicCallOutput(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "tx.Block", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "logic.Call", Args: map[string]any{
					"func":   "(func(ctx context.Context) ([]domain.SendTelegramAction, error) { return nil, nil })",
					"args":   []string{"ctx"},
					"output": "actions",
				}},
			},
		}},
		{Action: "flow.For", Args: map[string]any{
			"each": "actions",
			"as":   "action",
			"_do":  []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"published"}}}},
		}},
	}

	code := renderFlow(steps)
	declaration := "var actions []domain.SendTelegramAction"
	assignment := "actions, err = (func(ctx context.Context) ([]domain.SendTelegramAction, error) { return nil, nil })(ctx)"
	if !strings.Contains(code, declaration) || !strings.Contains(code, assignment) {
		t.Fatalf("expected tx.Block to hoist the typed result into outer scope:\n%s", code)
	}
	if strings.Index(code, declaration) > strings.Index(code, "s.txManager.WithTx") {
		t.Fatalf("expected transaction result to be declared before its callback:\n%s", code)
	}
}

func TestRenderFlow_ListMyTendersLike(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{"condition": "req.CompanyID != \"\"", "throw": "companyId is required"}},
		{Action: "repo.List", Args: map[string]any{"source": "Tender", "method": "ListByCompany", "input": "req.CompanyID", "output": "items"}},
		{Action: "str.Normalize", Args: map[string]any{"input": "req.Status", "mode": "lower", "output": "status"}},
		{Action: "list.Filter", Args: map[string]any{"from": "items", "as": "item", "condition": "status == \"\" || strings.EqualFold(item.Status, status)", "output": "filtered"}},
		{Action: "list.Paginate", Args: map[string]any{"input": "filtered", "offset": "req.Offset", "limit": "req.Limit", "defaultLimit": 50, "output": "page"}},
		{Action: "flow.For", Args: map[string]any{"each": "page", "as": "item", "_do": []normalizer.FlowStep{
			{Action: "list.Append", Args: map[string]any{"to": "resp.Data", "item": "item"}},
		}}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"if !(req.CompanyID != \"\")",
		"s.TenderRepo.ListByCompany(ctx, req.CompanyID)",
		"status := strings.ToLower(strings.TrimSpace(req.Status))",
		"filtered := items[:0:0]",
		"page := filtered[_start_",
		"resp.Data = append(resp.Data, item)",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated flow code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_StrNormalizeUpper(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "str.Normalize", Args: map[string]any{"input": "req.Status", "mode": "upper", "output": "status"}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "status := strings.ToUpper(strings.TrimSpace(req.Status))") {
		t.Fatalf("expected upper normalize in generated code\n\n%s", code)
	}
}

func TestRenderFlow_NoScopePollutionInLoops(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.For", Args: map[string]any{
			"each": "items",
			"as":   "item",
			"_do": []normalizer.FlowStep{
				{Action: "mapping.Assign", Args: map[string]any{"to": "tmp", "value": "item", "declare": true}},
			},
		}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "tmp", "value": "req.Name", "declare": true}},
		{Action: "flow.While", Args: map[string]any{
			"condition": "req.Offset < 10",
			"_do": []normalizer.FlowStep{
				{Action: "mapping.Assign", Args: map[string]any{"to": "w", "value": "req.Offset", "declare": true}},
			},
		}},
		{Action: "mapping.Assign", Args: map[string]any{"to": "w", "value": "req.Limit", "declare": true}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "tmp := req.Name") {
		t.Fatalf("expected tmp to be declared in outer scope after flow.For\n\n%s", code)
	}
	if !strings.Contains(code, "w := req.Limit") {
		t.Fatalf("expected w to be declared in outer scope after flow.While\n\n%s", code)
	}
}

func TestRenderFlow_UsesUniqueTempVarsForRepeatedActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "fs.ReadFile", Args: map[string]any{"path": `"a.txt"`, "output": "a"}},
		{Action: "fs.ReadFile", Args: map[string]any{"path": `"b.txt"`, "output": "b"}},
		{Action: "cache.Get", Args: map[string]any{"key": `"k1"`, "output": "c1"}},
		{Action: "cache.Get", Args: map[string]any{"key": `"k2"`, "output": "c2"}},
		{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://a.test"`, "output": "h1"}},
		{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://b.test"`, "output": "h2"}},
		{Action: "storage.GetURL", Args: map[string]any{"key": `"s1"`, "output": "u1"}},
		{Action: "storage.GetURL", Args: map[string]any{"key": `"s2"`, "output": "u2"}},
		{Action: "json.Marshal", Args: map[string]any{"input": "resp", "output": "j1"}},
		{Action: "json.Marshal", Args: map[string]any{"input": "resp", "output": "j2"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"_rfBytes_0, _rfErr_0 := os.ReadFile",
		"_rfBytes_1, _rfErr_1 := os.ReadFile",
		"_cacheRaw_2, _cacheErr_2 := s.cache.Get",
		"_cacheRaw_3, _cacheErr_3 := s.cache.Get",
		"_httpReq_4, _httpReqErr_4 := http.NewRequestWithContext",
		"_httpReq_5, _httpReqErr_5 := http.NewRequestWithContext",
		"_sURL_6, _sErr_6 := s.storage.GetURL",
		"_sURL_7, _sErr_7 := s.storage.GetURL",
		"_jb_8, _jErr_8 := json.Marshal",
		"_jb_9, _jErr_9 := json.Marshal",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_RandCodeUsesCryptoRandom(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "rand.Code", Args: map[string]any{"output": "otp", "length": 6}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "cryptorand.Read(") {
		t.Fatalf("expected rand.Code to use crypto/rand\n\n%s", code)
	}
	if strings.Contains(code, "mathrand.Intn") {
		t.Fatalf("rand.Code must not use math/rand\n\n%s", code)
	}
}

func TestRenderFlow_FsTempDirPropagatesError(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "fs.TempDir", Args: map[string]any{"output": "workDir", "pattern": `"ang-*"`}},
	}
	code := renderFlow(steps)
	mustContain := []string{
		"os.MkdirTemp(",
		"if _tmpDirErr_0 != nil",
		`fmt.Errorf("temp dir: %w", _tmpDirErr_0)`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_HttpAndWebhookHandleRequestBuildErrors(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "http.Call", Args: map[string]any{"method": "POST", "url": `"https://api.test"`, "body": `"{}"`, "output": "body"}},
		{Action: "webhook.Send", Args: map[string]any{"url": `"https://wh.test"`, "payload": "resp"}},
	}
	code := renderFlow(steps)
	mustContain := []string{
		"_httpReqErr_0",
		`fmt.Errorf("http: request: %w", _httpReqErr_0)`,
		"_whReqErr",
		"_whLastErr = _whReqErr",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_StorageUploadSupportsStringAndBytes(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "storage.Upload", Args: map[string]any{"key": `"files/a.txt"`, "data": "payload", "output": "url"}},
	}
	code := renderFlow(steps)
	mustContain := []string{
		"switch _v := any(payload).(type)",
		"case []byte:",
		"case string:",
		`fmt.Errorf("storage.Upload: data must be string or []byte")`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_StorageDownloadChecksReadError(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "storage.Download", Args: map[string]any{"key": `"files/a.txt"`, "output": "payload"}},
	}
	code := renderFlow(steps)
	mustContain := []string{
		"io.ReadAll(",
		"_sDlReadErr",
		"if _sDlReadErr",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_StorageDeleteSupported(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "storage.Delete", Args: map[string]any{"key": `"files/a.txt"`}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "s.storage.Delete(ctx") {
		t.Fatalf("expected generated code to call storage.Delete\n\n%s", code)
	}
}

func TestRenderFlow_StorageListSupported(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "storage.List", Args: map[string]any{"prefix": `"files/"`, "output": "keys"}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "s.storage.List(ctx") {
		t.Fatalf("expected generated code to call storage.List\n\n%s", code)
	}
}

func TestRenderFlow_NewResilienceActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Checkpoint", Args: map[string]any{"name": "before", "data": "req"}},
		{Action: "flow.Resume", Args: map[string]any{"name": "before", "output": "restored"}},
		{Action: "flow.Validate", Args: map[string]any{"condition": `req.ID != ""`, "message": "id required", "hint": "set req.ID"}},
		{Action: "flow.Try", Args: map[string]any{
			"retries":   2,
			"backoffMs": 5,
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
			"_catch": []normalizer.FlowStep{
				{Action: "flow.ExplainError", Args: map[string]any{"error": "_flowLastError", "output": "tryExplain"}},
			},
		}},
		{Action: "flow.Catch", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"retry", "open editor"}}},
			},
		}},
		{Action: "flow.Retry", Args: map[string]any{
			"attempts":  3,
			"backoffMs": 7,
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
		}},
		{Action: "flow.Fallback", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
			"_fallback": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"use fallback path"}}},
			},
		}},
		{Action: "flow.Timeout", Args: map[string]any{
			"duration": "2*time.Second",
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
			"_onTimeout": []normalizer.FlowStep{
				{Action: "flow.ExplainError", Args: map[string]any{"output": "timeoutExplain", "hint": "increase timeout"}},
			},
		}},
		{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"continue", "stop"}}},
		{Action: "flow.ExplainError", Args: map[string]any{"output": "lastExplain", "hint": "retry later"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"var _flowCheckpoints map[string]any",
		"var _flowLastError error",
		"_flowCheckpoints[\"before\"] = req",
		"restored := _ckptVal_",
		`errors.New(http.StatusBadRequest, "VALIDATION_FAILED", "id required (hint: set req.ID)")`,
		"_tryRun_",
		"_flowLastError = _tryErr_",
		"time.Sleep(time.Duration(_tryBackoff_",
		"if _flowLastError != nil {",
		"_retryRun_",
		"_fbRun_",
		"context.WithTimeout(ctx, 2*time.Second)",
		`slog.Info("flow.suggest_next", "options", []string{"continue", "stop"})`,
		"lastExplain := _expMsg_",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_TryPredeclaresFsTempDirAsString(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Try", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "fs.TempDir", Args: map[string]any{"output": "previewDir"}},
				{Action: "cue.WriteProjectFiles", Args: map[string]any{"root": "previewDir", "files": "projectFiles", "output": "previewWrite"}},
				{Action: "str.Concat", Args: map[string]any{"output": "previewMainPath", "parts": []string{"previewDir", `"/cue/main.cue"`}}},
				{Action: "fs.Remove", Args: map[string]any{"path": "previewDir"}},
			},
			"_catch": []normalizer.FlowStep{
				{Action: "flow.ExplainError", Args: map[string]any{"output": "streamErrorInfo"}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"var previewDir string",
		"previewDir = _tmpDir_",
		"filepath.Clean(previewDir)",
		"os.RemoveAll(previewDir)",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_DeterministicHistoryActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.RecordEvent", Args: map[string]any{"name": `"project.created"`, "payload": "req", "output": "evt"}},
		{Action: "flow.History.Get", Args: map[string]any{"output": "history"}},
		{Action: "flow.Replay", Args: map[string]any{
			"history": "history",
			"output":  "replayed",
			"_do": []normalizer.FlowStep{
				{Action: "flow.RecordEvent", Args: map[string]any{"name": `"should_not_append"`}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"var _flowHistory []map[string]any",
		"var _flowReplayMode bool",
		`map[string]any{"seq": len(_flowHistory) + 1, "name": "project.created", "payload": req}`,
		"_flowHistory = append(_flowHistory, _flowEvt_",
		"_flowHistOut_",
		"_flowReplayItems_",
		"if !_flowReplayMode {",
		"_flowReplayMode = true",
		"_flowReplayMode = _flowReplayPrev_",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_PerformanceProfileDefaults(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://api.test"`, "output": "body"}},
		{Action: "parallel.Run", Args: map[string]any{
			"_branches": map[string][]normalizer.FlowStep{
				"a": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
				"b": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		}},
		{Action: "queue.Enqueue", Args: map[string]any{"subject": `"events.core"`, "payload": "req"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"context.WithTimeout(ctx, 5*time.Second)",
		"for _httpTry_0 := 0; _httpTry_0 < 2; _httpTry_0++",
		"time.Sleep(time.Duration(150) * time.Millisecond)",
		"_pSem := make(chan struct{}, 8)",
		"context.WithTimeout(ctx, 3*time.Second)",
		"s.queuePublisher.Enqueue(_qCtx_",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_PerformanceProfileOverrides(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "http.Call", Args: map[string]any{
			"method":    "GET",
			"url":       `"https://api.test"`,
			"output":    "body",
			"attempts":  4,
			"backoffMs": 25,
			"timeout":   "2*time.Second",
		}},
		{Action: "parallel.Run", Args: map[string]any{
			"maxConcurrency": 3,
			"_branches": map[string][]normalizer.FlowStep{
				"a": {{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}},
			},
		}},
		{Action: "queue.Enqueue", Args: map[string]any{"subject": `"events.core"`, "payload": "req", "timeoutMs": 1200}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"context.WithTimeout(ctx, 2*time.Second)",
		"for _httpTry_0 := 0; _httpTry_0 < 4; _httpTry_0++",
		"time.Sleep(time.Duration(25) * time.Millisecond)",
		"_pSem := make(chan struct{}, 3)",
		"context.WithTimeout(ctx, time.Duration(1200) * time.Millisecond)",
		"s.queuePublisher.Enqueue(_qCtx_",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_MessagingAsyncActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "webhook.VerifySignature", Args: map[string]any{"payload": "req.Body", "signature": "req.Signature", "output": "sigOK"}},
		{Action: "webhook.Ack", Args: map[string]any{"status": 202, "body": `"accepted"`}},
		{Action: "queue.Dequeue", Args: map[string]any{"subject": `"events.core"`, "output": "msg", "ackToken": "msgID", "timeoutMs": 1200, "attempts": 3, "backoffMs": 25, "jitterMs": 10}},
		{Action: "queue.Ack", Args: map[string]any{"subject": `"events.core"`, "messageID": "msgID"}},
		{Action: "queue.Nack", Args: map[string]any{"subject": `"events.core"`, "messageID": "msgID", "reason": `"decode failed"`}},
		{Action: "dlq.Publish", Args: map[string]any{"subject": `"events.core"`, "payload": "msg", "reason": `"decode failed"`}},
		{Action: "event.Outbox", Args: map[string]any{"name": `"ProjectCreated"`, "payload": "domain.ProjectCreated{ID: req.ID}"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"_whMac := hmac.New(sha256.New",
		"webhook.Ack marker: transport should acknowledge",
		"s.queuePublisher.Dequeue(",
		"for _qdTry := 0; _qdTry < 3; _qdTry++",
		"cryptorand.Int(cryptorand.Reader, big.NewInt(",
		"s.queuePublisher.Ack(ctx",
		"s.queuePublisher.Nack(ctx",
		"s.queuePublisher.PublishDLQ(ctx",
		"s.outbox.SaveEvent(ctx",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_ReliabilityAndObservabilityActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "idempotency.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}},
		{Action: "idempotency.Check", Args: map[string]any{"key": "idemKey"}},
		{Action: "idempotency.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}},
		{Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID", "rps": 20}},
		{Action: "concurrency.Run", Args: map[string]any{
			"key": `"build"`, "max": 8,
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
		}},
		{Action: "circuit.Breaker", Args: map[string]any{
			"name": `"external-api"`, "threshold": 3, "openTTL": "30*time.Second",
			"_do": []normalizer.FlowStep{
				{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://api.test"`, "output": "body"}},
			},
		}},
		{Action: "bulkhead.Run", Args: map[string]any{
			"name": `"s3-upload"`, "max": 12,
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
		}},
		{Action: "log.Emit", Args: map[string]any{"level": `"warn"`, "message": `"project slow"`, "fields": map[string]string{"project_id": "req.ID"}}},
		{Action: "metric.Emit", Args: map[string]any{"name": `"project.created"`, "kind": `"counter"`, "value": "1", "labels": map[string]string{"service": `"sandbox"`}}},
		{Action: "trace.Span", Args: map[string]any{
			"name":  `"BuildProject"`,
			"attrs": map[string]string{"project_id": "req.ID"},
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
		}},
		{Action: "slo.Budget", Args: map[string]any{
			"name": `"build"`, "duration": "2*time.Second",
			"_do": []normalizer.FlowStep{
				{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"// idem.DeriveKey",
		"// ratelimit.Check",
		"// concurrency.Limit",
		"// circuit.Breaker",
		"defer func() {",
		"// bulkhead.Acquire",
		"slog.Warn(",
		"slog.Info(\"metric.emit\"",
		`otel.Tracer("ang.flow").Start(ctx, "BuildProject")`,
		"attribute.String(",
		"context.WithTimeout(ctx, _sloLimit",
		`slog.Warn("slo.budget.exceeded"`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_NewDataTransformActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "regex.Match", Args: map[string]any{"input": "req.Email", "pattern": `"^[^@]+@[^@]+$"`, "output": "emailOK"}},
		{Action: "regex.Replace", Args: map[string]any{"input": "req.Name", "pattern": `"\\s+"`, "repl": `"-"`, "output": "slug"}},
		{Action: "base64.Encode", Args: map[string]any{"input": "req.Payload", "output": "b64"}},
		{Action: "base64.Decode", Args: map[string]any{"input": "req.Encoded", "output": "raw"}},
		{Action: "url.Parse", Args: map[string]any{"input": "req.URL", "output": "parsedURL"}},
		{Action: "url.Build", Args: map[string]any{"base": `"https://api.test"`, "path": `"/v1/items"`, "query": map[string]string{"q": "req.Query"}, "output": "builtURL"}},
		{Action: "query.Encode", Args: map[string]any{"input": "req.QueryMap", "output": "rawQuery"}},
		{Action: "query.Decode", Args: map[string]any{"input": "req.RawQuery", "output": "queryVals"}},
		{Action: "hash.Sum", Args: map[string]any{"algorithm": `"sha256"`, "input": "req.Payload", "output": "digest"}},
		{Action: "hash.HMAC", Args: map[string]any{"algorithm": `"sha256"`, "key": "req.Secret", "input": "req.Payload", "output": "signature"}},
		{Action: "uuid.New", Args: map[string]any{"output": "id"}},
		{Action: "uuid.New", Args: map[string]any{"output": "resp.ID"}},
		{Action: "ulid.New", Args: map[string]any{"output": "ulid"}},
		{Action: "time.Now", Args: map[string]any{"output": "createdAt"}},
		{Action: "time.Now", Args: map[string]any{"output": "resp.CreatedAt"}},
		{Action: "time.Format", Args: map[string]any{"input": "createdAt", "output": "createdAtRFC3339", "format": "time.RFC3339"}},
		{Action: "time.Format", Args: map[string]any{"input": "createdAt", "output": "createdAtOptional", "format": "time.RFC3339", "zero": "empty"}},
		{Action: "math.Op", Args: map[string]any{"op": `"round"`, "value": "req.Amount", "precision": 2, "output": "rounded"}},
		{Action: "jsonpath.Get", Args: map[string]any{"input": "req.Payload", "path": `"$.user.email"`, "output": "email"}},
		{Action: "jsonpath.Set", Args: map[string]any{"input": "req.Payload", "path": `"$.user.role"`, "value": `"admin"`, "output": "patched"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"regexp.Compile",
		"base64.StdEncoding.EncodeToString",
		"url.Parse(",
		"url.ParseQuery",
		"sha256.Sum256",
		"hmac.New",
		"uuid.NewString()",
		"helpers.Assign(&resp.ID, uuid.NewString())",
		"base32.NewEncoding",
		"createdAt := time.Now().UTC()",
		"helpers.Assign(&resp.CreatedAt, time.Now().UTC())",
		"createdAtRFC3339 := createdAt.Format(time.RFC3339)",
		"createdAtOptional := func() string { if createdAt.IsZero() { return \"\" }; return createdAt.Format(time.RFC3339) }()",
		"math.Round",
		"helpers.JSONPathGet(req.Payload, \"$.user.email\")",
		"jsonpath.Set: input must be map[string]any",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_TimeFormatZeroEmptyWithTimezone(t *testing.T) {
	code := renderFlow([]normalizer.FlowStep{{
		Action: "time.Format",
		Args: map[string]any{
			"input":    "eventAt",
			"output":   "eventAtText",
			"format":   "time.RFC3339",
			"timezone": "req.Timezone",
			"zero":     "empty",
		},
	}})

	for _, want := range []string{
		`eventAtText := func() string { if eventAt.IsZero() { return "" }`,
		`time.LoadLocation(fmt.Sprint(req.Timezone))`,
		`return eventAt.In(_tz).Format(time.RFC3339)`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected generated code to contain %q\n\n%s", want, code)
		}
	}
}

func TestRenderFlow_NewSecurityActions(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "jwt.Sign", Args: map[string]any{"claims": "map[string]any{\"sub\": req.UserID}", "secret": `"super-secret"`, "ttl": `"1h"`, "output": "token"}},
		{Action: "jwt.Verify", Args: map[string]any{"token": "token", "secret": `"super-secret"`, "output": "claims"}},
		{Action: "oauth2.Token", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "clientID": `"cid"`, "clientSecret": `"csec"`, "scope": `"profile email"`, "output": "oauthToken"}},
		{Action: "oauth2.Refresh", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "refreshToken": `"r1"`, "clientID": `"cid"`, "clientSecret": `"csec"`, "output": "oauthRefreshed"}},
		{Action: "crypto.Encrypt", Args: map[string]any{"input": "req.Payload", "key": `"enc-key"`, "output": "cipher"}},
		{Action: "crypto.Decrypt", Args: map[string]any{"input": "cipher", "key": `"enc-key"`, "output": "plain"}},
		{Action: "rbac.CheckPermission", Args: map[string]any{"user": "currentUser", "permission": `"project.create"`}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"strings.ToUpper(strings.TrimSpace(",
		"base64.RawURLEncoding",
		"hmac.New(sha256.New",
		"strings.Split(token, \".\")",
		"url.Values{}",
		"http.NewRequestWithContext(ctx, http.MethodPost",
		"io.ReadAll(",
		"aes.NewCipher(",
		"cipher.NewGCM(",
		"base64.RawStdEncoding.EncodeToString",
		"rbac.CheckPermission(currentUser.Role,",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_EventPublishPayloadMap(t *testing.T) {
	steps := []normalizer.FlowStep{
		{
			Action: "event.Publish",
			Args: map[string]any{
				"name": "TenderClosed",
				"payloadMap": map[string]any{
					"TenderID": "req.TenderID",
					"Status":   "\"closed\"",
				},
			},
		},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"s.publisher.PublishTenderClosed",
		`domain.TenderClosed{Status: "closed", TenderID: req.TenderID}`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_EventPublishPayloadMapTypeCoercion(t *testing.T) {
	steps := []normalizer.FlowStep{
		{
			Action: "repo.Find",
			Args: map[string]any{
				"source": "Bid",
				"input":  "req.BidID",
				"output": "bid",
			},
		},
		{
			Action: "event.Publish",
			Args: map[string]any{
				"name": "BidPlaced",
				"payloadMap": map[string]any{
					"CreatedAt": "bid.CreatedAt",
					"Amount":    "bid.Amount",
				},
			},
		},
	}
	entities := []normalizer.Entity{
		{
			Name: "Bid",
			Fields: []normalizer.Field{
				{Name: "CreatedAt", Type: "time.Time"},
				{Name: "Amount", Type: "float64"},
			},
		},
	}
	events := []normalizer.EventDef{
		{
			Name: "BidPlaced",
			Fields: []normalizer.Field{
				{Name: "CreatedAt", Type: "string"},
				{Name: "Amount", Type: "string"},
			},
		},
	}

	code := renderFlowForServiceWithSchema("Tender", steps, entities, events)
	mustContain := []string{
		"s.publisher.PublishBidPlaced",
		"CreatedAt: bid.CreatedAt.Format(time.RFC3339)",
		"Amount: fmt.Sprint(bid.Amount)",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_EventPublish_EmptyPayloadMapStillPublishes(t *testing.T) {
	steps := []normalizer.FlowStep{
		{
			Action: "event.Publish",
			Args: map[string]any{
				"name":       "Heartbeat",
				"payloadMap": map[string]any{},
			},
		},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"s.publisher.PublishHeartbeat",
		"domain.Heartbeat{}",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_EventPublish_NoPayloadStillPublishesEmptyStruct(t *testing.T) {
	steps := []normalizer.FlowStep{
		{
			Action: "event.Publish",
			Args: map[string]any{
				"name": "Heartbeat",
			},
		},
		{
			Action: "event.Broadcast",
			Args: map[string]any{
				"name": "Heartbeat",
			},
		},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"s.publisher.PublishHeartbeat",
		"s.publisher.BroadcastHeartbeat",
		"domain.Heartbeat{}",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_LogicCheckStatus403(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "targetRole != \"owner\" || actorRole == \"admin\"",
			"throw":     "cannot change owner role",
			"status":    "403",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusForbidden") {
		t.Fatalf("expected StatusForbidden for logic.Check with status=403, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckDefaultStatus400(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "req.CompanyID != \"\"",
			"throw":     "companyId is required",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusBadRequest") {
		t.Fatalf("expected StatusBadRequest for logic.Check without status field, got:\n%s", code)
	}
	if !strings.Contains(code, "Validation Error") {
		t.Fatalf("expected message 'Validation Error' for logic.Check without status field, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckStatus404(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "item != nil",
			"throw":     "not found",
			"status":    "404",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusNotFound") {
		t.Fatalf("expected StatusNotFound for logic.Check with status=404, got:\n%s", code)
	}
	if !strings.Contains(code, "Not Found") {
		t.Fatalf("expected message 'Not Found' for logic.Check with status=404, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckStatus409(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "currentVersion == req.Version",
			"throw":     "version conflict",
			"status":    "409",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusConflict") {
		t.Fatalf("expected StatusConflict for logic.Check with status=409, got:\n%s", code)
	}
	if !strings.Contains(code, "Conflict") {
		t.Fatalf("expected message 'Conflict' for logic.Check with status=409, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckStatus401(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "token != \"\"",
			"throw":     "missing token",
			"status":    "401",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusUnauthorized") {
		t.Fatalf("expected StatusUnauthorized for logic.Check with status=401, got:\n%s", code)
	}
	if !strings.Contains(code, "Unauthorized") {
		t.Fatalf("expected message 'Unauthorized' for logic.Check with status=401, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckStatusForbiddenWord(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "actorRole == \"admin\"",
			"throw":     "admin only",
			"status":    "forbidden",
		}},
	}
	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusForbidden") {
		t.Fatalf("expected StatusForbidden for logic.Check with status='forbidden', got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheckNoInterceptInRepoMapping(t *testing.T) {
	step := normalizer.FlowStep{
		Action: "logic.Check",
		Args: map[string]any{
			"condition": "req.CompanyID != \"\"",
			"throw":     "companyId is required",
			"status":    "403",
		},
	}
	st := newDomainDispatchState()
	out, ok := renderFlowStepDomainRepoMapping(st, step, 1, "_x", domainDispatchArg(step), domainDispatchChild(step))
	if ok {
		t.Fatalf("expected repo/mapping dispatcher to return ok=false for logic.Check, got handled output:\n%s", out)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output when logic.Check is passed to repo/mapping dispatcher, got:\n%s", out)
	}
}

func TestRenderFlow_LogicCheck_Default400(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "req.CompanyID != \"\"",
			"throw":     "companyId is required",
		}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusBadRequest") {
		t.Fatalf("expected StatusBadRequest for logic.Check default status, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheck_Status404(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "item != nil",
			"throw":     "not found",
			"status":    "404",
		}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusNotFound") {
		t.Fatalf("expected StatusNotFound for logic.Check status=404, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheck_Status409(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "currentVersion == req.Version",
			"throw":     "version conflict",
			"status":    "409",
		}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusConflict") {
		t.Fatalf("expected StatusConflict for logic.Check status=409, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheck_Status401(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "token != \"\"",
			"throw":     "missing token",
			"status":    "401",
		}},
	}

	code := renderFlow(steps)
	if !strings.Contains(code, "http.StatusUnauthorized") {
		t.Fatalf("expected StatusUnauthorized for logic.Check status=401, got:\n%s", code)
	}
}

func TestRenderFlow_LogicCheck_NoInterceptInRepoMapping(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "logic.Check",
		Args: map[string]any{
			"condition": "req.CompanyID != \"\"",
			"throw":     "companyId is required",
		},
	}
	st := newDomainDispatchState()

	out, ok := renderFlowStepDomainRepoMapping(st, step, 1, "_x", domainDispatchArg(step), domainDispatchChild(step))
	if ok {
		t.Fatalf("expected repo/mapping dispatcher to ignore logic.Check, got handled output:\n%s", out)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output when logic.Check is passed to repo/mapping dispatcher, got:\n%s", out)
	}
}

func TestRenderOneFlowStep_DispatchChainOrder(t *testing.T) {
	t.Parallel()

	st := newDomainDispatchState()
	steps := []normalizer.FlowStep{
		{Action: "logic.Check", Args: map[string]any{
			"condition": "req.CompanyID != \"\"",
			"throw":     "companyId is required",
		}},
		{Action: "flow.If", Args: map[string]any{
			"condition": "req.Offset > 0",
			"_then": []normalizer.FlowStep{
				{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Offset", "value": "req.Offset"}},
			},
		}},
		{Action: "queue.Enqueue", Args: map[string]any{
			"subject": `"events.test"`,
			"payload": "req",
		}},
	}

	codeA := renderOneFlowStep(st, steps[0], 1)
	codeB := renderOneFlowStep(st, steps[1], 1)
	codeC := renderOneFlowStep(st, steps[2], 1)

	if !strings.Contains(codeA, "http.StatusBadRequest") {
		t.Fatalf("expected domain dispatcher to handle logic.Check first, got:\n%s", codeA)
	}
	if !strings.Contains(codeB, "if req.Offset > 0") {
		t.Fatalf("expected control dispatcher to handle flow.If, got:\n%s", codeB)
	}
	if !strings.Contains(codeC, "s.queuePublisher.Enqueue(") {
		t.Fatalf("expected infra dispatcher to handle queue.Enqueue, got:\n%s", codeC)
	}
}

func TestRenderFlow_NoCodegenCriticalActionEmitsErrorDiagnostic(t *testing.T) {
	t.Parallel()

	var diags []normalizer.Warning
	steps := []normalizer.FlowStep{
		{
			Action:  "logic.Check",
			Args:    map[string]any{"throw": "forbidden"},
			File:    "cue/api/auth_api.cue",
			Line:    42,
			Column:  3,
			CUEPath: "Auth.LoginUser.flow[2]",
		},
	}

	_ = renderFlowForServiceWithSchemaAndSink("Auth", "LoginUser", steps, nil, nil, func(w normalizer.Warning) {
		diags = append(diags, w)
	})

	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "FLOW_STEP_NO_CODEGEN_CRITICAL" {
		t.Fatalf("expected FLOW_STEP_NO_CODEGEN_CRITICAL, got %q", diags[0].Code)
	}
	if strings.ToLower(diags[0].Severity) != "error" {
		t.Fatalf("expected severity=error, got %q", diags[0].Severity)
	}
	if diags[0].Op != "Auth.LoginUser" {
		t.Fatalf("expected op Auth.LoginUser, got %q", diags[0].Op)
	}
}

func TestRenderFlow_NoCodegenNonCriticalActionEmitsWarnDiagnostic(t *testing.T) {
	t.Parallel()

	var diags []normalizer.Warning
	steps := []normalizer.FlowStep{
		{
			Action:  "cache.Set",
			Args:    map[string]any{"ttl": "60"},
			File:    "cue/api/auth_api.cue",
			Line:    43,
			Column:  3,
			CUEPath: "Auth.LoginUser.flow[3]",
		},
	}

	_ = renderFlowForServiceWithSchemaAndSink("Auth", "LoginUser", steps, nil, nil, func(w normalizer.Warning) {
		diags = append(diags, w)
	})

	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != "FLOW_STEP_NO_CODEGEN" {
		t.Fatalf("expected FLOW_STEP_NO_CODEGEN, got %q", diags[0].Code)
	}
	if strings.ToLower(diags[0].Severity) != "warn" {
		t.Fatalf("expected severity=warn, got %q", diags[0].Severity)
	}
}

func newDomainDispatchState() *flowRenderState {
	n := 0
	return &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
}

func domainDispatchArg(step normalizer.FlowStep) func(string) string {
	return func(name string) string {
		v, ok := step.Args[name]
		if !ok {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return normalizeFlowExpr(strings.TrimSpace(s))
	}
}

func domainDispatchChild(step normalizer.FlowStep) func(string) []normalizer.FlowStep {
	return func(name string) []normalizer.FlowStep {
		v, ok := step.Args[name]
		if !ok {
			return nil
		}
		steps, ok := v.([]normalizer.FlowStep)
		if !ok {
			return nil
		}
		return steps
	}
}
