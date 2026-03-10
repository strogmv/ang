package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// renderFlowParallel emits errgroup-style concurrent code for flow.Parallel.
// All branches must succeed; the first error cancels remaining branches (fail-fast).
func renderFlowParallel(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	_ = arg
	_ = child
	pad := strings.Repeat("\t", indent)
	pfx := "_fp" + sfx // e.g. "_fp_0"

	branches, _ := step.Args["_branches"].(map[string][]normalizer.FlowStep)
	if len(branches) == 0 {
		return ""
	}
	keys := make([]string, 0, len(branches))
	for k := range branches {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Probe branches to find new output variables; pre-declare them in outer scope.
	outerDeclared := make(map[string]bool, len(st.declared))
	for k, v := range st.declared {
		outerDeclared[k] = v
	}
	type newVar struct {
		typ   string
		isPtr bool
	}
	newVars := make(map[string]newVar)
	for _, k := range keys {
		probeState := cloneFlowState(st)
		_ = renderFlowSteps(probeState, branches[k], indent+1)
		for varName := range probeState.declared {
			if outerDeclared[varName] {
				continue
			}
			goType := probeState.types[varName]
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = newVar{goType, probeState.pointers[varName]}
		}
	}

	var b strings.Builder
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, varName, v.typ))
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	b.WriteString(fmt.Sprintf("%s%sCtx, %sCancel := context.WithCancel(ctx)\n", pad, pfx, pfx))
	b.WriteString(fmt.Sprintf("%sdefer %sCancel()\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sWg sync.WaitGroup\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sMu sync.Mutex\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sErr error\n", pad, pfx))

	for _, k := range keys {
		branchSteps := branches[k]
		branchState := cloneFlowState(st)
		branchState.concurrMode = "parallel"
		branchState.concurrVarPfx = pfx
		b.WriteString(fmt.Sprintf("%s%sWg.Add(1)\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%sgo func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefer %sWg.Done()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\tctx := %sCtx // shadow outer ctx; cancelled if sibling fails\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t_ = ctx\n", pad))
		b.WriteString(renderFlowSteps(branchState, branchSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}() // branch: %s\n", pad, k))
	}

	b.WriteString(fmt.Sprintf("%s%sWg.Wait()\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%sif %sErr != nil {\n", pad, pfx))
	b.WriteString(errReturn(st, pad+"\t", pfx+"Err"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// renderFlowJoin emits lenient concurrent code for flow.Join.
// All branches run to completion; if any fail, the first error is returned at the end.
func renderFlowJoin(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	_ = arg
	_ = child
	pad := strings.Repeat("\t", indent)
	pfx := "_fj" + sfx // e.g. "_fj_0"

	branches, _ := step.Args["_branches"].(map[string][]normalizer.FlowStep)
	if len(branches) == 0 {
		return ""
	}
	keys := make([]string, 0, len(branches))
	for k := range branches {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Probe branches to find new output variables; pre-declare them in outer scope.
	outerDeclared := make(map[string]bool, len(st.declared))
	for k, v := range st.declared {
		outerDeclared[k] = v
	}
	type newVar struct {
		typ   string
		isPtr bool
	}
	newVars := make(map[string]newVar)
	for _, k := range keys {
		probeState := cloneFlowState(st)
		_ = renderFlowSteps(probeState, branches[k], indent+1)
		for varName := range probeState.declared {
			if outerDeclared[varName] {
				continue
			}
			goType := probeState.types[varName]
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = newVar{goType, probeState.pointers[varName]}
		}
	}

	var b strings.Builder
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, varName, v.typ))
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	b.WriteString(fmt.Sprintf("%svar %sWg sync.WaitGroup\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sMu sync.Mutex\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sErrs []error\n", pad, pfx))

	for _, k := range keys {
		branchSteps := branches[k]
		branchState := cloneFlowState(st)
		branchState.concurrMode = "join"
		branchState.concurrVarPfx = pfx
		b.WriteString(fmt.Sprintf("%s%sWg.Add(1)\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%sgo func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefer %sWg.Done()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t_ = ctx\n", pad))
		b.WriteString(renderFlowSteps(branchState, branchSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}() // branch: %s\n", pad, k))
	}

	b.WriteString(fmt.Sprintf("%s%sWg.Wait()\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%sif len(%sErrs) > 0 {\n", pad, pfx))
	b.WriteString(errReturn(st, pad+"\t", pfx+"Errs[0]"))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// renderFlowRace emits first-wins concurrent code for flow.Race.
// The first branch to complete all steps wins; losing branches are cancelled.
// If all branches fail, an error is returned.
//
// Note: output variable assignments are not mutex-protected. This is safe in
// practice because losing branches are cancelled before their final steps complete.
func renderFlowRace(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	_ = arg
	_ = child
	pad := strings.Repeat("\t", indent)
	pfx := "_fr" + sfx // e.g. "_fr_0"

	branches, _ := step.Args["_branches"].(map[string][]normalizer.FlowStep)
	if len(branches) == 0 {
		return ""
	}
	keys := make([]string, 0, len(branches))
	for k := range branches {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Probe all branches to find new output variables; pre-declare them in outer scope
	// so all competing goroutines can write to them.
	outerDeclared := make(map[string]bool, len(st.declared))
	for k, v := range st.declared {
		outerDeclared[k] = v
	}
	type newVar struct {
		typ   string
		isPtr bool
	}
	newVars := make(map[string]newVar)
	for _, k := range keys {
		probeState := cloneFlowState(st)
		_ = renderFlowSteps(probeState, branches[k], indent+1)
		for varName := range probeState.declared {
			if outerDeclared[varName] {
				continue
			}
			goType := probeState.types[varName]
			if goType == "" {
				goType = "any"
			}
			newVars[varName] = newVar{goType, probeState.pointers[varName]}
		}
	}

	var b strings.Builder
	newVarNames := make([]string, 0, len(newVars))
	for n := range newVars {
		newVarNames = append(newVarNames, n)
	}
	sort.Strings(newVarNames)
	for _, varName := range newVarNames {
		v := newVars[varName]
		b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, varName, v.typ))
		st.declared[varName] = true
		st.pointers[varName] = v.isPtr
		st.types[varName] = v.typ
	}

	b.WriteString(fmt.Sprintf("%s%sCtx, %sCancel := context.WithCancel(ctx)\n", pad, pfx, pfx))
	b.WriteString(fmt.Sprintf("%sdefer %sCancel()\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sWg sync.WaitGroup\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sMu sync.Mutex\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%svar %sWon bool\n", pad, pfx))

	for _, k := range keys {
		branchSteps := branches[k]
		branchState := cloneFlowState(st)
		branchState.concurrMode = "race"
		branchState.concurrVarPfx = pfx
		b.WriteString(fmt.Sprintf("%s%sWg.Add(1)\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%sgo func() {\n", pad))
		b.WriteString(fmt.Sprintf("%s\tdefer %sWg.Done()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\tctx := %sCtx\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t_ = ctx\n", pad))
		b.WriteString(renderFlowSteps(branchState, branchSteps, indent+1))
		// Win block: first branch to reach this point claims the win and cancels others.
		b.WriteString(fmt.Sprintf("%s\t%sMu.Lock()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\tif !%sWon {\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t\t%sWon = true\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t\t%sCancel()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%sMu.Unlock()\n", pad, pfx))
		b.WriteString(fmt.Sprintf("%s}() // branch: %s\n", pad, k))
	}

	b.WriteString(fmt.Sprintf("%s%sWg.Wait()\n", pad, pfx))
	b.WriteString(fmt.Sprintf("%sif !%sWon {\n", pad, pfx))
	b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("flow.Race: all branches failed")`))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
