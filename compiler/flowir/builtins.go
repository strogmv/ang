package flowir

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func init() {
	registerScalarActions()
	Register(ActionSpec{Name: "cache.Get", Description: "Read a string value from cache", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "optional", Kind: ArgBool}}, Decode: decodeCacheGet})
	Register(ActionSpec{Name: "cache.Set", Description: "Write a value to cache", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression, Required: true}, {Name: "ttl", Kind: ArgExpression}}, Decode: decodeCacheSet})
	Register(ActionSpec{Name: "cache.Del", Description: "Delete a cache value", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}}, Decode: decodeCacheDelete})
	Register(ActionSpec{Name: "state.Get", Description: "Read explicit application state", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "default", Kind: ArgExpression}}, Decode: decodeStateGet})
	Register(ActionSpec{Name: "state.Set", Description: "Write explicit application state", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression, Required: true}, {Name: "ttl", Kind: ArgExpression}}, Decode: decodeStateSet})
	Register(ActionSpec{Name: "state.Delete", Description: "Delete explicit application state", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}}, Decode: decodeStateDelete})
	Register(ActionSpec{Name: "storage.Upload", Description: "Upload bytes to object storage", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "data", Kind: ArgExpression, Required: true}, {Name: "contentType", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeStorageUpload})
	Register(ActionSpec{Name: "storage.Download", Description: "Download bytes from object storage", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStorageDownload})
	Register(ActionSpec{Name: "storage.Delete", Description: "Delete an object", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}}, Decode: decodeStorageDelete})
	Register(ActionSpec{Name: "storage.List", Description: "List object keys", Args: []ArgSpec{{Name: "prefix", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStorageList})
	Register(ActionSpec{Name: "storage.GetURL", Description: "Resolve an object URL", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStorageGetURL})
	Register(ActionSpec{Name: "mapping.Assign", Description: "Assign a typed expression to a field or local", Args: []ArgSpec{{Name: "to", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression, Required: true}, {Name: "declare", Kind: ArgBool}}, Decode: decodeMappingAssign})
	Register(ActionSpec{Name: "mapping.Map", Description: "Declare or map an entity/value", Args: []ArgSpec{{Name: "input", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "entity", Kind: ArgString}}, Decode: decodeMappingMap})
	Register(ActionSpec{Name: "logic.Check", Description: "Require a boolean expression", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString, Required: true}, {Name: "status", Kind: ArgString}, {Name: "params", Kind: ArgExpressions}}, Decode: decodeLogicCheck})
	Register(ActionSpec{Name: "flow.If", Description: "Conditional typed flow", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}}, Decode: decodeFlowIf})
	Register(ActionSpec{Name: "flow.Block", Description: "Lexical flow block", Args: nil, Decode: func(step normalizer.FlowStep) (Action, error) { return decodeFlowBlock(step, false) }})
	Register(ActionSpec{Name: "tx.Block", Description: "Transactional flow block", Args: nil, Decode: func(step normalizer.FlowStep) (Action, error) { return decodeFlowBlock(step, true) }})
	Register(ActionSpec{Name: "event.Publish", Description: "Publish a typed event", Args: []ArgSpec{{Name: "name", Kind: ArgString, Required: true}, {Name: "payload", Kind: ArgExpression}}, Decode: func(step normalizer.FlowStep) (Action, error) { return decodeEventPublish(step, false) }})
	Register(ActionSpec{Name: "event.Broadcast", Description: "Broadcast a typed event", Args: []ArgSpec{{Name: "name", Kind: ArgString, Required: true}, {Name: "payload", Kind: ArgExpression}}, Decode: func(step normalizer.FlowStep) (Action, error) { return decodeEventPublish(step, true) }})
	Register(ActionSpec{Name: "logic.Call", Description: "Call a Go function", Args: callArgs(true), Decode: decodeLogicCall})
	Register(ActionSpec{Name: "service.Call", Description: "Call a declared service dependency", Args: append([]ArgSpec{{Name: "service", Kind: ArgString, Required: true}, {Name: "method", Kind: ArgString, Required: true}}, callArgs(false)...), Decode: decodeServiceCall})
	for _, action := range []RepositoryOperation{RepoSave, RepoFind, RepoDelete, RepoList, RepoQuery, RepoGet, RepoGetForUpdate, RepoExists, RepoCount, RepoUpsert} {
		action := action
		Register(ActionSpec{Name: string(action), Description: "Typed repository operation", Args: []ArgSpec{{Name: "source", Kind: ArgString, Required: true}, {Name: "input", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}}, Decode: func(step normalizer.FlowStep) (Action, error) {
			return decodeRepositoryCall(action, step)
		}})
	}
}

func registerScalarActions() {
	registerListActions()
	registerMapJSONActions()
	registerTransformActions()
	registerErrorActions()
	out := []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}}
	Register(ActionSpec{Name: "uuid.New", Args: out, Decode: func(s normalizer.FlowStep) (Action, error) {
		v, e := output(s)
		return UUIDNew{v}, e
	}})
	Register(ActionSpec{Name: "ulid.New", Args: out, Decode: func(s normalizer.FlowStep) (Action, error) {
		v, e := output(s)
		return ULIDNew{v}, e
	}})
	Register(ActionSpec{Name: "rand.Code", Args: []ArgSpec{{Name: "length", Kind: ArgInt}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeRandomCode})
	Register(ActionSpec{Name: "rand.Token", Args: []ArgSpec{{Name: "bytes", Kind: ArgInt}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeRandomToken})
	Register(ActionSpec{Name: "regex.Match", Args: expressionActionArgs("input", "pattern"), Decode: decodeRegexMatch})
	Register(ActionSpec{Name: "regex.Replace", Args: expressionActionArgs("input", "pattern", "repl"), Decode: decodeRegexReplace})
	Register(ActionSpec{Name: "str.Format", Args: []ArgSpec{{Name: "template", Kind: ArgExpression, Required: true}, {Name: "args", Kind: ArgExpressions}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStringFormat})
	Register(ActionSpec{Name: "str.Concat", Args: []ArgSpec{{Name: "parts", Kind: ArgExpressions, Required: true}, {Name: "sep", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStringConcat})
	Register(ActionSpec{Name: "str.StripMarkdown", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeStringStripMarkdown})
	Register(ActionSpec{Name: "str.ReplaceAll", Args: expressionActionArgs("input", "old", "new"), Decode: decodeStringReplaceAll})
	Register(ActionSpec{Name: "str.TrimSpace", Args: expressionActionArgs("input"), Decode: decodeStringTrimSpace})
	Register(ActionSpec{Name: "str.Normalize", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "mode", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeStringNormalize})
	Register(ActionSpec{Name: "time.Now", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "format", Kind: ArgString}}, Decode: decodeTimeNow})
	Register(ActionSpec{Name: "time.Parse", Args: []ArgSpec{{Name: "value", Kind: ArgExpression, Required: true}, {Name: "format", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTimeParse})
	Register(ActionSpec{Name: "time.Format", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "format", Kind: ArgString}, {Name: "timezone", Kind: ArgString}, {Name: "zero", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTimeFormat})
	Register(ActionSpec{Name: "time.InZone", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "timezone", Kind: ArgString, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTimeInZone})
	Register(ActionSpec{Name: "time.Add", Args: expressionActionArgs("input", "duration"), Decode: decodeTimeAdd})
	Register(ActionSpec{Name: "time.Sub", Args: expressionActionArgs("a", "b"), Decode: decodeTimeSub})
	Register(ActionSpec{Name: "time.Diff", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "to", Kind: ArgExpression, Required: true}, {Name: "unit", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTimeDiff})
	Register(ActionSpec{Name: "time.CheckExpiry", Args: []ArgSpec{{Name: "value", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString, Required: true}, {Name: "mustBe", Kind: ArgString, Required: true}}, Decode: decodeTimeCheckExpiry})
}

func registerErrorActions() {
	registerSecurityActions()
	Register(ActionSpec{Name: "auth.RequireRole", Args: []ArgSpec{{Name: "userID", Kind: ArgExpression, Required: true}, {Name: "companyID", Kind: ArgExpression, Required: true}, {Name: "roles", Kind: ArgExpressions, Required: true}, {Name: "output", Kind: ArgIdentifier}, {Name: "adminBypass", Kind: ArgBool}}, Decode: decodeAuthRequireRole})
	Register(ActionSpec{Name: "auth.CheckRole", Args: []ArgSpec{{Name: "user", Kind: ArgExpression, Required: true}, {Name: "roles", Kind: ArgExpressions, Required: true}, {Name: "companyID", Kind: ArgExpression}}, Decode: decodeAuthCheckRole})
	Register(ActionSpec{Name: "jsonpath.Get", Args: expressionActionArgs("input", "path"), Decode: func(s normalizer.FlowStep) (Action, error) {
		x, e := exprs(s, "input", "path")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return JSONPathGet{x[0], x[1], o}, e
	}})
	Register(ActionSpec{Name: "jsonpath.Set", Args: expressionActionArgs("input", "path", "value"), Decode: func(s normalizer.FlowStep) (Action, error) {
		x, e := exprs(s, "input", "path", "value")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return JSONPathSet{x[0], x[1], x[2], o}, e
	}})
	Register(ActionSpec{Name: "errors.New", Args: []ArgSpec{{Name: "message", Kind: ArgExpression, Required: true}, {Name: "status", Kind: ArgExpression}, {Name: "code", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "throw", Kind: ArgBool}}, Decode: decodeErrorNew})
	Register(ActionSpec{Name: "errors.ThrowIf", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString, Required: true}, {Name: "status", Kind: ArgExpression}, {Name: "code", Kind: ArgExpression}}, Decode: decodeErrorThrowIf})
	Register(ActionSpec{Name: "errors.Wrap", Args: []ArgSpec{{Name: "err", Kind: ArgExpression, Required: true}, {Name: "message", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeErrorWrap})
	Register(ActionSpec{Name: "errors.Map", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "cases", Kind: ArgExpression, Required: true}, {Name: "mode", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier}, {Name: "defaultMessage", Kind: ArgString}, {Name: "defaultCode", Kind: ArgString}, {Name: "defaultStatus", Kind: ArgString}}, Decode: decodeErrorMap})
}

func registerSecurityActions() {
	registerFlowResilienceActions()
	Register(ActionSpec{Name: "http.Call", Args: httpArgs(false), Decode: decodeHTTPCall})
	Register(ActionSpec{Name: "flow.For", Args: []ArgSpec{{Name: "each", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier, Required: true}}, Decode: decodeFlowFor})
	Register(ActionSpec{Name: "flow.While", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}}, Decode: decodeFlowWhile})
	Register(ActionSpec{Name: "flow.Switch", Args: []ArgSpec{{Name: "value", Kind: ArgExpression, Required: true}, {Name: "match", Kind: ArgString}}, Decode: decodeFlowSwitch})
	Register(ActionSpec{Name: "http.Request", Args: httpArgs(true), Decode: decodeHTTPRequest})
	Register(ActionSpec{Name: "http.RetryPolicy", Args: append(httpArgs(true), ArgSpec{Name: "attempts", Kind: ArgInt}, ArgSpec{Name: "backoffMs", Kind: ArgInt}, ArgSpec{Name: "retryOn", Kind: ArgExpression}), Decode: decodeHTTPRetryPolicy})
	Register(ActionSpec{Name: "http.Paginate", Args: []ArgSpec{{Name: "url", Kind: ArgExpression, Required: true}, {Name: "method", Kind: ArgString}, {Name: "into", Kind: ArgString, Required: true}, {Name: "as", Kind: ArgIdentifier, Required: true}, {Name: "cursor_expr", Kind: ArgExpression, Required: true}, {Name: "items_expr", Kind: ArgExpression}, {Name: "cursor_param", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier}, {Name: "output_type", Kind: ArgString}, {Name: "max_pages", Kind: ArgInt}, {Name: "headers", Kind: ArgExpression}, {Name: "auth", Kind: ArgString}}, Decode: decodeHTTPPaginate})
	Register(ActionSpec{Name: "http.SOAP", Args: []ArgSpec{{Name: "url", Kind: ArgExpression, Required: true}, {Name: "namespace", Kind: ArgExpression, Required: true}, {Name: "operation", Kind: ArgExpression, Required: true}, {Name: "request", Kind: ArgExpression}, {Name: "headers", Kind: ArgExpression}, {Name: "soapAction", Kind: ArgExpression}, {Name: "timeout", Kind: ArgExpression}, {Name: "into", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier}, {Name: "statusVar", Kind: ArgIdentifier}, {Name: "failOnError", Kind: ArgBool}}, Decode: decodeHTTPSOAP})
	Register(ActionSpec{Name: "log.Emit", Args: []ArgSpec{{Name: "message", Kind: ArgExpression, Required: true}, {Name: "level", Kind: ArgString}, {Name: "fields", Kind: ArgExpression}}, Decode: decodeLogEmit})
	Register(ActionSpec{Name: "metric.Emit", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "kind", Kind: ArgString}, {Name: "value", Kind: ArgExpression}, {Name: "labels", Kind: ArgExpression}}, Decode: decodeMetricEmit})
	Register(ActionSpec{Name: "trace.Span", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "attrs", Kind: ArgExpression}}, Decode: decodeTraceSpan})
	Register(ActionSpec{Name: "slo.Budget", Args: []ArgSpec{{Name: "name", Kind: ArgString}, {Name: "duration", Kind: ArgExpression, Required: true}}, Decode: decodeSLOBudget})
	Register(ActionSpec{Name: "context.Trim", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "max_bytes", Kind: ArgInt}, {Name: "strategy", Kind: ArgExpression}}, Decode: decodeContextTrim})
	Register(ActionSpec{Name: "profile.Require", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "tier", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: decodeProfileRequire})
	Register(ActionSpec{Name: "concurrency.Limit", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "max", Kind: ArgInt, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeConcurrencyLimit(s) }})
	Register(ActionSpec{Name: "concurrency.Run", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "max", Kind: ArgInt, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: decodeConcurrencyRun})
	Register(ActionSpec{Name: "mutex.With", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString}, {Name: "wait", Kind: ArgExpression}, {Name: "poll", Kind: ArgExpression}}, Decode: decodeMutexWith})
	for _, name := range []string{"circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "threshold", Kind: ArgInt}, {Name: "openTTL", Kind: ArgExpression}, {Name: "throw", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeCircuit(s, n) }})
	}
	for _, name := range []string{"bulkhead.Acquire", "bulkhead.Run"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "max", Kind: ArgInt, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeBulkhead(s, n) }})
	}
	for _, name := range []string{"idem.DeriveKey", "idempotency.DeriveKey"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "from", Kind: ArgExpressions, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "prefix", Kind: ArgExpression}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeIdemDerive(s, n) }})
	}
	for _, name := range []string{"idem.Check", "idempotency.Check"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
			k, e := requiredExpression(s, "key")
			return IdempotencyCheck{n, k}, e
		}})
	}
	for _, name := range []string{"idem.SaveResult", "idempotency.SaveResult"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "ttl", Kind: ArgExpression}}, Decode: func(s normalizer.FlowStep) (Action, error) {
			k, e := requiredExpression(s, "key")
			ttl := optionalExpression(s, "ttl")
			if ttl.Source == "" {
				ttl.Source = "24 * time.Hour"
			}
			return IdempotencySaveResult{n, k, ttl}, e
		}})
	}
	Register(ActionSpec{Name: "dedupe.Once", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "ttl", Kind: ArgExpression}}, Decode: decodeDedupeOnce})
	for _, name := range []string{"ratelimit.Check", "ratelimit.Limit"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "rps", Kind: ArgInt, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeRateLimit(s, n) }})
	}
	Register(ActionSpec{Name: "quota.Check", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "limit", Kind: ArgInt, Required: true}, {Name: "window", Kind: ArgString, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: decodeQuotaCheck})
	Register(ActionSpec{Name: "budget.Check", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "limit", Kind: ArgInt, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: decodeBudgetCheck})
	Register(ActionSpec{Name: "budget.Consume", Args: []ArgSpec{{Name: "key", Kind: ArgExpression, Required: true}, {Name: "tokens", Kind: ArgExpression, Required: true}, {Name: "ttl", Kind: ArgExpression}}, Decode: decodeBudgetConsume})
	Register(ActionSpec{Name: "policy.Check", Args: []ArgSpec{{Name: "policy", Kind: ArgString, Required: true}, {Name: "user", Kind: ArgExpression, Required: true}, {Name: "companyID", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "throw", Kind: ArgExpression}, {Name: "code", Kind: ArgExpression}, {Name: "status", Kind: ArgExpression}}, Decode: decodePolicyCheck})
	for _, name := range []string{"policy.Evaluate", "policy.Require", "policy.Decide"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "policyKey", Kind: ArgExpression, Required: true}, {Name: "subject", Kind: ArgExpression}, {Name: "resource", Kind: ArgExpression}, {Name: "operation", Kind: ArgExpression}, {Name: "tenant", Kind: ArgExpression}, {Name: "attrs", Kind: ArgExpression}, {Name: "context", Kind: ArgExpression}, {Name: "throw", Kind: ArgExpression}, {Name: "code", Kind: ArgExpression}, {Name: "status", Kind: ArgExpression}, {Name: "decision", Kind: ArgIdentifier}, {Name: "reason", Kind: ArgIdentifier}, {Name: "effects", Kind: ArgIdentifier}, {Name: "output", Kind: ArgIdentifier, Required: n == "policy.Decide"}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodePolicyDecision(s, n) }})
	}
	Register(ActionSpec{Name: "approval.Request", Args: []ArgSpec{{Name: "approvalKey", Kind: ArgExpression, Required: true}, {Name: "title", Kind: ArgExpression, Required: true}, {Name: "description", Kind: ArgExpression}, {Name: "requestedBy", Kind: ArgExpression, Required: true}, {Name: "approvers", Kind: ArgExpressions, Required: true}, {Name: "policy", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression, Required: true}, {Name: "deadline", Kind: ArgExpression}, {Name: "ttl", Kind: ArgExpression}, {Name: "approvalId", Kind: ArgIdentifier}, {Name: "status", Kind: ArgIdentifier}}, Decode: decodeApprovalRequest})
	Register(ActionSpec{Name: "approval.Wait", Args: []ArgSpec{{Name: "approvalId", Kind: ArgExpression, Required: true}, {Name: "timeout", Kind: ArgExpression}, {Name: "onTimeout", Kind: ArgExpression}, {Name: "decision", Kind: ArgIdentifier}, {Name: "status", Kind: ArgIdentifier}, {Name: "decidedBy", Kind: ArgIdentifier}, {Name: "decidedAt", Kind: ArgIdentifier}, {Name: "reason", Kind: ArgIdentifier}}, Decode: decodeApprovalWait})
	Register(ActionSpec{Name: "approval.Decide", Args: []ArgSpec{{Name: "approvalId", Kind: ArgExpression, Required: true}, {Name: "decision", Kind: ArgExpression, Required: true}, {Name: "actor", Kind: ArgExpression, Required: true}, {Name: "reason", Kind: ArgExpression}, {Name: "status", Kind: ArgIdentifier}}, Decode: decodeApprovalDecide})
	Register(ActionSpec{Name: "mail.Send", Args: []ArgSpec{{Name: "to", Kind: ArgExpression, Required: true}, {Name: "subject", Kind: ArgExpression, Required: true}, {Name: "body", Kind: ArgExpression, Required: true}, {Name: "html", Kind: ArgExpression}}, Decode: decodeMailSend})
	Register(ActionSpec{Name: "notify.Send", Args: notifyArgs(true), Decode: decodeNotifySend})
	Register(ActionSpec{Name: "notify.Email", Args: notifyArgs(false), Decode: decodeNotifyEmail})
	for _, name := range []string{"notification.Dispatch", "notify.Dispatch"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "event", Kind: ArgExpression}, {Name: "message", Kind: ArgExpression}, {Name: "type", Kind: ArgExpression}, {Name: "userID", Kind: ArgExpression}, {Name: "entityID", Kind: ArgExpression}, {Name: "payload", Kind: ArgExpression}, {Name: "template", Kind: ArgExpression}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeNotificationDispatch(s, n) }})
	}
	Register(ActionSpec{Name: "queue.Enqueue", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression, Required: true}, {Name: "timeout", Kind: ArgExpression}, {Name: "timeoutMs", Kind: ArgInt}}, Decode: decodeQueueEnqueue})
	Register(ActionSpec{Name: "queue.Dequeue", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "ackToken", Kind: ArgIdentifier}, {Name: "attempts", Kind: ArgInt}, {Name: "retries", Kind: ArgInt}, {Name: "backoffMs", Kind: ArgInt}, {Name: "jitterMs", Kind: ArgInt}, {Name: "timeout", Kind: ArgExpression}, {Name: "timeoutMs", Kind: ArgInt}}, Decode: decodeQueueDequeue})
	Register(ActionSpec{Name: "queue.Ack", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "messageID", Kind: ArgExpression, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
		x, e := exprs(s, "subject", "messageID")
		return QueueAck{x[0], x[1]}, e
	}})
	Register(ActionSpec{Name: "queue.Nack", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "messageID", Kind: ArgExpression, Required: true}, {Name: "reason", Kind: ArgExpression}}, Decode: decodeQueueNack})
	Register(ActionSpec{Name: "dlq.Publish", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression, Required: true}, {Name: "reason", Kind: ArgExpression}}, Decode: decodeDLQPublish})
	Register(ActionSpec{Name: "webhook.Send", Args: []ArgSpec{{Name: "url", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression, Required: true}, {Name: "event", Kind: ArgExpression}, {Name: "retries", Kind: ArgInt}}, Decode: decodeWebhookSend})
	Register(ActionSpec{Name: "webhook.VerifySignature", Args: []ArgSpec{{Name: "payload", Kind: ArgExpression, Required: true}, {Name: "signature", Kind: ArgExpression, Required: true}, {Name: "secret", Kind: ArgExpression}, {Name: "algorithm", Kind: ArgExpression}, {Name: "strict", Kind: ArgBool}, {Name: "output", Kind: ArgIdentifier}, {Name: "throw", Kind: ArgExpression}}, Decode: decodeWebhookVerify})
	Register(ActionSpec{Name: "webhook.Ack", Args: []ArgSpec{{Name: "status", Kind: ArgInt}, {Name: "body", Kind: ArgExpression}}, Decode: decodeWebhookAck})
	oauthArgs := []ArgSpec{{Name: "tokenURL", Kind: ArgExpression, Required: true}, {Name: "clientID", Kind: ArgExpression}, {Name: "clientSecret", Kind: ArgExpression}, {Name: "scope", Kind: ArgExpression}, {Name: "audience", Kind: ArgExpression}, {Name: "grantType", Kind: ArgExpression}, {Name: "username", Kind: ArgExpression}, {Name: "password", Kind: ArgExpression}, {Name: "code", Kind: ArgExpression}, {Name: "redirectURI", Kind: ArgExpression}, {Name: "refreshToken", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}
	Register(ActionSpec{Name: "oauth2.Token", Args: oauthArgs, Decode: func(s normalizer.FlowStep) (Action, error) {
		f, e := decodeOAuth2Fields(s, false)
		return OAuth2Token{f}, e
	}})
	Register(ActionSpec{Name: "oauth2.Refresh", Args: oauthArgs, Decode: func(s normalizer.FlowStep) (Action, error) {
		f, e := decodeOAuth2Fields(s, true)
		return OAuth2Refresh{f}, e
	}})
	Register(ActionSpec{Name: "jwt.Sign", Args: []ArgSpec{{Name: "claims", Kind: ArgExpression, Required: true}, {Name: "secret", Kind: ArgExpression}, {Name: "alg", Kind: ArgExpression}, {Name: "ttl", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeJWTSign})
	Register(ActionSpec{Name: "jwt.Verify", Args: []ArgSpec{{Name: "token", Kind: ArgExpression, Required: true}, {Name: "secret", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeJWTVerify})
	Register(ActionSpec{Name: "token.Generate", Args: []ArgSpec{{Name: "subject", Kind: ArgExpression, Required: true}, {Name: "purpose", Kind: ArgExpression}, {Name: "claims", Kind: ArgExpression}, {Name: "secret", Kind: ArgExpression}, {Name: "ttl", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTokenGenerate})
	Register(ActionSpec{Name: "token.Verify", Args: []ArgSpec{{Name: "token", Kind: ArgExpression, Required: true}, {Name: "purpose", Kind: ArgExpression}, {Name: "secret", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTokenVerify})
	Register(ActionSpec{Name: "crypto.Hash", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "algo", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeCryptoHash})
	for _, name := range []string{"crypto.Encrypt", "crypto.Decrypt"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "key", Kind: ArgExpression}, {Name: "aad", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
			i, e := requiredExpression(s, "input")
			if e != nil {
				return nil, e
			}
			o, e := output(s)
			return CryptoCipher{n == "crypto.Decrypt", i, optionalExpression(s, "key"), optionalExpression(s, "aad"), o}, e
		}})
	}
}

func registerFlowResilienceActions() {
	Register(ActionSpec{Name: "flow.Try", Args: []ArgSpec{{Name: "retries", Kind: ArgInt}, {Name: "backoffMs", Kind: ArgInt}}, Decode: decodeFlowTry})
	Register(ActionSpec{Name: "flow.Retry", Args: []ArgSpec{{Name: "attempts", Kind: ArgInt}, {Name: "retries", Kind: ArgInt}, {Name: "backoffMs", Kind: ArgInt}}, Decode: decodeFlowRetry})
	Register(ActionSpec{Name: "flow.Timeout", Args: []ArgSpec{{Name: "duration", Kind: ArgExpression, Required: true}}, Decode: decodeFlowTimeout})
	Register(ActionSpec{Name: "flow.Fallback", Decode: decodeFlowFallback})
	Register(ActionSpec{Name: "flow.Parallel", Decode: func(s normalizer.FlowStep) (Action, error) { _, e := requiredBranches(s); return FlowParallel{}, e }})
	Register(ActionSpec{Name: "flow.Join", Decode: func(s normalizer.FlowStep) (Action, error) { _, e := requiredBranches(s); return FlowJoin{}, e }})
	Register(ActionSpec{Name: "flow.Race", Decode: func(s normalizer.FlowStep) (Action, error) { _, e := requiredBranches(s); return FlowRace{}, e }})
	Register(ActionSpec{Name: "parallel.Run", Args: []ArgSpec{{Name: "maxConcurrency", Kind: ArgInt}, {Name: "maxParallel", Kind: ArgInt}}, Decode: decodeParallelRun})
	Register(ActionSpec{Name: "flow.Delay", Args: []ArgSpec{{Name: "duration", Kind: ArgExpression, Required: true}}, Decode: decodeFlowDelay})
	Register(ActionSpec{Name: "flow.Schedule", Args: []ArgSpec{{Name: "at", Kind: ArgExpression, Required: true}}, Decode: decodeFlowSchedule})
	Register(ActionSpec{Name: "flow.Cron", Args: []ArgSpec{{Name: "window", Kind: ArgString, Required: true}, {Name: "timezone", Kind: ArgString}}, Decode: decodeFlowCron})
	Register(ActionSpec{Name: "flow.Saga", Decode: func(s normalizer.FlowStep) (Action, error) {
		_, e := nestedSteps(s, "_do", true)
		return FlowSaga{}, e
	}})
	Register(ActionSpec{Name: "flow.Compensate", Decode: func(s normalizer.FlowStep) (Action, error) {
		_, e := nestedSteps(s, "_do", true)
		return FlowCompensate{}, e
	}})
	Register(ActionSpec{Name: "flow.Rollback", Args: []ArgSpec{{Name: "error", Kind: ArgExpression}}, Decode: func(s normalizer.FlowStep) (Action, error) { return FlowRollback{optionalExpression(s, "error")}, nil }})
	Register(ActionSpec{Name: "flow.Checkpoint", Args: []ArgSpec{{Name: "name", Kind: ArgString, Required: true}, {Name: "data", Kind: ArgExpression}}, Decode: decodeFlowCheckpoint})
	Register(ActionSpec{Name: "flow.Resume", Args: []ArgSpec{{Name: "name", Kind: ArgString, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "into", Kind: ArgString}}, Decode: decodeFlowResume})
	Register(ActionSpec{Name: "flow.RecordEvent", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeFlowRecordEvent})
	Register(ActionSpec{Name: "flow.History.Get", Args: []ArgSpec{{Name: "name", Kind: ArgExpression}, {Name: "limit", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeFlowHistoryGet})
	Register(ActionSpec{Name: "flow.Replay", Args: []ArgSpec{{Name: "history", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeFlowReplay})
	Register(ActionSpec{Name: "flow.Validate", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString}, {Name: "message", Kind: ArgString}, {Name: "hint", Kind: ArgString}, {Name: "code", Kind: ArgString}, {Name: "status", Kind: ArgExpression}}, Decode: decodeFlowValidate})
	Register(ActionSpec{Name: "flow.Catch", Decode: func(s normalizer.FlowStep) (Action, error) {
		_, e := nestedSteps(s, "_do", true)
		return FlowCatch{}, e
	}})
	Register(ActionSpec{Name: "flow.Defer", Decode: func(s normalizer.FlowStep) (Action, error) {
		_, e := nestedSteps(s, "_do", true)
		return FlowDefer{}, e
	}})
	Register(ActionSpec{Name: "flow.SuggestNext", Args: []ArgSpec{{Name: "options", Kind: ArgExpressions, Required: true}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeFlowSuggestNext})
	Register(ActionSpec{Name: "flow.ExplainError", Args: []ArgSpec{{Name: "error", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "message", Kind: ArgString}, {Name: "hint", Kind: ArgString}}, Decode: decodeFlowExplainError})
	Register(ActionSpec{Name: "flow.Tag", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression}}, Decode: decodeFlowTag})
	Register(ActionSpec{Name: "flow.Return", Args: []ArgSpec{{Name: "set", Kind: ArgExpression}, {Name: "value", Kind: ArgExpression}}, Decode: decodeFlowReturn})
	Register(ActionSpec{Name: "flow.Call", Args: []ArgSpec{{Name: "op", Kind: ArgString, Required: true}, {Name: "args", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "ignoreErr", Kind: ArgBool}, {Name: "ignoreErrReason", Kind: ArgString}}, Decode: decodeFlowCall})
}

func decodeFlowCheckpoint(s normalizer.FlowStep) (Action, error) {
	n, err := requiredString(s, "name")
	if err != nil {
		return nil, err
	}
	d := optionalExpression(s, "data")
	if d.Source == "" {
		d = Expression{Source: `map[string]any{"resp": resp}`, Type: TypeRef{Kind: TypeMap}}
	}
	return FlowCheckpoint{Name: n, Data: d}, nil
}
func decodeFlowResume(s normalizer.FlowStep) (Action, error) {
	n, err := requiredString(s, "name")
	if err != nil {
		return nil, err
	}
	o, err := requiredIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	i, _ := optionalString(s, "into")
	_, _ = nestedSteps(s, "_onMissing", false)
	return FlowResume{Name: n, Output: o, Into: i}, nil
}
func decodeFlowRecordEvent(s normalizer.FlowStep) (Action, error) {
	n, err := requiredExpression(s, "name")
	if err != nil {
		return nil, err
	}
	o, err := optionalIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	return FlowRecordEvent{Name: n, Payload: optionalExpression(s, "payload"), Output: o}, nil
}
func decodeFlowHistoryGet(s normalizer.FlowStep) (Action, error) {
	o, err := requiredIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	return FlowHistoryGet{Name: optionalExpression(s, "name"), Limit: optionalAnyExpression(s, "limit"), Output: o}, nil
}
func decodeFlowReplay(s normalizer.FlowStep) (Action, error) {
	h, err := requiredExpression(s, "history")
	if err != nil {
		return nil, err
	}
	o, err := optionalIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	_, _ = nestedSteps(s, "_do", false)
	_, _ = nestedSteps(s, "_onMismatch", false)
	return FlowReplay{History: h, Output: o}, nil
}

func decodeFlowValidate(s normalizer.FlowStep) (Action, error) {
	c, err := requiredExpression(s, "condition")
	if err != nil {
		return nil, err
	}
	m, _ := optionalString(s, "message")
	if m == "" {
		m, _ = optionalString(s, "throw")
	}
	if m == "" {
		m = "validation failed"
	}
	h, _ := optionalString(s, "hint")
	code, _ := optionalString(s, "code")
	if code == "" {
		code = "VALIDATION_FAILED"
	}
	status := optionalExpression(s, "status")
	if status.Source == "" {
		status.Source = "http.StatusBadRequest"
	}
	return FlowValidate{Condition: c, Message: m, Hint: h, Code: code, Status: status}, nil
}

func decodeFlowSuggestNext(s normalizer.FlowStep) (Action, error) {
	var opts []string
	switch v := s.Args["options"].(type) {
	case []string:
		opts = append(opts, v...)
	case string:
		if strings.TrimSpace(v) != "" {
			opts = []string{v}
		}
	default:
		return nil, fmt.Errorf("options must be a string list")
	}
	if len(opts) == 0 {
		return nil, fmt.Errorf("options is required")
	}
	o, err := optionalIdentifier(s, "output")
	return FlowSuggestNext{Options: opts, Output: o}, err
}

func decodeFlowExplainError(s normalizer.FlowStep) (Action, error) {
	o, err := requiredIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	m, _ := optionalString(s, "message")
	h, _ := optionalString(s, "hint")
	e := optionalExpression(s, "error")
	if e.Source == "" {
		e.Source = "_flowLastError"
	}
	return FlowExplainError{Error: e, Output: o, Message: m, Hint: h}, nil
}

func decodeFlowTag(s normalizer.FlowStep) (Action, error) {
	n, err := requiredExpression(s, "name")
	return FlowTag{Name: n, Value: optionalExpression(s, "value")}, err
}

func decodeFlowReturn(s normalizer.FlowStep) (Action, error) {
	return FlowReturn{Set: optionalExpression(s, "set"), Value: optionalExpression(s, "value")}, nil
}

func decodeFlowCall(s normalizer.FlowStep) (Action, error) {
	op, err := requiredString(s, "op")
	if err != nil {
		return nil, err
	}
	o, err := optionalIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	ignore, err := optionalBool(s, "ignoreErr")
	if err != nil {
		return nil, err
	}
	reason, _ := optionalString(s, "ignoreErrReason")
	args := map[string]Expression{}
	if raw, ok := s.Args["args"]; ok {
		args, err = expressionMap(raw)
		if err != nil {
			return nil, err
		}
	}
	return FlowCall{Operation: op, Output: o, Arguments: args, IgnoreError: ignore, IgnoreErrReason: reason}, nil
}

func decodeFlowDelay(s normalizer.FlowStep) (Action, error) {
	v, err := requiredExpression(s, "duration")
	if err != nil {
		return nil, err
	}
	v.Type = TypeRef{Kind: TypeDuration}
	return FlowDelay{Duration: v}, nil
}

func decodeFlowSchedule(s normalizer.FlowStep) (Action, error) {
	v, err := requiredExpression(s, "at")
	if err != nil {
		return nil, err
	}
	v.Type = TypeRef{Kind: TypeTime}
	return FlowSchedule{At: v}, nil
}

func decodeFlowCron(s normalizer.FlowStep) (Action, error) {
	w, err := requiredString(s, "window")
	if err != nil {
		return nil, err
	}
	tz, _ := optionalString(s, "timezone")
	if tz == "" {
		tz = "UTC"
	}
	_, _ = nestedSteps(s, "_onMismatch", false)
	return FlowCron{Window: w, Timezone: tz}, nil
}

func decodeParallelRun(s normalizer.FlowStep) (Action, error) {
	_, err := requiredBranches(s)
	if err != nil {
		return nil, err
	}
	max, err := optionalInt(s, "maxConcurrency", 0)
	if err != nil {
		return nil, err
	}
	if max <= 0 {
		max, err = optionalInt(s, "maxParallel", 0)
		if err != nil {
			return nil, err
		}
	}
	if max <= 0 {
		max = 8
	}
	return ParallelRun{MaxConcurrency: max}, nil
}

func nestedSteps(s normalizer.FlowStep, key string, required bool) ([]normalizer.FlowStep, error) {
	v, ok := s.Args[key].([]normalizer.FlowStep)
	if required && (!ok || len(v) == 0) {
		return nil, fmt.Errorf("%s is required", strings.TrimPrefix(key, "_"))
	}
	return v, nil
}

func requiredBranches(s normalizer.FlowStep) (map[string][]normalizer.FlowStep, error) {
	v, ok := s.Args["_branches"].(map[string][]normalizer.FlowStep)
	if !ok || len(v) == 0 {
		return nil, fmt.Errorf("branches is required")
	}
	return v, nil
}

func decodeFlowTry(s normalizer.FlowStep) (Action, error) {
	_, e := nestedSteps(s, "_do", true)
	if e != nil {
		return nil, e
	}
	_, _ = nestedSteps(s, "_catch", false)
	r, e := optionalInt(s, "retries", 0)
	if e != nil {
		return nil, e
	}
	b, e := optionalInt(s, "backoffMs", 0)
	if e != nil {
		return nil, e
	}
	return FlowTry{Retries: r, BackoffMS: b}, nil
}

func decodeFlowRetry(s normalizer.FlowStep) (Action, error) {
	_, e := nestedSteps(s, "_do", true)
	if e != nil {
		return nil, e
	}
	_, _ = nestedSteps(s, "_catch", false)
	a, e := optionalInt(s, "attempts", -1)
	if e != nil {
		return nil, e
	}
	if a < 0 {
		r, err := optionalInt(s, "retries", -1)
		if err != nil {
			return nil, err
		}
		if r >= 0 {
			a = r + 1
		} else {
			a = 3
		}
	}
	if a <= 0 {
		a = 1
	}
	b, e := optionalInt(s, "backoffMs", 0)
	if e != nil {
		return nil, e
	}
	return FlowRetry{Attempts: a, BackoffMS: b}, nil
}

func decodeFlowTimeout(s normalizer.FlowStep) (Action, error) {
	d, e := requiredExpression(s, "duration")
	if e != nil {
		return nil, e
	}
	_, e = nestedSteps(s, "_do", true)
	if e != nil {
		return nil, e
	}
	_, _ = nestedSteps(s, "_onTimeout", false)
	return FlowTimeout{Duration: d}, nil
}

func decodeFlowFallback(s normalizer.FlowStep) (Action, error) {
	_, e := nestedSteps(s, "_do", true)
	if e != nil {
		return nil, e
	}
	_, e = nestedSteps(s, "_fallback", true)
	if e != nil {
		return nil, e
	}
	return FlowFallback{}, nil
}
func decodeFlowFor(s normalizer.FlowStep) (Action, error) {
	e, err := requiredExpression(s, "each")
	if err != nil {
		return nil, err
	}
	a, err := requiredIdentifier(s, "as")
	_, ok := s.Args["_do"].([]normalizer.FlowStep)
	if !ok {
		return nil, fmt.Errorf("do block is required")
	}
	return FlowFor{Each: e, As: a}, err
}
func decodeFlowWhile(s normalizer.FlowStep) (Action, error) {
	c, e := requiredExpression(s, "condition")
	if e != nil {
		return nil, e
	}
	_, ok := s.Args["_do"].([]normalizer.FlowStep)
	if !ok {
		return nil, fmt.Errorf("do block is required")
	}
	return FlowWhile{Condition: c}, nil
}
func decodeFlowSwitch(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "value")
	if e != nil {
		return nil, e
	}
	m, _ := optionalString(s, "match")
	if m == "" {
		m = "exact"
	}
	switch m {
	case "exact", "prefix", "suffix", "contains", "glob":
	default:
		return nil, fmt.Errorf("unsupported match mode %q", m)
	}
	cases, ok := s.Args["_cases"].(map[string][]normalizer.FlowStep)
	if !ok || len(cases) == 0 {
		return nil, fmt.Errorf("cases are required")
	}
	return FlowSwitch{Value: v, Match: m}, nil
}
func intList(raw any) ([]int, error) {
	if raw == nil {
		return nil, nil
	}
	out := []int{}
	switch v := raw.(type) {
	case []int:
		return append(out, v...), nil
	case []int64:
		for _, n := range v {
			out = append(out, int(n))
		}
	case []float64:
		for _, n := range v {
			out = append(out, int(n))
		}
	case []any:
		for _, x := range v {
			switch n := x.(type) {
			case int:
				out = append(out, n)
			case int64:
				out = append(out, int(n))
			case float64:
				out = append(out, int(n))
			default:
				return nil, fmt.Errorf("integer list contains %T", x)
			}
		}
	default:
		return nil, fmt.Errorf("expected integer list, got %T", raw)
	}
	return out, nil
}
func decodeHTTPRetryPolicy(s normalizer.FlowStep) (Action, error) {
	r, e := decodeHTTPRequest(s)
	if e != nil {
		return nil, e
	}
	a, _ := optionalInt(s, "attempts", 3)
	if a < 1 {
		a = 1
	}
	b, _ := optionalInt(s, "backoffMs", 500)
	retry, e := intList(s.Args["retryOn"])
	if len(retry) == 0 {
		retry = []int{429, 503}
	}
	return HTTPRetryPolicy{r.(HTTPRequest), a, b, retry}, e
}
func decodeHTTPPaginate(s normalizer.FlowStep) (Action, error) {
	u, e := requiredExpression(s, "url")
	if e != nil {
		return nil, e
	}
	into, e := requiredString(s, "into")
	if e != nil {
		return nil, e
	}
	as, e := requiredString(s, "as")
	if e != nil {
		return nil, e
	}
	cursor, e := requiredExpression(s, "cursor_expr")
	if e != nil {
		return nil, e
	}
	method, _ := optionalString(s, "method")
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)
	if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
		return nil, fmt.Errorf("unsupported HTTP method %q", method)
	}
	param, _ := optionalString(s, "cursor_param")
	if param == "" {
		param = "cursor"
	}
	o, _ := optionalString(s, "output")
	ot, _ := optionalString(s, "output_type")
	if ot == "" {
		ot = "[]any"
	}
	max, _ := optionalInt(s, "max_pages", 100)
	if max < 1 {
		max = 1
	}
	h, e := expressionMap(s.Args["headers"])
	auth, _ := optionalString(s, "auth")
	return HTTPPaginate{u, method, into, as, cursor, optionalExpression(s, "items_expr"), param, o, ot, max, h, auth}, e
}
func decodeHTTPSOAP(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "url", "namespace", "operation")
	if e != nil {
		return nil, e
	}
	req, e := expressionMap(s.Args["request"])
	if e != nil {
		return nil, e
	}
	h, e := expressionMap(s.Args["headers"])
	if e != nil {
		return nil, e
	}
	soap := optionalExpression(s, "soapAction")
	if soap.Source == "" {
		soap = x[2]
	}
	timeout := optionalExpression(s, "timeout")
	if timeout.Source == "" {
		timeout.Source = "10*time.Second"
	}
	into, _ := optionalString(s, "into")
	o, _ := optionalString(s, "output")
	status, _ := optionalString(s, "statusVar")
	fail := true
	if _, ok := s.Args["failOnError"]; ok {
		fail, e = optionalBool(s, "failOnError")
	}
	return HTTPSOAP{x[0], x[1], x[2], soap, timeout, req, h, into, o, status, fail}, e
}
func httpArgs(advanced bool) []ArgSpec {
	a := []ArgSpec{{Name: "method", Kind: ArgString, Required: true}, {Name: "url", Kind: ArgExpression, Required: true}, {Name: "body", Kind: ArgExpression}, {Name: "headers", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "statusVar", Kind: ArgIdentifier}, {Name: "failOnError", Kind: ArgBool}, {Name: "timeout", Kind: ArgExpression}, {Name: "attempts", Kind: ArgInt}, {Name: "backoffMs", Kind: ArgInt}}
	if advanced {
		a = append(a, ArgSpec{Name: "query", Kind: ArgExpression}, ArgSpec{Name: "auth", Kind: ArgString}, ArgSpec{Name: "into", Kind: ArgString})
	}
	return a
}
func decodeHTTPCall(s normalizer.FlowStep) (Action, error) {
	m, e := requiredString(s, "method")
	if e != nil {
		return nil, e
	}
	u, e := requiredExpression(s, "url")
	if e != nil {
		return nil, e
	}
	h, e := expressionMap(s.Args["headers"])
	if e != nil {
		return nil, e
	}
	o, _ := optionalString(s, "output")
	status, _ := optionalString(s, "statusVar")
	fail := true
	if _, ok := s.Args["failOnError"]; ok {
		fail, e = optionalBool(s, "failOnError")
	}
	timeout := optionalExpression(s, "timeout")
	if timeout.Source == "" {
		timeout.Source = "5*time.Second"
	}
	attempts, attemptsErr := optionalInt(s, "attempts", 2)
	if e == nil {
		e = attemptsErr
	}
	backoffMS, backoffErr := optionalInt(s, "backoffMs", 150)
	if e == nil {
		e = backoffErr
	}
	m = strings.ToUpper(m)
	if m != "GET" && m != "POST" && m != "PUT" && m != "DELETE" && m != "PATCH" {
		return nil, fmt.Errorf("unsupported HTTP method %q", m)
	}
	return HTTPCall{Method: m, URL: u, Body: optionalExpression(s, "body"), Timeout: timeout, Headers: h, Output: o, StatusVar: status, FailOnError: fail, Attempts: attempts, BackoffMS: backoffMS}, e
}
func decodeHTTPRequest(s normalizer.FlowStep) (Action, error) {
	base, e := decodeHTTPCall(s)
	if e != nil {
		return nil, e
	}
	b := base.(HTTPCall)
	q, e := expressionMap(s.Args["query"])
	if e != nil {
		return nil, e
	}
	auth, _ := optionalString(s, "auth")
	timeout := optionalExpression(s, "timeout")
	if timeout.Source == "" {
		timeout.Source = "10*time.Second"
	}
	into, _ := optionalString(s, "into")
	return HTTPRequest{b.Method, b.URL, b.Body, timeout, b.Headers, q, auth, into, b.Output, b.StatusVar, b.FailOnError}, nil
}
func decodeLogEmit(s normalizer.FlowStep) (Action, error) {
	m, e := requiredExpression(s, "message")
	if e != nil {
		return nil, e
	}
	l, _ := optionalString(s, "level")
	if l == "" {
		l = "info"
	}
	f, e := expressionMap(s.Args["fields"])
	return LogEmit{m, l, f}, e
}
func decodeMetricEmit(s normalizer.FlowStep) (Action, error) {
	n, e := requiredExpression(s, "name")
	if e != nil {
		return nil, e
	}
	k, _ := optionalString(s, "kind")
	if k == "" {
		k = "counter"
	}
	v := optionalExpression(s, "value")
	if v.Source == "" {
		v.Source = "1"
		v.Type = TypeRef{Kind: TypeInt}
	}
	l, e := expressionMap(s.Args["labels"])
	return MetricEmit{n, k, v, l}, e
}
func decodeTraceSpan(s normalizer.FlowStep) (Action, error) {
	n, e := requiredExpression(s, "name")
	if e != nil {
		return nil, e
	}
	a, e := expressionMap(s.Args["attrs"])
	return TraceSpan{Name: n, Attributes: a}, e
}
func decodeSLOBudget(s normalizer.FlowStep) (Action, error) {
	d, e := requiredExpression(s, "duration")
	if e != nil {
		return nil, e
	}
	n, _ := optionalString(s, "name")
	return SLOBudget{Name: n, Duration: d}, nil
}
func decodeContextTrim(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	m, _ := optionalInt(s, "max_bytes", 8000)
	if m <= 0 {
		m = 8000
	}
	strategy := optionalExpression(s, "strategy")
	if strategy.Source == "" {
		strategy.Source = `"lines"`
	}
	return ContextTrim{i, o, m, strategy}, e
}
func decodeProfileRequire(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "key", "tier")
	if e != nil {
		return nil, e
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "Upgrade required"
	}
	return ProfileRequire{x[0], x[1], t}, nil
}
func decodeConcurrencyLimit(s normalizer.FlowStep) (ConcurrencyLimit, error) {
	k, e := requiredExpression(s, "key")
	if e != nil {
		return ConcurrencyLimit{}, e
	}
	m, e := optionalInt(s, "max", 0)
	if e != nil || m <= 0 {
		return ConcurrencyLimit{}, fmt.Errorf("max must be positive")
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "concurrency limit exceeded"
	}
	return ConcurrencyLimit{k, m, t}, nil
}
func decodeConcurrencyRun(s normalizer.FlowStep) (Action, error) {
	l, e := decodeConcurrencyLimit(s)
	return ConcurrencyRun{ConcurrencyLimit: l}, e
}
func decodeMutexWith(s normalizer.FlowStep) (Action, error) {
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	w := optionalExpression(s, "wait")
	if w.Source == "" {
		w.Source = "0"
	}
	p := optionalExpression(s, "poll")
	if p.Source == "" {
		p.Source = "50 * time.Millisecond"
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "mutex busy"
	}
	return MutexWith{Key: k, Wait: w, Poll: p, Throw: t}, nil
}
func decodeCircuit(s normalizer.FlowStep, alias string) (Action, error) {
	n, e := requiredExpression(s, "name")
	if e != nil {
		return nil, e
	}
	threshold, _ := optionalInt(s, "threshold", 5)
	ttl := optionalExpression(s, "openTTL")
	if ttl.Source == "" {
		ttl.Source = "60 * time.Second"
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "circuit breaker open: " + strings.Trim(n.Source, "\"")
	}
	return CircuitAction{Alias: alias, Name: n, OpenTTL: ttl, Threshold: threshold, Throw: t}, nil
}
func decodeBulkhead(s normalizer.FlowStep, alias string) (Action, error) {
	n, e := requiredExpression(s, "name")
	if e != nil {
		return nil, e
	}
	m, e := optionalInt(s, "max", 0)
	if e != nil || m <= 0 {
		return nil, fmt.Errorf("max must be positive")
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "bulkhead full: " + strings.Trim(n.Source, "\"")
	}
	return BulkheadAction{Alias: alias, Name: n, Max: m, Throw: t}, nil
}
func decodeIdemDerive(s normalizer.FlowStep, alias string) (Action, error) {
	from, e := expressionList(s.Args["from"])
	if e != nil || len(from) == 0 {
		return nil, fmt.Errorf("from is required")
	}
	o, e := output(s)
	p := optionalExpression(s, "prefix")
	if p.Source == "" {
		p.Source = `"idem:"`
	}
	return IdempotencyDeriveKey{alias, from, o, p}, e
}
func decodeDedupeOnce(s normalizer.FlowStep) (Action, error) {
	k, e := requiredExpression(s, "key")
	ttl := optionalExpression(s, "ttl")
	if ttl.Source == "" {
		ttl.Source = "24 * time.Hour"
	}
	return DedupeOnce{Key: k, TTL: ttl}, e
}
func decodeRateLimit(s normalizer.FlowStep, alias string) (Action, error) {
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	r, e := optionalInt(s, "rps", 0)
	if e != nil || r <= 0 {
		return nil, fmt.Errorf("rps must be positive")
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "rate limit exceeded"
	}
	return RateLimit{alias, k, r, t}, nil
}
func decodeQuotaCheck(s normalizer.FlowStep) (Action, error) {
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	l, e := optionalInt(s, "limit", 0)
	if e != nil || l <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	w, _ := optionalString(s, "window")
	if w == "" {
		w = "day"
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "quota exceeded"
	}
	return QuotaCheck{k, l, w, t}, nil
}
func decodeBudgetCheck(s normalizer.FlowStep) (Action, error) {
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	l, e := optionalInt(s, "limit", 0)
	if e != nil || l <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	t, _ := optionalString(s, "throw")
	if t == "" {
		t = "Budget exhausted"
	}
	return BudgetCheck{k, l, t}, nil
}
func decodeBudgetConsume(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "key", "tokens")
	if e != nil {
		return nil, e
	}
	ttl := optionalExpression(s, "ttl")
	if ttl.Source == "" {
		ttl.Source = "0"
	}
	return BudgetConsume{x[0], x[1], ttl}, nil
}
func decodePolicyCheck(s normalizer.FlowStep) (Action, error) {
	p, e := requiredString(s, "policy")
	if e != nil {
		return nil, e
	}
	u, e := requiredExpression(s, "user")
	if e != nil {
		return nil, e
	}
	status := optionalExpression(s, "status")
	if status.Source == "" {
		status.Source = "http.StatusForbidden"
	}
	code := optionalExpression(s, "code")
	if code.Source == "" {
		code.Source = `"FORBIDDEN"`
	}
	resolved, _ := optionalBool(s, "_policyResolved")
	roles := []string{}
	switch v := s.Args["_policyRoles"].(type) {
	case []string:
		roles = append(roles, v...)
	case []any:
		for _, x := range v {
			if r := strings.TrimSpace(fmt.Sprint(x)); r != "" {
				roles = append(roles, r)
			}
		}
	}
	same, _ := optionalBool(s, "_policySameCompany")
	admin := true
	if _, ok := s.Args["_policyAllowAdminOverride"]; ok {
		admin, _ = optionalBool(s, "_policyAllowAdminOverride")
	}
	o, _ := optionalString(s, "output")
	return PolicyCheck{p, u, optionalExpression(s, "companyID"), status, code, optionalExpression(s, "throw"), o, resolved, roles, same, admin}, nil
}
func decodePolicyDecision(s normalizer.FlowStep, alias string) (Action, error) {
	k, e := requiredExpression(s, "policyKey")
	if e != nil {
		return nil, e
	}
	status := optionalExpression(s, "status")
	if status.Source == "" {
		status.Source = "http.StatusForbidden"
	}
	code := optionalExpression(s, "code")
	if code.Source == "" {
		code.Source = `"POLICY_DENIED"`
	}
	d, _ := optionalString(s, "decision")
	r, _ := optionalString(s, "reason")
	effects, _ := optionalString(s, "effects")
	o, _ := optionalString(s, "output")
	return PolicyDecisionAction{alias, k, optionalExpression(s, "subject"), optionalExpression(s, "resource"), optionalExpression(s, "operation"), optionalExpression(s, "tenant"), optionalExpression(s, "attrs"), optionalExpression(s, "context"), status, code, optionalExpression(s, "throw"), d, r, effects, o}, nil
}
func decodeApprovalRequest(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "approvalKey", "title", "requestedBy", "policy", "payload")
	if e != nil {
		return nil, e
	}
	a, e := expressionList(s.Args["approvers"])
	if e != nil || len(a) == 0 {
		return nil, fmt.Errorf("approvers is required")
	}
	_, scalar := s.Args["approvers"].(string)
	if !scalar {
		for i := range a {
			if _, err := strconv.Unquote(a[i].Source); err != nil {
				a[i].Source = strconv.Quote(a[i].Source)
			}
			a[i].Type = TypeRef{Kind: TypeString}
		}
	}
	id, _ := optionalString(s, "approvalId")
	status, _ := optionalString(s, "status")
	return ApprovalRequest{x[0], x[1], optionalExpression(s, "description"), x[2], x[3], x[4], optionalExpression(s, "deadline"), optionalExpression(s, "ttl"), a, !scalar, id, status}, nil
}
func decodeApprovalWait(s normalizer.FlowStep) (Action, error) {
	id, e := requiredExpression(s, "approvalId")
	if e != nil {
		return nil, e
	}
	timeout := optionalExpression(s, "timeout")
	if timeout.Source == "" {
		timeout.Source = "15 * time.Minute"
	}
	steps, _ := s.Args["_onTimeout"].([]normalizer.FlowStep)
	mode := optionalExpression(s, "onTimeout")
	if mode.Source == "" {
		if len(steps) > 0 {
			mode.Source = `"fallback"`
		} else {
			mode.Source = `"reject"`
		}
	}
	d, _ := optionalString(s, "decision")
	st, _ := optionalString(s, "status")
	by, _ := optionalString(s, "decidedBy")
	at, _ := optionalString(s, "decidedAt")
	r, _ := optionalString(s, "reason")
	return ApprovalWait{ApprovalID: id, Timeout: timeout, TimeoutMode: mode, Decision: d, Status: st, DecidedBy: by, DecidedAt: at, Reason: r}, nil
}
func decodeApprovalDecide(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "approvalId", "decision", "actor")
	if e != nil {
		return nil, e
	}
	st, _ := optionalString(s, "status")
	return ApprovalDecide{x[0], x[1], x[2], optionalExpression(s, "reason"), st}, nil
}
func notifyArgs(channel bool) []ArgSpec {
	a := []ArgSpec{}
	if channel {
		a = append(a, ArgSpec{Name: "channel", Kind: ArgExpression, Required: true})
	}
	return append(a, ArgSpec{Name: "to", Kind: ArgExpression, Required: true}, ArgSpec{Name: "template", Kind: ArgExpression}, ArgSpec{Name: "text", Kind: ArgExpression}, ArgSpec{Name: "subject", Kind: ArgExpression}, ArgSpec{Name: "html", Kind: ArgExpression}, ArgSpec{Name: "data", Kind: ArgExpression}, ArgSpec{Name: "locale", Kind: ArgExpression}, ArgSpec{Name: "output", Kind: ArgIdentifier})
}
func decodeMailSend(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "to", "subject", "body")
	if e != nil {
		return nil, e
	}
	return MailSend{x[0], x[1], x[2], optionalExpression(s, "html")}, nil
}
func requireNotifyContent(s normalizer.FlowStep) (Expression, Expression, error) {
	t := optionalExpression(s, "template")
	x := optionalExpression(s, "text")
	if t.Source == "" && x.Source == "" {
		return t, x, fmt.Errorf("template or text is required")
	}
	return t, x, nil
}
func decodeNotifySend(s normalizer.FlowStep) (Action, error) {
	c, e := requiredExpression(s, "channel")
	if e != nil {
		return nil, e
	}
	to, e := requiredExpression(s, "to")
	if e != nil {
		return nil, e
	}
	t, x, e := requireNotifyContent(s)
	o, _ := optionalString(s, "output")
	return NotifySend{c, to, t, x, optionalExpression(s, "subject"), optionalExpression(s, "html"), optionalExpression(s, "data"), o}, e
}
func decodeNotifyEmail(s normalizer.FlowStep) (Action, error) {
	to, e := requiredExpression(s, "to")
	if e != nil {
		return nil, e
	}
	t, x, e := requireNotifyContent(s)
	o, _ := optionalString(s, "output")
	return NotifyEmail{to, t, x, optionalExpression(s, "subject"), optionalExpression(s, "html"), optionalExpression(s, "data"), optionalExpression(s, "locale"), o}, e
}
func decodeNotificationDispatch(s normalizer.FlowStep, alias string) (Action, error) {
	e := optionalExpression(s, "event")
	if e.Source == "" {
		e = optionalExpression(s, "message")
	}
	if e.Source == "" {
		return nil, fmt.Errorf("event or message is required")
	}
	return NotificationDispatch{alias, e, optionalExpression(s, "type"), optionalExpression(s, "userID"), optionalExpression(s, "entityID"), optionalExpression(s, "payload"), optionalExpression(s, "template")}, nil
}
func queueTimeout(s normalizer.FlowStep) Expression {
	t := optionalExpression(s, "timeout")
	if t.Source != "" {
		return t
	}
	ms, _ := optionalInt(s, "timeoutMs", 0)
	if ms > 0 {
		t.Source = fmt.Sprintf("time.Duration(%d) * time.Millisecond", ms)
	} else {
		t.Source = "3*time.Second"
	}
	t.Type = TypeRef{Kind: TypeDuration}
	return t
}
func decodeQueueEnqueue(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "subject", "payload")
	return QueueEnqueue{x[0], x[1], queueTimeout(s)}, e
}
func decodeQueueDequeue(s normalizer.FlowStep) (Action, error) {
	sub, e := requiredExpression(s, "subject")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	ack, _ := optionalString(s, "ackToken")
	attempts, _ := optionalInt(s, "attempts", 0)
	if attempts <= 0 {
		if _, ok := s.Args["retries"]; ok {
			r, _ := optionalInt(s, "retries", 0)
			attempts = r + 1
		} else {
			attempts = 2
		}
	}
	b, _ := optionalInt(s, "backoffMs", 150)
	if b < 0 {
		b = 0
	}
	j, _ := optionalInt(s, "jitterMs", 50)
	if j < 0 {
		j = 0
	}
	return QueueDequeue{sub, queueTimeout(s), o, ack, attempts, b, j}, e
}
func decodeQueueNack(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "subject", "messageID")
	if e != nil {
		return nil, e
	}
	r := optionalExpression(s, "reason")
	if r.Source == "" {
		r.Source = `"nack"`
	}
	return QueueNack{x[0], x[1], r}, nil
}
func decodeDLQPublish(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "subject", "payload")
	if e != nil {
		return nil, e
	}
	r := optionalExpression(s, "reason")
	if r.Source == "" {
		r.Source = `"unspecified"`
	}
	return DLQPublish{x[0], x[1], r}, nil
}
func decodeWebhookSend(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "url", "payload")
	if e != nil {
		return nil, e
	}
	r, e := optionalInt(s, "retries", 3)
	return WebhookSend{x[0], x[1], optionalExpression(s, "event"), r}, e
}
func decodeWebhookVerify(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "payload", "signature")
	if e != nil {
		return nil, e
	}
	a := optionalExpression(s, "algorithm")
	if a.Source == "" {
		a.Source = `"sha256"`
	}
	secret := optionalExpression(s, "secret")
	if secret.Source == "" {
		secret.Source = `os.Getenv("WEBHOOK_SECRET")`
	}
	throw := optionalExpression(s, "throw")
	if throw.Source == "" {
		throw.Source = `"invalid webhook signature"`
	}
	strict := true
	if _, ok := s.Args["strict"]; ok {
		strict, e = optionalBool(s, "strict")
	}
	o, _ := optionalString(s, "output")
	return WebhookVerifySignature{x[0], x[1], secret, a, throw, strict, o}, e
}
func decodeWebhookAck(s normalizer.FlowStep) (Action, error) {
	n, e := optionalInt(s, "status", 200)
	b := optionalExpression(s, "body")
	if b.Source == "" {
		b.Source = `"ok"`
	}
	return WebhookAck{n, b}, e
}
func decodeOAuth2Fields(s normalizer.FlowStep, refresh bool) (OAuth2Fields, error) {
	u, e := requiredExpression(s, "tokenURL")
	if e != nil {
		return OAuth2Fields{}, e
	}
	r := optionalExpression(s, "refreshToken")
	if refresh && r.Source == "" {
		return OAuth2Fields{}, fmt.Errorf("refreshToken is required")
	}
	o, e := output(s)
	return OAuth2Fields{u, optionalExpression(s, "clientID"), optionalExpression(s, "clientSecret"), optionalExpression(s, "scope"), optionalExpression(s, "audience"), optionalExpression(s, "grantType"), optionalExpression(s, "username"), optionalExpression(s, "password"), optionalExpression(s, "code"), optionalExpression(s, "redirectURI"), r, o}, e
}
func decodeJWTSign(s normalizer.FlowStep) (Action, error) {
	c, e := requiredExpression(s, "claims")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	a := optionalExpression(s, "alg")
	if a.Source == "" {
		a.Source = `"HS256"`
	}
	return JWTSign{c, optionalExpression(s, "secret"), a, optionalExpression(s, "ttl"), o}, e
}
func decodeJWTVerify(s normalizer.FlowStep) (Action, error) {
	t, e := requiredExpression(s, "token")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return JWTVerify{t, optionalExpression(s, "secret"), o}, e
}
func decodeTokenGenerate(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "subject")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	ttl := optionalExpression(s, "ttl")
	if ttl.Source == "" {
		ttl.Source = `"15m"`
	}
	return TokenGenerate{v, optionalExpression(s, "purpose"), optionalExpression(s, "claims"), optionalExpression(s, "secret"), ttl, o}, e
}
func decodeTokenVerify(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "token")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return TokenVerify{v, optionalExpression(s, "purpose"), optionalExpression(s, "secret"), o}, e
}
func decodeCryptoHash(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "algo")
	o, e := output(s)
	return CryptoHash{i, a, o}, e
}

func decodeAuthRequireRole(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "userID", "companyID")
	if e != nil {
		return nil, e
	}
	r, e := requiredExpression(s, "roles")
	if e != nil {
		return nil, e
	}
	o, _ := optionalString(s, "output")
	if o == "" {
		o = "currentUser"
	}
	a := true
	if _, ok := s.Args["adminBypass"]; ok {
		a, e = optionalBool(s, "adminBypass")
	}
	return AuthRequireRole{x[0], x[1], r, o, a}, e
}
func decodeAuthCheckRole(s normalizer.FlowStep) (Action, error) {
	u, e := requiredExpression(s, "user")
	if e != nil {
		return nil, e
	}
	r, e := requiredExpression(s, "roles")
	return AuthCheckRole{u, r, optionalExpression(s, "companyID")}, e
}

func optionalAnyExpression(s normalizer.FlowStep, k string) Expression {
	if _, ok := s.Args[k]; !ok {
		return Expression{}
	}
	v, e := expressionFromAny(s, k)
	if e != nil {
		if text, ok := optionalString(s, k); ok {
			return Expression{Source: text, Type: TypeRef{Kind: TypeUnknown}}
		}
		return Expression{}
	}
	return v
}
func decodeErrorNew(s normalizer.FlowStep) (Action, error) {
	m, e := requiredExpression(s, "message")
	if e != nil {
		return nil, e
	}
	o, _ := optionalString(s, "output")
	t, e := optionalBool(s, "throw")
	return ErrorNew{m, optionalAnyExpression(s, "status"), optionalExpression(s, "code"), o, t}, e
}
func decodeErrorThrowIf(s normalizer.FlowStep) (Action, error) {
	c, e := requiredExpression(s, "condition")
	if e != nil {
		return nil, e
	}
	t, e := requiredString(s, "throw")
	return ErrorThrowIf{c, t, optionalAnyExpression(s, "status"), optionalExpression(s, "code")}, e
}
func decodeErrorWrap(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "err", "message")
	if e != nil {
		return nil, e
	}
	o, _ := optionalString(s, "output")
	return ErrorWrap{x[0], x[1], o}, nil
}
func decodeErrorMap(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	cases, e := errorMapCases(s.Args["cases"])
	if e != nil {
		return nil, e
	}
	m, _ := optionalString(s, "mode")
	if m == "" {
		m = "contains"
	}
	o, _ := optionalString(s, "output")
	dm, _ := optionalString(s, "defaultMessage")
	dc, _ := optionalString(s, "defaultCode")
	ds, _ := optionalString(s, "defaultStatus")
	return ErrorMap{i, cases, m, o, dm, dc, ds}, nil
}
func errorMapCases(raw any) (map[string]ErrorMapCase, error) {
	out := map[string]ErrorMapCase{}
	outer, ok := raw.(map[string]any)
	if !ok {
		if typed, yes := raw.(map[string]map[string]string); yes {
			for k, v := range typed {
				out[k] = ErrorMapCase{v["status"], v["code"], v["message"]}
			}
			return out, nil
		}
		return nil, fmt.Errorf("cases must be an object")
	}
	for key, value := range outer {
		c := ErrorMapCase{}
		switch cfg := value.(type) {
		case map[string]string:
			c = ErrorMapCase{cfg["status"], cfg["code"], cfg["message"]}
		case map[string]any:
			c = ErrorMapCase{fmt.Sprint(cfg["status"]), fmt.Sprint(cfg["code"]), fmt.Sprint(cfg["message"])}
		default:
			return nil, fmt.Errorf("case %s must be an object", key)
		}
		out[key] = c
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cases is required")
	}
	return out, nil
}

func registerTransformActions() {
	Register(ActionSpec{Name: "base64.Encode", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return Base64Encode{i, o}, e
	}})
	Register(ActionSpec{Name: "base64.Decode", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return Base64Decode{i, o}, e
	}})
	Register(ActionSpec{Name: "url.Parse", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return URLParse{i, o}, e
	}})
	Register(ActionSpec{Name: "path.Base", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return PathBase{i, o}, e
	}})
	Register(ActionSpec{Name: "url.Build", Args: []ArgSpec{{Name: "base", Kind: ArgExpression, Required: true}, {Name: "path", Kind: ArgExpression}, {Name: "segments", Kind: ArgExpressions}, {Name: "query", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeURLBuild})
	Register(ActionSpec{Name: "query.Encode", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return QueryEncode{i, o}, e
	}})
	Register(ActionSpec{Name: "query.Decode", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return QueryDecode{i, o}, e
	}})
	Register(ActionSpec{Name: "hash.Sum", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "algorithm", Kind: ArgExpression}, {Name: "algo", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeHashSum})
	Register(ActionSpec{Name: "hash.HMAC", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "key", Kind: ArgExpression, Required: true}, {Name: "algorithm", Kind: ArgExpression}, {Name: "algo", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeHashHMAC})
	for _, name := range []string{"num.Add", "num.Sub", "num.Mul", "num.Div"} {
		n := name
		Register(ActionSpec{Name: n, Args: expressionActionArgs("a", "b"), Decode: func(s normalizer.FlowStep) (Action, error) {
			x, e := exprs(s, "a", "b")
			if e != nil {
				return nil, e
			}
			o, e := output(s)
			return NumberBinary{n, x[0], x[1], o}, e
		}})
	}
	Register(ActionSpec{Name: "math.Expr", Args: []ArgSpec{{Name: "expr", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgExpression, Required: true}, {Name: "declare", Kind: ArgBool}}, Decode: decodeMathExpression})
	Register(ActionSpec{Name: "math.Op", Args: []ArgSpec{{Name: "op", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgExpression, Required: true}, {Name: "a", Kind: ArgExpression}, {Name: "b", Kind: ArgExpression}, {Name: "value", Kind: ArgExpression}, {Name: "min", Kind: ArgExpression}, {Name: "max", Kind: ArgExpression}, {Name: "precision", Kind: ArgInt}}, Decode: decodeMathOperation})
}

func decodeURLBuild(s normalizer.FlowStep) (Action, error) {
	b, e := requiredExpression(s, "base")
	if e != nil {
		return nil, e
	}
	segs, e := expressionList(s.Args["segments"])
	if e != nil {
		return nil, e
	}
	q, e := expressionMap(s.Args["query"])
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return URLBuild{b, optionalExpression(s, "path"), segs, q, o}, e
}
func hashAlgorithm(s normalizer.FlowStep) Expression {
	a := optionalExpression(s, "algorithm")
	if a.Source == "" {
		a = optionalExpression(s, "algo")
	}
	if a.Source == "" {
		a.Source = `"sha256"`
	}
	return a
}
func decodeHashSum(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return HashSum{i, hashAlgorithm(s), o}, e
}
func decodeHashHMAC(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "key")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return HashHMAC{x[0], x[1], hashAlgorithm(s), o}, e
}
func decodeMathExpression(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "expr")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	if e != nil {
		return nil, e
	}
	d, e := optionalBoolish(s, "declare")
	return MathExpression{v, o, d}, e
}
func decodeMathOperation(s normalizer.FlowStep) (Action, error) {
	op, e := requiredExpression(s, "op")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	if e != nil {
		return nil, e
	}
	p, e := optionalInt(s, "precision", 0)
	return MathOperation{op, optionalExpression(s, "a"), optionalExpression(s, "b"), optionalExpression(s, "value"), optionalExpression(s, "min"), optionalExpression(s, "max"), p, o}, e
}

func registerMapJSONActions() {
	Register(ActionSpec{Name: "value.Coalesce", Args: []ArgSpec{{Name: "values", Kind: ArgExpressions, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "mode", Kind: ArgString}, {Name: "into", Kind: ArgString}}, Decode: decodeValueCoalesce})
	Register(ActionSpec{Name: "map.Build", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier}, {Name: "key", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "valueType", Kind: ArgString}}, Decode: decodeMapBuild})
	Register(ActionSpec{Name: "map.New", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "type", Kind: ArgString, Required: true}}, Decode: decodeMapNew})
	Register(ActionSpec{Name: "map.Get", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "key", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "into", Kind: ArgString}, {Name: "default", Kind: ArgExpression}, {Name: "found", Kind: ArgIdentifier}}, Decode: decodeMapGet})
	Register(ActionSpec{Name: "map.Has", Args: expressionActionArgs("input", "key"), Decode: decodeMapHas})
	Register(ActionSpec{Name: "map.Set", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "key", Kind: ArgExpression, Required: true}, {Name: "value", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}}, Decode: decodeMapSet})
	Register(ActionSpec{Name: "map.Merge", Args: expressionActionArgs("left", "right"), Decode: decodeMapMerge})
	Register(ActionSpec{Name: "cast.ToString", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "format", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeCastToString})
	Register(ActionSpec{Name: "convert.ToFloat", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return ConvertToFloat{i, o}, e
	}})
	Register(ActionSpec{Name: "convert.ToInt", Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
		i, e := requiredExpression(s, "input")
		if e != nil {
			return nil, e
		}
		o, e := output(s)
		return ConvertToInt{i, o}, e
	}})
	Register(ActionSpec{Name: "json.Parse", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "into", Kind: ArgString, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeJSONParse})
	for _, name := range []string{"json.Marshal", "json.Stringify"} {
		n := name
		Register(ActionSpec{Name: n, Args: expressionActionArgs("input"), Decode: func(s normalizer.FlowStep) (Action, error) {
			i, e := requiredExpression(s, "input")
			if e != nil {
				return nil, e
			}
			o, e := output(s)
			return JSONMarshal{i, o, n == "json.Stringify"}, e
		}})
	}
}

func decodeValueCoalesce(s normalizer.FlowStep) (Action, error) {
	v, e := expressionList(s.Args["values"])
	if e != nil {
		return nil, e
	}
	if len(v) == 0 {
		return nil, fmt.Errorf("values is required")
	}
	o, e := output(s)
	m, _ := optionalString(s, "mode")
	if m == "" {
		m = "non_empty"
	}
	i, _ := optionalString(s, "into")
	return ValueCoalesce{v, o, m, i}, e
}
func decodeMapBuild(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	v, e := requiredExpression(s, "value")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "as")
	if a == "" {
		a = "_item"
	}
	o, e := output(s)
	t, _ := optionalString(s, "valueType")
	if t == "" {
		t = "string"
	}
	return MapBuild{f, a, k, v, o, t}, e
}
func decodeMapNew(s normalizer.FlowStep) (Action, error) {
	o, e := output(s)
	if e != nil {
		return nil, e
	}
	t, e := requiredString(s, "type")
	return MapNew{o, t}, e
}
func decodeMapGet(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "key")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	i, _ := optionalString(s, "into")
	f, _ := optionalString(s, "found")
	return MapGet{x[0], x[1], optionalExpression(s, "default"), o, i, f}, e
}
func decodeMapHas(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "key")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return MapHas{x[0], x[1], o}, e
}
func decodeMapSet(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "key", "value")
	if e != nil {
		return nil, e
	}
	o, _ := optionalString(s, "output")
	return MapSet{x[0], x[1], x[2], o}, nil
}
func decodeMapMerge(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "left", "right")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return MapMerge{x[0], x[1], o}, e
}
func decodeCastToString(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	f, _ := optionalString(s, "format")
	o, e := output(s)
	return CastToString{i, f, o}, e
}
func decodeJSONParse(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	t, e := requiredString(s, "into")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return JSONParse{i, t, o}, e
}

func registerListActions() {
	Register(ActionSpec{Name: "list.Append", Args: []ArgSpec{{Name: "to", Kind: ArgExpression, Required: true}, {Name: "item", Kind: ArgExpression, Required: true}}, Decode: decodeListAppend})
	Register(ActionSpec{Name: "list.Len", Args: expressionActionArgs("input"), Decode: decodeListLen})
	Register(ActionSpec{Name: "list.New", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "type", Kind: ArgString, Required: true}, {Name: "cap", Kind: ArgExpression}}, Decode: decodeListNew})
	Register(ActionSpec{Name: "list.Filter", Args: listPredicateArgs(false), Decode: decodeListFilter})
	Register(ActionSpec{Name: "list.Paginate", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "offset", Kind: ArgExpression, Required: true}, {Name: "limit", Kind: ArgExpression, Required: true}, {Name: "defaultLimit", Kind: ArgInt}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "total", Kind: ArgExpression}}, Decode: decodeListPaginate})
	Register(ActionSpec{Name: "list.Find", Args: append(listPredicateArgs(false), ArgSpec{Name: "into", Kind: ArgString}, ArgSpec{Name: "found", Kind: ArgExpression}), Decode: decodeListFind})
	Register(ActionSpec{Name: "list.Any", Args: listPredicateArgs(false), Decode: func(s normalizer.FlowStep) (Action, error) { return decodeListBoolean(s, false) }})
	Register(ActionSpec{Name: "list.All", Args: listPredicateArgs(false), Decode: func(s normalizer.FlowStep) (Action, error) { return decodeListBoolean(s, true) }})
	Register(ActionSpec{Name: "list.Map", Args: listTransformArgs("expr"), Decode: decodeListMap})
	Register(ActionSpec{Name: "list.Reduce", Args: append(listTransformArgs("expr"), ArgSpec{Name: "initial", Kind: ArgExpression}), Decode: decodeListReduce})
	Register(ActionSpec{Name: "list.GroupBy", Args: listTransformArgs("key"), Decode: decodeListGroupBy})
	Register(ActionSpec{Name: "list.Distinct", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier}, {Name: "key", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeListDistinct})
	Register(ActionSpec{Name: "list.Chunk", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "size", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeListChunk})
	Register(ActionSpec{Name: "batch.Run", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "size", Kind: ArgExpression}, {Name: "as", Kind: ArgIdentifier}}, Decode: decodeBatchRun})
	for _, name := range []string{"exec.Run", "exec.Stream"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "cmd", Kind: ArgExpression, Required: true}, {Name: "args", Kind: ArgExpressions}, {Name: "stdin", Kind: ArgExpression}, {Name: "timeout", Kind: ArgExpression}, {Name: "timeoutMs", Kind: ArgInt}, {Name: "output", Kind: ArgIdentifier}, {Name: "exitCodeVar", Kind: ArgIdentifier}, {Name: "failOnError", Kind: ArgBool}, {Name: "throw", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeExecCommand(n, s) }})
	}
	Register(ActionSpec{Name: "fs.TempDir", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "pattern", Kind: ArgExpression}}, Decode: decodeFSTempDir})
	Register(ActionSpec{Name: "fs.WriteFile", Args: []ArgSpec{{Name: "path", Kind: ArgExpression, Required: true}, {Name: "data", Kind: ArgExpression, Required: true}}, Decode: decodeFSWriteFile})
	Register(ActionSpec{Name: "fs.ReadFile", Args: []ArgSpec{{Name: "path", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "optional", Kind: ArgBool}}, Decode: decodeFSReadFile})
	Register(ActionSpec{Name: "fs.Remove", Args: []ArgSpec{{Name: "path", Kind: ArgExpression, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
		p, e := requiredExpression(s, "path")
		return FSRemove{p}, e
	}})
	Register(ActionSpec{Name: "archive.ZipDir", Args: []ArgSpec{{Name: "path", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeArchiveZipDir})
	Register(ActionSpec{Name: "claude.Chat", Args: aiChatArgs(), Decode: decodeClaudeChat})
	Register(ActionSpec{Name: "openai.Chat", Args: append(aiChatArgs(), ArgSpec{Name: "tools", Kind: ArgExpressions}, ArgSpec{Name: "tool_choice", Kind: ArgExpression}, ArgSpec{Name: "max_rounds", Kind: ArgInt}, ArgSpec{Name: "output_usage", Kind: ArgIdentifier}, ArgSpec{Name: "output_tool_calls", Kind: ArgIdentifier}, ArgSpec{Name: "response_json_schema", Kind: ArgExpression}, ArgSpec{Name: "response_json_name", Kind: ArgExpression}, ArgSpec{Name: "response_json_strict", Kind: ArgBool}, ArgSpec{Name: "output_json", Kind: ArgIdentifier}), Decode: decodeOpenAIChat})
	Register(ActionSpec{Name: "openai.Embed", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "model", Kind: ArgExpression}, {Name: "dimensions", Kind: ArgInt}, {Name: "output_usage", Kind: ArgIdentifier}}, Decode: decodeOpenAIEmbed})
	Register(ActionSpec{Name: "openai.Stream", Args: aiChatArgs(), Decode: decodeOpenAIStream})
	Register(ActionSpec{Name: "plan.BuildAutomata", Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodePlanBuildAutomata})
	Register(ActionSpec{Name: "plan.BuildMicroPlan", Args: []ArgSpec{{Name: "usecases", Kind: ArgExpression, Required: true}, {Name: "automata", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodePlanBuildMicroPlan})
	Register(ActionSpec{Name: "cue.EmitProject", Args: []ArgSpec{{Name: "usecases", Kind: ArgExpression, Required: true}, {Name: "micro_plan", Kind: ArgExpression, Required: true}, {Name: "layout", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeCueEmitProject})
	Register(ActionSpec{Name: "cue.ValidateProject", Args: []ArgSpec{{Name: "files", Kind: ArgExpression, Required: true}, {Name: "binary", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeCueValidateProject})
	Register(ActionSpec{Name: "cue.WriteProjectFiles", Args: []ArgSpec{{Name: "root", Kind: ArgExpression, Required: true}, {Name: "files", Kind: ArgExpression, Required: true}, {Name: "mode", Kind: ArgExpression}, {Name: "prefixes", Kind: ArgExpressions}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeCueWriteProjectFiles})
	Register(ActionSpec{Name: "audit.Log", Args: []ArgSpec{{Name: "actor", Kind: ArgExpression, Required: true}, {Name: "company", Kind: ArgExpression, Required: true}, {Name: "event", Kind: ArgExpression, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
		x, e := exprs(s, "actor", "company", "event")
		if e != nil {
			return nil, e
		}
		return AuditLog{x[0], x[1], x[2]}, nil
	}})
	Register(ActionSpec{Name: "rbac.CheckPermission", Args: []ArgSpec{{Name: "user", Kind: ArgExpression, Required: true}, {Name: "permission", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}, {Name: "throw", Kind: ArgString}, {Name: "code", Kind: ArgString}, {Name: "status", Kind: ArgExpression}}, Decode: decodeRBACCheckPermission})
	Register(ActionSpec{Name: "secret.Get", Args: kvResolveArgs("key"), Decode: func(s normalizer.FlowStep) (Action, error) {
		k, d, o, e := decodeKVResolve(s, "key")
		return SecretGet{k, d, o}, e
	}})
	Register(ActionSpec{Name: "config.Get", Args: kvResolveArgs("key"), Decode: func(s normalizer.FlowStep) (Action, error) {
		k, d, o, e := decodeKVResolve(s, "key")
		return ConfigGet{k, d, o}, e
	}})
	Register(ActionSpec{Name: "model.Resolve", Args: kvResolveArgs("name"), Decode: func(s normalizer.FlowStep) (Action, error) {
		k, d, o, e := decodeKVResolve(s, "name")
		return ModelResolve{k, d, o}, e
	}})
	Register(ActionSpec{Name: "stream.Emit", Args: []ArgSpec{{Name: "data", Kind: ArgExpression, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
		v, e := requiredExpression(s, "data")
		return StreamEmit{v}, e
	}})
	Register(ActionSpec{Name: "locale.Resolve", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier}, {Name: "sources", Kind: ArgString}, {Name: "default", Kind: ArgExpression}}, Decode: decodeLocaleResolve})
	Register(ActionSpec{Name: "template.Render", Args: []ArgSpec{{Name: "template", Kind: ArgExpression, Required: true}, {Name: "data", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeTemplateRender})
	Register(ActionSpec{Name: "pdf.Render", Args: []ArgSpec{{Name: "template", Kind: ArgExpression, Required: true}, {Name: "data", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodePDFRender})
	Register(ActionSpec{Name: "session.Get", Args: []ArgSpec{{Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) { o, e := requiredOutput(s); return SessionGet{o}, e }})
	for _, name := range []string{"db.Get", "db.List", "db.Query", "db.Insert", "db.Update", "db.Upsert", "db.Delete", "db.Lock", "db.SelectForUpdate"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "source", Kind: ArgString, Required: true}, {Name: "input", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier}, {Name: "method", Kind: ArgString}, {Name: "error", Kind: ArgString}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeDBAction(n, s) }})
	}
	Register(ActionSpec{Name: "event.EmitIf", Args: []ArgSpec{{Name: "condition", Kind: ArgExpression, Required: true}, {Name: "name", Kind: ArgString, Required: true}, {Name: "payload", Kind: ArgExpression}, {Name: "payloadMap", Kind: ArgExpression}}, Decode: decodeEventEmitIf})
	Register(ActionSpec{Name: "event.Outbox", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "payload", Kind: ArgExpression, Required: true}, {Name: "id", Kind: ArgExpression}}, Decode: decodeEventOutbox})
	Register(ActionSpec{Name: "event.Wait", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "timeout", Kind: ArgExpression}, {Name: "match", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "into", Kind: ArgString}}, Decode: decodeEventWait})
	Register(ActionSpec{Name: "event.Subscribe", Args: []ArgSpec{{Name: "name", Kind: ArgExpression, Required: true}, {Name: "match", Kind: ArgExpression, Required: true}}, Decode: decodeEventSubscribe})
	Register(ActionSpec{Name: "event.Match", Args: []ArgSpec{{Name: "event", Kind: ArgExpression, Required: true}, {Name: "match", Kind: ArgExpression, Required: true}, {Name: "throw", Kind: ArgString}}, Decode: decodeEventMatch})
	Register(ActionSpec{Name: "entity.PatchNonZero", Args: []ArgSpec{{Name: "target", Kind: ArgExpression, Required: true}, {Name: "from", Kind: ArgExpression, Required: true}, {Name: "fields", Kind: ArgString, Required: true}}, Decode: decodeEntityPatchNonZero})
	Register(ActionSpec{Name: "field.CopyNonEmpty", Args: []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "to", Kind: ArgExpression, Required: true}, {Name: "fields", Kind: ArgString}}, Decode: decodeFieldCopyNonEmpty})
	Register(ActionSpec{Name: "entity.PatchValidated", Args: []ArgSpec{{Name: "target", Kind: ArgExpression, Required: true}, {Name: "from", Kind: ArgExpression, Required: true}, {Name: "source", Kind: ArgString}, {Name: "fields", Kind: ArgExpression, Required: true}}, Decode: decodeEntityPatchValidated})
	Register(ActionSpec{Name: "enum.Validate", Args: []ArgSpec{{Name: "value", Kind: ArgExpression, Required: true}, {Name: "allowed", Kind: ArgString, Required: true}, {Name: "throw", Kind: ArgString, Required: true}}, Decode: decodeEnumValidate})
	Register(ActionSpec{Name: "fsm.Transition", Args: []ArgSpec{{Name: "entity", Kind: ArgExpression, Required: true}, {Name: "to", Kind: ArgString, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) {
		e, err := requiredExpression(s, "entity")
		if err != nil {
			return nil, err
		}
		to, err := requiredString(s, "to")
		return FSMTransition{e, to}, err
	}})
	Register(ActionSpec{Name: "list.Enrich", Args: []ArgSpec{{Name: "items", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier}, {Name: "lookupSource", Kind: ArgString, Required: true}, {Name: "lookupInput", Kind: ArgExpression, Required: true}, {Name: "set", Kind: ArgString, Required: true}}, Decode: decodeListEnrich})
	Register(ActionSpec{Name: "oauth.Google.GetURL", Args: []ArgSpec{{Name: "clientID", Kind: ArgExpression, Required: true}, {Name: "redirectURL", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "state", Kind: ArgExpression}, {Name: "scopes", Kind: ArgExpression}}, Decode: decodeOAuthGoogleGetURL})
	Register(ActionSpec{Name: "oauth.Google.Exchange", Args: []ArgSpec{{Name: "clientID", Kind: ArgExpression, Required: true}, {Name: "clientSecret", Kind: ArgExpression, Required: true}, {Name: "redirectURL", Kind: ArgExpression, Required: true}, {Name: "code", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "scopes", Kind: ArgExpression}}, Decode: decodeOAuthGoogleExchange})
	Register(ActionSpec{Name: "oauth.Google.UserInfo", Args: []ArgSpec{{Name: "token", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: decodeOAuthGoogleUserInfo})
	Register(ActionSpec{Name: "list.Sort", Args: []ArgSpec{{Name: "items", Kind: ArgExpression, Required: true}, {Name: "by", Kind: ArgString, Required: true}, {Name: "desc", Kind: ArgBool}}, Decode: decodeListSort})
	for _, name := range []string{"list.Sum", "list.Avg"} {
		n := name
		Register(ActionSpec{Name: n, Args: []ArgSpec{{Name: "input", Kind: ArgExpression, Required: true}, {Name: "field", Kind: ArgString}, {Name: "output", Kind: ArgIdentifier, Required: true}}, Decode: func(s normalizer.FlowStep) (Action, error) { return decodeListAggregate(s, n) }})
	}
}

func kvResolveArgs(key string) []ArgSpec {
	return []ArgSpec{{Name: key, Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}, {Name: "default", Kind: ArgExpression}}
}
func decodeKVResolve(s normalizer.FlowStep, key string) (Expression, Expression, string, error) {
	k, e := requiredExpression(s, key)
	if e != nil {
		return Expression{}, Expression{}, "", e
	}
	o, e := requiredOutput(s)
	return k, optionalExpression(s, "default"), o, e
}
func decodeRBACCheckPermission(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "user", "permission")
	if e != nil {
		return nil, e
	}
	o, e := optionalIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	t, _ := optionalString(s, "throw")
	c, _ := optionalString(s, "code")
	if c == "" {
		c = "FORBIDDEN"
	}
	status := optionalExpression(s, "status")
	if status.Source == "" {
		status.Source = "http.StatusForbidden"
	}
	return RBACCheckPermission{x[0], x[1], status, o, t, c}, nil
}
func decodeLocaleResolve(s normalizer.FlowStep) (Action, error) {
	o, e := optionalIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	if o == "" {
		o = "locale"
	}
	src, _ := optionalString(s, "sources")
	d := optionalExpression(s, "default")
	if d.Source == "" {
		d.Source = `"en"`
	}
	return LocaleResolve{src, d, o}, nil
}
func decodeTemplateRender(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "template", "data")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return TemplateRender{x[0], x[1], o}, e
}
func decodePDFRender(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "template", "data")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return PDFRender{x[0], x[1], o}, e
}

func requiredOutput(s normalizer.FlowStep) (string, error) { return requiredIdentifier(s, "output") }
func decodeDBAction(name string, s normalizer.FlowStep) (Action, error) {
	source, err := requiredString(s, "source")
	if err != nil {
		return nil, err
	}
	input, err := requiredExpression(s, "input")
	if err != nil {
		return nil, err
	}
	output, err := optionalIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	method, _ := optionalString(s, "method")
	message, _ := optionalString(s, "error")
	if (name == "db.Lock" || name == "db.SelectForUpdate") && output == "" {
		output = strings.ToLower(source[:1]) + source[1:]
	}
	f := DBFields{Source: source, Method: method, Output: output, Error: message, Input: input}
	switch name {
	case "db.Get":
		return DBGet{f}, nil
	case "db.List":
		return DBList{f}, nil
	case "db.Query":
		if method == "" {
			return nil, fmt.Errorf("method is required")
		}
		return DBQuery{f}, nil
	case "db.Insert":
		return DBInsert{f}, nil
	case "db.Update":
		return DBUpdate{f}, nil
	case "db.Upsert":
		return DBUpsert{f}, nil
	case "db.Delete":
		return DBDelete{f}, nil
	case "db.Lock":
		return DBLock{f}, nil
	case "db.SelectForUpdate":
		return DBSelectForUpdate{f}, nil
	}
	return nil, fmt.Errorf("unsupported DB action %q", name)
}
func decodeEventEmitIf(s normalizer.FlowStep) (Action, error) {
	c, e := requiredExpression(s, "condition")
	if e != nil {
		return nil, e
	}
	n, e := requiredString(s, "name")
	if e != nil {
		return nil, e
	}
	m, e := expressionMap(s.Args["payloadMap"])
	return EventEmitIf{c, n, optionalExpression(s, "payload"), m}, e
}
func decodeEventOutbox(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "name", "payload")
	if e != nil {
		return nil, e
	}
	id := optionalExpression(s, "id")
	if id.Source == "" {
		id.Source = "uuid.NewString()"
	}
	return EventOutbox{x[0], x[1], id}, nil
}
func decodeEventWait(s normalizer.FlowStep) (Action, error) {
	n, e := requiredExpression(s, "name")
	if e != nil {
		return nil, e
	}
	o, e := optionalIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	into, _ := optionalString(s, "into")
	timeout := optionalExpression(s, "timeout")
	if timeout.Source == "" {
		timeout.Source = "5*time.Minute"
	}
	return EventWait{n, timeout, optionalExpression(s, "match"), o, into}, nil
}
func decodeEventSubscribe(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "name", "match")
	if e != nil {
		return nil, e
	}
	_, e = nestedSteps(s, "_do", true)
	return EventSubscribe{Name: x[0], Match: x[1]}, e
}
func decodeEventMatch(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "event", "match")
	if e != nil {
		return nil, e
	}
	t, _ := optionalString(s, "throw")
	return EventMatch{x[0], x[1], t}, nil
}
func commaList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func decodeEntityPatchNonZero(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "target", "from")
	if e != nil {
		return nil, e
	}
	f, e := requiredString(s, "fields")
	return EntityPatchNonZero{x[0], x[1], commaList(f)}, e
}
func decodeFieldCopyNonEmpty(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "from", "to")
	if e != nil {
		return nil, e
	}
	f, _ := optionalString(s, "fields")
	return FieldCopyNonEmpty{x[0], x[1], commaList(f)}, nil
}
func decodeEntityPatchValidated(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "target", "from")
	if e != nil {
		return nil, e
	}
	source, _ := optionalString(s, "source")
	raw, ok := s.Args["fields"].(map[string]map[string]string)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("fields map is required")
	}
	fields := make(map[string]PatchFieldRule, len(raw))
	for name, rules := range raw {
		fields[name] = PatchFieldRule{Normalize: rules["normalize"], Format: rules["format"], Unique: rules["unique"]}
	}
	return EntityPatchValidated{x[0], x[1], source, fields}, nil
}
func decodeEnumValidate(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "value")
	if e != nil {
		return nil, e
	}
	a, e := requiredString(s, "allowed")
	if e != nil {
		return nil, e
	}
	t, e := requiredString(s, "throw")
	return EnumValidate{v, commaList(a), t}, e
}
func decodeListEnrich(s normalizer.FlowStep) (Action, error) {
	items, e := requiredExpression(s, "items")
	if e != nil {
		return nil, e
	}
	lookup, e := requiredExpression(s, "lookupInput")
	if e != nil {
		return nil, e
	}
	as, e := optionalIdentifier(s, "as")
	if e != nil {
		return nil, e
	}
	if as == "" {
		as = "_item"
	}
	source, e := requiredString(s, "lookupSource")
	if e != nil {
		return nil, e
	}
	set, e := requiredString(s, "set")
	if e != nil {
		return nil, e
	}
	var fields []EnrichField
	for _, pair := range strings.Split(set, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("set contains invalid mapping %q", pair)
		}
		fields = append(fields, EnrichField{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
	}
	return ListEnrich{items, lookup, as, source, fields}, nil
}
func defaultExpr(s normalizer.FlowStep, key, value string) Expression {
	e := optionalExpression(s, key)
	if e.Source == "" {
		e.Source = value
	}
	return e
}
func decodeOAuthGoogleGetURL(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "clientID", "redirectURL")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return OAuthGoogleGetURL{x[0], x[1], defaultExpr(s, "state", `""`), defaultExpr(s, "scopes", `"openid email profile"`), o}, e
}
func decodeOAuthGoogleExchange(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "clientID", "clientSecret", "redirectURL", "code")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return OAuthGoogleExchange{x[0], x[1], x[2], x[3], defaultExpr(s, "scopes", `"openid email profile"`), o}, e
}
func decodeOAuthGoogleUserInfo(s normalizer.FlowStep) (Action, error) {
	t, e := requiredExpression(s, "token")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return OAuthGoogleUserInfo{t, o}, e
}
func decodePlanBuildAutomata(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return PlanBuildAutomata{i, o}, e
}
func decodePlanBuildMicroPlan(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "usecases", "automata")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	return PlanBuildMicroPlan{x[0], x[1], o}, e
}
func decodeCueEmitProject(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "usecases", "micro_plan")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	l := optionalExpression(s, "layout")
	if l.Source == "" {
		l.Source = `"split"`
	}
	return CueEmitProject{x[0], x[1], l, o}, e
}
func decodeCueValidateProject(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "files")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	b := optionalExpression(s, "binary")
	if b.Source == "" {
		b.Source = `"ang"`
	}
	return CueValidateProject{f, b, o}, e
}
func decodeCueWriteProjectFiles(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "root", "files")
	if e != nil {
		return nil, e
	}
	o, e := requiredOutput(s)
	if e != nil {
		return nil, e
	}
	m := optionalExpression(s, "mode")
	if m.Source == "" {
		m.Source = `"upsert"`
	}
	var p []string
	if raw, ok := s.Args["prefixes"]; ok {
		switch v := raw.(type) {
		case []string:
			p = append(p, v...)
		case []any:
			for _, item := range v {
				value, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("prefixes must contain strings")
				}
				p = append(p, value)
			}
		default:
			return nil, fmt.Errorf("prefixes must be a string list")
		}
	}
	return CueWriteProjectFiles{x[0], x[1], m, p, o}, nil
}

func aiChatArgs() []ArgSpec {
	return []ArgSpec{{Name: "user_message", Kind: ArgExpression, Required: true}, {Name: "system", Kind: ArgExpression}, {Name: "system_context", Kind: ArgExpression}, {Name: "history", Kind: ArgExpression}, {Name: "output", Kind: ArgIdentifier}, {Name: "model", Kind: ArgExpression}, {Name: "max_tokens", Kind: ArgInt}, {Name: "locale", Kind: ArgExpression}, {Name: "timezone", Kind: ArgExpression}}
}
func aiOutput(s normalizer.FlowStep, key, fallback string) (string, error) {
	v, e := optionalIdentifier(s, key)
	if e != nil {
		return "", e
	}
	if v == "" {
		v = fallback
	}
	return v, nil
}
func decodeClaudeChat(s normalizer.FlowStep) (Action, error) {
	u, e := requiredExpression(s, "user_message")
	if e != nil {
		return nil, e
	}
	o, e := aiOutput(s, "output", "claudeReply")
	if e != nil {
		return nil, e
	}
	n, e := optionalInt(s, "max_tokens", 4096)
	m := optionalExpression(s, "model")
	if m.Source == "" {
		m.Source = `"claude-sonnet-4-6"`
	}
	return ClaudeChat{optionalExpression(s, "system"), optionalExpression(s, "system_context"), u, optionalExpression(s, "history"), m, optionalExpression(s, "locale"), optionalExpression(s, "timezone"), o, n}, e
}
func decodeOpenAIChat(s normalizer.FlowStep) (Action, error) {
	u, decodeErr := requiredExpression(s, "user_message")
	var e error
	o, e := aiOutput(s, "output", "openaiReply")
	if e != nil {
		return nil, e
	}
	mt, e := optionalInt(s, "max_tokens", 4096)
	if e != nil {
		return nil, e
	}
	mr, e := optionalInt(s, "max_rounds", 6)
	if e != nil {
		return nil, e
	}
	strict := true
	if _, ok := s.Args["response_json_strict"]; ok {
		strict, e = optionalBool(s, "response_json_strict")
		if e != nil {
			return nil, e
		}
	}
	m := optionalExpression(s, "model")
	if m.Source == "" {
		m.Source = `"gpt-4o-mini"`
	}
	rn := optionalExpression(s, "response_json_name")
	if rn.Source == "" {
		rn.Source = `"structured_response"`
	}
	var tools []string
	if raw, ok := s.Args["tools"]; ok {
		switch v := raw.(type) {
		case []string:
			tools = append(tools, v...)
		case string:
			if strings.TrimSpace(v) != "" {
				tools = []string{v}
			}
		default:
			return nil, fmt.Errorf("tools must be a string list")
		}
	}
	ou, e := optionalIdentifier(s, "output_usage")
	if e != nil {
		return nil, e
	}
	ot, e := optionalIdentifier(s, "output_tool_calls")
	if e != nil {
		return nil, e
	}
	oj, e := optionalIdentifier(s, "output_json")
	if e != nil {
		return nil, e
	}
	return OpenAIChat{optionalExpression(s, "system"), optionalExpression(s, "system_context"), u, optionalExpression(s, "history"), m, optionalExpression(s, "tool_choice"), optionalExpression(s, "response_json_schema"), rn, o, ou, ot, oj, tools, mt, mr, strict}, decodeErr
}
func decodeOpenAIEmbed(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := requiredIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	u, e := optionalIdentifier(s, "output_usage")
	if e != nil {
		return nil, e
	}
	d, e := optionalInt(s, "dimensions", 0)
	m := optionalExpression(s, "model")
	if m.Source == "" {
		m.Source = `"text-embedding-3-small"`
	}
	return OpenAIEmbed{i, m, o, u, d}, e
}
func decodeOpenAIStream(s normalizer.FlowStep) (Action, error) {
	u, e := requiredExpression(s, "user_message")
	if e != nil {
		return nil, e
	}
	o, e := optionalIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	n, e := optionalInt(s, "max_tokens", 4096)
	m := optionalExpression(s, "model")
	if m.Source == "" {
		m.Source = `"gpt-4o"`
	}
	return OpenAIStream{optionalExpression(s, "system"), optionalExpression(s, "system_context"), u, optionalExpression(s, "history"), m, o, n}, e
}

func decodeExecCommand(name string, s normalizer.FlowStep) (Action, error) {
	cmd, err := requiredExpression(s, "cmd")
	if err != nil {
		return nil, err
	}
	args, err := expressionList(s.Args["args"])
	if err != nil {
		return nil, err
	}
	tms, err := optionalInt(s, "timeoutMs", 0)
	if err != nil {
		return nil, err
	}
	out, err := optionalIdentifier(s, "output")
	if err != nil {
		return nil, err
	}
	exit, err := optionalIdentifier(s, "exitCodeVar")
	if err != nil {
		return nil, err
	}
	fail := true
	if _, ok := s.Args["failOnError"]; ok {
		fail, err = optionalBool(s, "failOnError")
		if err != nil {
			return nil, err
		}
	}
	throw, _ := optionalString(s, "throw")
	return ExecCommand{Alias: name, Command: cmd, Arguments: args, Stdin: optionalExpression(s, "stdin"), Timeout: optionalExpression(s, "timeout"), TimeoutMS: tms, Output: out, ExitCodeVar: exit, FailOnError: fail, Throw: throw}, nil
}
func decodeFSTempDir(s normalizer.FlowStep) (Action, error) {
	o, e := requiredIdentifier(s, "output")
	p := optionalExpression(s, "pattern")
	if p.Source == "" {
		p.Source = `"ang-tmp-*"`
	}
	return FSTempDir{p, o}, e
}
func decodeFSWriteFile(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "path", "data")
	if e != nil {
		return nil, e
	}
	return FSWriteFile{x[0], x[1]}, nil
}
func decodeFSReadFile(s normalizer.FlowStep) (Action, error) {
	p, e := requiredExpression(s, "path")
	if e != nil {
		return nil, e
	}
	o, e := requiredIdentifier(s, "output")
	if e != nil {
		return nil, e
	}
	optional, e := optionalBool(s, "optional")
	return FSReadFile{p, o, optional}, e
}
func decodeArchiveZipDir(s normalizer.FlowStep) (Action, error) {
	p, e := requiredExpression(s, "path")
	if e != nil {
		return nil, e
	}
	o, e := requiredIdentifier(s, "output")
	return ArchiveZipDir{p, o}, e
}

func decodeBatchRun(s normalizer.FlowStep) (Action, error) {
	from, err := requiredExpression(s, "from")
	if err != nil {
		return nil, err
	}
	size := optionalAnyExpression(s, "size")
	if size.Source == "" {
		size = Expression{Source: "100", Type: TypeRef{Kind: TypeInt}}
	}
	as, err := optionalIdentifier(s, "as")
	if err != nil {
		return nil, err
	}
	if as == "" {
		as = "batch"
	}
	_, err = nestedSteps(s, "_do", true)
	if err != nil {
		return nil, err
	}
	return BatchRun{From: from, Size: size, As: as}, nil
}

func listPredicateArgs(_ bool) []ArgSpec {
	return []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier}, {Name: "condition", Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}
}
func listTransformArgs(value string) []ArgSpec {
	return []ArgSpec{{Name: "from", Kind: ArgExpression, Required: true}, {Name: "as", Kind: ArgIdentifier}, {Name: value, Kind: ArgExpression, Required: true}, {Name: "output", Kind: ArgIdentifier, Required: true}}
}
func expressionFromAny(s normalizer.FlowStep, k string) (Expression, error) {
	raw, ok := s.Args[k]
	if !ok {
		return Expression{}, fmt.Errorf("%s is required", k)
	}
	switch v := raw.(type) {
	case string:
		return Expression{Source: strings.TrimSpace(v), Type: TypeRef{Kind: TypeUnknown}}, nil
	case int:
		return Expression{Source: fmt.Sprint(v), Type: TypeRef{Kind: TypeInt}}, nil
	case int64:
		return Expression{Source: fmt.Sprint(v), Type: TypeRef{Kind: TypeInt}}, nil
	case float64:
		return Expression{Source: fmt.Sprint(int(v)), Type: TypeRef{Kind: TypeInt}}, nil
	}
	return Expression{}, fmt.Errorf("%s must be expression, got %T", k, raw)
}
func decodeListAppend(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "to", "item")
	if e != nil {
		return nil, e
	}
	return ListAppend{x[0], x[1]}, nil
}
func decodeListLen(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return ListLen{i, o}, e
}
func decodeListNew(s normalizer.FlowStep) (Action, error) {
	o, e := output(s)
	if e != nil {
		return nil, e
	}
	g, e := requiredString(s, "type")
	if e != nil {
		return nil, e
	}
	c := Expression{}
	if _, ok := s.Args["cap"]; ok {
		c, e = expressionFromAny(s, "cap")
	}
	return ListNew{o, parseListType(g), g, c}, e
}
func decodeListFilter(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	c, e := requiredExpression(s, "condition")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "as")
	if a == "" {
		a = "item"
	}
	o, e := output(s)
	return ListFilter{f, a, c, o}, e
}
func decodeListPaginate(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "offset", "limit")
	if e != nil {
		return nil, e
	}
	d, e := optionalInt(s, "defaultLimit", 50)
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	t, _ := optionalString(s, "total")
	return ListPaginate{x[0], x[1], x[2], d, o, t}, e
}
func decodeListFind(s normalizer.FlowStep) (Action, error) {
	f, e := decodeListFilter(s)
	if e != nil {
		return nil, e
	}
	v := f.(ListFilter)
	i, _ := optionalString(s, "into")
	found, _ := optionalString(s, "found")
	return ListFind{v.From, v.As, v.Condition, v.Output, i, found}, nil
}
func decodeListBoolean(s normalizer.FlowStep, all bool) (Action, error) {
	f, e := decodeListFilter(s)
	if e != nil {
		return nil, e
	}
	v := f.(ListFilter)
	if all {
		return ListAll{v.From, v.As, v.Condition, v.Output}, nil
	}
	return ListAny{v.From, v.As, v.Condition, v.Output}, nil
}
func decodeListMap(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	v, e := requiredExpression(s, "expr")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "as")
	if a == "" {
		a = "item"
	}
	o, e := output(s)
	return ListMap{f, a, v, o}, e
}
func decodeListReduce(s normalizer.FlowStep) (Action, error) {
	m, e := decodeListMap(s)
	if e != nil {
		return nil, e
	}
	v := m.(ListMap)
	initial := optionalExpression(s, "initial")
	if initial.Source == "" {
		initial = optionalExpression(s, "init")
	}
	if initial.Source == "" {
		initial = optionalExpression(s, "seed")
	}
	return ListReduce{v.From, v.As, v.Value, initial, v.Output}, nil
}
func decodeListGroupBy(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	k, e := requiredExpression(s, "key")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "as")
	if a == "" {
		a = "item"
	}
	o, e := output(s)
	return ListGroupBy{f, a, k, o}, e
}
func decodeListDistinct(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	a, _ := optionalString(s, "as")
	if a == "" {
		a = "item"
	}
	o, e := output(s)
	return ListDistinct{f, a, optionalExpression(s, "key"), o}, e
}
func decodeListChunk(s normalizer.FlowStep) (Action, error) {
	f, e := requiredExpression(s, "from")
	if e != nil {
		return nil, e
	}
	z, e := expressionFromAny(s, "size")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return ListChunk{f, z, o}, e
}
func decodeListSort(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "items")
	if e != nil {
		return nil, e
	}
	b, e := requiredString(s, "by")
	if e != nil {
		return nil, e
	}
	d := false
	o := optionalExpression(s, "order")
	if _, ok := s.Args["desc"]; ok {
		d, e = optionalBool(s, "desc")
		if o.Source == "" {
			if d {
				o.Source = "desc"
			} else {
				o.Source = "asc"
			}
		}
	}
	return ListSort{Items: i, Order: o, By: b, Descending: d}, e
}
func decodeListAggregate(s normalizer.FlowStep, n string) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	f, _ := optionalString(s, "field")
	o, e := output(s)
	return ListAggregate{n, i, f, o}, e
}
func parseListType(s string) TypeRef {
	if !strings.HasPrefix(strings.TrimSpace(s), "[]") {
		return TypeRef{Kind: TypeUnknown, Name: s}
	}
	elem := parseTypeHint(strings.TrimPrefix(strings.TrimSpace(s), "[]"))
	return TypeRef{Kind: TypeList, Elem: &elem}
}

func expressionActionArgs(names ...string) []ArgSpec {
	a := make([]ArgSpec, 0, len(names)+1)
	for _, n := range names {
		a = append(a, ArgSpec{Name: n, Kind: ArgExpression, Required: true})
	}
	return append(a, ArgSpec{Name: "output", Kind: ArgIdentifier, Required: true})
}
func output(s normalizer.FlowStep) (string, error) { return requiredString(s, "output") }
func exprs(s normalizer.FlowStep, names ...string) ([]Expression, error) {
	r := make([]Expression, 0, len(names))
	for _, n := range names {
		v, e := requiredExpression(s, n)
		if e != nil {
			return nil, e
		}
		r = append(r, v)
	}
	return r, nil
}
func optionalInt(s normalizer.FlowStep, k string, d int) (int, error) {
	raw, ok := s.Args[k]
	if !ok {
		return d, nil
	}
	switch v := raw.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	}
	return 0, fmt.Errorf("%s must be int, got %T", k, raw)
}
func decodeRandomCode(s normalizer.FlowStep) (Action, error) {
	n, e := optionalInt(s, "length", 6)
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return RandomCode{n, o}, e
}
func decodeRandomToken(s normalizer.FlowStep) (Action, error) {
	n, e := optionalInt(s, "bytes", 32)
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return RandomToken{n, o}, e
}
func decodeRegexMatch(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "pattern")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return RegexMatch{x[0], x[1], o}, e
}
func decodeRegexReplace(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "pattern", "repl")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return RegexReplace{x[0], x[1], x[2], o}, e
}
func decodeStringFormat(s normalizer.FlowStep) (Action, error) {
	t, e := requiredExpression(s, "template")
	if e != nil {
		return nil, e
	}
	a, e := expressionList(s.Args["args"])
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return StringFormat{t, a, o}, e
}
func decodeStringConcat(s normalizer.FlowStep) (Action, error) {
	p, e := expressionList(s.Args["parts"])
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return StringConcat{p, optionalExpression(s, "sep"), o}, e
}
func decodeStringStripMarkdown(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := optionalIdentifier(s, "output")
	return StringStripMarkdown{i, o}, e
}
func decodeStringReplaceAll(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "old", "new")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return StringReplaceAll{x[0], x[1], x[2], o}, e
}
func decodeStringTrimSpace(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return StringTrimSpace{i, o}, e
}
func decodeStringNormalize(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	m, _ := optionalString(s, "mode")
	if m == "" {
		m = "lower"
	}
	o, e := output(s)
	return StringNormalize{i, m, o}, e
}
func decodeTimeNow(s normalizer.FlowStep) (Action, error) {
	o, e := output(s)
	f, _ := optionalString(s, "format")
	return TimeNow{o, f}, e
}
func decodeTimeParse(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "value")
	if e != nil {
		return nil, e
	}
	f, _ := optionalString(s, "format")
	o, e := output(s)
	return TimeParse{v, f, o}, e
}
func decodeTimeFormat(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	f, _ := optionalString(s, "format")
	z, _ := optionalString(s, "timezone")
	zero, _ := optionalString(s, "zero")
	if zero == "" {
		zero = "format"
	}
	if zero != "format" && zero != "empty" {
		return nil, fmt.Errorf("unsupported zero mode %q (use \"format\" or \"empty\")", zero)
	}
	o, e := output(s)
	return TimeFormat{Input: i, Format: f, Timezone: z, Zero: zero, Output: o}, e
}
func decodeTimeInZone(s normalizer.FlowStep) (Action, error) {
	i, e := requiredExpression(s, "input")
	if e != nil {
		return nil, e
	}
	z, e := requiredString(s, "timezone")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return TimeInZone{i, z, o}, e
}
func decodeTimeAdd(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "input", "duration")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return TimeAdd{x[0], x[1], o}, e
}
func decodeTimeSub(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "a", "b")
	if e != nil {
		return nil, e
	}
	o, e := output(s)
	return TimeSub{x[0], x[1], o}, e
}
func decodeTimeDiff(s normalizer.FlowStep) (Action, error) {
	x, e := exprs(s, "from", "to")
	if e != nil {
		return nil, e
	}
	u, _ := optionalString(s, "unit")
	o, e := output(s)
	return TimeDiff{x[0], x[1], u, o}, e
}
func decodeTimeCheckExpiry(s normalizer.FlowStep) (Action, error) {
	v, e := requiredExpression(s, "value")
	if e != nil {
		return nil, e
	}
	t, e := requiredString(s, "throw")
	if e != nil {
		return nil, e
	}
	m, _ := optionalString(s, "mustBe")
	if m == "" {
		m = "future"
	}
	return TimeCheckExpiry{v, t, m}, e
}

func decodeCacheGet(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	output, err := requiredIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	optional := true
	if _, exists := step.Args["optional"]; exists {
		optional, err = optionalBool(step, "optional")
		if err != nil {
			return nil, err
		}
	}
	return CacheGet{Key: key, Output: output, Optional: optional}, nil
}

func decodeCacheSet(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	value, err := requiredExpression(step, "value")
	if err != nil {
		return nil, err
	}
	ttl := optionalExpression(step, "ttl")
	return CacheSet{Key: key, Value: value, TTL: ttl}, nil
}

func decodeCacheDelete(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	return CacheDelete{Key: key}, nil
}

func decodeStateGet(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	output, err := requiredIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	into, _ := optionalString(step, "into")
	return StateGet{Key: key, Output: output, Default: optionalExpression(step, "default"), Into: into}, nil
}

func decodeStateSet(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	value, err := requiredExpression(step, "value")
	if err != nil {
		return nil, err
	}
	return StateSet{Key: key, Value: value, TTL: optionalExpression(step, "ttl")}, nil
}

func decodeStateDelete(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	return StateDelete{Key: key}, nil
}

func decodeStorageUpload(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	data, err := requiredExpression(step, "data")
	if err != nil {
		return nil, err
	}
	output, err := optionalIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	return StorageUpload{Key: key, Data: data, ContentType: optionalExpression(step, "contentType"), Output: output}, nil
}

func decodeStorageDownload(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	output, err := requiredIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	return StorageDownload{Key: key, Output: output}, nil
}

func decodeStorageDelete(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	return StorageDelete{Key: key}, nil
}

func decodeStorageList(step normalizer.FlowStep) (Action, error) {
	prefix, err := requiredExpression(step, "prefix")
	if err != nil {
		return nil, err
	}
	output, err := requiredIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	return StorageList{Prefix: prefix, Output: output}, nil
}

func decodeStorageGetURL(step normalizer.FlowStep) (Action, error) {
	key, err := requiredExpression(step, "key")
	if err != nil {
		return nil, err
	}
	output, err := requiredIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	return StorageGetURL{Key: key, Output: output}, nil
}

func decodeMappingAssign(step normalizer.FlowStep) (Action, error) {
	target, err := requiredString(step, "to")
	if err != nil {
		return nil, err
	}
	value, err := requiredString(step, "value")
	if err != nil {
		return nil, err
	}
	declare, err := optionalBoolish(step, "declare")
	if err != nil {
		return nil, err
	}
	typeName, _ := optionalString(step, "type")
	return MappingAssign{Target: Expression{Source: target, Type: TypeRef{Kind: TypeUnknown}}, Value: Expression{Source: value, Type: TypeRef{Kind: TypeUnknown}}, Declare: declare, Type: parseTypeHint(typeName)}, nil
}

func decodeMappingMap(step normalizer.FlowStep) (Action, error) {
	input, _ := optionalString(step, "input")
	if input == "" {
		input, _ = optionalString(step, "from")
	}
	output, _ := optionalString(step, "output")
	if output == "" {
		output, _ = optionalString(step, "to")
	}
	entity, _ := optionalString(step, "entity")
	if entity != "" {
		if output == "" {
			return nil, fmt.Errorf("mapping.Map with entity requires output/to")
		}
		return MappingMap{Input: Expression{Source: input, Type: TypeRef{Kind: TypeUnknown}}, Output: output, Entity: entity}, nil
	}
	if input == "" || output == "" {
		return nil, fmt.Errorf("mapping.Map without entity requires both input/from and output/to")
	}
	return MappingMap{Input: Expression{Source: input, Type: TypeRef{Kind: TypeUnknown}}, Output: output, Entity: entity}, nil
}

func decodeLogicCheck(step normalizer.FlowStep) (Action, error) {
	condition, err := requiredString(step, "condition")
	if err != nil {
		return nil, err
	}
	throwMessage, err := requiredString(step, "throw")
	if err != nil {
		return nil, err
	}
	params, err := expressionList(step.Args["params"])
	if err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	status, _ := optionalString(step, "status")
	return LogicCheck{Condition: Expression{Source: condition, Type: TypeRef{Kind: TypeBool}}, Throw: throwMessage, Status: status, Params: params}, nil
}

func decodeFlowIf(step normalizer.FlowStep) (Action, error) {
	condition, err := requiredString(step, "condition")
	if err != nil {
		return nil, err
	}
	_, ok := step.Args["_then"].([]normalizer.FlowStep)
	if !ok {
		return nil, fmt.Errorf("then block is required")
	}
	return FlowIf{Condition: Expression{Source: condition, Type: TypeRef{Kind: TypeBool}}}, nil
}

func decodeFlowBlock(step normalizer.FlowStep, transactional bool) (Action, error) {
	_, ok := step.Args["_do"].([]normalizer.FlowStep)
	if !ok {
		return nil, fmt.Errorf("do block is required")
	}
	return FlowBlock{Transactional: transactional}, nil
}

func decodeEventPublish(step normalizer.FlowStep, broadcast bool) (Action, error) {
	event, err := requiredString(step, "name")
	if err != nil {
		return nil, err
	}
	payload, _ := optionalString(step, "payload")
	payloadMap, err := expressionMap(step.Args["payloadMap"])
	if err != nil {
		return nil, fmt.Errorf("payloadMap: %w", err)
	}
	return EventPublish{Event: event, Payload: Expression{Source: payload, Type: TypeRef{Kind: TypeUnknown}}, PayloadMap: payloadMap, Broadcast: broadcast}, nil
}

func callArgs(includeFunction bool) []ArgSpec {
	args := []ArgSpec{}
	if includeFunction {
		args = append(args, ArgSpec{Name: "func", Kind: ArgExpression, Required: true})
	}
	return append(args,
		ArgSpec{Name: "args", Kind: ArgExpressions},
		ArgSpec{Name: "output", Kind: ArgIdentifier},
		ArgSpec{Name: "ignoreErr", Kind: ArgBool},
		ArgSpec{Name: "ignoreErrReason", Kind: ArgString},
	)
}

func decodeLogicCall(step normalizer.FlowStep) (Action, error) {
	function, err := requiredString(step, "func")
	if err != nil {
		return nil, err
	}
	arguments, err := expressionList(step.Args["args"])
	if err != nil {
		return nil, fmt.Errorf("args: %w", err)
	}
	options, err := decodeCallOptions(step)
	if err != nil {
		return nil, err
	}
	return LogicCall{Function: Expression{Source: function, Type: TypeRef{Kind: TypeUnknown}}, Arguments: arguments, CallOptions: options}, nil
}

func decodeServiceCall(step normalizer.FlowStep) (Action, error) {
	service, err := requiredString(step, "service")
	if err != nil {
		return nil, err
	}
	method, err := requiredString(step, "method")
	if err != nil {
		return nil, err
	}
	arguments, err := expressionList(step.Args["args"])
	if err != nil {
		return nil, fmt.Errorf("args: %w", err)
	}
	options, err := decodeCallOptions(step)
	if err != nil {
		return nil, err
	}
	return ServiceCall{Service: service, Method: method, Arguments: arguments, CallOptions: options}, nil
}

func decodeRepositoryCall(operation RepositoryOperation, step normalizer.FlowStep) (Action, error) {
	entity, err := requiredString(step, "source")
	if err != nil {
		return nil, err
	}
	input, _ := optionalString(step, "input")
	output, err := optionalIdentifier(step, "output")
	if err != nil {
		return nil, err
	}
	method, _ := optionalString(step, "method")
	errorMessage, _ := optionalString(step, "error")
	required, err := optionalBool(step, "required")
	if err != nil {
		return nil, err
	}
	arguments, err := expressionList(step.Args["args"])
	if err != nil {
		return nil, fmt.Errorf("args: %w", err)
	}
	list, err := optionalBool(step, "list")
	if err != nil {
		return nil, err
	}
	find, _ := optionalString(step, "find")
	return RepositoryCall{Operation: operation, Entity: entity, Input: Expression{Source: input, Type: TypeRef{Kind: TypeUnknown}}, Output: output, Method: method, Error: errorMessage, Required: required, Arguments: arguments, List: list, Find: Expression{Source: find, Type: TypeRef{Kind: TypeUnknown}}}, nil
}

func decodeCallOptions(step normalizer.FlowStep) (CallOptions, error) {
	output, err := optionalIdentifier(step, "output")
	if err != nil {
		return CallOptions{}, err
	}
	ignore, err := optionalBool(step, "ignoreErr")
	if err != nil {
		return CallOptions{}, err
	}
	reason, _ := optionalString(step, "ignoreErrReason")
	return CallOptions{Output: output, IgnoreError: ignore, IgnoreErrReason: reason}, nil
}

func requiredString(step normalizer.FlowStep, key string) (string, error) {
	value, ok := optionalString(step, key)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func requiredExpression(step normalizer.FlowStep, key string) (Expression, error) {
	value, err := requiredString(step, key)
	if err != nil {
		return Expression{}, err
	}
	return Expression{Source: value, Type: TypeRef{Kind: TypeUnknown}}, nil
}

func optionalExpression(step normalizer.FlowStep, key string) Expression {
	value, _ := optionalString(step, key)
	return Expression{Source: value, Type: TypeRef{Kind: TypeUnknown}}
}

func requiredIdentifier(step normalizer.FlowStep, key string) (string, error) {
	value, err := optionalIdentifier(step, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func optionalString(step normalizer.FlowStep, key string) (string, bool) {
	raw, exists := step.Args[key]
	if !exists || raw == nil {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func optionalIdentifier(step normalizer.FlowStep, key string) (string, error) {
	value, exists := optionalString(step, key)
	if !exists || value == "" {
		return "", nil
	}
	if !token.IsIdentifier(value) || token.Lookup(value).IsKeyword() {
		return "", fmt.Errorf("%s must be a Go identifier, got %q", key, value)
	}
	return value, nil
}

func optionalBool(step normalizer.FlowStep, key string) (bool, error) {
	raw, exists := step.Args[key]
	if !exists || raw == nil {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be bool, got %T", key, raw)
	}
	return value, nil
}

func optionalBoolish(step normalizer.FlowStep, key string) (bool, error) {
	raw, exists := step.Args[key]
	if !exists || raw == nil {
		return false, nil
	}
	switch value := raw.(type) {
	case bool:
		return value, nil
	case string:
		value = strings.TrimSpace(value)
		if strings.EqualFold(value, "true") {
			return true, nil
		}
		if strings.EqualFold(value, "false") || value == "" {
			return false, nil
		}
	}
	return false, fmt.Errorf("%s must be bool, got %T", key, raw)
}

func parseTypeHint(value string) TypeRef {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "map[") {
		return TypeRef{Kind: TypeMap, Name: value}
	}
	switch strings.ToLower(value) {
	case "string":
		return TypeRef{Kind: TypeString}
	case "bool":
		return TypeRef{Kind: TypeBool}
	case "int", "int32", "int64":
		return TypeRef{Kind: TypeInt}
	case "":
		return TypeRef{Kind: TypeUnknown}
	default:
		return TypeRef{Kind: TypeEntity, Name: strings.TrimPrefix(value, "domain.")}
	}
}

func expressionMap(raw any) (map[string]Expression, error) {
	if raw == nil {
		return nil, nil
	}
	out := map[string]Expression{}
	switch values := raw.(type) {
	case map[string]string:
		for key, value := range values {
			out[key] = Expression{Source: strings.TrimSpace(value), Type: TypeRef{Kind: TypeUnknown}}
		}
	case map[string]any:
		for key, rawValue := range values {
			value, ok := rawValue.(string)
			if !ok {
				return nil, fmt.Errorf("field %s must be string, got %T", key, rawValue)
			}
			out[key] = Expression{Source: strings.TrimSpace(value), Type: TypeRef{Kind: TypeUnknown}}
		}
	default:
		return nil, fmt.Errorf("expected string expression map, got %T", raw)
	}
	return out, nil
}

func expressionList(raw any) ([]Expression, error) {
	var values []string
	switch typed := raw.(type) {
	case nil:
		return nil, nil
	case string:
		values = []string{typed}
	case []string:
		values = typed
	case []any:
		values = make([]string, 0, len(typed))
		for index, item := range typed {
			value, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("item %d must be string, got %T", index, item)
			}
			values = append(values, value)
		}
	default:
		return nil, fmt.Errorf("expected string or string array, got %T", raw)
	}
	out := make([]Expression, 0, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("item %d is empty", index)
		}
		out = append(out, Expression{Source: value, Type: TypeRef{Kind: TypeUnknown}})
	}
	return out, nil
}
