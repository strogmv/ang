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
		ProducesTags: []SafetyTag{ProduceTokensConsumed},
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
	if logos, ok := Registry[strings.TrimSpace(action)]; ok {
		return logos, true
	}
	if logos, ok := inferByPrefix(action); ok {
		return logos, true
	}
	return ActionLogos{}, false
}

func inferByPrefix(action string) (ActionLogos, bool) {
	switch {
	case strings.HasPrefix(action, "repo."), strings.HasPrefix(action, "db."):
		return ActionLogos{Effect: EffectDB, TxCompatible: true}, true
	case strings.HasPrefix(action, "openai."), strings.HasPrefix(action, "anthropic."), strings.HasPrefix(action, "claude."):
		return ActionLogos{Effect: EffectAI, TxCompatible: false}, true
	case strings.HasPrefix(action, "storage."):
		return ActionLogos{Effect: EffectStorage, TxCompatible: false}, true
	case strings.HasPrefix(action, "session."):
		return ActionLogos{Effect: EffectSession, TxCompatible: true}, true
	case strings.HasPrefix(action, "event."), strings.HasPrefix(action, "outbox."):
		return ActionLogos{Effect: EffectEvents, TxCompatible: true}, true
	case strings.HasPrefix(action, "http."):
		return ActionLogos{Effect: EffectHTTP, TxCompatible: false}, true
	case strings.HasPrefix(action, "cache."):
		return ActionLogos{Effect: EffectCache, TxCompatible: true}, true
	case strings.HasPrefix(action, "state."), strings.HasPrefix(action, "quota."), strings.HasPrefix(action, "budget."), strings.HasPrefix(action, "ratelimit."):
		return ActionLogos{Effect: EffectState, TxCompatible: true}, true
	case strings.HasPrefix(action, "config."):
		return ActionLogos{Effect: EffectConfig, TxCompatible: true}, true
	case strings.HasPrefix(action, "time."):
		return ActionLogos{Effect: EffectTime, TxCompatible: true}, true
	case strings.HasPrefix(action, "uuid."), strings.HasPrefix(action, "ulid."):
		return ActionLogos{Effect: EffectID, TxCompatible: true}, true
	case strings.HasPrefix(action, "hash."), strings.HasPrefix(action, "hmac."), strings.HasPrefix(action, "crypto."):
		return ActionLogos{Effect: EffectCrypto, TxCompatible: true}, true
	case strings.HasPrefix(action, "mapping."), strings.HasPrefix(action, "logic."), strings.HasPrefix(action, "flow."):
		return ActionLogos{Effect: EffectPure, TxCompatible: true}, true
	default:
		return ActionLogos{}, false
	}
}
