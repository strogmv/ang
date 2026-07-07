package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowTryLegacy(st *flowRenderState, action flowir.FlowTry, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	doSteps, catchSteps := action.Steps, action.Catch
	if len(doSteps) == 0 {
		return ""
	}
	retries, backoffMs := action.Retries, action.BackoffMS

	newVars := collectFlowBranchNewVars(st, indent, doSteps, catchSteps)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		b.WriteString(fmt.Sprintf("%s\tvar %s %s\n", pad, varName, v.typ))
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	tryRunV, tryErrV, tryMaxV, tryBackoffV := "_tryRun"+sfx, "_tryErr"+sfx, "_tryMax"+sfx, "_tryBackoff"+sfx
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryMaxV, retries))
	b.WriteString(fmt.Sprintf("%s\tif %s < 0 { %s = 0 }\n", pad, tryMaxV, tryMaxV))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, tryBackoffV, backoffMs))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, tryRunV))
	tryState := cloneFlowState(st)
	tryState.returnErrOnly = true
	b.WriteString(renderFlowSteps(tryState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt <= %s; _tryAttempt++ {\n", pad, tryMaxV))
	b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, tryErrV, tryRunV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt < %s && %s > 0 {\n", pad, tryMaxV, tryBackoffV))
	b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, tryBackoffV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, tryErrV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, tryErrV))
	if len(catchSteps) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t", tryErrV))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowRetryLegacy(st *flowRenderState, action flowir.FlowRetry, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	doSteps, catchSteps := action.Steps, action.Catch
	if len(doSteps) == 0 {
		return ""
	}
	attempts, backoffMs := action.Attempts, action.BackoffMS

	runV, errV, attemptsV, backoffV := "_retryRun"+sfx, "_retryErr"+sfx, "_retryAttempts"+sfx, "_retryBackoff"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, attemptsV, attempts))
	b.WriteString(fmt.Sprintf("%s\t%s := %d\n", pad, backoffV, backoffMs))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
	retryState := cloneFlowState(st)
	retryState.returnErrOnly = true
	b.WriteString(renderFlowSteps(retryState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\tfor _tryAttempt := 0; _tryAttempt < %s; _tryAttempt++ {\n", pad, attemptsV))
	b.WriteString(fmt.Sprintf("%s\t\t%s = %s()\n", pad, errV, runV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s == nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t\tbreak\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tif _tryAttempt+1 < %s && %s > 0 {\n", pad, attemptsV, backoffV))
	b.WriteString(fmt.Sprintf("%s\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n", pad, backoffV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
	if len(catchSteps) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), catchSteps, indent+2))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t", errV))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowFallbackLegacy(st *flowRenderState, action flowir.FlowFallback, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	mainSteps, fallbackSteps := action.Steps, action.Fallback
	if len(mainSteps) == 0 || len(fallbackSteps) == 0 {
		return ""
	}
	runV, errV := "_fbRun"+sfx, "_fbErr"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := func() error {\n", pad, runV))
	fbState := cloneFlowState(st)
	fbState.returnErrOnly = true
	b.WriteString(renderFlowSteps(fbState, mainSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %s()\n", pad, errV, runV))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, errV))
	b.WriteString(renderFlowSteps(cloneFlowState(st), fallbackSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

func renderFlowTimeoutLegacy(st *flowRenderState, action flowir.FlowTimeout, indent int, sfx string) string {
	pad := strings.Repeat("\t", indent)
	duration := normalizeFlowExpr(action.Duration.Source)
	doSteps, onTimeout := action.Steps, action.OnTimeout
	if duration == "" || len(doSteps) == 0 {
		return ""
	}
	toCtxV, toCancelV, toRunV, toErrV := "_toCtx"+sfx, "_toCancel"+sfx, "_toRun"+sfx, "_toErr"+sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s, %s := context.WithTimeout(ctx, %s)\n", pad, toCtxV, toCancelV, duration))
	b.WriteString(fmt.Sprintf("%s\tdefer %s()\n", pad, toCancelV))
	b.WriteString(fmt.Sprintf("%s\t%s := func(ctx context.Context) error {\n", pad, toRunV))
	toState := cloneFlowState(st)
	toState.returnErrOnly = true
	b.WriteString(renderFlowSteps(toState, doSteps, indent+2))
	b.WriteString(fmt.Sprintf("%s\t\treturn nil\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := %s(%s)\n", pad, toErrV, toRunV, toCtxV))
	b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, toErrV))
	b.WriteString(fmt.Sprintf("%s\t\t_flowLastError = %s\n", pad, toErrV))
	b.WriteString(fmt.Sprintf("%s\t\tif %s.Err() == context.DeadlineExceeded {\n", pad, toCtxV))
	if len(onTimeout) > 0 {
		b.WriteString(renderFlowSteps(cloneFlowState(st), onTimeout, indent+3))
	} else {
		b.WriteString(errReturn(st, pad+"\t\t\t", "errors.New(http.StatusGatewayTimeout, \"TIMEOUT\", \"flow step timed out\")"))
	}
	b.WriteString(fmt.Sprintf("%s\t\t} else {\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", toErrV))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
