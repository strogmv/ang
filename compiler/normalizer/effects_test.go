package normalizer

import (
	"testing"

	sharedeffects "github.com/strogmv/ang/compiler/effects"
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
	es.Tags[sharedeffects.RequireTxOpen] = true
	es.Tags[sharedeffects.RequireRateChecked] = true
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
