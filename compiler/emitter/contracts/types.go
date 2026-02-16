package contracts

import "github.com/strogmv/ang/compiler/normalizer"

// Aliases expose normalizer DTOs through a dedicated emitter contract package.
// Subpackages should depend on these aliases instead of importing normalizer directly.
type (
	ConfigDef        = normalizer.ConfigDef
	AuthDef          = normalizer.AuthDef
	RBACDef          = normalizer.RBACDef
	ScenarioDef      = normalizer.ScenarioDef
	EndpointDef      = normalizer.Endpoint
	EmailTemplateDef = normalizer.EmailTemplateDef
	ProjectDef       = normalizer.ProjectDef
	NotificationMute = normalizer.NotificationMutingDef
	InfraResolved    = normalizer.InfraResolvedStep
)

const (
	InfraKeyAuth               = normalizer.InfraKeyAuth
	InfraKeyNotificationMuting = normalizer.InfraKeyNotificationMuting
)
