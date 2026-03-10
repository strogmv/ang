package effects

import "strings"

type EffectKind string

const (
	EffectPure    EffectKind = ""
	EffectDB      EffectKind = "db"
	EffectAI      EffectKind = "ai"
	EffectStorage EffectKind = "storage"
	EffectSession EffectKind = "session"
	EffectEvents  EffectKind = "events"
	EffectHTTP    EffectKind = "http"
	EffectCache   EffectKind = "cache"
	EffectState   EffectKind = "state"
	EffectConfig  EffectKind = "config"
	EffectTime    EffectKind = "time"
	EffectID      EffectKind = "id"
	EffectCrypto  EffectKind = "crypto"
)

type SafetyTag string

const (
	RequireSessionPresent SafetyTag = "session.present"
	RequireQuotaChecked   SafetyTag = "quota.checked"
	RequireRateChecked    SafetyTag = "ratelimit.checked"
	RequireBudgetChecked  SafetyTag = "budget.checked"
	RequireTxOpen         SafetyTag = "tx.open"
	RequireIdempotencyKey SafetyTag = "idem.key"

	ProduceSessionPresent SafetyTag = "session.present"
	ProduceQuotaChecked   SafetyTag = "quota.checked"
	ProduceRateChecked    SafetyTag = "ratelimit.checked"
	ProduceBudgetChecked  SafetyTag = "budget.checked"
	ProduceTxOpen         SafetyTag = "tx.open"
	ProduceIdempotencyKey SafetyTag = "idem.key"
	ProduceTokensConsumed SafetyTag = "tokens.consumed"
)

type ActionLogos struct {
	Effect       EffectKind
	RequiresTags []SafetyTag
	ProducesTags []SafetyTag
	RequiresVars []string
	ProducesVar  string
	ChildTags    []SafetyTag
	TxCompatible bool
	RequiresTx   bool
}

var Registry = map[string]ActionLogos{
	"session.Get": {
		Effect:       EffectSession,
		ProducesTags: []SafetyTag{ProduceSessionPresent},
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"quota.Check": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireSessionPresent},
		ProducesTags: []SafetyTag{ProduceQuotaChecked},
		TxCompatible: true,
	},
	"ratelimit.Check": {
		Effect:       EffectState,
		ProducesTags: []SafetyTag{ProduceRateChecked},
		TxCompatible: true,
	},
	"budget.Check": {
		Effect:       EffectState,
		ProducesTags: []SafetyTag{ProduceBudgetChecked},
		TxCompatible: true,
	},
	"budget.Consume": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireBudgetChecked},
		ProducesTags: []SafetyTag{ProduceTokensConsumed},
		TxCompatible: true,
	},
	"auth.RequireRole": {
		Effect:       EffectSession,
		RequiresTags: []SafetyTag{RequireSessionPresent},
		TxCompatible: true,
	},
	"auth.CheckRole": {
		Effect:       EffectSession,
		RequiresTags: []SafetyTag{RequireSessionPresent},
		TxCompatible: true,
	},
	"repo.Find": {
		Effect:       EffectDB,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"repo.Get": {
		Effect:       EffectDB,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"repo.List": {
		Effect:       EffectDB,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"repo.Save": {
		Effect:       EffectDB,
		TxCompatible: true,
	},
	"repo.Update": {
		Effect:       EffectDB,
		TxCompatible: true,
	},
	"repo.Delete": {
		Effect:       EffectDB,
		TxCompatible: true,
	},
	"db.Lock": {
		Effect:       EffectDB,
		RequiresTags: []SafetyTag{RequireTxOpen},
		RequiresTx:   true,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"db.SelectForUpdate": {
		Effect:       EffectDB,
		RequiresTags: []SafetyTag{RequireTxOpen},
		RequiresTx:   true,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"db.Insert": {
		Effect:       EffectDB,
		TxCompatible: true,
	},
	"db.Update": {
		Effect:       EffectDB,
		TxCompatible: true,
	},
	"openai.Chat": {
		Effect:       EffectAI,
		RequiresTags: []SafetyTag{RequireQuotaChecked, RequireBudgetChecked},
		ProducesTags: []SafetyTag{ProduceTokensConsumed},
		ProducesVar:  "output",
		TxCompatible: false,
	},
	"openai.Stream": {
		Effect:       EffectAI,
		RequiresTags: []SafetyTag{RequireQuotaChecked, RequireBudgetChecked},
		ProducesTags: []SafetyTag{ProduceTokensConsumed},
		ProducesVar:  "output",
		TxCompatible: false,
	},
	"storage.Upload": {
		Effect:       EffectStorage,
		TxCompatible: false,
	},
	"storage.Download": {
		Effect:       EffectStorage,
		ProducesVar:  "output",
		TxCompatible: false,
	},
	"http.Request": {
		Effect:       EffectHTTP,
		RequiresTags: []SafetyTag{RequireRateChecked},
		ProducesVar:  "output",
		TxCompatible: false,
	},
	"http.Call": {
		Effect:       EffectHTTP,
		RequiresTags: []SafetyTag{RequireRateChecked},
		ProducesVar:  "output",
		TxCompatible: false,
	},
	"event.Publish": {
		Effect:       EffectEvents,
		TxCompatible: true,
	},
	"event.Outbox": {
		Effect:       EffectEvents,
		TxCompatible: true,
	},
	"cache.Get": {
		Effect:       EffectCache,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"cache.Set": {
		Effect:       EffectCache,
		TxCompatible: true,
	},
	"cache.Del": {
		Effect:       EffectCache,
		TxCompatible: true,
	},
	"config.Get": {
		Effect:       EffectConfig,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"model.Resolve": {
		Effect:       EffectConfig,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"stream.Emit": {
		Effect:       EffectPure,
		TxCompatible: false,
	},
	"secret.Get": {
		Effect:       EffectConfig,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"time.Now": {
		Effect:       EffectTime,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"uuid.New": {
		Effect:       EffectID,
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"tx.Block": {
		Effect:       EffectDB,
		ChildTags:    []SafetyTag{ProduceTxOpen},
		TxCompatible: true,
	},
	"approval.Request": {
		Effect:       EffectState,
		ProducesVar:  "approvalId",
		TxCompatible: true,
	},
	"approval.Wait": {
		Effect:       EffectState,
		ProducesVar:  "decision",
		TxCompatible: false,
	},
	"approval.Decide": {
		Effect:       EffectState,
		TxCompatible: true,
	},
	"idem.DeriveKey": {
		Effect:       EffectState,
		ProducesTags: []SafetyTag{ProduceIdempotencyKey},
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"idem.Check": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireIdempotencyKey},
		TxCompatible: true,
	},
	"idem.SaveResult": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireIdempotencyKey},
		TxCompatible: true,
	},
	"idempotency.DeriveKey": {
		Effect:       EffectState,
		ProducesTags: []SafetyTag{ProduceIdempotencyKey},
		ProducesVar:  "output",
		TxCompatible: true,
	},
	"idempotency.Check": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireIdempotencyKey},
		TxCompatible: true,
	},
	"idempotency.SaveResult": {
		Effect:       EffectState,
		RequiresTags: []SafetyTag{RequireIdempotencyKey},
		TxCompatible: true,
	},
	"mapping.Assign": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
	"mapping.Map": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
	"logic.Check": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
	"flow.If": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
	"flow.For": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
	"context.Trim": {
		Effect:       EffectPure,
		TxCompatible: true,
	},
}

func LookupLogos(action string) (ActionLogos, bool) {
	action = strings.TrimSpace(action)
	if logos, ok := Registry[action]; ok {
		return enrichLogos(action, logos), true
	}
	if logos, ok := inferByPrefix(action); ok {
		return enrichLogos(action, logos), true
	}
	return ActionLogos{}, false
}

func enrichLogos(action string, logos ActionLogos) ActionLogos {
	if len(logos.RequiresVars) == 0 {
		logos.RequiresVars = requiredVarArgs(action)
	}
	return logos
}

func requiredVarArgs(action string) []string {
	switch action {
	case "repo.Find", "repo.Get", "repo.GetForUpdate", "repo.Save", "repo.Delete", "repo.List",
		"db.Get", "db.List", "db.Insert", "db.Update", "db.Delete", "db.Lock", "db.SelectForUpdate":
		return []string{"input"}
	case "repo.Query", "db.Query":
		return []string{"input", "args"}
	case "repo.Upsert", "db.Upsert":
		return []string{"input"}
	case "mapping.Assign":
		return []string{"value"}
	case "mapping.Map":
		return []string{"input", "from"}
	case "logic.Check", "flow.If", "flow.While":
		return []string{"condition"}
	case "logic.Call", "service.Call":
		return []string{"args"}
	case "flow.Call":
		return []string{"args"}
	case "flow.Switch":
		return []string{"value"}
	case "flow.For":
		return []string{"each"}
	case "list.Append":
		return []string{"item"}
	case "list.Filter":
		return []string{"from", "condition"}
	case "list.Paginate":
		return []string{"input", "offset", "limit"}
	case "list.Sort":
		return []string{"items"}
	case "list.Enrich":
		return []string{"items", "lookupInput"}
	case "math.Expr":
		return []string{"expr"}
	case "time.Parse":
		return []string{"value", "input"}
	case "str.Normalize", "str.StripMarkdown":
		return []string{"input"}
	case "str.Format":
		return []string{"template", "args"}
	case "str.Concat":
		return []string{"parts"}
	case "enum.Validate":
		return []string{"value"}
	case "auth.RequireRole":
		return []string{"userID", "companyID"}
	case "auth.CheckRole", "rbac.CheckPermission":
		return []string{"user"}
	case "audit.Log":
		return []string{"actor", "company", "event"}
	case "entity.PatchNonZero", "entity.PatchValidated":
		return []string{"target", "from"}
	case "field.CopyNonEmpty":
		return []string{"from", "to"}
	case "cache.Get", "cache.Del", "storage.GetURL", "storage.Delete", "storage.List":
		return []string{"key", "prefix"}
	case "cache.Set":
		return []string{"key", "value"}
	case "storage.Upload":
		return []string{"data", "input", "key"}
	case "storage.Download":
		return []string{"key"}
	case "jwt.Sign":
		return []string{"claims", "secret"}
	case "jwt.Verify":
		return []string{"token", "secret"}
	case "crypto.Hash":
		return []string{"input"}
	case "event.Wait", "event.Match":
		return []string{"timeout", "match", "event"}
	case "event.Publish", "event.Outbox", "event.Broadcast":
		return []string{"payload", "payloadMap"}
	case "notification.Dispatch", "notify.Dispatch":
		return []string{"userID", "entityID", "payload"}
	case "notify.Send":
		return []string{"to", "template", "text", "subject", "html", "data"}
	case "approval.Request":
		return []string{"approvalKey", "title", "requestedBy", "approvers", "policy", "payload", "description", "deadline", "ttl"}
	case "approval.Wait":
		return []string{"approvalId", "timeout"}
	case "approval.Decide":
		return []string{"approvalId", "decision", "actor", "reason"}
	case "policy.Check":
		return []string{"user", "companyID", "status", "code", "throw"}
	case "policy.Evaluate", "policy.Require", "policy.Decide":
		return []string{"policyKey", "subject", "resource", "operation", "tenant", "attrs", "context", "status", "code", "throw"}
	case "exec.Run", "exec.Stream":
		return []string{"cmd", "stdin", "timeout", "args"}
	case "fs.WriteFile":
		return []string{"path", "data"}
	case "fs.ReadFile", "fs.Remove", "archive.ZipDir":
		return []string{"path"}
	case "http.Call", "http.Request", "http.RetryPolicy":
		return []string{"url", "body", "timeout", "auth", "headers", "query"}
	case "http.Paginate":
		return []string{"url", "body", "timeout", "cursor", "next", "headers", "query"}
	case "queue.Enqueue", "dlq.Publish":
		return []string{"subject", "payload", "reason", "timeout"}
	case "queue.Dequeue":
		return []string{"subject", "timeout"}
	case "queue.Ack", "queue.Nack":
		return []string{"subject", "messageID", "reason"}
	case "webhook.Send":
		return []string{"url", "payload", "event"}
	case "webhook.VerifySignature":
		return []string{"payload", "signature", "secret"}
	case "webhook.Ack":
		return []string{"body"}
	case "state.Get":
		return []string{"key", "default"}
	case "state.Set":
		return []string{"key", "value", "ttl"}
	case "state.Delete":
		return []string{"key"}
	case "idem.DeriveKey", "idempotency.DeriveKey":
		return []string{"from", "prefix"}
	case "idem.Check", "idempotency.Check", "idem.SaveResult", "idempotency.SaveResult":
		return []string{"key", "ttl"}
	case "dedupe.Once":
		return []string{"key", "ttl"}
	case "ratelimit.Check", "ratelimit.Limit":
		return []string{"key", "throw"}
	case "quota.Check":
		return []string{"key", "throw"}
	case "budget.Check":
		return []string{"key", "throw"}
	case "budget.Consume":
		return []string{"key", "tokens", "ttl"}
	case "profile.Require":
		return []string{"key", "tier", "throw"}
	case "context.Trim":
		return []string{"input", "strategy"}
	case "openai.Chat", "openai.Stream", "claude.Chat":
		return []string{"system", "system_context", "user_message", "history", "model"}
	case "concurrency.Limit", "concurrency.Run":
		return []string{"key", "throw"}
	case "circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker":
		return []string{"name", "throw", "openTTL"}
	case "bulkhead.Acquire", "bulkhead.Run":
		return []string{"name", "throw"}
	case "time.Format":
		return []string{"input", "format"}
	case "math.Op", "num.Add", "num.Sub", "num.Mul", "num.Div":
		return []string{"a", "b", "value"}
	case "jsonpath.Get", "jsonpath.Set":
		return []string{"input", "path", "value"}
	case "cast.ToString", "json.Parse", "json.Marshal", "regex.Match", "regex.Replace",
		"base64.Encode", "base64.Decode", "url.Parse", "url.Build", "query.Encode", "query.Decode",
		"hash.Sum", "hash.HMAC", "oauth2.Token", "oauth2.Refresh",
		"crypto.Encrypt", "crypto.Decrypt", "secret.Get", "config.Get", "model.Resolve":
		return []string{"input", "key", "path", "url", "value", "pattern", "replacement", "secret"}
	case "map.Build":
		return []string{"from", "key", "value"}
	default:
		return commonReferenceArgs(action)
	}
}

func commonReferenceArgs(action string) []string {
	if action == "" {
		return nil
	}
	return []string{"input", "from", "value", "to", "key", "data", "url", "path", "payload", "token", "claims",
		"subject", "messageID", "reason", "signature", "secret", "actor", "company", "user", "companyID",
		"target", "resource", "operation", "tenant", "attrs", "context", "timeout", "ttl", "deadline",
		"event", "body", "match", "id", "prefix", "a", "b", "name", "tier", "window", "tokens", "strategy",
		"args", "parts", "headers", "query", "payloadMap"}
}

func inferByPrefix(action string) (ActionLogos, bool) {
	switch {
	case strings.HasPrefix(action, "repo."), strings.HasPrefix(action, "db."):
		return ActionLogos{Effect: EffectDB, TxCompatible: true}, true
	case strings.HasPrefix(action, "openai."), strings.HasPrefix(action, "anthropic."), strings.HasPrefix(action, "claude."):
		return ActionLogos{Effect: EffectAI, TxCompatible: false}, true
	case strings.HasPrefix(action, "storage."), strings.HasPrefix(action, "fs."), strings.HasPrefix(action, "archive."), strings.HasPrefix(action, "pdf."), strings.HasPrefix(action, "exec."):
		return ActionLogos{Effect: EffectStorage, TxCompatible: false}, true
	case strings.HasPrefix(action, "session."), strings.HasPrefix(action, "auth."):
		return ActionLogos{Effect: EffectSession, TxCompatible: true}, true
	case strings.HasPrefix(action, "event."), strings.HasPrefix(action, "outbox."), strings.HasPrefix(action, "queue."), strings.HasPrefix(action, "dlq."), strings.HasPrefix(action, "notify."), strings.HasPrefix(action, "notification."):
		return ActionLogos{Effect: EffectEvents, TxCompatible: false}, true
	case strings.HasPrefix(action, "http."), strings.HasPrefix(action, "webhook."), strings.HasPrefix(action, "oauth2."), strings.HasPrefix(action, "mail."), strings.HasPrefix(action, "service."):
		return ActionLogos{Effect: EffectHTTP, TxCompatible: false}, true
	case strings.HasPrefix(action, "cache."):
		return ActionLogos{Effect: EffectCache, TxCompatible: true}, true
	case strings.HasPrefix(action, "state."), strings.HasPrefix(action, "quota."), strings.HasPrefix(action, "budget."), strings.HasPrefix(action, "ratelimit."), strings.HasPrefix(action, "idem."), strings.HasPrefix(action, "idempotency."), strings.HasPrefix(action, "dedupe."), strings.HasPrefix(action, "profile."), strings.HasPrefix(action, "concurrency."), strings.HasPrefix(action, "circuit."), strings.HasPrefix(action, "bulkhead."), strings.HasPrefix(action, "approval."):
		return ActionLogos{Effect: EffectState, TxCompatible: true}, true
	case strings.HasPrefix(action, "config."), strings.HasPrefix(action, "secret."):
		return ActionLogos{Effect: EffectConfig, TxCompatible: true}, true
	case strings.HasPrefix(action, "time."):
		return ActionLogos{Effect: EffectTime, TxCompatible: true}, true
	case strings.HasPrefix(action, "uuid."), strings.HasPrefix(action, "ulid."), strings.HasPrefix(action, "rand."):
		return ActionLogos{Effect: EffectID, TxCompatible: true}, true
	case strings.HasPrefix(action, "hash."), strings.HasPrefix(action, "hmac."), strings.HasPrefix(action, "crypto."), strings.HasPrefix(action, "jwt."):
		return ActionLogos{Effect: EffectCrypto, TxCompatible: true}, true
	case strings.HasPrefix(action, "mapping."), strings.HasPrefix(action, "logic."), strings.HasPrefix(action, "flow."), strings.HasPrefix(action, "field."), strings.HasPrefix(action, "entity."), strings.HasPrefix(action, "enum."), strings.HasPrefix(action, "list."), strings.HasPrefix(action, "map."), strings.HasPrefix(action, "str."), strings.HasPrefix(action, "cast."), strings.HasPrefix(action, "convert."), strings.HasPrefix(action, "num."), strings.HasPrefix(action, "math."), strings.HasPrefix(action, "regex."), strings.HasPrefix(action, "base64."), strings.HasPrefix(action, "url."), strings.HasPrefix(action, "query."), strings.HasPrefix(action, "json."), strings.HasPrefix(action, "jsonpath."), strings.HasPrefix(action, "parallel."), strings.HasPrefix(action, "batch."), strings.HasPrefix(action, "audit."), strings.HasPrefix(action, "rbac."), strings.HasPrefix(action, "policy."), strings.HasPrefix(action, "trace."), strings.HasPrefix(action, "metric."), strings.HasPrefix(action, "log."), strings.HasPrefix(action, "slo."), strings.HasPrefix(action, "fsm."), strings.HasPrefix(action, "plan."), strings.HasPrefix(action, "cue."):
		return ActionLogos{Effect: EffectPure, TxCompatible: true}, true
	default:
		return ActionLogos{}, false
	}
}
