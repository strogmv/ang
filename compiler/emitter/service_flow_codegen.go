package emitter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

var reFlowSelectorID = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9]*)Id\b`)
var reFlowTimeIsZero = regexp.MustCompile(`(\w+At)\s*==\s*""`)
var rePayloadTimeAt = regexp.MustCompile(`(\w+At):\s*([\w.]+At)\b`)

func normalizeFlowExpr(s string) string {
	if s == "" {
		return s
	}
	s = reFlowSelectorID.ReplaceAllString(s, ".$1ID")
	// time.Time fields can't be compared with ""; convert to .IsZero()
	s = reFlowTimeIsZero.ReplaceAllString(s, "$1.IsZero()")
	return s
}

// normalizePayloadExpr transforms event payload struct literals: XxxAt fields that
// reference time.Time domain variables are converted to .Format(time.RFC3339) so they
// satisfy the string type of the domain event struct fields.
func normalizePayloadExpr(s string) string {
	return rePayloadTimeAt.ReplaceAllString(s, "$1: $2.Format(time.RFC3339)")
}

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
		"repo.Query", "repo.Upsert",
		"mapping.Assign", "mapping.Map",
		"flow.If", "flow.For", "flow.Block", "flow.Switch", "flow.While", "flow.Call", "tx.Block",
		"flow.Checkpoint", "flow.Resume", "flow.Validate", "flow.Try", "flow.Catch",
		"flow.RecordEvent", "flow.Replay", "flow.History.Get",
		"flow.Retry", "flow.Fallback", "flow.Timeout", "flow.SuggestNext", "flow.ExplainError",
		"list.Filter", "list.Paginate", "list.Append", "list.Sort",
		"list.Map", "list.Reduce", "list.GroupBy", "list.Distinct", "list.Chunk",
		"batch.Run",
		"list.Enrich",
		"str.Normalize",
		"event.Publish", "logic.Call", "service.Call",
		"event.Wait", "event.Subscribe", "event.Match", "event.Broadcast",
		"exec.Run",
		"fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove",
		"archive.ZipDir",
		"audit.Log",
		"auth.RequireRole", "auth.CheckRole", "rbac.CheckPermission",
		"entity.PatchNonZero", "entity.PatchValidated", "field.CopyNonEmpty",
		"enum.Validate",
		"time.Now", "time.Parse", "time.CheckExpiry",
		"map.Build",
		"fsm.Transition",
		"notification.Dispatch", "notify.Dispatch", "notify.Send",
		"cache.Get", "cache.Set", "cache.Del",
		"mail.Send",
		"storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List",
		"http.Call", "http.Request", "http.RetryPolicy", "http.Paginate",
		"rand.Code", "rand.Token",
		"str.Format",
		"json.Parse", "json.Marshal",
		"regex.Match", "regex.Replace",
		"base64.Encode", "base64.Decode",
		"url.Parse", "url.Build",
		"query.Encode", "query.Decode",
		"hash.Sum", "hash.HMAC",
		"uuid.New", "ulid.New",
		"math.Op", "math.Expr",
		"jsonpath.Get", "jsonpath.Set",
		"jwt.Sign", "jwt.Verify",
		"oauth2.Token", "oauth2.Refresh",
		"crypto.Encrypt", "crypto.Decrypt", "crypto.Hash",
		"parallel.Run",
		"pdf.Render",
		"webhook.Send",
		"webhook.VerifySignature", "webhook.Ack",
		"queue.Enqueue", "queue.Dequeue", "queue.Ack", "queue.Nack",
		"dlq.Publish",
		"event.Outbox",
		"approval.Request", "approval.Wait", "approval.Decide",
		"policy.Check", "policy.Evaluate", "policy.Require", "policy.Decide",
		"session.Get",
		"flow.Parallel", "flow.Join", "flow.Race",
		"flow.Delay", "flow.Schedule", "flow.Cron",
		"flow.Saga", "flow.Compensate", "flow.Rollback", "flow.Tag",
		"state.Get", "state.Set", "state.Delete",
		"idem.DeriveKey", "idem.Check", "idem.SaveResult",
		"idempotency.DeriveKey", "idempotency.Check", "idempotency.SaveResult",
		"dedupe.Once",
		"ratelimit.Check", "ratelimit.Limit",
		"concurrency.Limit", "concurrency.Run",
		"circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker",
		"bulkhead.Acquire", "bulkhead.Run",
		"log.Emit", "metric.Emit", "trace.Span", "slo.Budget",
		"db.Get", "db.List", "db.Query",
		"db.Insert", "db.Update", "db.Upsert", "db.Delete",
		"db.Lock", "db.SelectForUpdate",
		"secret.Get", "config.Get":
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
	if v, ok := step.Args["_catch"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_fallback"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_onTimeout"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_onMissing"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_onMismatch"].([]normalizer.FlowStep); ok && len(v) > 0 {
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
	if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(branches))
		for k := range branches {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(branches[k]) > 0 {
				out = append(out, branches[k])
			}
		}
	}
	return out
}

type flowRenderState struct {
	declared map[string]bool
	// pointers tracks whether a declared variable is a pointer type (from repo.Find/Get)
	// vs a value type (from mapping.Assign declare). Used to decide whether to pass &var to repo.Save.
	pointers      map[string]bool
	types         map[string]string // varName → Go type (e.g. "*domain.User", "[]domain.Order")
	goroutineMode bool              // if true, errors set _pErr/_mu instead of returning
	stepN         *int              // shared monotonic counter; unique suffix for internal temp vars
	returnErrOnly bool              // if true, errReturn emits `return err` for inner error closures
	concurrMode   string            // "parallel" | "join" | "race" for flow.Parallel/Join/Race goroutines
	concurrVarPfx string            // e.g. "_fp_0" / "_fj_0" / "_fr_0"
	sagaCompVar   string            // variable name for compensation slice
	serviceName   string            // current service name for flow.Call self-call resolution
}

func cloneFlowState(st *flowRenderState) *flowRenderState {
	cp := &flowRenderState{
		declared:      make(map[string]bool, len(st.declared)),
		pointers:      make(map[string]bool, len(st.pointers)),
		types:         make(map[string]string, len(st.types)),
		goroutineMode: st.goroutineMode,
		stepN:         st.stepN, // share counter across all clones
		returnErrOnly: st.returnErrOnly,
		concurrMode:   st.concurrMode,
		concurrVarPfx: st.concurrVarPfx,
		sagaCompVar:   st.sagaCompVar,
		serviceName:   st.serviceName,
	}
	for k, v := range st.declared {
		cp.declared[k] = v
	}
	for k, v := range st.pointers {
		cp.pointers[k] = v
	}
	for k, v := range st.types {
		cp.types[k] = v
	}
	return cp
}

func renderFlow(steps []normalizer.FlowStep) string {
	return renderFlowForService("", steps)
}

func renderFlowForService(serviceName string, steps []normalizer.FlowStep) string {
	n := 0
	st := &flowRenderState{
		declared:    map[string]bool{"resp": true, "err": true},
		pointers:    map[string]bool{},
		types:       map[string]string{},
		stepN:       &n,
		serviceName: strings.TrimSpace(serviceName),
	}
	var b strings.Builder
	if flowHasAction(steps, "flow.Checkpoint", "flow.Resume") {
		st.declared["_flowCheckpoints"] = true
		st.pointers["_flowCheckpoints"] = false
		st.types["_flowCheckpoints"] = "map[string]any"
		b.WriteString("var _flowCheckpoints map[string]any\n")
	}
	if flowHasAction(steps, "flow.Try", "flow.Catch", "flow.Retry", "flow.Fallback", "flow.Timeout", "flow.ExplainError") {
		st.declared["_flowLastError"] = true
		st.pointers["_flowLastError"] = false
		st.types["_flowLastError"] = "error"
		b.WriteString("var _flowLastError error\n")
	}
	if flowHasAction(steps, "flow.RecordEvent", "flow.Replay", "flow.History.Get") {
		st.declared["_flowHistory"] = true
		st.pointers["_flowHistory"] = false
		st.types["_flowHistory"] = "[]map[string]any"
		st.declared["_flowReplayMode"] = true
		st.pointers["_flowReplayMode"] = false
		st.types["_flowReplayMode"] = "bool"
		b.WriteString("var _flowHistory []map[string]any\n")
		b.WriteString("var _flowReplayMode bool\n")
	}
	b.WriteString(renderFlowSteps(st, steps, 0))
	return b.String()
}

func flowHasAction(steps []normalizer.FlowStep, actions ...string) bool {
	need := make(map[string]struct{}, len(actions))
	for _, a := range actions {
		need[a] = struct{}{}
	}
	var walk func([]normalizer.FlowStep) bool
	walk = func(items []normalizer.FlowStep) bool {
		for _, s := range items {
			if _, ok := need[s.Action]; ok {
				return true
			}
			for _, child := range flowChildSteps(s) {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(steps)
}

func renderFlowSteps(st *flowRenderState, steps []normalizer.FlowStep, indent int) string {
	var b strings.Builder
	for _, step := range steps {
		b.WriteString(renderOneFlowStep(st, step, indent))
	}
	return b.String()
}

// errReturn generates the appropriate error-return code depending on context.
// In goroutine mode, errors are captured via mutex rather than returning from the outer func.
func errReturn(st *flowRenderState, pad, errExpr string) string {
	switch st.concurrMode {
	case "parallel":
		pfx := st.concurrVarPfx
		return fmt.Sprintf("%s%sMu.Lock()\nif %sErr == nil { %sErr = %s; %sCancel() }\n%s%sMu.Unlock()\n%sreturn\n",
			pad, pfx, pfx, pfx, errExpr, pfx, pad, pfx, pad)
	case "join":
		pfx := st.concurrVarPfx
		return fmt.Sprintf("%s%sMu.Lock()\n%s%sErrs = append(%sErrs, %s)\n%s%sMu.Unlock()\n%sreturn\n",
			pad, pfx, pad, pfx, pfx, errExpr, pad, pfx, pad)
	case "race":
		return fmt.Sprintf("%sreturn\n", pad)
	}
	if st.goroutineMode {
		return fmt.Sprintf("%s_mu.Lock()\nif _pErr == nil { _pErr = %s; _pCancel() }\n%s_mu.Unlock()\n%sreturn\n", pad, errExpr, pad, pad)
	}
	if st.returnErrOnly {
		return fmt.Sprintf("%sreturn %s\n", pad, errExpr)
	}
	return fmt.Sprintf("%sreturn resp, %s\n", pad, errExpr)
}

func renderOneFlowStep(st *flowRenderState, step normalizer.FlowStep, indent int) string {
	sfx := fmt.Sprintf("_%d", *st.stepN)
	*st.stepN++
	_ = sfx // consumed by actions with internal temp vars
	arg := func(name string) string {
		if v, ok := step.Args[name]; ok {
			if s, ok := v.(string); ok {
				return normalizeFlowExpr(strings.TrimSpace(s))
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

	if out, ok := renderFlowStepDomain(st, step, indent, sfx, arg, child); ok {
		return out
	}

	switch step.Action {
	case "flow.If", "flow.For", "flow.Block", "tx.Block", "list.Filter", "list.Paginate", "list.Append", "list.Sort", "list.Map", "list.Reduce", "list.GroupBy", "list.Distinct", "list.Chunk", "batch.Run", "str.Normalize", "mapping.Map", "event.Publish", "logic.Call", "service.Call", "flow.Call", "exec.Run", "fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove", "archive.ZipDir", "session.Get", "flow.Switch", "flow.While", "flow.Checkpoint", "flow.Resume", "flow.RecordEvent", "flow.Replay", "flow.History.Get", "flow.Validate", "flow.Try", "flow.Catch", "flow.Retry", "flow.Fallback", "flow.Timeout", "flow.SuggestNext", "flow.ExplainError",
		"flow.Parallel", "flow.Join", "flow.Race",
		"flow.Delay", "flow.Schedule", "flow.Cron",
		"flow.Saga", "flow.Compensate", "flow.Rollback", "flow.Tag":
		return renderFlowStepControl(st, step, indent, sfx, arg, child)

		// -------------------------------------------------------------------------
		// STAGE 2: Infrastructure actions
		// -------------------------------------------------------------------------

	case "cache.Get", "cache.Set", "cache.Del", "mail.Send", "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List", "http.Call", "http.Request", "http.RetryPolicy", "http.Paginate", "rand.Code", "rand.Token", "json.Parse", "json.Marshal", "regex.Match", "regex.Replace", "base64.Encode", "base64.Decode", "url.Parse", "url.Build", "query.Encode", "query.Decode", "hash.Sum", "hash.HMAC", "uuid.New", "ulid.New", "time.Now", "math.Op", "jsonpath.Get", "jsonpath.Set", "jwt.Sign", "jwt.Verify", "oauth2.Token", "oauth2.Refresh", "crypto.Encrypt", "crypto.Decrypt", "crypto.Hash", "parallel.Run", "pdf.Render", "webhook.Send", "webhook.VerifySignature", "webhook.Ack", "queue.Enqueue", "queue.Dequeue", "queue.Ack", "queue.Nack", "dlq.Publish", "event.Outbox", "secret.Get", "config.Get",
		"event.Wait", "event.Subscribe", "event.Match", "event.Broadcast",
		"notify.Send", "approval.Request", "approval.Wait", "approval.Decide",
		"policy.Check", "policy.Evaluate", "policy.Require", "policy.Decide",
		"state.Get", "state.Set", "state.Delete",
		"idem.DeriveKey", "idem.Check", "idem.SaveResult",
		"idempotency.DeriveKey", "idempotency.Check", "idempotency.SaveResult",
		"dedupe.Once",
		"ratelimit.Check", "ratelimit.Limit",
		"concurrency.Limit", "concurrency.Run",
		"circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker",
		"bulkhead.Acquire", "bulkhead.Run",
		"log.Emit", "metric.Emit", "trace.Span", "slo.Budget":
		return renderFlowStepInfra(st, step, indent, sfx, arg, child)

	default:
		return ""
	}
}
