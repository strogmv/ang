package port

import "context"

// PolicyInput is the canonical payload used by policy.Evaluate/Require/Decide flow actions.
type PolicyInput struct {
	PolicyKey string
	Subject   string
	Resource  string
	Operation string
	Tenant    string
	Attrs     any
	Context   any
}

// PolicyDecision describes resolved policy verdict and optional side effects.
type PolicyDecision struct {
	Decision string
	Reason   string
	Effects  map[string]any
}

// PolicyEngine evaluates policy inputs into deterministic decisions.
type PolicyEngine interface {
	Evaluate(ctx context.Context, input PolicyInput) (PolicyDecision, error)
}
