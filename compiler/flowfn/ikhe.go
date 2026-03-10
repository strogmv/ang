package flowfn

import (
	"errors"
	"fmt"

	"github.com/strogmv/ang/compiler/flowsem"
)

type Diagnostic struct {
	Code    string
	Message string
	Line    int
	Column  int
}

type ValidateOptions struct {
	InStreamingMethod bool
}

func ParseValidateTranspile(source string) ([]Step, []Diagnostic, error) {
	return ParseValidateTranspileWithOptions(source, ValidateOptions{})
}

func ParseValidateTranspileWithOptions(source string, opts ValidateOptions) ([]Step, []Diagnostic, error) {
	program, err := Parse(source)
	if err != nil {
		return nil, nil, err
	}
	expanded, err := ExpandFragments(program)
	if err != nil {
		return nil, nil, err
	}
	steps, err := Transpile(expanded)
	if err != nil {
		return nil, nil, err
	}
	diagnostics := ValidateStepsWithOptions(steps, opts)
	return steps, diagnostics, nil
}

func ValidateProgram(program Program) []Diagnostic {
	return ValidateProgramWithOptions(program, ValidateOptions{})
}

func ValidateProgramWithOptions(program Program, opts ValidateOptions) []Diagnostic {
	expanded, err := ExpandFragments(program)
	if err != nil {
		pos := firstProgramPosition(program)
		return []Diagnostic{{
			Code:    "E_FLOW_MACRO",
			Message: err.Error(),
			Line:    pos.Line,
			Column:  pos.Column,
		}}
	}
	steps, err := Transpile(expanded)
	if err != nil {
		pos := firstProgramPosition(expanded)
		return []Diagnostic{{
			Code:    "E_FLOW_TRANSPILER",
			Message: err.Error(),
			Line:    pos.Line,
			Column:  pos.Column,
		}}
	}
	return ValidateStepsWithOptions(steps, opts)
}

func Validate(program Program) error {
	diagnostics := ValidateProgram(program)
	return ValidateDiagnostics(diagnostics)
}

func ValidateDiagnostics(diagnostics []Diagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	errs := make([]error, 0, len(diagnostics))
	for _, diag := range diagnostics {
		errs = append(errs, fmt.Errorf("%s at %d:%d: %s", diag.Code, diag.Line, diag.Column, diag.Message))
	}
	return errors.Join(errs...)
}

func ValidateStepsWithOptions(steps []Step, opts ValidateOptions) []Diagnostic {
	issues := flowsem.ValidateWithOptions(toFlowsemSteps(steps), flowsem.ValidateOptions{
		InStreamingMethod: opts.InStreamingMethod,
	})
	out := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		out = append(out, Diagnostic{
			Code:    issue.Code,
			Message: issue.Message,
			Line:    issue.Line,
			Column:  issue.Column,
		})
	}
	return out
}

func toFlowsemSteps(steps []Step) []flowsem.Step {
	out := make([]flowsem.Step, 0, len(steps))
	for _, step := range steps {
		item := flowsem.Step{
			Action:   step.Action,
			Args:     step.Args,
			Children: toFlowsemChildren(step.Children),
			Line:     step.Line,
			Column:   step.Column,
		}
		out = append(out, item)
	}
	return out
}

func toFlowsemChildren(children map[string][]Step) map[string][]flowsem.Step {
	if len(children) == 0 {
		return nil
	}
	out := make(map[string][]flowsem.Step, len(children))
	for name, steps := range children {
		out[name] = toFlowsemSteps(steps)
	}
	return out
}

func firstProgramPosition(program Program) Position {
	for _, node := range program.Nodes {
		if node != nil {
			return node.Position()
		}
	}
	return Position{Line: 1, Column: 1}
}
