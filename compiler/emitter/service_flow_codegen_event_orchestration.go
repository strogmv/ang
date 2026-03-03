package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepEventOrchestration(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "event.Broadcast":
		name := arg("name")
		payload := arg("payload")
		if name == "" || payload == "" {
			return "", true
		}
		payload = normalizePayloadExpr(payload)
		return fmt.Sprintf("%sif s.publisher != nil {\n%s\t_ = s.publisher.Broadcast%s(ctx, %s)\n%s}\n",
			pad, pad, ExportName(name), payload, pad), true

	case "event.Outbox":
		name := arg("name")
		payload := arg("payload")
		if name == "" || payload == "" {
			return "", true
		}
		payload = normalizePayloadExpr(payload)
		idExpr := arg("id")
		if idExpr == "" {
			idExpr = "uuid.NewString()"
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif s.outbox == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `errors.New(http.StatusInternalServerError, "OUTBOX_NOT_CONFIGURED", "event.Outbox requires outbox repository wiring")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_obPayload, _obMarshalErr := json.Marshal(%s)\n", pad, payload))
		b.WriteString(fmt.Sprintf("%s\tif _obMarshalErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"event.Outbox: marshal: %w\", _obMarshalErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_obID := %s\n", pad, idExpr))
		b.WriteString(fmt.Sprintf("%s\tif _obErr := s.outbox.SaveEvent(ctx, _obID, %s, _obPayload); _obErr != nil {\n", pad, name))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"event.Outbox: %w\", _obErr)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Wait":
		name := arg("name")
		timeout := arg("timeout")
		match := arg("match")
		output := arg("output")
		if name == "" {
			return "", true
		}
		if timeout == "" {
			timeout = "5*time.Minute"
		}

		var b strings.Builder
		if output != "" && !st.declared[output] {
			b.WriteString(fmt.Sprintf("%svar %s any\n", pad, output))
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "any"
		}

		b.WriteString(fmt.Sprintf("%s// event.Wait: %s\n", pad, name))
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_waitCtx, _waitCancel := context.WithTimeout(ctx, %s)\n", pad, timeout))
		b.WriteString(fmt.Sprintf("%s\tdefer _waitCancel()\n", pad))

		waitCall := fmt.Sprintf("s.publisher.Wait(_waitCtx, %q, %q)", name, match)

		b.WriteString(fmt.Sprintf("%s\t_evt, _waitErr := %s\n", pad, waitCall))
		b.WriteString(fmt.Sprintf("%s\tif _waitErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"event.Wait(%%s): %%w\", %q, _waitErr)", name)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))

		if output != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = _evt\n", pad, output))
		}

		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Subscribe":
		name := arg("name")
		match := arg("match")
		doSteps := child("_do")
		if name == "" || len(doSteps) == 0 {
			return "", true
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// event.Subscribe: %s\n", pad, name))
		b.WriteString(fmt.Sprintf("%sif s.publisher != nil {\n", pad))
		b.WriteString(fmt.Sprintf("%s\ts.publisher.Subscribe(ctx, %q, %q, func(ctx context.Context, evt any) {\n", pad, name, match))

		subState := cloneFlowState(st)
		subState.goroutineMode = true
		subState.declared["evt"] = true
		subState.types["evt"] = "any"

		b.WriteString(renderFlowSteps(subState, doSteps, indent+2))
		b.WriteString(fmt.Sprintf("%s\t})\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "event.Match":
		evtVar := arg("event")
		matchCriteria := arg("match")
		throwMsg := arg("throw")
		if evtVar == "" || matchCriteria == "" {
			return "", true
		}
		if throwMsg == "" {
			throwMsg = fmt.Sprintf("event match failed for %s", matchCriteria)
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !helpers.MatchEvent(%s, %q) {\n", pad, evtVar, matchCriteria))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"EVENT_MISMATCH\", %q)", throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
