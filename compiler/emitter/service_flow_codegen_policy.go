package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepPolicy(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	_ = child
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "policy.Evaluate", "policy.Require", "policy.Decide":
		policyKey := arg("policyKey")
		if policyKey == "" {
			return "", true
		}

		subject := arg("subject")
		resource := arg("resource")
		operation := arg("operation")
		tenant := arg("tenant")
		attrs := arg("attrs")
		ctxExpr := arg("context")

		decisionOut := arg("decision")
		reasonOut := arg("reason")
		effectsOut := arg("effects")
		output := arg("output")

		declareMap := map[string]bool{}
		if decisionOut != "" && !st.declared[decisionOut] {
			declareMap[decisionOut] = true
			st.declared[decisionOut] = true
			st.pointers[decisionOut] = false
			st.types[decisionOut] = "string"
		}
		if reasonOut != "" && !st.declared[reasonOut] {
			declareMap[reasonOut] = true
			st.declared[reasonOut] = true
			st.pointers[reasonOut] = false
			st.types[reasonOut] = "string"
		}
		if effectsOut != "" && !st.declared[effectsOut] {
			declareMap[effectsOut] = true
			st.declared[effectsOut] = true
			st.pointers[effectsOut] = false
			st.types[effectsOut] = "map[string]any"
		}
		if output != "" && !st.declared[output] {
			declareMap[output] = true
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "port.PolicyDecision"
		}

		statusExpr := arg("status")
		if statusExpr == "" {
			statusExpr = "http.StatusForbidden"
		}
		codeExpr := arg("code")
		if codeExpr == "" {
			codeExpr = `"POLICY_DENIED"`
		}
		throwExpr := arg("throw")

		inputVar := "_policyInput" + sfx
		decisionVar := "_policyDecision" + sfx
		errVar := "_policyErr" + sfx
		codeVar := "_policyCode" + sfx
		msgVar := "_policyMsg" + sfx

		var b strings.Builder
		for _, out := range []string{decisionOut, reasonOut, effectsOut, output} {
			if !declareMap[out] {
				continue
			}
			switch out {
			case effectsOut:
				b.WriteString(fmt.Sprintf("%svar %s map[string]any\n", pad, out))
			case output:
				b.WriteString(fmt.Sprintf("%svar %s port.PolicyDecision\n", pad, out))
			default:
				b.WriteString(fmt.Sprintf("%svar %s string\n", pad, out))
			}
		}

		b.WriteString(fmt.Sprintf("%sif s.policyEngine == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `errors.New(http.StatusInternalServerError, "POLICY_ENGINE_NOT_CONFIGURED", "policy action requires policy engine wiring")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		b.WriteString(fmt.Sprintf("%s%s := port.PolicyInput{PolicyKey: %s", pad, inputVar, policyKey))
		if subject != "" {
			b.WriteString(fmt.Sprintf(", Subject: %s", subject))
		}
		if resource != "" {
			b.WriteString(fmt.Sprintf(", Resource: %s", resource))
		}
		if operation != "" {
			b.WriteString(fmt.Sprintf(", Operation: %s", operation))
		}
		if tenant != "" {
			b.WriteString(fmt.Sprintf(", Tenant: %s", tenant))
		}
		if attrs != "" {
			b.WriteString(fmt.Sprintf(", Attrs: %s", attrs))
		}
		if ctxExpr != "" {
			b.WriteString(fmt.Sprintf(", Context: %s", ctxExpr))
		}
		b.WriteString("}\n")
		b.WriteString(fmt.Sprintf("%s%s, %s := s.policyEngine.Evaluate(ctx, %s)\n", pad, decisionVar, errVar, inputVar))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%s: %%w\", %s)", step.Action, errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		if decisionOut != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s.Decision\n", pad, decisionOut, decisionVar))
		}
		if reasonOut != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s.Reason\n", pad, reasonOut, decisionVar))
		}
		if effectsOut != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s.Effects\n", pad, effectsOut, decisionVar))
		}
		if output != "" {
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, decisionVar))
		}

		if step.Action == "policy.Require" {
			b.WriteString(fmt.Sprintf("%sif !strings.EqualFold(strings.TrimSpace(%s.Decision), \"allow\") {\n", pad, decisionVar))
			b.WriteString(fmt.Sprintf("%s\t%s := strings.TrimSpace(fmt.Sprint(%s))\n", pad, codeVar, codeExpr))
			b.WriteString(fmt.Sprintf("%s\tif %s == \"\" { %s = \"POLICY_DENIED\" }\n", pad, codeVar, codeVar))
			if throwExpr != "" {
				b.WriteString(fmt.Sprintf("%s\t%s := strings.TrimSpace(fmt.Sprint(%s))\n", pad, msgVar, throwExpr))
			} else {
				b.WriteString(fmt.Sprintf("%s\t%s := \"\"\n", pad, msgVar))
			}
			b.WriteString(fmt.Sprintf("%s\tif %s == \"\" { %s = strings.TrimSpace(%s.Reason) }\n", pad, msgVar, msgVar, decisionVar))
			b.WriteString(fmt.Sprintf("%s\tif %s == \"\" { %s = \"policy denied\" }\n", pad, msgVar, msgVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %s, %s)", statusExpr, codeVar, msgVar)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}

		return b.String(), true
	}

	return "", false
}
