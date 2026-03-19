package policy

import (
	"context"
	"strings"

	"github.com/example/blog/internal/port"
)

// Rule evaluates a policy input and returns a decision.
type Rule func(ctx context.Context, input port.PolicyInput) (port.PolicyDecision, error)

// Engine is a tiny deterministic policy runtime with pluggable rules by key.
type Engine struct {
	rules map[string]Rule
}

// NewEngine creates default engine. Unknown policy keys return allow decision.
func NewEngine() *Engine {
	return &Engine{
		rules: map[string]Rule{},
	}
}

// Register binds/overrides policy rule by key.
func (e *Engine) Register(policyKey string, rule Rule) {
	if e == nil || strings.TrimSpace(policyKey) == "" || rule == nil {
		return
	}
	if e.rules == nil {
		e.rules = map[string]Rule{}
	}
	e.rules[strings.TrimSpace(policyKey)] = rule
}

// Evaluate returns decision for a given policy input.
func (e *Engine) Evaluate(ctx context.Context, input port.PolicyInput) (port.PolicyDecision, error) {
	if e != nil && e.rules != nil {
		key := strings.TrimSpace(input.PolicyKey)
		if rule, ok := e.rules[key]; ok && rule != nil {
			return rule(ctx, input)
		}
	}
	return port.PolicyDecision{
		Decision: "allow",
		Reason:   "policy key not registered",
		Effects:  map[string]any{},
	}, nil
}

var _ port.PolicyEngine = (*Engine)(nil)
