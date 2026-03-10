package flowfn

import (
	"fmt"
)

func Transpile(program Program) ([]Step, error) {
	expanded, err := ExpandFragments(program)
	if err != nil {
		return nil, err
	}
	return transpileNodes(expanded.Nodes)
}

func ParseTranspile(source string) ([]Step, error) {
	program, err := Parse(source)
	if err != nil {
		return nil, err
	}
	return Transpile(program)
}

func transpileNodes(nodes []Node) ([]Step, error) {
	steps := make([]Step, 0, len(nodes))
	for _, node := range nodes {
		step, many, err := transpileNode(node)
		if err != nil {
			return nil, err
		}
		if len(many) > 0 {
			steps = append(steps, many...)
			continue
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func transpileNode(node Node) (Step, []Step, error) {
	switch n := node.(type) {
	case *CallNode:
		step := Step{Action: n.Action, Args: map[string]any{}, Line: n.Pos.Line, Column: n.Pos.Column}
		for k, v := range n.Args {
			step.Args[k] = v.Interface()
		}
		for name, body := range n.Blocks {
			child, err := transpileNodes(body)
			if err != nil {
				return Step{}, nil, err
			}
			step.Args["_"+name] = child
		}
		return step, nil, nil
	case *IfNode:
		thenBody, err := transpileNodes(n.Then)
		if err != nil {
			return Step{}, nil, err
		}
		elseBody, err := transpileNodes(n.Else)
		if err != nil {
			return Step{}, nil, err
		}
		return Step{
			Action: "flow.If",
			Args:   map[string]any{"condition": n.Condition, "_then": thenBody, "_else": elseBody},
			Line:   n.Pos.Line,
			Column: n.Pos.Column,
		}, nil, nil
	case *ForNode:
		body, err := transpileNodes(n.Do)
		if err != nil {
			return Step{}, nil, err
		}
		return Step{
			Action: "flow.For",
			Args:   map[string]any{"alias": n.Alias, "each": n.Each, "_do": body},
			Line:   n.Pos.Line,
			Column: n.Pos.Column,
		}, nil, nil
	case *TryNode:
		doBody, err := transpileNodes(n.Do)
		if err != nil {
			return Step{}, nil, err
		}
		catchBody, err := transpileNodes(n.Catch)
		if err != nil {
			return Step{}, nil, err
		}
		return Step{
			Action: "flow.Try",
			Args:   map[string]any{"_do": doBody, "_catch": catchBody},
			Line:   n.Pos.Line,
			Column: n.Pos.Column,
		}, nil, nil
	case *FragmentNode, *UseNode:
		return Step{}, nil, fmt.Errorf("unexpanded macro node %T", node)
	default:
		return Step{}, nil, fmt.Errorf("unsupported node %T", node)
	}
}
