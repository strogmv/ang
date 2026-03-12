package main

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// ─────────────────────────────────────────────────────────────────────────────
// Action Logos — semantic passport for every flow action.
//
// Each action carries a machine-readable description of:
//   - Requires  : capabilities that MUST exist before this step fires
//   - Provides  : capabilities this step guarantees after success
//   - Effects   : side-effects (db.write, external.call, event.emit, …)
//   - FailureMode: how the action fails (retryable | terminal | compensatable | idempotent)
//   - Security  : auth requirements ("authenticated", "rbac", "none")
//
// The graph engine uses these to build per-operation proof reports.
// ─────────────────────────────────────────────────────────────────────────────

// ActionLogos is the semantic passport of a single flow action.
type ActionLogos struct {
	// Requires: capability tokens that must be present in the accumulated
	// capability set BEFORE this step runs.
	Requires []string
	// Provides: capability tokens this step adds to the set after success.
	Provides []string
	// Effects: observable side-effects this step may produce.
	Effects []string
	// FailureMode: behaviour on failure.
	//   "retryable"     — safe to retry (idempotent read or external GET)
	//   "terminal"      — must not retry (non-idempotent, e.g. email send)
	//   "compensatable" — paired with a compensation step in a saga
	//   "idempotent"    — write that is idempotent by design
	FailureMode string
	// Security: security constraints.
	Security []string
}

// Capability token constants — used in Requires/Provides.
const (
	capAuthVerified       = "auth.verified"
	capInputValidated     = "input.validated"
	capEntityLoaded       = "entity.loaded"
	capEntityWritten      = "entity.written"
	capEventScheduled     = "event.scheduled"
	capIdempotencyChecked = "idempotency.checked"
	capIdempotencySaved   = "idempotency.saved"
	capCompensationReady  = "compensation.ready"
)

// Effect token constants — used in Effects.
const (
	effectDBRead    = "db.read"
	effectDBWrite   = "db.write"
	effectExtCall   = "external.call"
	effectEventEmit = "event.emit"
	effectStateMut  = "state.mutate"
	effectAudit     = "audit.log"
	effectNotify    = "notify.send"
)

// logosDB maps canonical action names to their semantic passports.
// Actions absent from the catalog are treated as unknown logos (nil).
var logosDB = map[string]ActionLogos{
	// ── Auth / authz ────────────────────────────────────────────────────────
	"auth.Verify": {
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"none"},
	},
	"auth.RequireRole": {
		Requires:    []string{capAuthVerified},
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"authenticated"},
	},
	"auth.CheckRole": {
		Requires:    []string{capAuthVerified},
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"authenticated"},
	},
	"rbac.CheckPermission": {
		Requires:    []string{capAuthVerified},
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"authenticated"},
	},
	"policy.Require": {
		Requires:    []string{capAuthVerified},
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"authenticated"},
	},
	"auth.GetClaims": {
		Provides:    []string{capAuthVerified},
		FailureMode: "terminal",
		Security:    []string{"none"},
	},

	// ── Input validation ────────────────────────────────────────────────────
	"logic.Validate": {
		Provides:    []string{capInputValidated},
		FailureMode: "terminal",
	},
	"logic.Check": {
		Provides:    []string{capInputValidated},
		FailureMode: "terminal",
	},
	"logic.Assert": {
		Provides:    []string{capInputValidated},
		FailureMode: "terminal",
	},

	// ── Idempotency ─────────────────────────────────────────────────────────
	"idempotency.DeriveKey": {
		Provides:    []string{capIdempotencyChecked},
		FailureMode: "retryable",
	},
	"idempotency.Check": {
		Provides:    []string{capIdempotencyChecked},
		FailureMode: "retryable",
	},
	"idempotency.SaveResult": {
		Requires:    []string{capIdempotencyChecked},
		Provides:    []string{capIdempotencySaved},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"dedupe.Once": {
		Provides:    []string{capIdempotencyChecked, capIdempotencySaved},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},

	// ── Repository reads ─────────────────────────────────────────────────────
	"repo.Get": {
		Provides:    []string{capEntityLoaded},
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.Find": {
		Provides:    []string{capEntityLoaded},
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.List": {
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.ListAll": {
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.Count": {
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.Exists": {
		Effects:     []string{effectDBRead},
		FailureMode: "retryable",
	},
	"repo.GetForUpdate": {
		Provides:    []string{capEntityLoaded},
		Effects:     []string{effectDBRead, effectDBWrite}, // row lock
		FailureMode: "retryable",
	},

	// ── Repository writes ────────────────────────────────────────────────────
	"repo.Save": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.Upsert": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "idempotent",
	},
	"repo.SaveOrUpdate": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "idempotent",
	},
	"repo.Update": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.BulkSave": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.Delete": {
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.BulkDelete": {
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.DeleteByQuery": {
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},
	"repo.Patch": {
		Provides:    []string{capEntityWritten},
		Effects:     []string{effectDBWrite},
		FailureMode: "retryable",
	},

	// ── Event emission ───────────────────────────────────────────────────────
	"event.Publish": {
		Effects:     []string{effectEventEmit},
		FailureMode: "terminal", // not transactional
	},
	"event.Broadcast": {
		Effects:     []string{effectEventEmit},
		FailureMode: "terminal",
	},
	"event.Outbox": {
		Provides:    []string{capEventScheduled},
		Effects:     []string{effectDBWrite, effectEventEmit},
		FailureMode: "retryable", // atomic with tx
	},
	"outbox.Save": {
		Provides:    []string{capEventScheduled},
		Effects:     []string{effectDBWrite, effectEventEmit},
		FailureMode: "retryable",
	},

	// ── External calls ───────────────────────────────────────────────────────
	"http.Request": {
		Effects:     []string{effectExtCall},
		FailureMode: "compensatable",
	},
	"http.Call": {
		Effects:     []string{effectExtCall},
		FailureMode: "compensatable",
	},
	"http.Get": {
		Effects:     []string{effectExtCall},
		FailureMode: "retryable",
	},
	"http.Post": {
		Effects:     []string{effectExtCall},
		FailureMode: "compensatable",
	},
	"http.Paginate": {
		Effects:     []string{effectExtCall},
		FailureMode: "retryable",
	},

	// ── Flow control (structural — neutral capabilities) ────────────────────
	"tx.Block": {
		// tx.Block itself doesn't require/provide capabilities;
		// child steps accumulate inside and bubble up via nested graph.
		FailureMode: "retryable",
	},
	"flow.Try": {
		Provides:    []string{capCompensationReady},
		FailureMode: "compensatable",
	},
	"flow.Saga": {
		Provides:    []string{capCompensationReady},
		FailureMode: "compensatable",
	},
	"flow.If": {
		FailureMode: "retryable",
	},
	"flow.Switch": {
		FailureMode: "retryable",
	},

	// ── Notifications ────────────────────────────────────────────────────────
	"notify.Send": {
		Effects:     []string{effectNotify},
		FailureMode: "terminal",
	},
	"notify.Email": {
		Effects:     []string{effectNotify},
		FailureMode: "terminal",
	},
	"mail.Send": {
		Effects:     []string{effectNotify},
		FailureMode: "terminal",
	},
	"notification.Send": {
		Effects:     []string{effectNotify},
		FailureMode: "terminal",
	},

	// ── Audit ────────────────────────────────────────────────────────────────
	"audit.Log": {
		Effects:     []string{effectAudit},
		FailureMode: "retryable",
	},

	// ── Pure transformations (no side effects) ───────────────────────────────
	"mapping.Map":     {FailureMode: "retryable"},
	"mapping.Assign":  {FailureMode: "retryable"},
	"mapping.Merge":   {FailureMode: "retryable"},
	"logic.Return":    {FailureMode: "retryable"},
	"logic.Throw":     {FailureMode: "terminal"},
	"logic.Set":       {FailureMode: "retryable"},
	"logic.Condition": {FailureMode: "retryable"},
}

// logosFor looks up the logos for an action, returning nil if unknown.
func logosFor(action string) *ActionLogos {
	l, ok := logosDB[action]
	if !ok {
		return nil
	}
	return &l
}

// ─────────────────────────────────────────────────────────────────────────────
// FlowGraph — directed graph representing relationships between flow steps.
// ─────────────────────────────────────────────────────────────────────────────

// EdgeKind describes the semantic type of a relationship between two steps.
type EdgeKind string

const (
	// EdgeData: step J consumes the output variable produced by step I.
	EdgeData EdgeKind = "data"
	// EdgeEffect: step I's effect must complete before step J for correctness.
	EdgeEffect EdgeKind = "effect"
	// EdgeSafety: step I is a guard (auth/validation) required by step J.
	EdgeSafety EdgeKind = "safety"
	// EdgeControl: step J is nested inside step I's child block.
	EdgeControl EdgeKind = "control"
)

// FlowEdge is a directed edge from step I to step J.
type FlowEdge struct {
	From int
	To   int
	Kind EdgeKind
	// Label is a human-readable description of why this edge exists.
	Label string
}

// FlowNode is a decorated flow step with its resolved logos.
type FlowNode struct {
	Index int // 0-based position in the flat step list
	Step  normalizer.FlowStep
	Logos *ActionLogos // nil if unknown action
	// CapsAfter is the set of capabilities available AFTER this step executes.
	CapsAfter map[string]bool
}

// FlowGraph is the full interaction graph for one operation's flow.
type FlowGraph struct {
	Nodes []FlowNode
	Edges []FlowEdge
}

// buildFlowGraph constructs a FlowGraph by walking the flow linearly and
// resolving data-dependency + safety edges from the logos catalog.
func buildFlowGraph(flow []normalizer.FlowStep) FlowGraph {
	// Flatten the flow into a linear sequence with edge annotation.
	// For nested blocks (tx.Block, flow.If, etc.) we recurse and record
	// EdgeControl from the parent step.
	var nodes []FlowNode
	var edges []FlowEdge
	flattenFlow(flow, -1, &nodes, &edges)
	return FlowGraph{Nodes: nodes, Edges: edges}
}

func flattenFlow(steps []normalizer.FlowStep, parentIdx int, nodes *[]FlowNode, edges *[]FlowEdge) {
	for _, step := range steps {
		idx := len(*nodes)
		logos := logosFor(step.Action)
		node := FlowNode{
			Index: idx,
			Step:  step,
			Logos: logos,
		}
		*nodes = append(*nodes, node)
		if parentIdx >= 0 {
			*edges = append(*edges, FlowEdge{
				From:  parentIdx,
				To:    idx,
				Kind:  EdgeControl,
				Label: fmt.Sprintf("nested inside %s", (*nodes)[parentIdx].Step.Action),
			})
		}
		// Recurse into child blocks
		for key, raw := range step.Args {
			if !strings.HasPrefix(key, "_") {
				continue
			}
			switch v := raw.(type) {
			case []normalizer.FlowStep:
				flattenFlow(v, idx, nodes, edges)
			case map[string][]normalizer.FlowStep:
				for _, branch := range v {
					flattenFlow(branch, idx, nodes, edges)
				}
			}
		}
	}
}

// capabilitySetAtStep returns the set of capabilities accumulated by steps
// 0..N-1 (exclusive) in the linear node list (ignoring control nesting for
// simplicity — linear order is a conservative approximation).
func capabilitySetUpTo(nodes []FlowNode, upToExclusive int) map[string]bool {
	caps := make(map[string]bool)
	for i := 0; i < upToExclusive && i < len(nodes); i++ {
		l := nodes[i].Logos
		if l == nil {
			continue
		}
		for _, p := range l.Provides {
			caps[p] = true
		}
	}
	return caps
}

// ─────────────────────────────────────────────────────────────────────────────
// L3 chain lints — reason about sequences and relationships between steps.
// ─────────────────────────────────────────────────────────────────────────────

// L3 chain lint codes.
const (
	codeChainMissingAuthGate             = "E_CHAIN_MISSING_AUTH_GATE"
	codeChainExternalWithoutCompensation = "E_CHAIN_EXTERNAL_EFFECT_WITHOUT_COMPENSATION"
	codeChainUntrustedInputToSink        = "E_CHAIN_UNTRUSTED_INPUT_TO_SENSITIVE_SINK"
	codeChainUnguardedRetry              = "E_CHAIN_UNGUARDED_RETRY_ON_NON_IDEMPOTENT_ACTION"
	codeChainDBWriteBeforeValidation     = "E_CHAIN_DB_WRITE_BEFORE_VALIDATION"
)

// runChainLints builds the flow graph for each operation and runs L3 lints.
func runChainLints(services []normalizer.Service, endpoints []normalizer.Endpoint, profile lintProfile) []normalizer.Warning {
	endpointByOp := buildEndpointIndex(endpoints)
	var out []normalizer.Warning
	for _, svc := range services {
		for _, method := range svc.Methods {
			opPath := svc.Name + "." + method.Name
			if len(method.Flow) == 0 {
				continue
			}
			graph := buildFlowGraph(method.Flow)
			endpoint, hasEndpoint := endpointByOp[strings.ToLower(opPath)]
			_ = hasEndpoint
			out = append(out, lintChainMissingAuthGate(opPath, graph, endpoint, profile)...)
			out = append(out, lintChainExternalWithoutCompensation(opPath, graph)...)
			out = append(out, lintChainDBWriteBeforeValidation(opPath, graph)...)
		}
	}
	return out
}

// ── L3 lint: sensitive sink without prior auth capability ────────────────────

// sensitiveWriteActions are writes where missing auth is a security violation.
var sensitiveWriteActions = map[string]bool{
	"repo.Save":   true,
	"repo.Update": true,
	"repo.Delete": true,
	"repo.Upsert": true,
	"repo.Patch":  true,
}

func lintChainMissingAuthGate(opPath string, g FlowGraph, endpoint normalizer.Endpoint, profile lintProfile) []normalizer.Warning {
	// Skip if endpoint already declares auth at the transport level.
	if endpointHasAuthz(endpoint) || isPublicAuthEndpoint(endpoint) {
		return nil
	}
	// Skip non-mutating endpoints.
	if !isMutatingHTTPMethod(endpoint.Method) {
		return nil
	}

	var out []normalizer.Warning
	for i, node := range g.Nodes {
		if !sensitiveWriteActions[node.Step.Action] {
			continue
		}
		caps := capabilitySetUpTo(g.Nodes, i)
		if caps[capAuthVerified] {
			continue
		}
		out = append(out, normalizer.Warning{
			Kind:     "chain",
			Code:     codeChainMissingAuthGate,
			Severity: severityForProfile(codeChainMissingAuthGate, profile, "warn"),
			Message: fmt.Sprintf(
				"'%s' at flow[%d] writes to DB but no auth.* / rbac.* step precedes it in the flow — trust boundary unguarded",
				node.Step.Action, node.Index,
			),
			Op:      opPath,
			Step:    node.Index + 1,
			Action:  node.Step.Action,
			File:    node.Step.File,
			Line:    node.Step.Line,
			Column:  node.Step.Column,
			CUEPath: stepPath(opPath, node.Step, node.Index+1),
			Hint:    "Add auth.RequireRole / rbac.CheckPermission / policy.Require before DB writes, OR declare endpoint auth (jwt/roles) in the CUE endpoint block.",
		})
	}
	return out
}

// ── L3 lint: external call without compensation / try-catch ──────────────────

func lintChainExternalWithoutCompensation(opPath string, g FlowGraph) []normalizer.Warning {
	// Check whether the flow as a whole has any compensation container.
	hasCompensation := false
	for _, node := range g.Nodes {
		if l := node.Logos; l != nil {
			for _, p := range l.Provides {
				if p == capCompensationReady {
					hasCompensation = true
				}
			}
		}
	}

	var out []normalizer.Warning
	for _, node := range g.Nodes {
		l := logosFor(node.Step.Action)
		if l == nil || l.FailureMode != "compensatable" {
			continue
		}
		if hasCompensation {
			continue
		}
		// Check if this node is directly inside a flow.Try / flow.Saga parent.
		if hasControlParentOfKind(g, node.Index, "flow.Try", "flow.Saga") {
			continue
		}
		out = append(out, normalizer.Warning{
			Kind:     "chain",
			Code:     codeChainExternalWithoutCompensation,
			Severity: "warn",
			Message: fmt.Sprintf(
				"'%s' at flow[%d] makes an external call (FailureMode=compensatable) but the flow has no flow.Try / flow.Saga compensation wrapper",
				node.Step.Action, node.Index,
			),
			Op:      opPath,
			Step:    node.Index + 1,
			Action:  node.Step.Action,
			File:    node.Step.File,
			Line:    node.Step.Line,
			Column:  node.Step.Column,
			CUEPath: stepPath(opPath, node.Step, node.Index+1),
			Hint:    "Wrap the external call in {action: \"flow.Try\", do: [...], catch: [...]} or use flow.Saga to handle partial failures with compensation.",
		})
	}
	return out
}

// ── L3 lint: DB write occurs before input is validated ───────────────────────

func lintChainDBWriteBeforeValidation(opPath string, g FlowGraph) []normalizer.Warning {
	// Find the index of the first validation step.
	firstValidationIdx := -1
	for i, node := range g.Nodes {
		l := node.Logos
		if l == nil {
			continue
		}
		for _, p := range l.Provides {
			if p == capInputValidated {
				if firstValidationIdx < 0 {
					firstValidationIdx = i
				}
			}
		}
	}
	if firstValidationIdx < 0 {
		// No validation at all — let other lints handle the auth/validation absence.
		return nil
	}

	var out []normalizer.Warning
	for i, node := range g.Nodes {
		if !dbWriteActions[node.Step.Action] {
			continue
		}
		if i > firstValidationIdx {
			// Write comes after validation — OK.
			continue
		}
		// Write comes before ANY validation step.
		out = append(out, normalizer.Warning{
			Kind:     "chain",
			Code:     codeChainDBWriteBeforeValidation,
			Severity: "warn",
			Message: fmt.Sprintf(
				"'%s' at flow[%d] writes to DB before input is validated (first logic.Validate at flow[%d])",
				node.Step.Action, node.Index, firstValidationIdx,
			),
			Op:      opPath,
			Step:    node.Index + 1,
			Action:  node.Step.Action,
			File:    node.Step.File,
			Line:    node.Step.Line,
			Column:  node.Step.Column,
			CUEPath: stepPath(opPath, node.Step, node.Index+1),
			Hint:    "Move logic.Validate / logic.Check before any repo.Save / repo.Update to ensure invalid data never reaches the database.",
		})
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Proof report — per-operation proof of safety properties.
// ─────────────────────────────────────────────────────────────────────────────

// ProofResult is the verdict for one safety property.
type ProofResult string

const (
	ProofProved   ProofResult = "proved"
	ProofViolated ProofResult = "violated"
	ProofUnknown  ProofResult = "unknown"
)

// ChainViolation describes one violated edge in the flow graph.
type ChainViolation struct {
	From           string          `json:"from"` // "flow[i].action"
	To             string          `json:"to"`   // "flow[j].action"
	Code           string          `json:"code"`
	Message        string          `json:"message"`
	AutoFix        string          `json:"auto_fix,omitempty"`
	SuggestedPatch *normalizer.Fix `json:"suggested_patch,omitempty"`
}

// PropertyProof holds the proof verdict for one named safety property.
type PropertyProof struct {
	Property   string           `json:"property"`
	Result     ProofResult      `json:"result"`
	Violations []ChainViolation `json:"violations,omitempty"`
}

// OperationProof is the full proof report for one operation.
type OperationProof struct {
	Operation  string          `json:"operation"`
	Properties []PropertyProof `json:"properties"`
}

// ProofReport holds proof results for all operations.
type ProofReport struct {
	Schema     string           `json:"schema"` // "ang/proof/v1"
	Operations []OperationProof `json:"operations"`
}

// BuildProofReport generates the full proof report for all services.
func BuildProofReport(services []normalizer.Service, endpoints []normalizer.Endpoint) ProofReport {
	endpointByOp := buildEndpointIndex(endpoints)
	var ops []OperationProof
	for _, svc := range services {
		for _, method := range svc.Methods {
			opPath := svc.Name + "." + method.Name
			if len(method.Flow) == 0 {
				continue
			}
			graph := buildFlowGraph(method.Flow)
			endpoint := endpointByOp[strings.ToLower(opPath)]
			op := proveOperation(opPath, graph, endpoint)
			ops = append(ops, op)
		}
	}
	return ProofReport{Schema: "ang/proof/v1", Operations: ops}
}

// proveOperation checks a predefined set of safety properties for one operation.
func proveOperation(opPath string, g FlowGraph, endpoint normalizer.Endpoint) OperationProof {
	return OperationProof{
		Operation: opPath,
		Properties: []PropertyProof{
			proveAuthBeforeWrite(opPath, g, endpoint),
			proveValidationBeforeWrite(opPath, g),
			proveExternalCallCompensated(opPath, g),
			proveEventAtomicity(opPath, g),
		},
	}
}

// ── Property: auth.verified before every sensitive DB write ─────────────────

func proveAuthBeforeWrite(opPath string, g FlowGraph, endpoint normalizer.Endpoint) PropertyProof {
	p := PropertyProof{Property: "auth_before_write"}

	if endpointHasAuthz(endpoint) || isPublicAuthEndpoint(endpoint) {
		p.Result = ProofProved
		return p
	}

	for i, node := range g.Nodes {
		if !sensitiveWriteActions[node.Step.Action] {
			continue
		}
		caps := capabilitySetUpTo(g.Nodes, i)
		if !caps[capAuthVerified] {
			p.Result = ProofViolated
			p.Violations = append(p.Violations, ChainViolation{
				From:    "flow[?].auth.*",
				To:      fmt.Sprintf("flow[%d].%s", node.Index, node.Step.Action),
				Code:    codeChainMissingAuthGate,
				Message: "sensitive write without prior auth verification",
				AutoFix: "Add auth.RequireRole / rbac.CheckPermission before DB write, or declare endpoint jwt/roles.",
				SuggestedPatch: &normalizer.Fix{
					Op:      "insert",
					CUEPath: stepPath(opPath, node.Step, node.Index+1),
					Value: map[string]any{
						"position": "before",
						"step": map[string]any{
							"action": "auth.RequireRole",
							"role":   "ops",
						},
					},
				},
			})
		}
	}
	if p.Result == "" {
		p.Result = ProofProved
	}
	return p
}

// ── Property: input.validated before every DB write ─────────────────────────

func proveValidationBeforeWrite(opPath string, g FlowGraph) PropertyProof {
	p := PropertyProof{Property: "validation_before_write"}

	firstValidation := -1
	for i, node := range g.Nodes {
		l := node.Logos
		if l == nil {
			continue
		}
		for _, prov := range l.Provides {
			if prov == capInputValidated && firstValidation < 0 {
				firstValidation = i
			}
		}
	}

	hasAnyWrite := false
	for i, node := range g.Nodes {
		if !dbWriteActions[node.Step.Action] {
			continue
		}
		hasAnyWrite = true
		if firstValidation < 0 || i < firstValidation {
			p.Result = ProofViolated
			p.Violations = append(p.Violations, ChainViolation{
				From:    "flow[?].logic.Validate",
				To:      fmt.Sprintf("flow[%d].%s", node.Index, node.Step.Action),
				Code:    codeChainDBWriteBeforeValidation,
				Message: "DB write before input validation",
				AutoFix: "Move logic.Validate before repo.Save/Update.",
				SuggestedPatch: &normalizer.Fix{
					Op:      "insert",
					CUEPath: stepPath(opPath, node.Step, node.Index+1),
					Value: map[string]any{
						"position": "before",
						"step": map[string]any{
							"action":    "logic.Validate",
							"input":     "req",
							"validator": "default",
						},
					},
				},
			})
		}
	}
	if !hasAnyWrite {
		p.Result = ProofUnknown // no writes → property vacuously true but uninteresting
		return p
	}
	if p.Result == "" {
		p.Result = ProofProved
	}
	return p
}

// ── Property: every external call is compensated ────────────────────────────

func proveExternalCallCompensated(opPath string, g FlowGraph) PropertyProof {
	p := PropertyProof{Property: "external_call_compensated"}

	hasCompensation := false
	hasExtCall := false
	for _, node := range g.Nodes {
		l := node.Logos
		if l == nil {
			continue
		}
		for _, prov := range l.Provides {
			if prov == capCompensationReady {
				hasCompensation = true
			}
		}
		for _, eff := range l.Effects {
			if eff == effectExtCall {
				hasExtCall = true
			}
		}
	}

	if !hasExtCall {
		p.Result = ProofUnknown
		return p
	}

	for _, node := range g.Nodes {
		l := node.Logos
		if l == nil || l.FailureMode != "compensatable" {
			continue
		}
		if !hasCompensation && !hasControlParentOfKind(g, node.Index, "flow.Try", "flow.Saga") {
			p.Result = ProofViolated
			p.Violations = append(p.Violations, ChainViolation{
				From:    fmt.Sprintf("flow[%d].%s", node.Index, node.Step.Action),
				To:      "flow[?].compensation",
				Code:    codeChainExternalWithoutCompensation,
				Message: "external call without compensation wrapper",
				AutoFix: "Wrap in {action: \"flow.Try\", do: [...], catch: [...]} or use flow.Saga.",
				SuggestedPatch: &normalizer.Fix{
					Op:      "replace",
					CUEPath: stepPath(opPath, node.Step, node.Index+1),
					Value: map[string]any{
						"action": "flow.Try",
						"do": []map[string]any{
							{"action": node.Step.Action},
						},
						"catch": []map[string]any{
							{"action": "audit.Log", "message": "external call failed"},
						},
					},
				},
			})
		}
	}
	if p.Result == "" {
		p.Result = ProofProved
	}
	return p
}

// ── Property: event publish is atomic with DB write ─────────────────────────

func proveEventAtomicity(opPath string, g FlowGraph) PropertyProof {
	p := PropertyProof{Property: "event_atomicity"}

	hasDBWrite := false
	hasUnsafePublish := false
	hasOutbox := false

	for _, node := range g.Nodes {
		l := node.Logos
		if l == nil {
			continue
		}
		for _, eff := range l.Effects {
			if eff == effectDBWrite {
				hasDBWrite = true
			}
		}
		if node.Step.Action == "event.Publish" || node.Step.Action == "event.Broadcast" {
			hasUnsafePublish = true
		}
		for _, prov := range l.Provides {
			if prov == capEventScheduled {
				hasOutbox = true
			}
		}
	}

	// Property only applies when the flow both writes to DB and emits events.
	hasAnyEventEmit := hasUnsafePublish || hasOutbox
	if !hasDBWrite || !hasAnyEventEmit {
		p.Result = ProofUnknown
		return p
	}

	// DB write + outbox (with or without unsafe publish) → atomic by design.
	if hasOutbox {
		p.Result = ProofProved
		return p
	}

	// DB write + unsafe publish only → violation.
	p.Result = ProofViolated
	p.Violations = append(p.Violations, ChainViolation{
		From:    "flow[?].repo.Save",
		To:      "flow[?].event.Publish",
		Code:    codeOutboxPreferred,
		Message: "event.Publish without transactional outbox — event may be lost on crash",
		AutoFix: "Replace event.Publish with event.Outbox inside tx.Block.",
		SuggestedPatch: &normalizer.Fix{
			Op:    "replace",
			Value: map[string]any{"action": "event.Outbox"},
			After: `action: "event.Outbox"`,
		},
	})
	return p
}

// ─────────────────────────────────────────────────────────────────────────────
// Graph helpers
// ─────────────────────────────────────────────────────────────────────────────

// hasControlParentOfKind returns true if node at `nodeIdx` is directly
// nested inside a parent step whose action is one of `parentActions`.
func hasControlParentOfKind(g FlowGraph, nodeIdx int, parentActions ...string) bool {
	wanted := make(map[string]bool, len(parentActions))
	for _, a := range parentActions {
		wanted[a] = true
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeControl && e.To == nodeIdx {
			parentAction := g.Nodes[e.From].Step.Action
			if wanted[parentAction] {
				return true
			}
		}
	}
	return false
}
