package main

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestDeriveFlowCases(t *testing.T) {
	method := normalizer.Method{
		Name: "UpdateOrder",
		Flow: []normalizer.FlowStep{
			{
				Action: "repo.Find",
				Args: map[string]any{
					"source": "Order",
					"error":  "Order not found",
				},
			},
			{
				Action: "logic.Check",
				Args: map[string]any{
					"condition": "order.UserID == req.UserID",
					"throw":     "Access denied",
				},
			},
			{
				Action: "flow.If",
				Args: map[string]any{
					"condition": "order.Status == \"draft\"",
				},
			},
		},
	}
	ep := normalizer.Endpoint{Method: "POST", Path: "/orders/{id}", ServiceName: "orders", RPC: "UpdateOrder"}
	cases := deriveFlowCases("orders", method, ep)
	if len(cases) != 4 {
		t.Fatalf("len(cases)=%d, want 4", len(cases))
	}

	var has404, has403, hasThen, hasElse bool
	for _, c := range cases {
		if c.Kind == "repo_not_found" && c.ExpectedStatus == 404 {
			has404 = true
		}
		if c.Kind == "logic_check" && c.ExpectedStatus == 403 {
			has403 = true
		}
		if c.ID == "orders.UpdateOrder.flow_if_then.3" {
			hasThen = true
		}
		if c.ID == "orders.UpdateOrder.flow_if_else.3" {
			hasElse = true
		}
	}
	if !has404 || !has403 || !hasThen || !hasElse {
		t.Fatalf("missing expected generated cases: has404=%v has403=%v hasThen=%v hasElse=%v", has404, has403, hasThen, hasElse)
	}
}

func TestInferCheckStatus(t *testing.T) {
	if got := inferCheckStatus("Access denied"); got != 403 {
		t.Fatalf("status=%d, want 403", got)
	}
	if got := inferCheckStatus("Validation failed"); got != 400 {
		t.Fatalf("status=%d, want 400", got)
	}
}
