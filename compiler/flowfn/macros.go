package flowfn

import "fmt"

func ExpandFragments(program Program) (Program, error) {
	fragments := map[string][]Node{}
	for _, node := range program.Nodes {
		if frag, ok := node.(*FragmentNode); ok {
			fragments[frag.Name] = cloneNodes(frag.Body)
		}
	}
	seen := map[string]bool{}
	expanded, err := expandNodes(program.Nodes, fragments, seen)
	if err != nil {
		return Program{}, err
	}
	return Program{Nodes: expanded}, nil
}

func expandNodes(nodes []Node, fragments map[string][]Node, stack map[string]bool) ([]Node, error) {
	var out []Node
	for _, node := range nodes {
		switch n := node.(type) {
		case *FragmentNode:
			continue
		case *UseNode:
			body, ok := fragments[n.Name]
			if !ok {
				return nil, fmt.Errorf("undefined fragment %q at %d:%d", n.Name, n.Pos.Line, n.Pos.Column)
			}
			if stack[n.Name] {
				return nil, fmt.Errorf("cyclic fragment use %q", n.Name)
			}
			stack[n.Name] = true
			expanded, err := expandNodes(cloneNodes(body), fragments, stack)
			delete(stack, n.Name)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
		default:
			cloned := cloneNode(node)
			if err := expandNodeInPlace(cloned, fragments, stack); err != nil {
				return nil, err
			}
			out = append(out, cloned)
		}
	}
	return out, nil
}

func expandNodeInPlace(node Node, fragments map[string][]Node, stack map[string]bool) error {
	switch n := node.(type) {
	case *CallNode:
		for name, body := range n.Blocks {
			expanded, err := expandNodes(body, fragments, stack)
			if err != nil {
				return err
			}
			n.Blocks[name] = expanded
		}
	case *IfNode:
		thenBody, err := expandNodes(n.Then, fragments, stack)
		if err != nil {
			return err
		}
		elseBody, err := expandNodes(n.Else, fragments, stack)
		if err != nil {
			return err
		}
		n.Then = thenBody
		n.Else = elseBody
	case *ForNode:
		body, err := expandNodes(n.Do, fragments, stack)
		if err != nil {
			return err
		}
		n.Do = body
	case *TryNode:
		doBody, err := expandNodes(n.Do, fragments, stack)
		if err != nil {
			return err
		}
		catchBody, err := expandNodes(n.Catch, fragments, stack)
		if err != nil {
			return err
		}
		n.Do = doBody
		n.Catch = catchBody
	}
	return nil
}

func cloneNodes(nodes []Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, cloneNode(node))
	}
	return out
}

func cloneNode(node Node) Node {
	switch n := node.(type) {
	case *CallNode:
		args := make(map[string]Value, len(n.Args))
		for k, v := range n.Args {
			args[k] = v
		}
		var blocks map[string][]Node
		if len(n.Blocks) > 0 {
			blocks = make(map[string][]Node, len(n.Blocks))
			for name, body := range n.Blocks {
				blocks[name] = cloneNodes(body)
			}
		}
		return &CallNode{Pos: n.Pos, Action: n.Action, Args: args, Blocks: blocks}
	case *IfNode:
		return &IfNode{Pos: n.Pos, Condition: n.Condition, Then: cloneNodes(n.Then), Else: cloneNodes(n.Else)}
	case *ForNode:
		return &ForNode{Pos: n.Pos, Alias: n.Alias, Each: n.Each, Do: cloneNodes(n.Do)}
	case *TryNode:
		return &TryNode{Pos: n.Pos, Do: cloneNodes(n.Do), Catch: cloneNodes(n.Catch)}
	case *FragmentNode:
		return &FragmentNode{Pos: n.Pos, Name: n.Name, Body: cloneNodes(n.Body)}
	case *UseNode:
		return &UseNode{Pos: n.Pos, Name: n.Name}
	default:
		return nil
	}
}
