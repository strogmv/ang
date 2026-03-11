package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderFlowStepEventOrchestration_NotifyEmailIsFirstClass(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "notify.Email",
		Args: map[string]any{
			"to":     "req.Email",
			"text":   `"Hello"`,
			"output": "notificationID",
		},
	}

	st := newInfraTestFlowState()
	got, ok := renderFlowStepEventOrchestration(st, step, 1, "_x", infraTestArg(step), infraTestChild(step))
	if !ok {
		t.Fatalf("expected notify.Email to be handled by event orchestration renderer")
	}
	for _, snippet := range []string{
		`_notifyChannel_x := strings.ToLower(strings.TrimSpace(fmt.Sprint("email")))`,
		`notify.Email requires notification dispatcher wiring`,
		`fmt.Errorf("notify.Email: %w", _notifyErr_x)`,
		`notificationID := ""`,
		`notificationID = uuid.NewString()`,
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected notify.Email render snippet %q, got:\n%s", snippet, got)
		}
	}
	for _, unwanted := range []string{
		`notify.Send requires non-empty channel`,
		`fmt.Errorf("notify.Send: %w", _notifyErr_x)`,
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("unexpected notify.Send compatibility fallback in notify.Email render: %q\n%s", unwanted, got)
		}
	}
}
