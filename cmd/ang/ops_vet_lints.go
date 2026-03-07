package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/strogmv/ang/compiler/flowsem"
	"github.com/strogmv/ang/compiler/normalizer"
)

// ── Thresholds ──────────────────────────────────────────────────────────────

const flowTooLargeThreshold = 20

// ── Stable error codes ──────────────────────────────────────────────────────

const (
	codeFlowTooLarge                  = "E_FLOW_TOO_LARGE"
	codeHTTPNoTimeout                 = "W_FLOW_HTTP_NO_TIMEOUT"
	codeOutboxPreferred               = "W_FLOW_OUTBOX_PREFERRED"
	codeInvalidNestingKeys            = "E_FLOW_INVALID_NESTING_KEYS"
	codeDBWriteWithoutTxWhenRequired  = "E_DB_WRITE_WITHOUT_TX_WHEN_REQUIRED"
	codeMutatingWithoutIdempotency    = "E_MUTATING_ENDPOINT_WITHOUT_IDEMPOTENCY"
	codeProtectedOperationWithoutAuth = "E_PROTECTED_OPERATION_WITHOUT_AUTHZ"
)

// ── Entry point ──────────────────────────────────────────────────────────────

// runSemanticLints checks higher-level rules that the CUE schema and flowsem
// engine do not cover. It operates on already-normalized service definitions.
func runSemanticLints(services []normalizer.Service, endpoints []normalizer.Endpoint, profile lintProfile) []normalizer.Warning {
	var out []normalizer.Warning
	endpointByOp := buildEndpointIndex(endpoints)
	for _, svc := range services {
		for _, method := range svc.Methods {
			opPath := svc.Name + "." + method.Name
			endpoint, hasEndpoint := endpointByOp[strings.ToLower(opPath)]
			out = append(out, lintFlowTooLarge(opPath, method.Flow)...)
			out = append(out, lintInvalidNestingKeys(opPath, method.Flow)...)
			out = append(out, lintHTTPNoTimeout(opPath, method.Flow, profile)...)
			out = append(out, lintOutboxPreferred(opPath, method.Flow, profile)...)
			out = append(out, lintDBWriteWithoutTxWhenRequired(opPath, method.Flow)...)
			if hasEndpoint {
				out = append(out, lintMutatingEndpointWithoutIdempotency(opPath, endpoint, method.Flow, profile)...)
				out = append(out, lintProtectedOperationWithoutAuthz(opPath, endpoint, method.Flow, profile)...)
			}
		}
	}
	return out
}

// ── Lint: flow too large ─────────────────────────────────────────────────────

func lintFlowTooLarge(opPath string, flow []normalizer.FlowStep) []normalizer.Warning {
	n := len(flow)
	if n <= flowTooLargeThreshold {
		return nil
	}
	return []normalizer.Warning{{
		Kind:     "semantic",
		Code:     codeFlowTooLarge,
		Severity: "error",
		Message:  fmt.Sprintf("flow has %d steps (limit %d); large flows are hard to maintain and test", n, flowTooLargeThreshold),
		Op:       opPath,
		CUEPath:  opPath + ".flow",
		Hint:     "Split into sub-operations using 'flow.Call', or extract repeated patterns into a shared preset in cue/schema/flow_helpers.cue.",
	}}
}

// ── Lint: http action without timeout ────────────────────────────────────────

var httpActions = map[string]bool{
	"http.Request":  true,
	"http.Call":     true,
	"http.Paginate": true,
	"http.Get":      true,
	"http.Post":     true,
}

func lintHTTPNoTimeout(opPath string, flow []normalizer.FlowStep, profile lintProfile) []normalizer.Warning {
	var out []normalizer.Warning
	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if !httpActions[step.Action] {
			return
		}
		if _, ok := step.Args["timeout"]; ok {
			return
		}
		if inTimeout {
			return
		}
		out = append(out, normalizer.Warning{
			Kind:     "semantic",
			Code:     codeHTTPNoTimeout,
			Severity: severityForProfile(codeHTTPNoTimeout, profile, "warn"),
			Message:  fmt.Sprintf("'%s' at flow[%d] has no timeout — external HTTP calls can hang indefinitely", step.Action, i),
			Op:       opPath,
			Step:     i,
			Action:   step.Action,
			File:     step.File,
			Line:     step.Line,
			Column:   step.Column,
			CUEPath:  stepPath(opPath, step, i),
			Hint:     `Add timeout: "5s" (or appropriate value) to the step args. Example: {action: "http.Request", url: "...", timeout: "5s"}`,
		})
	})
	return out
}

// ── Lint: db write + event.Publish without outbox ────────────────────────────

var dbWriteActions = map[string]bool{
	"repo.Save":          true,
	"repo.Upsert":        true,
	"repo.SaveOrUpdate":  true,
	"repo.Update":        true,
	"repo.BulkSave":      true,
	"repo.Delete":        true,
	"repo.BulkDelete":    true,
	"repo.DeleteByQuery": true,
}

var outboxActions = map[string]bool{
	"event.Outbox": true,
	"outbox.Save":  true,
}

func lintOutboxPreferred(opPath string, flow []normalizer.FlowStep, profile lintProfile) []normalizer.Warning {
	hasDBWrite := false
	hasPublish := false
	hasOutbox := false

	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if dbWriteActions[step.Action] {
			hasDBWrite = true
		}
		if step.Action == "event.Publish" || step.Action == "event.Broadcast" {
			hasPublish = true
		}
		if outboxActions[step.Action] {
			hasOutbox = true
		}
	})

	if hasDBWrite && hasPublish && !hasOutbox {
		return []normalizer.Warning{{
			Kind:     "semantic",
			Code:     codeOutboxPreferred,
			Severity: severityForProfile(codeOutboxPreferred, profile, "warn"),
			Message:  "flow writes to DB and publishes an event without the outbox pattern — event may be lost on crash between the DB commit and the publish",
			Op:       opPath,
			CUEPath:  opPath + ".flow",
			Hint:     "Use event.Outbox inside tx.Block instead of event.Publish. This ensures the event is stored atomically with the DB write and delivered at-least-once.",
		}}
	}
	return nil
}

// ── Lint: invalid structural keys for action ────────────────────────────────

type lintProfile string

const (
	lintProfileMini lintProfile = "mini"
	lintProfileSaaS lintProfile = "saas"
	lintProfileProd lintProfile = "prod"
)

func parseLintProfile(raw string) (lintProfile, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(lintProfileMini):
		return lintProfileMini, nil
	case string(lintProfileSaaS):
		return lintProfileSaaS, nil
	case string(lintProfileProd):
		return lintProfileProd, nil
	default:
		return "", fmt.Errorf("unknown profile %q (allowed: mini|saas|prod)", raw)
	}
}

func severityForProfile(code string, profile lintProfile, base string) string {
	if base != "warn" {
		return base
	}
	switch profile {
	case lintProfileProd:
		switch code {
		case codeHTTPNoTimeout, codeOutboxPreferred, codeMutatingWithoutIdempotency, codeProtectedOperationWithoutAuth:
			return "error"
		}
	case lintProfileSaaS:
		switch code {
		case codeMutatingWithoutIdempotency, codeProtectedOperationWithoutAuth:
			return "error"
		}
	}
	return base
}

var (
	actionKeysOnce     sync.Once
	knownActions       map[string]struct{}
	allowedNestedByAct map[string]map[string]struct{}
)

func initActionKeyCatalog() {
	actionKeysOnce.Do(func() {
		knownActions = make(map[string]struct{})
		allowedNestedByAct = make(map[string]map[string]struct{})
		for _, entry := range flowsem.ActionCatalog() {
			knownActions[entry.Name] = struct{}{}
			set := make(map[string]struct{})
			for _, key := range entry.NestedKeys {
				set[key] = struct{}{}
			}
			allowedNestedByAct[entry.Name] = set
		}
	})
}

func lintInvalidNestingKeys(opPath string, flow []normalizer.FlowStep) []normalizer.Warning {
	initActionKeyCatalog()
	var out []normalizer.Warning
	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if _, ok := knownActions[step.Action]; !ok && !hasKnownActionPrefix(step.Action) {
			return
		}
		allowedSet := allowedNestedByAct[step.Action]
		allowed := make([]string, 0, len(allowedSet))
		for key := range allowedSet {
			allowed = append(allowed, key)
		}
		sort.Strings(allowed)

		for key := range step.Args {
			if !strings.HasPrefix(key, "_") {
				continue
			}
			if _, ok := allowedSet[key]; ok {
				continue
			}
			hint := "Remove unsupported nested key and keep only allowed structural blocks for this action."
			if len(allowed) > 0 {
				hint = fmt.Sprintf("Allowed structural keys for '%s': %s", step.Action, strings.Join(allowed, ", "))
			}
			out = append(out, normalizer.Warning{
				Kind:     "semantic",
				Code:     codeInvalidNestingKeys,
				Severity: "error",
				Message:  fmt.Sprintf("'%s' does not support nested key '%s'", step.Action, strings.TrimPrefix(key, "_")),
				Op:       opPath,
				Step:     i,
				Action:   step.Action,
				File:     step.File,
				Line:     step.Line,
				Column:   step.Column,
				CUEPath:  stepPath(opPath, step, i),
				Hint:     hint,
				Allowed:  allowed,
			})
		}
	})
	return out
}

var knownActionPrefixes = []string{
	"repo.", "mapping.", "logic.", "event.", "fsm.", "flow.", "tx.",
	"list.", "notification.", "notify.", "approval.", "policy.", "audit.", "auth.", "entity.", "field.",
	"rbac.",
	"str.", "enum.", "time.", "map.",
	"exec.", "fs.",
	"cache.", "mail.", "storage.",
	"webhook.", "queue.", "dlq.",
	"http.", "rand.", "json.", "regex.", "base64.", "url.", "query.", "hash.", "uuid.", "ulid.", "math.", "jsonpath.", "batch.", "parallel.",
	"jwt.", "oauth2.", "crypto.",
	"idem.", "idempotency.", "dedupe.", "ratelimit.", "concurrency.", "circuit.", "bulkhead.",
	"log.", "metric.", "trace.", "slo.",
	"pdf.",
}

func hasKnownActionPrefix(action string) bool {
	for _, prefix := range knownActionPrefixes {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

// ── Lint: db writes that must be in tx.Block ────────────────────────────────

var lockActions = map[string]bool{
	"repo.GetForUpdate": true,
	"db.Lock":           true,
}

func lintDBWriteWithoutTxWhenRequired(opPath string, flow []normalizer.FlowStep) []normalizer.Warning {
	dbWritesOutsideTx := 0
	locksOutsideTx := 0

	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if dbWriteActions[step.Action] && !inTx {
			dbWritesOutsideTx++
		}
		if lockActions[step.Action] && !inTx {
			locksOutsideTx++
		}
	})

	if dbWritesOutsideTx >= 2 || (dbWritesOutsideTx >= 1 && locksOutsideTx >= 1) {
		return []normalizer.Warning{{
			Kind:     "semantic",
			Code:     codeDBWriteWithoutTxWhenRequired,
			Severity: "error",
			Message:  "flow has multiple DB mutations (or mutation + row lock) outside tx.Block",
			Op:       opPath,
			CUEPath:  opPath + ".flow",
			Hint:     "Wrap mutating/locking steps in {action: \"tx.Block\", do: [...]} to guarantee atomicity.",
		}}
	}
	return nil
}

// ── Lint: mutating endpoint without idempotency ─────────────────────────────

var idempotencyActions = map[string]bool{
	"idempotency.DeriveKey":  true,
	"idempotency.Check":      true,
	"idempotency.SaveResult": true,
	"dedupe.Once":            true,
}

func lintMutatingEndpointWithoutIdempotency(opPath string, endpoint normalizer.Endpoint, flow []normalizer.FlowStep, profile lintProfile) []normalizer.Warning {
	if !isMutatingHTTPMethod(endpoint.Method) {
		return nil
	}
	hasDBWrite := false
	hasIdempotency := endpoint.Idempotency || strings.TrimSpace(endpoint.DedupeKey) != ""

	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if dbWriteActions[step.Action] {
			hasDBWrite = true
		}
		if idempotencyActions[step.Action] {
			hasIdempotency = true
		}
	})
	if !hasDBWrite || hasIdempotency {
		return nil
	}
	return []normalizer.Warning{{
		Kind:     "semantic",
		Code:     codeMutatingWithoutIdempotency,
		Severity: severityForProfile(codeMutatingWithoutIdempotency, profile, "warn"),
		Message:  "mutating endpoint writes to DB without idempotency/dedup guard",
		Op:       opPath,
		CUEPath:  opPath + ".flow",
		Hint:     "Add idempotency.DeriveKey + idempotency.Check + idempotency.SaveResult (or dedupe.Once) before DB writes.",
	}}
}

// ── Lint: protected operation without authz guard ───────────────────────────

var authzActions = map[string]bool{
	"auth.RequireRole":     true,
	"auth.CheckRole":       true,
	"rbac.CheckPermission": true,
	"policy.Require":       true,
}

func lintProtectedOperationWithoutAuthz(opPath string, endpoint normalizer.Endpoint, flow []normalizer.FlowStep, profile lintProfile) []normalizer.Warning {
	if !isMutatingHTTPMethod(endpoint.Method) || isPublicAuthEndpoint(endpoint) {
		return nil
	}
	hasDBWrite := false
	hasFlowAuthz := false
	walkFlow(flow, false, false, func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool) {
		if dbWriteActions[step.Action] {
			hasDBWrite = true
		}
		if authzActions[step.Action] {
			hasFlowAuthz = true
		}
	})
	if !hasDBWrite {
		return nil
	}
	if endpointHasAuthz(endpoint) || hasFlowAuthz {
		return nil
	}
	return []normalizer.Warning{{
		Kind:     "semantic",
		Code:     codeProtectedOperationWithoutAuth,
		Severity: severityForProfile(codeProtectedOperationWithoutAuth, profile, "warn"),
		Message:  "mutating external endpoint has no auth/authz gate",
		Op:       opPath,
		CUEPath:  opPath + ".flow",
		Hint:     "Add endpoint auth (jwt/roles/permission) or start flow with auth.RequireRole / rbac.CheckPermission.",
	}}
}

func endpointHasAuthz(endpoint normalizer.Endpoint) bool {
	return strings.TrimSpace(endpoint.AuthType) != "" ||
		len(endpoint.AuthRoles) > 0 ||
		strings.TrimSpace(endpoint.Permission) != "" ||
		strings.TrimSpace(endpoint.AuthCheck) != ""
}

func isPublicAuthEndpoint(endpoint normalizer.Endpoint) bool {
	rpc := strings.ToLower(strings.TrimSpace(endpoint.RPC))
	path := strings.ToLower(strings.TrimSpace(endpoint.Path))
	return rpc == "login" || rpc == "register" || strings.HasSuffix(path, "/auth/login") || strings.HasSuffix(path, "/auth/register")
}

func isMutatingHTTPMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func buildEndpointIndex(endpoints []normalizer.Endpoint) map[string]normalizer.Endpoint {
	out := make(map[string]normalizer.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		if strings.TrimSpace(ep.ServiceName) == "" || strings.TrimSpace(ep.RPC) == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(ep.ServiceName) + "." + strings.TrimSpace(ep.RPC))
		out[key] = ep
	}
	return out
}

func stepPath(opPath string, step normalizer.FlowStep, i int) string {
	if strings.TrimSpace(step.CUEPath) != "" {
		return step.CUEPath
	}
	return fmt.Sprintf("%s.flow[%d]", opPath, i-1)
}

func walkFlow(steps []normalizer.FlowStep, inTx bool, inTimeout bool, visit func(step normalizer.FlowStep, i int, inTx bool, inTimeout bool)) {
	for i, step := range steps {
		idx := i + 1
		visit(step, idx, inTx, inTimeout)

		nextTx := inTx || step.Action == "tx.Block"
		nextTimeout := inTimeout || step.Action == "flow.Timeout"
		for _, children := range childBlocks(step.Args) {
			if len(children) == 0 {
				continue
			}
			walkFlow(children, nextTx, nextTimeout, visit)
		}
	}
}

func childBlocks(args map[string]any) [][]normalizer.FlowStep {
	if len(args) == 0 {
		return nil
	}
	var out [][]normalizer.FlowStep
	for key, raw := range args {
		if !strings.HasPrefix(key, "_") {
			continue
		}
		switch v := raw.(type) {
		case []normalizer.FlowStep:
			out = append(out, v)
		case map[string][]normalizer.FlowStep:
			for _, branch := range v {
				out = append(out, branch)
			}
		}
	}
	return out
}

// ── Enrichment: unknown action → Allowed + did-you-mean ──────────────────────

// enrichUnknownActionDiags post-processes warnings from the compiler pipeline:
// for any E_FLOW_UNKNOWN_ACTION / UNKNOWN_ACTION it adds the Allowed list of
// same-namespace actions and a "Did you mean…" hint.
func enrichUnknownActionDiags(diags []normalizer.Warning) []normalizer.Warning {
	catalog := flowsem.ActionCatalog()
	allNames := make([]string, 0, len(catalog))
	for _, e := range catalog {
		allNames = append(allNames, e.Name)
	}
	sort.Strings(allNames)

	for i, d := range diags {
		if !isUnknownActionCode(d.Code) {
			continue
		}
		action := d.Action
		if action == "" {
			// Try to extract from message: "Unknown action 'foo.Bar'"
			action = extractActionFromMessage(d.Message)
		}
		if action == "" {
			continue
		}
		similar := findSimilarActions(action, allNames)
		if len(similar) == 0 {
			continue
		}
		diags[i].Allowed = similar
		if diags[i].Hint == "" {
			diags[i].Hint = fmt.Sprintf("Did you mean '%s'? Run 'ang actions --json' for the full catalog.", similar[0])
		}
	}
	return diags
}

func isUnknownActionCode(code string) bool {
	return code == "UNKNOWN_ACTION" || code == "E_FLOW_UNKNOWN_ACTION" ||
		strings.HasSuffix(code, "_UNKNOWN_ACTION")
}

func extractActionFromMessage(msg string) string {
	// "Unknown action 'http.Reqeust'" → "http.Reqeust"
	s := msg
	if idx := strings.Index(s, "'"); idx >= 0 {
		s = s[idx+1:]
		if end := strings.Index(s, "'"); end >= 0 {
			return s[:end]
		}
	}
	return ""
}

// findSimilarActions returns up to 5 catalog actions ordered by similarity.
// Priority: same namespace (http.*, repo.*, etc.) → sorted by edit distance.
func findSimilarActions(input string, catalog []string) []string {
	ns := ""
	if idx := strings.LastIndex(input, "."); idx > 0 {
		ns = input[:idx]
	}

	// Phase 1: same namespace
	var sameNS []string
	for _, a := range catalog {
		if ns != "" && strings.HasPrefix(a, ns+".") {
			sameNS = append(sameNS, a)
		}
	}
	if len(sameNS) > 0 {
		sort.Slice(sameNS, func(i, j int) bool {
			return editDistance(input, sameNS[i]) < editDistance(input, sameNS[j])
		})
		if len(sameNS) > 5 {
			sameNS = sameNS[:5]
		}
		return sameNS
	}

	// Phase 2: global closest by edit distance (threshold = len/2 + 3)
	type scored struct {
		name string
		dist int
	}
	threshold := len(input)/2 + 3
	var candidates []scored
	for _, a := range catalog {
		d := editDistance(input, a)
		if d <= threshold {
			candidates = append(candidates, scored{a, d})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})
	out := make([]string, 0, 3)
	for _, c := range candidates {
		out = append(out, c.name)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			mn := del
			if ins < mn {
				mn = ins
			}
			if sub < mn {
				mn = sub
			}
			curr[j] = mn
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// ── Path computation ─────────────────────────────────────────────────────────

// computeVetPath builds the human-readable "file:location" path from a Warning.
// Format mirrors what the user specified: "operations/create_user.cue:flow[3]"
func computeVetPath(w normalizer.Warning) string {
	if w.File != "" && w.CUEPath != "" {
		loc := trimPackagePrefix(w.CUEPath)
		return w.File + ":" + loc
	}
	if w.File != "" && w.Step > 0 {
		return fmt.Sprintf("%s:flow[%d]", w.File, w.Step-1)
	}
	if w.CUEPath != "" {
		return trimPackagePrefix(w.CUEPath)
	}
	return w.File
}

// trimPackagePrefix removes the leading CUE package qualifier.
// "api.CreateUser.flow[3]" → "CreateUser.flow[3]"
func trimPackagePrefix(cuePath string) string {
	parts := strings.SplitN(cuePath, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return cuePath
}
