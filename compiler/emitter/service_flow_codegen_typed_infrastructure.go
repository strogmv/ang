package emitter

import "github.com/strogmv/ang/compiler/flowir"

// renderTypedStepInfrastructure is the typed production entry for AI and
// project-planning actions. Planning and locale actions consume TypedStep
// directly; the remaining AI adapters receive source metadata only, never raw
// scalar arguments or legacy child steps.
func renderTypedStepInfrastructure(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	switch step.Name {
	case "plan.BuildAutomata", "plan.BuildMicroPlan", "cue.EmitProject", "cue.ValidateProject", "cue.WriteProjectFiles":
		return renderTypedStepMetaPlan(st, step, indent, sfx)
	case "locale.Resolve":
		return renderTypedStepLocale(st, step, indent, sfx)
	}

	if out, ok := renderTypedStepClaude(st, step, indent, sfx); ok {
		return out, true
	}

	if out, ok := renderTypedStepOpenAI(st, step, indent, sfx); ok {
		return out, true
	}
	return "", false
}
