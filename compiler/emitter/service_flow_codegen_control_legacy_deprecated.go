package emitter

import "github.com/strogmv/ang-ir/normalizer"

// renderFlowStepControlLegacyDeprecated intentionally avoids duplicating logic
// already covered by dedicated control modules and modern legacy core handlers.
// It only delegates to the historical monolith for potential edge actions.
func renderFlowStepControlLegacyDeprecated(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) string {
	switch step.Action {
	case "flow.If", "flow.For", "flow.Block", "tx.Block",
		"flow.Switch", "flow.While", "flow.Checkpoint", "flow.Resume", "flow.Validate",
		"flow.Try", "flow.Catch", "flow.Defer", "flow.Retry", "flow.Fallback", "flow.Timeout",
		"flow.SuggestNext", "flow.ExplainError":
		return ""
	case "list.Filter", "list.Paginate", "list.Append", "list.Sort", "str.Normalize":
		return ""
	case "mapping.Map", "event.Publish", "logic.Call", "service.Call":
		return ""
	case "exec.Run", "exec.Stream", "fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove":
		return ""
	default:
		// Deprecated fallback is intentionally no-op for unknown actions:
		// supported actions are validated and routed by dedicated modules.
		return ""
	}
}
