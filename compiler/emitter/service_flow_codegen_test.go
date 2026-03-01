package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
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
	if !strings.Contains(code, `sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })`) {
		t.Fatalf("expected static asc sort\n\n%s", code)
	}

	// static desc
	steps2 := []normalizer.FlowStep{
		{Action: "list.Sort", Args: map[string]any{"items": "items", "by": "CreatedAt", "order": "desc"}},
	}
	code2 := renderFlow(steps2)
	if !strings.Contains(code2, `sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })`) {
		t.Fatalf("expected static desc sort\n\n%s", code2)
	}

	// dynamic order from request
	steps3 := []normalizer.FlowStep{
		{Action: "list.Sort", Args: map[string]any{"items": "items", "by": "Name", "order": "req.SortOrder"}},
	}
	code3 := renderFlow(steps3)
	mustContain3 := []string{
		`sort.Slice(items, func(i, j int) bool {`,
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
