package main

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestDeriveFlowCases(t *testing.T) {
	method := normalizer.Method{
		Name:  "UpdateOrder",
		Input: normalizer.Entity{Fields: []normalizer.Field{{Name: "ID"}}},
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
			{Action: "tx.Block", Args: map[string]any{"_do": []normalizer.FlowStep{}}},
		},
	}
	ep := normalizer.Endpoint{Method: "POST", Path: "/orders/{id}", ServiceName: "orders", RPC: "UpdateOrder", Permission: "orders.update"}
	cases := deriveFlowCases("orders", method, ep)
	if len(cases) != 7 {
		t.Fatalf("len(cases)=%d, want 7", len(cases))
	}

	var has404, has403, hasThen, hasElse, hasPolicy, hasRequired, hasRollback bool
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
		hasPolicy = hasPolicy || c.Kind == "policy_forbidden"
		hasRequired = hasRequired || c.Kind == "required_field"
		hasRollback = hasRollback || c.Kind == "tx_rollback"
	}
	if !has404 || !has403 || !hasThen || !hasElse || !hasPolicy || !hasRequired || !hasRollback {
		t.Fatalf("missing expected generated cases: %#v", cases)
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
