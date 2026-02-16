package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// flowRenderable reports whether all actions inside steps are supported by RenderFlow.
func flowRenderable(steps []normalizer.FlowStep) bool {
	for _, step := range steps {
		if !flowActionSupported(step.Action) {
			return false
		}
		for _, child := range flowChildSteps(step) {
			if !flowRenderable(child) {
				return false
			}
		}
	}
	return true
}

func flowActionSupported(action string) bool {
	switch action {
	case "logic.Check",
		"repo.Find", "repo.Get", "repo.GetForUpdate", "repo.List", "repo.Save", "repo.Delete",
		"mapping.Assign",
		"flow.If", "flow.For", "flow.Block", "tx.Block",
		"list.Filter", "list.Paginate", "list.Append", "list.Sort",
		"str.Normalize":
		return true
	default:
		return false
	}
}

func flowChildSteps(step normalizer.FlowStep) [][]normalizer.FlowStep {
	var out [][]normalizer.FlowStep
	if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(cases))
		for k := range cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(cases[k]) > 0 {
				out = append(out, cases[k])
			}
		}
	}
	return out
}

type flowRenderState struct {
	declared map[string]bool
}

func renderFlow(steps []normalizer.FlowStep) string {
	st := &flowRenderState{
		declared: map[string]bool{
			"resp": true,
			"err":  true,
		},
	}
	return renderFlowSteps(st, steps, 0)
}

func renderFlowSteps(st *flowRenderState, steps []normalizer.FlowStep, indent int) string {
	var b strings.Builder
	for _, step := range steps {
		b.WriteString(renderOneFlowStep(st, step, indent))
	}
	return b.String()
}

func renderOneFlowStep(st *flowRenderState, step normalizer.FlowStep, indent int) string {
	pad := strings.Repeat("\t", indent)
	arg := func(name string) string {
		if v, ok := step.Args[name]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	child := func(name string) []normalizer.FlowStep {
		if v, ok := step.Args[name].([]normalizer.FlowStep); ok {
			return v
		}
		return nil
	}

	switch step.Action {
	case "logic.Check":
		cond := arg("condition")
		throw := arg("throw")
		if cond == "" {
			return ""
		}
		if throw == "" {
			throw = "validation failed"
		}
		return fmt.Sprintf("%sif !(%s) {\n%s\treturn resp, errors.New(http.StatusBadRequest, \"Validation Error\", %q)\n%s}\n", pad, cond, pad, throw, pad)

	case "repo.Get", "repo.Find", "repo.GetForUpdate", "repo.List":
		source := arg("source")
		if source == "" {
			return ""
		}
		method := arg("method")
		input := arg("input")
		output := arg("output")
		if method == "" {
			switch step.Action {
			case "repo.List":
				method = "ListAll"
			case "repo.GetForUpdate":
				method = "GetByIDForUpdate"
			default:
				method = "FindByID"
			}
		}
		call := "ctx"
		if input != "" {
			call += ", " + input
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			return fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n%sif err != nil {\n%s\treturn resp, err\n%s}\n", pad, output+", err", assign, ExportName(source), method, call, pad, pad, pad)
		}
		return fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n%s\treturn resp, err\n%s}\n", pad, ExportName(source), method, call, pad, pad)

	case "repo.Save", "repo.Delete":
		source := arg("source")
		if source == "" {
			return ""
		}
		method := arg("method")
		if method == "" {
			if step.Action == "repo.Save" {
				method = "Save"
			} else {
				method = "Delete"
			}
		}
		input := arg("input")
		call := "ctx"
		if input != "" {
			call += ", " + input
		}
		return fmt.Sprintf("%sif err := s.%sRepo.%s(%s); err != nil {\n%s\treturn resp, err\n%s}\n", pad, ExportName(source), method, call, pad, pad)

	case "mapping.Assign":
		to := arg("to")
		val := arg("value")
		if to == "" || val == "" {
			return ""
		}
		declare := false
		if v, ok := step.Args["declare"]; ok {
			switch x := v.(type) {
			case bool:
				declare = x
			case string:
				declare = strings.EqualFold(strings.TrimSpace(x), "true")
			}
		}
		op := "="
		if declare && !st.declared[to] {
			op = ":="
			st.declared[to] = true
		}
		return fmt.Sprintf("%s%s %s %s\n", pad, to, op, val)

	case "flow.If":
		cond := arg("condition")
		if cond == "" {
			return ""
		}
		thenSteps := child("_then")
		elseSteps := child("_else")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		b.WriteString(renderFlowSteps(st, thenSteps, indent+1))
		b.WriteString(fmt.Sprintf("%s}", pad))
		if len(elseSteps) > 0 {
			b.WriteString(" else {\n")
			b.WriteString(renderFlowSteps(st, elseSteps, indent+1))
			b.WriteString(fmt.Sprintf("%s}", pad))
		}
		b.WriteString("\n")
		return b.String()

	case "flow.For":
		each := arg("each")
		as := arg("as")
		if each == "" {
			return ""
		}
		if as == "" {
			as = "item"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, each))
		b.WriteString(renderFlowSteps(st, child("_do"), indent+1))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.Block", "tx.Block":
		return renderFlowSteps(st, child("_do"), indent)

	case "list.Filter":
		from := arg("from")
		as := arg("as")
		cond := arg("condition")
		out := arg("output")
		if from == "" || out == "" || cond == "" {
			return ""
		}
		if as == "" {
			as = "item"
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		return fmt.Sprintf("%s%s := %s[:0]\n%sfor _, %s := range %s {\n%s\tif %s {\n%s\t\t%s = append(%s, %s)\n%s\t}\n%s}\n",
			pad, out, from,
			pad, as, from,
			pad, cond,
			pad, out, out, as,
			pad, pad)

	case "list.Paginate":
		in := arg("input")
		off := arg("offset")
		lim := arg("limit")
		out := arg("output")
		if in == "" || off == "" || lim == "" || out == "" {
			return ""
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		defaultLimit := 50
		if v, ok := step.Args["defaultLimit"]; ok {
			switch n := v.(type) {
			case int:
				defaultLimit = n
			case int64:
				defaultLimit = int(n)
			case float64:
				defaultLimit = int(n)
			}
		}
		return fmt.Sprintf(`%s_off := %s
%sif _off < 0 { _off = 0 }
%s_lim := %s
%sif _lim <= 0 { _lim = %d }
%s_start := _off
%sif _start > len(%s) { _start = len(%s) }
%s_end := _start + _lim
%sif _end > len(%s) { _end = len(%s) }
%s%s := %s[_start:_end]
`, pad, off,
			pad,
			pad, lim,
			pad, defaultLimit,
			pad,
			pad, in, in,
			pad,
			pad, in, in,
			pad, out, in)

	case "list.Append":
		to := arg("to")
		item := arg("item")
		if to == "" || item == "" {
			return ""
		}
		return fmt.Sprintf("%s%s = append(%s, %s)\n", pad, to, to, item)

	case "list.Sort":
		in := arg("input")
		out := arg("output")
		by := arg("by")
		order := strings.ToLower(arg("order"))
		if in == "" || out == "" || by == "" {
			return ""
		}
		if order != "desc" {
			order = "asc"
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		cmp := "<"
		if order == "desc" {
			cmp = ">"
		}
		return fmt.Sprintf("%s%s := append(%s[:0:0], %s...)\n%ssort.Slice(%s, func(i, j int) bool { return %s[i].%s %s %s[j].%s })\n", pad, out, in, in, pad, out, out, by, cmp, out, by)

	case "str.Normalize":
		in := arg("input")
		mode := strings.ToLower(arg("mode"))
		out := arg("output")
		if in == "" || out == "" {
			return ""
		}
		if !st.declared[out] {
			st.declared[out] = true
		}
		switch mode {
		case "trim":
			return fmt.Sprintf("%s%s := strings.TrimSpace(%s)\n", pad, out, in)
		default:
			return fmt.Sprintf("%s%s := strings.ToLower(strings.TrimSpace(%s))\n", pad, out, in)
		}
	default:
		return ""
	}
}
