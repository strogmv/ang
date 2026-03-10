package flowfn

import (
	"errors"
	"fmt"

	sharedeffects "github.com/strogmv/ang/compiler/effects"
)

type Diagnostic struct {
	Code    string
	Message string
	Line    int
	Column  int
}

type effectState struct {
	Tags map[sharedeffects.SafetyTag]bool
}

func newEffectState() *effectState {
	return &effectState{Tags: map[sharedeffects.SafetyTag]bool{}}
}

func (s *effectState) Clone() *effectState {
	next := newEffectState()
	for tag, ok := range s.Tags {
		next.Tags[tag] = ok
	}
	return next
}

func (s *effectState) Apply(logos sharedeffects.ActionLogos) {
	for _, tag := range logos.ProducesTags {
		s.Tags[tag] = true
	}
}

func (s *effectState) ApplyChildTags(tags []sharedeffects.SafetyTag) {
	for _, tag := range tags {
		s.Tags[tag] = true
	}
}

func ParseValidateTranspile(source string) ([]Step, []Diagnostic, error) {
	program, err := Parse(source)
	if err != nil {
		return nil, nil, err
	}
	expanded, err := ExpandFragments(program)
	if err != nil {
		return nil, nil, err
	}
	diagnostics := ValidateProgram(expanded)
	steps, err := Transpile(expanded)
	return steps, diagnostics, err
}

func ValidateProgram(program Program) []Diagnostic {
	state := newEffectState()
	var out []Diagnostic
	out = append(out, validateNodes(program.Nodes, state)...)
	return out
}

func Validate(program Program) error {
	diagnostics := ValidateProgram(program)
	if len(diagnostics) == 0 {
		return nil
	}
	errs := make([]error, 0, len(diagnostics))
	for _, diag := range diagnostics {
		errs = append(errs, fmt.Errorf("%s at %d:%d: %s", diag.Code, diag.Line, diag.Column, diag.Message))
	}
	return errors.Join(errs...)
}

func validateNodes(nodes []Node, current *effectState) []Diagnostic {
	var out []Diagnostic
	for _, node := range nodes {
		switch n := node.(type) {
		case *CallNode:
			logos, ok := sharedeffects.LookupLogos(n.Action)
			if !ok {
				out = append(out, Diagnostic{Code: "E_FLOW_UNKNOWN_ACTION", Message: fmt.Sprintf("unknown action %q", n.Action), Line: n.Pos.Line, Column: n.Pos.Column})
				continue
			}
			for _, req := range logos.RequiresTags {
				if !current.Tags[req] {
					out = append(out, Diagnostic{
						Code:    "MISSING_EFFECT_PREREQUISITE",
						Message: fmt.Sprintf("%s requires %s to be established earlier in flow", n.Action, req),
						Line:    n.Pos.Line,
						Column:  n.Pos.Column,
					})
				}
			}
			if current.Tags[sharedeffects.RequireTxOpen] && logos.Effect != sharedeffects.EffectPure && !logos.TxCompatible {
				out = append(out, Diagnostic{
					Code:    "EXTERNAL_EFFECT_IN_TX",
					Message: fmt.Sprintf("%s cannot be called inside tx.Block (external effect)", n.Action),
					Line:    n.Pos.Line,
					Column:  n.Pos.Column,
				})
			}

			next := current.Clone()
			next.Apply(logos)
			childState := next.Clone()
			childState.ApplyChildTags(logos.ChildTags)
			for _, body := range n.Blocks {
				out = append(out, validateNodes(body, childState.Clone())...)
			}
			current = next
		case *IfNode:
			out = append(out, validateNodes(n.Then, current.Clone())...)
			out = append(out, validateNodes(n.Else, current.Clone())...)
		case *ForNode:
			out = append(out, validateNodes(n.Do, current.Clone())...)
		case *TryNode:
			out = append(out, validateNodes(n.Do, current.Clone())...)
			out = append(out, validateNodes(n.Catch, current.Clone())...)
		}
	}
	return out
}
