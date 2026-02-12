package compiler

import (
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// AttachNotificationInfra copies parsed notification infra blocks into IR.
// This keeps runtime generation language-agnostic while preserving cue/infra intent.
func AttachNotificationInfra(schema *ir.Schema, channels *normalizer.NotificationChannelsDef, policies *normalizer.NotificationPoliciesDef) {
	if schema == nil {
		return
	}
	if channels == nil && policies == nil {
		return
	}
	if schema.Notifications == nil {
		schema.Notifications = &ir.NotificationsConfig{}
	}

	if channels != nil {
		out := &ir.NotificationChannels{
			Enabled:         channels.Enabled,
			DefaultChannels: append([]string(nil), channels.DefaultChannels...),
			Channels:        make(map[string]ir.NotificationChannelSpec, len(channels.Channels)),
		}
		for name, cfg := range channels.Channels {
			out.Channels[name] = ir.NotificationChannelSpec{
				Enabled:    cfg.Enabled,
				Driver:     cfg.Driver,
				Topic:      cfg.Topic,
				Subject:    cfg.Subject,
				Template:   cfg.Template,
				DSNEnv:     cfg.DSNEnv,
				BrokersEnv: cfg.BrokersEnv,
			}
		}
		schema.Notifications.Channels = out
	}

	if policies != nil {
		out := &ir.NotificationPolicies{
			Enabled: policies.Enabled,
			Rules:   make([]ir.NotificationPolicyRule, 0, len(policies.Rules)),
		}
		for _, r := range policies.Rules {
			out.Rules = append(out.Rules, ir.NotificationPolicyRule{
				Enabled:  r.Enabled,
				Event:    r.Event,
				Type:     r.Type,
				Audience: r.Audience,
				Channels: append([]string(nil), r.Channels...),
				Template: r.Template,
				MuteKey:  r.MuteKey,
			})
		}
		schema.Notifications.Policies = out
	}
}
