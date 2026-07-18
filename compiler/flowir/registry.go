package flowir

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/strogmv/ang-ir/normalizer"
)

type TypedStep struct {
	Name        string
	Action      Action
	Source      Source
	Children    map[string][]TypedStep
	Branches    map[string][]TypedStep
	DecodeError error
	ScalarArgs  map[string]ScalarArg
}

type StepMeta struct {
	Name   string
	Source Source
}

func (s TypedStep) Meta() StepMeta { return StepMeta{Name: s.Name, Source: s.Source} }

type ScalarKind string

const (
	ScalarString ScalarKind = "string"
	ScalarBool   ScalarKind = "bool"
	ScalarInt    ScalarKind = "int"
	ScalarFloat  ScalarKind = "float"
)

type ScalarArg struct {
	Kind   ScalarKind
	String string
	Bool   bool
	Int    int64
	Float  float64
}

func (a ScalarArg) Source() string {
	switch a.Kind {
	case ScalarString:
		return strings.TrimSpace(a.String)
	case ScalarBool:
		return strconv.FormatBool(a.Bool)
	case ScalarInt:
		return strconv.FormatInt(a.Int, 10)
	case ScalarFloat:
		return strconv.FormatFloat(a.Float, 'g', -1, 64)
	default:
		return ""
	}
}

func SourceOf(step normalizer.FlowStep) Source {
	return Source{File: step.File, Line: step.Line, Column: step.Column, CUEPath: step.CUEPath}
}

var nestedStepKeys = []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"}

func DecodeSteps(steps []normalizer.FlowStep) ([]TypedStep, error) {
	result := make([]TypedStep, 0, len(steps))
	var problems []error
	for _, raw := range steps {
		step, errs := decodeStepTree(raw)
		result = append(result, step)
		problems = append(problems, errs...)
	}
	return result, errors.Join(problems...)
}

func decodeStepTree(raw normalizer.FlowStep) (TypedStep, []error) {
	step := TypedStep{Name: raw.Action, Source: SourceOf(raw), Children: map[string][]TypedStep{}, Branches: map[string][]TypedStep{}, ScalarArgs: map[string]ScalarArg{}}
	for key, value := range raw.Args {
		switch typed := value.(type) {
		case string:
			step.ScalarArgs[key] = ScalarArg{Kind: ScalarString, String: typed}
		case bool:
			step.ScalarArgs[key] = ScalarArg{Kind: ScalarBool, Bool: typed}
		case int:
			step.ScalarArgs[key] = ScalarArg{Kind: ScalarInt, Int: int64(typed)}
		case int64:
			step.ScalarArgs[key] = ScalarArg{Kind: ScalarInt, Int: typed}
		case float64:
			step.ScalarArgs[key] = ScalarArg{Kind: ScalarFloat, Float: typed}
		}
	}
	var problems []error
	action, err := Decode(raw)
	step.Action = action
	if err != nil {
		step.DecodeError = err
		problems = append(problems, err)
	}
	for _, key := range nestedStepKeys {
		children, ok := raw.Args[key].([]normalizer.FlowStep)
		if !ok || len(children) == 0 {
			continue
		}
		decoded, childErr := DecodeSteps(children)
		step.Children[key] = decoded
		if childErr != nil {
			problems = append(problems, childErr)
		}
	}
	if branches, ok := raw.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(branches))
		for key := range branches {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			decoded, childErr := DecodeSteps(branches[key])
			step.Branches[key] = decoded
			if childErr != nil {
				problems = append(problems, childErr)
			}
		}
	}
	if cases, ok := raw.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(cases))
		for key := range cases {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			decoded, childErr := DecodeSteps(cases[key])
			step.Branches[key] = decoded
			if childErr != nil {
				problems = append(problems, childErr)
			}
		}
	}
	return step, problems
}

type ArgKind string

const (
	ArgString      ArgKind = "string"
	ArgExpression  ArgKind = "expression"
	ArgExpressions ArgKind = "expressions"
	ArgIdentifier  ArgKind = "identifier"
	ArgBool        ArgKind = "bool"
	ArgInt         ArgKind = "int"
)

type ArgSpec struct {
	Name     string  `json:"name"`
	Kind     ArgKind `json:"kind"`
	Required bool    `json:"required"`
}

type ActionSpec struct {
	Name          string                                    `json:"name"`
	Description   string                                    `json:"description"`
	Args          []ArgSpec                                 `json:"args"`
	RendererGroup RendererGroup                             `json:"renderer_group"`
	Decode        func(normalizer.FlowStep) (Action, error) `json:"-"`
}

var (
	registryMu sync.RWMutex
	registry   = map[string]ActionSpec{}
)

func Register(spec ActionSpec) {
	if strings.TrimSpace(spec.Name) == "" || spec.Decode == nil {
		panic("flowir: action spec requires name and decoder")
	}
	if spec.RendererGroup == "" {
		spec.RendererGroup = RendererGroupFor(spec.Name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[spec.Name]; exists {
		panic("flowir: duplicate action " + spec.Name)
	}
	registry[spec.Name] = spec
}

func Lookup(name string) (ActionSpec, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	spec, ok := registry[name]
	return spec, ok
}

func All() []ActionSpec {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]ActionSpec, 0, len(registry))
	for _, spec := range registry {
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func Decode(step normalizer.FlowStep) (Action, error) {
	spec, ok := Lookup(step.Action)
	if !ok {
		return nil, fmt.Errorf("action %q is not registered in typed Flow IR", step.Action)
	}
	action, err := spec.Decode(step)
	if err != nil {
		if strings.TrimSpace(step.File) == "" && step.Line == 0 {
			return action, err
		}
		return action, fmt.Errorf("%s:%d %s: %w", step.File, step.Line, step.Action, err)
	}
	return action, nil
}

func DecodeAs[T Action](step normalizer.FlowStep) (T, error) {
	var zero T
	action, err := Decode(step)
	typed, ok := action.(T)
	if !ok {
		if err != nil {
			return zero, err
		}
		return zero, fmt.Errorf("action %q decoded as %T", step.Action, action)
	}
	return typed, err
}
