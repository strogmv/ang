package flowir

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/strogmv/ang-ir/normalizer"
)

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
	Name        string                                    `json:"name"`
	Description string                                    `json:"description"`
	Args        []ArgSpec                                 `json:"args"`
	Decode      func(normalizer.FlowStep) (Action, error) `json:"-"`
}

var (
	registryMu sync.RWMutex
	registry   = map[string]ActionSpec{}
)

func Register(spec ActionSpec) {
	if strings.TrimSpace(spec.Name) == "" || spec.Decode == nil {
		panic("flowir: action spec requires name and decoder")
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
		return nil, fmt.Errorf("%s:%d %s: %w", step.File, step.Line, step.Action, err)
	}
	return action, nil
}

func DecodeAs[T Action](step normalizer.FlowStep) (T, error) {
	var zero T
	action, err := Decode(step)
	if err != nil {
		return zero, err
	}
	typed, ok := action.(T)
	if !ok {
		return zero, fmt.Errorf("action %q decoded as %T", step.Action, action)
	}
	return typed, nil
}
