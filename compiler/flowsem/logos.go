package flowsem

import sharedeffects "github.com/strogmv/ang/compiler/effects"

type EffectKind = sharedeffects.EffectKind
type SafetyTag = sharedeffects.SafetyTag
type ActionLogos = sharedeffects.ActionLogos

const (
	EffectPure    = sharedeffects.EffectPure
	EffectDB      = sharedeffects.EffectDB
	EffectAI      = sharedeffects.EffectAI
	EffectStorage = sharedeffects.EffectStorage
	EffectSession = sharedeffects.EffectSession
	EffectEvents  = sharedeffects.EffectEvents
	EffectHTTP    = sharedeffects.EffectHTTP
	EffectCache   = sharedeffects.EffectCache
	EffectState   = sharedeffects.EffectState
	EffectConfig  = sharedeffects.EffectConfig
	EffectTime    = sharedeffects.EffectTime
	EffectID      = sharedeffects.EffectID
	EffectCrypto  = sharedeffects.EffectCrypto

	RequireSessionPresent = sharedeffects.RequireSessionPresent
	RequireQuotaChecked   = sharedeffects.RequireQuotaChecked
	RequireRateChecked    = sharedeffects.RequireRateChecked
	RequireBudgetChecked  = sharedeffects.RequireBudgetChecked
	RequireTxOpen         = sharedeffects.RequireTxOpen
	RequireIdempotencyKey = sharedeffects.RequireIdempotencyKey

	ProduceSessionPresent = sharedeffects.ProduceSessionPresent
	ProduceQuotaChecked   = sharedeffects.ProduceQuotaChecked
	ProduceRateChecked    = sharedeffects.ProduceRateChecked
	ProduceBudgetChecked  = sharedeffects.ProduceBudgetChecked
	ProduceTxOpen         = sharedeffects.ProduceTxOpen
	ProduceIdempotencyKey = sharedeffects.ProduceIdempotencyKey
	ProduceTokensConsumed = sharedeffects.ProduceTokensConsumed
)

var Registry = sharedeffects.Registry

func LookupLogos(action string) (ActionLogos, bool) {
	return sharedeffects.LookupLogos(action)
}
