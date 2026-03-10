package normalizer

import (
	"fmt"
	"sort"
	"strings"

	sharedeffects "github.com/strogmv/ang/compiler/effects"
)

type ValidationError struct {
	Code    string
	Message string
	Hint    string
}

type EffectSet struct {
	Kinds map[sharedeffects.EffectKind]bool
	Tags  map[sharedeffects.SafetyTag]bool
	Scope map[string]string
}

func NewEffectSet() *EffectSet {
	scope := make(map[string]string, len(flowKnownRoots))
	for name := range flowKnownRoots {
		scope[name] = "builtin"
	}
	return &EffectSet{
		Kinds: make(map[sharedeffects.EffectKind]bool),
		Tags:  make(map[sharedeffects.SafetyTag]bool),
		Scope: scope,
	}
}

func (es *EffectSet) HasVar(name string) bool {
	if es == nil {
		return false
	}
	_, ok := es.Scope[strings.TrimSpace(name)]
	return ok
}

func (es *EffectSet) Declare(name, typ string) {
	if es == nil {
		return
	}
	name = strings.TrimSpace(name)
	if !isSimpleIdent(name) {
		return
	}
	typ = strings.TrimSpace(typ)
	if typ == "" {
		typ = "any"
	}
	es.Scope[name] = typ
}

func (es *EffectSet) Clone() *EffectSet {
	if es == nil {
		return NewEffectSet()
	}
	next := NewEffectSet()
	for kind, ok := range es.Kinds {
		next.Kinds[kind] = ok
	}
	for tag, ok := range es.Tags {
		next.Tags[tag] = ok
	}
	for name, typ := range es.Scope {
		next.Scope[name] = typ
	}
	return next
}

func (es *EffectSet) Apply(logos sharedeffects.ActionLogos, outputVar string) {
	if logos.Effect != sharedeffects.EffectPure {
		es.Kinds[logos.Effect] = true
	}
	for _, tag := range logos.ProducesTags {
		es.Tags[tag] = true
	}
	if outputVar != "" {
		es.Scope[outputVar] = "any"
	}
}

func (es *EffectSet) ApplyChildTags(tags []sharedeffects.SafetyTag) {
	for _, tag := range tags {
		es.Tags[tag] = true
	}
}

func ValidateStep(step FlowStep, current *EffectSet) []ValidationError {
	logos, ok := sharedeffects.LookupLogos(step.Action)
	if !ok {
		return nil
	}
	if current == nil {
		current = NewEffectSet()
	}

	var errs []ValidationError
	for _, expr := range flowStepReferenceExprs(step) {
		for _, root := range flowExprRoots(expr.Expr) {
			if isKnownFlowRoot(root) {
				continue
			}
			if current.HasVar(root) {
				continue
			}
			if _, ok := expr.LocalScope[root]; ok {
				continue
			}
			errs = append(errs, ValidationError{
				Code:    "UNDECLARED_FLOW_VAR",
				Message: fmt.Sprintf("undefined flow variable '%s' in %s arg '%s'", root, step.Action, expr.ArgName),
				Hint:    fmt.Sprintf("Declare '%s' in the same scope before usage, or move this step inside the branch where '%s' is declared.", root, root),
			})
		}
	}
	for _, req := range logos.RequiresTags {
		if !current.Tags[req] {
			errs = append(errs, ValidationError{
				Code:    "MISSING_EFFECT_PREREQUISITE",
				Message: fmt.Sprintf("%s requires %s to be established earlier in flow", step.Action, req),
				Hint:    effectPrerequisiteHint(req),
			})
		}
	}

	if current.Tags[sharedeffects.RequireTxOpen] && logos.Effect != sharedeffects.EffectPure && !logos.TxCompatible {
		errs = append(errs, ValidationError{
			Code:    "EXTERNAL_EFFECT_IN_TX",
			Message: fmt.Sprintf("%s cannot be called inside tx.Block (external effect)", step.Action),
			Hint:    "Move the external step outside tx.Block, or persist intent in DB/outbox and execute it after commit.",
		})
	}

	return errs
}

func DeriveOperationEffects(steps []FlowStep) []sharedeffects.EffectKind {
	seen := make(map[sharedeffects.EffectKind]bool)
	var walk func([]FlowStep)
	walk = func(items []FlowStep) {
		for _, step := range items {
			if logos, ok := sharedeffects.LookupLogos(step.Action); ok && logos.Effect != sharedeffects.EffectPure {
				seen[logos.Effect] = true
			}
			for _, branch := range nestedFlowChildren(step.Args) {
				walk(branch)
			}
		}
	}
	walk(steps)

	out := make([]sharedeffects.EffectKind, 0, len(seen))
	for kind := range seen {
		out = append(out, kind)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func validateFlowEffects(opName string, steps []FlowStep) []FlowWarning {
	var warnings []FlowWarning
	var walk func([]FlowStep, *EffectSet)
	walk = func(items []FlowStep, current *EffectSet) {
		for idx, step := range items {
			for _, err := range ValidateStep(step, current) {
				warnings = append(warnings, FlowWarning{
					Op:       opName,
					Step:     idx + 1,
					Action:   step.Action,
					Message:  err.Message,
					Code:     err.Code,
					Severity: "error",
					Hint:     err.Hint,
					File:     step.File,
					Line:     step.Line,
					Column:   step.Column,
					CUEPath:  step.CUEPath,
				})
			}

			logos, ok := sharedeffects.LookupLogos(step.Action)
			nextCurrent := current.Clone()
			if ok {
				outputVar := outputVarForLogos(step, logos)
				nextCurrent.Apply(logos, outputVar)
			}
			for _, name := range flowStepDeclaredVars(step) {
				nextCurrent.Declare(name, "any")
			}

			childState := nextCurrent.Clone()
			if ok && len(logos.ChildTags) > 0 {
				childState.ApplyChildTags(logos.ChildTags)
			}
			if step.Action == "flow.For" {
				if as, _ := step.Args["as"].(string); isSimpleIdent(strings.TrimSpace(as)) {
					childState.Declare(strings.TrimSpace(as), "any")
				}
			}
			for _, branch := range nestedFlowChildren(step.Args) {
				walk(branch, childState.Clone())
			}
			current = nextCurrent
		}
	}

	walk(steps, NewEffectSet())
	return warnings
}

func nestedFlowChildren(args map[string]any) [][]FlowStep {
	if len(args) == 0 {
		return nil
	}
	var out [][]FlowStep
	for key, raw := range args {
		if !strings.HasPrefix(key, "_") {
			continue
		}
		switch v := raw.(type) {
		case []FlowStep:
			out = append(out, v)
		case map[string][]FlowStep:
			keys := make([]string, 0, len(v))
			for name := range v {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			for _, name := range keys {
				out = append(out, v[name])
			}
		}
	}
	return out
}

func outputVarForLogos(step FlowStep, logos sharedeffects.ActionLogos) string {
	key := strings.TrimSpace(logos.ProducesVar)
	if key == "" {
		return ""
	}
	raw, ok := step.Args[key]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func effectPrerequisiteHint(tag sharedeffects.SafetyTag) string {
	switch tag {
	case sharedeffects.RequireSessionPresent:
		return `Add {action: "session.Get", output: "session"} before this step.`
	case sharedeffects.RequireQuotaChecked:
		return `Add {action: "quota.Check", ...} before this step.`
	case sharedeffects.RequireRateChecked:
		return `Add {action: "ratelimit.Check", ...} before this step.`
	case sharedeffects.RequireBudgetChecked:
		return `Add {action: "budget.Check", ...} before this step.`
	case sharedeffects.RequireTxOpen:
		return `Wrap the step in {action: "tx.Block", do: [ ... ]}.`
	case sharedeffects.RequireIdempotencyKey:
		return `Establish idempotency first, for example with {action: "idempotency.DeriveKey", ...}.`
	default:
		return "Establish the required effect earlier in the flow."
	}
}
