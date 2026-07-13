package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderFlowStepEventOrchestration preserves focused unit-test ergonomics while
// production dispatch enters the typed renderer directly.
func renderFlowStepEventOrchestration(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, _ func(string) string, _ func(string) []normalizer.FlowStep) (string, bool) {
	typedSteps, _ := flowir.DecodeSteps([]normalizer.FlowStep{step})
	if len(typedSteps) != 1 {
		return "", false
	}
	previous := st.currentTyped
	st.currentTyped = &typedSteps[0]
	defer func() { st.currentTyped = previous }()
	return renderTypedStepEventOrchestration(st, typedSteps[0], indent, sfx)
}

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
		`notify.Email requires notification dispatcher wiring`,
		`fmt.Errorf("notify.Email: %w", _notifyErr_x)`,
		`_notifyMsg_x := port.NotificationMessage{Metadata: _notifyMeta_x`,
		`notificationID := ""`,
		`notificationID = uuid.NewString()`,
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected notify.Email render snippet %q, got:\n%s", snippet, got)
		}
	}
	for _, unwanted := range []string{
		`_notifyChannel_x :=`,
		`Channels: []string{_notifyChannel_x}`,
		`notify.Send requires non-empty channel`,
		`fmt.Errorf("notify.Send: %w", _notifyErr_x)`,
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("unexpected notify.Send compatibility fallback in notify.Email render: %q\n%s", unwanted, got)
		}
	}
}
