package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderFlow_Race_GeneratesFirstWinsPattern(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Race", Args: map[string]any{
			"_branches": map[string][]normalizer.FlowStep{
				"cache": {
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "cache miss"}},
				},
				"database": {
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "db miss"}},
				},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"_fr_0Ctx, _fr_0Cancel := context.WithCancel(ctx)",
		"var _fr_0Wg sync.WaitGroup",
		"if !_fr_0Won",
		`fmt.Errorf("flow.Race: all branches failed")`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_SagaCompensateRollback_GeneratesCompensationStack(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "flow.Saga", Args: map[string]any{
			"_do": []normalizer.FlowStep{
				{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "newOrder"}},
				{Action: "flow.Compensate", Args: map[string]any{
					"_do": []normalizer.FlowStep{
						{Action: "repo.Delete", Args: map[string]any{"source": "Order", "input": "newOrder.ID"}},
					},
				}},
				{Action: "flow.Rollback", Args: map[string]any{"error": `fmt.Errorf("forced rollback")`}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"var _sagaCompensations_0 []func(context.Context) error",
		"for i := len(_sagaCompensations_0) - 1; i >= 0; i--",
		"_sagaCompensations_0 = append(_sagaCompensations_0, func(ctx context.Context) error {",
		`err = fmt.Errorf("forced rollback")`,
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_ApprovalWait_GeneratesTimeoutLoopAndFallback(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "approval.Wait", Args: map[string]any{
			"approvalId": "approvalID",
			"timeout":    "2 * time.Minute",
			"onTimeout":  `"fallback"`,
			"decision":   "decision",
			"status":     "approvalStatus",
			"_onTimeout": []normalizer.FlowStep{
				{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"notify", "retry"}}},
			},
		}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		"approval.Wait requires state store wiring",
		"context.WithTimeout(ctx, 2 * time.Minute)",
		"time.NewTicker(500 * time.Millisecond)",
		"approval.Wait: timeout persist",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}

func TestRenderFlow_FSMTransition_GeneratesTransitionCall(t *testing.T) {
	steps := []normalizer.FlowStep{
		{Action: "repo.Find", Args: map[string]any{"source": "Order", "input": "req.OrderID", "output": "order"}},
		{Action: "fsm.Transition", Args: map[string]any{"entity": "order", "to": "confirmed"}},
		{Action: "repo.Save", Args: map[string]any{"source": "Order", "input": "order"}},
	}

	code := renderFlow(steps)
	mustContain := []string{
		`if err := order.TransitionTo("confirmed"); err != nil {`,
		"if err := s.OrderRepo.Save(ctx, order); err != nil {",
	}
	for _, part := range mustContain {
		if !strings.Contains(code, part) {
			t.Fatalf("expected generated code to contain %q\n\n%s", part, code)
		}
	}
}
