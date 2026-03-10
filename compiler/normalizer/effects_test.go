package normalizer

import (
	"testing"

	"github.com/strogmv/ang/compiler/flowsem"
)

func TestValidateStep_MissingEffectPrerequisite(t *testing.T) {
	t.Parallel()
	errs := ValidateStep(FlowStep{Action: "openai.Chat"}, NewEffectSet())
	if len(errs) == 0 {
		t.Fatalf("expected prerequisite error")
	}
	if errs[0].Code != "MISSING_EFFECT_PREREQUISITE" {
		t.Fatalf("code=%q", errs[0].Code)
	}
}

func TestValidateStep_ExternalEffectInTx(t *testing.T) {
	t.Parallel()
	es := NewEffectSet()
	es.Tags[flowsem.RequireTxOpen] = true
	es.Tags[flowsem.RequireRateChecked] = true
	errs := ValidateStep(FlowStep{Action: "http.Request"}, es)
	if len(errs) == 0 {
		t.Fatalf("expected tx incompatibility error")
	}
	if errs[0].Code != "EXTERNAL_EFFECT_IN_TX" {
		t.Fatalf("code=%q", errs[0].Code)
	}
}

func TestDeriveOperationEffects_Recursive(t *testing.T) {
	t.Parallel()
	steps := []FlowStep{
		{Action: "session.Get", Args: map[string]any{"output": "session"}},
		{
			Action: "tx.Block",
			Args: map[string]any{
				"_do": []FlowStep{
					{Action: "repo.Save"},
				},
			},
		},
		{Action: "openai.Chat"},
	}
	got := DeriveOperationEffects(steps)
	if len(got) != 3 {
		t.Fatalf("effects=%v", got)
	}
}

func TestValidateFlowEffects_TxBlockAddsChildTag(t *testing.T) {
	t.Parallel()
	warns := validateFlowEffects("Sandbox.Build", []FlowStep{
		{
			Action: "tx.Block",
			Args: map[string]any{
				"_do": []FlowStep{
					{Action: "db.Lock", Args: map[string]any{"output": "row"}},
				},
			},
		},
	})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %+v", warns)
	}
}

func TestValidateFlowEffects_SequentialTagsAccumulate(t *testing.T) {
	t.Parallel()
	warns := validateFlowEffects("Sandbox.Chat", []FlowStep{
		{Action: "session.Get", Args: map[string]any{"output": "session"}},
		{Action: "quota.Check", Args: map[string]any{"key": "session", "limit": 1, "window": "day"}},
		{Action: "budget.Check", Args: map[string]any{"key": "session", "limit": 100}},
		{Action: "openai.Chat", Args: map[string]any{"output": "reply"}},
	})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %+v", warns)
	}
}

func TestValidateStep_UndeclaredFlowVar(t *testing.T) {
	t.Parallel()
	es := NewEffectSet()
	es.Tags[flowsem.RequireQuotaChecked] = true
	es.Tags[flowsem.RequireBudgetChecked] = true
	errs := ValidateStep(FlowStep{Action: "openai.Chat", Args: map[string]any{"model": "gptModel", "user_message": "req.Prompt"}}, es)
	found := false
	for _, err := range errs {
		if err.Code == "UNDECLARED_FLOW_VAR" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNDECLARED_FLOW_VAR, got %+v", errs)
	}
}

func TestValidateFlowEffects_FlowForLocalVarScope(t *testing.T) {
	t.Parallel()
	warns := validateFlowEffects("Sandbox.Iterate", []FlowStep{
		{Action: "list.New", Args: map[string]any{"output": "items"}},
		{
			Action: "flow.For",
			Args: map[string]any{
				"each": "items",
				"as":   "item",
				"_do": []FlowStep{{
					Action: "mapping.Assign",
					Args:   map[string]any{"to": "resp.CurrentID", "value": "item.ID"},
				}},
			},
		},
	})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %+v", warns)
	}
}

func TestValidateFlowEffects_BranchVarDoesNotLeak(t *testing.T) {
	t.Parallel()
	warns := validateFlowEffects("Sandbox.BranchLeak", []FlowStep{
		{
			Action: "flow.If",
			Args: map[string]any{
				"condition": "req.Enabled",
				"_then":     []FlowStep{{Action: "session.Get", Args: map[string]any{"output": "sessionID"}}},
			},
		},
		{Action: "quota.Check", Args: map[string]any{"key": "sessionID", "limit": 1, "window": "day"}},
	})
	found := false
	for _, warn := range warns {
		if warn.Code == "UNDECLARED_FLOW_VAR" && warn.Action == "quota.Check" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNDECLARED_FLOW_VAR for branch-local sessionID leak, got %+v", warns)
	}
}
