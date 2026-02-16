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
		{Action: "notification.Dispatch", Args: map[string]any{"message": "port.NotificationMessage{}"}},
	}
	if flowRenderable(bad) {
		t.Fatal("expected unsupported flow to be non-renderable")
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
		"filtered := items[:0]",
		"page := filtered[_start:_end]",
		"resp.Data = append(resp.Data, item)",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated flow code to contain %q\n\n%s", part, code)
		}
	}
}

