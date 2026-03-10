package normalizer

import (
	"fmt"

	"github.com/strogmv/ang/compiler/flowfn"
)

func (n *Normalizer) parseFlowFn(src string) ([]FlowStep, error) {
	steps, diags, err := flowfn.ParseValidateTranspile(src)
	if err != nil {
		return nil, err
	}
	if err := flowfn.ValidateDiagnostics(diags); err != nil {
		return nil, err
	}
	return convertFlowFnSteps(steps), nil
}

func convertFlowFnSteps(steps []flowfn.Step) []FlowStep {
	out := make([]FlowStep, 0, len(steps))
	for _, step := range steps {
		args := map[string]any{}
		for k, v := range step.Args {
			args[k] = v
		}
		for name, child := range step.Children {
			args[name] = convertFlowFnSteps(child)
		}
		out = append(out, FlowStep{
			Action: step.Action,
			Args:   args,
		})
	}
	return out
}

func parseFlowFnString(value string, svcName, opName string) ([]FlowStep, error) {
	n := New()
	steps, err := n.parseFlowFn(value)
	if err != nil {
		return nil, fmt.Errorf("parse flowfn for %s.%s: %w", svcName, opName, err)
	}
	return steps, nil
}
