package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/parser"
	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/flowir"
)

// goldenFlowStep is a flow step read from a golden CUE file by syntax only, so
// the file does not have to compile against the schema package to be checked.
type goldenFlowStep struct {
	Step normalizer.FlowStep
	Line int
}

var goldenChildKeys = map[string]string{"then": "_then", "else": "_else", "do": "_do", "ifNew": "_ifNew", "ifExists": "_ifExists", "default": "_default", "catch": "_catch", "fallback": "_fallback", "onTimeout": "_onTimeout", "onMissing": "_onMissing", "onMismatch": "_onMismatch"}

// goldenFlowSteps returns every struct literal with an action field in src.
func goldenFlowSteps(filename string, src []byte) ([]goldenFlowStep, error) {
	file, err := parser.ParseFile(filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []goldenFlowStep
	var walkErr error
	ast.Walk(file, func(n ast.Node) bool {
		sl, ok := n.(*ast.StructLit)
		if !ok || !goldenHasAction(sl) {
			return true
		}
		step, err := goldenStepOf(sl)
		if err != nil && walkErr == nil {
			walkErr = fmt.Errorf("%s:%d: %w", filename, sl.Pos().Line(), err)
		}
		out = append(out, goldenFlowStep{Step: step, Line: sl.Pos().Line()})
		return true
	}, nil)
	return out, walkErr
}

func goldenLabel(l ast.Label) string {
	switch v := l.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); err == nil {
			return s
		}
	}
	return ""
}

func goldenHasAction(sl *ast.StructLit) bool {
	for _, d := range sl.Elts {
		if f, ok := d.(*ast.Field); ok && goldenLabel(f.Label) == "action" {
			return true
		}
	}
	return false
}

func goldenStepOf(sl *ast.StructLit) (normalizer.FlowStep, error) {
	step := normalizer.FlowStep{Args: map[string]any{}}
	for _, d := range sl.Elts {
		f, ok := d.(*ast.Field)
		if !ok {
			continue
		}
		label := goldenLabel(f.Label)
		value, err := goldenValue(f.Value)
		if err != nil {
			return step, fmt.Errorf("%s: %w", label, err)
		}
		switch {
		case label == "action":
			s, isString := value.(string)
			if !isString {
				return step, fmt.Errorf("action is not a string")
			}
			step.Action = s
		case (label == "cases" || label == "branches") && goldenIsBranchMap(value):
			// flow.Switch cases and flow.Parallel branches are step lists by
			// name; errors.Map cases are plain objects and stay an argument.
			branches := map[string][]normalizer.FlowStep{}
			for k, v := range value.(map[string]any) {
				branches[k] = v.([]normalizer.FlowStep)
			}
			step.Args["_"+label] = branches
		case goldenChildKeys[label] != "":
			// The same key is a nested step list for control actions (flow.Try
			// catch) and a plain expression for others (approval.Wait onTimeout,
			// locale.Resolve default), as the normalizer reads it.
			if _, isSteps := value.([]normalizer.FlowStep); isSteps {
				step.Args[goldenChildKeys[label]] = value
			} else {
				step.Args[label] = value
			}
		default:
			step.Args[label] = value
		}
	}
	return step, nil
}

func goldenValue(e ast.Expr) (any, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if strings.HasPrefix(v.Value, `"`) || strings.HasPrefix(v.Value, "#") {
			return literal.Unquote(v.Value)
		}
		switch v.Value {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		}
		if i, err := strconv.Atoi(v.Value); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(v.Value, 64); err == nil {
			return f, nil
		}
	case *ast.Ident:
		switch v.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	case *ast.ListLit:
		if len(v.Elts) > 0 {
			if first, ok := v.Elts[0].(*ast.StructLit); ok && goldenHasAction(first) {
				steps := make([]normalizer.FlowStep, 0, len(v.Elts))
				for _, el := range v.Elts {
					sl, ok := el.(*ast.StructLit)
					if !ok {
						return nil, fmt.Errorf("mixed step list")
					}
					step, err := goldenStepOf(sl)
					if err != nil {
						return nil, err
					}
					steps = append(steps, step)
				}
				return steps, nil
			}
		}
		strs := make([]string, 0, len(v.Elts))
		anys := make([]any, 0, len(v.Elts))
		allStrings := true
		for _, el := range v.Elts {
			x, err := goldenValue(el)
			if err != nil {
				return nil, err
			}
			if s, ok := x.(string); ok {
				strs = append(strs, s)
			} else {
				allStrings = false
			}
			anys = append(anys, x)
		}
		if allStrings {
			return strs, nil
		}
		return anys, nil
	case *ast.StructLit:
		m := map[string]any{}
		inner := map[string]map[string]string{}
		allStringMaps := len(v.Elts) > 0
		for _, d := range v.Elts {
			f, ok := d.(*ast.Field)
			if !ok {
				continue
			}
			x, err := goldenValue(f.Value)
			if err != nil {
				return nil, err
			}
			key := goldenLabel(f.Label)
			m[key] = x
			sub, isMap := x.(map[string]any)
			if !isMap {
				allStringMaps = false
				continue
			}
			strMap := map[string]string{}
			for k, sv := range sub {
				s, isString := sv.(string)
				if !isString {
					allStringMaps = false
					break
				}
				strMap[k] = s
			}
			inner[key] = strMap
		}
		if allStringMaps {
			return inner, nil
		}
		return m, nil
	}
	return nil, fmt.Errorf("unsupported value %T", e)
}

const goldenReferenceHeader = `// ============================================================================
// GOLDEN ACTIONS REFERENCE (GENERATED — DO NOT EDIT)
// ============================================================================
// One minimal operation per Typed Flow IR action that cue/GOLDEN_EXAMPLES.cue
// does not already show. Each step is the action's catalog example
// (compiler/flowir/examples.go), which a test decodes with the action's own
// decoder. Regenerate: ANG_UPDATE_GOLDEN=1 go test ./cmd/ang -run TestGoldenActionsReferenceInSync

package examples

import "github.com/strogmv/ang/cue/schema"
`

// renderGoldenActionsReference builds cue/GOLDEN_ACTIONS_REFERENCE.cue for the
// actions not present in covered.
func renderGoldenActionsReference(covered map[string]bool) string {
	var b strings.Builder
	b.WriteString(goldenReferenceHeader)
	names := make([]string, 0)
	for _, spec := range flowir.All() {
		if !covered[spec.Name] {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		example, ok := flowir.ExampleCUE(name)
		if !ok {
			continue
		}
		ident := "Ref"
		for _, part := range strings.Split(name, ".") {
			ident += emitter.ExportName(part)
		}
		fmt.Fprintf(&b, "\n// %s\n%s: schema.#Operation & {\n\tservice: \"reference\"\n\tinput: {id: string}\n\toutput: {ok: bool}\n\tflow: [\n\t\t%s,\n\t\t{action: \"mapping.Assign\", to: \"resp.Ok\", value: \"true\"},\n\t]\n}\n", name, ident, example)
	}
	return b.String()
}

// goldenIsBranchMap reports whether value is an object whose every field is a
// list of flow steps.
func goldenIsBranchMap(value any) bool {
	m, ok := value.(map[string]any)
	if !ok || len(m) == 0 {
		return false
	}
	for _, v := range m {
		if _, isSteps := v.([]normalizer.FlowStep); !isSteps {
			return false
		}
	}
	return true
}
