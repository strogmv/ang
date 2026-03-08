package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractNotificationMuting(val cue.Value) (*NotificationMutingDef, error) {
	mutingVal := val.LookupPath(cue.ParsePath("#NotificationMuting"))
	if !mutingVal.Exists() {
		return nil, nil
	}

	enabled, _ := mutingVal.LookupPath(cue.ParsePath("enabled")).Bool()
	if !enabled {
		return nil, nil
	}

	userEntity := getString(mutingVal, "userEntity")
	if userEntity == "" {
		userEntity = "User"
	}
	muteAllField := getString(mutingVal, "muteAllField")
	if muteAllField == "" {
		muteAllField = "notificationMuteAll"
	}
	mutedTypesField := getString(mutingVal, "mutedTypesField")
	if mutedTypesField == "" {
		mutedTypesField = "notificationMutedTypes"
	}

	return &NotificationMutingDef{
		Enabled:         true,
		UserEntity:      userEntity,
		MuteAllField:    muteAllField,
		MutedTypesField: mutedTypesField,
	}, nil
}

// ExtractNotificationChannels parses #NotificationChannels from infra.
func (n *Normalizer) ExtractNotificationChannels(val cue.Value) (*NotificationChannelsDef, error) {
	v := val.LookupPath(cue.ParsePath("#NotificationChannels"))
	if !v.Exists() {
		return nil, nil
	}

	enabled := true
	if b, err := v.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
		enabled = b
	}
	if !enabled {
		return nil, nil
	}

	def := &NotificationChannelsDef{
		Enabled:         true,
		DefaultChannels: []string{"in_app"},
		Channels:        map[string]NotificationChannelConfig{},
	}

	if listVal := v.LookupPath(cue.ParsePath("defaultChannels")); listVal.Exists() {
		if it, err := listVal.List(); err == nil {
			parsed := make([]string, 0)
			for it.Next() {
				s, _ := it.Value().String()
				s = strings.TrimSpace(strings.Trim(s, "\""))
				if s != "" {
					parsed = append(parsed, s)
				}
			}
			if len(parsed) > 0 {
				def.DefaultChannels = parsed
			}
		}
	}

	channelsVal := v.LookupPath(cue.ParsePath("channels"))
	if channelsVal.Exists() {
		iter, _ := channelsVal.Fields()
		for iter.Next() {
			name := strings.TrimSpace(iter.Selector().String())
			if name == "" {
				continue
			}
			cv := iter.Value()
			cfg := NotificationChannelConfig{
				Enabled: true,
				Driver:  name,
			}
			if b, err := cv.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
				cfg.Enabled = b
			}
			if s := strings.TrimSpace(getString(cv, "driver")); s != "" {
				cfg.Driver = s
			}
			cfg.Topic = strings.TrimSpace(getString(cv, "topic"))
			cfg.Subject = strings.TrimSpace(getString(cv, "subject"))
			cfg.Template = strings.TrimSpace(getString(cv, "template"))
			cfg.DSNEnv = strings.TrimSpace(getString(cv, "dsnEnv"))
			cfg.BrokersEnv = strings.TrimSpace(getString(cv, "brokersEnv"))
			def.Channels[name] = cfg
		}
	}

	if len(def.Channels) == 0 {
		def.Channels["in_app"] = NotificationChannelConfig{Enabled: true, Driver: "in_app"}
	}

	return def, nil
}

func parseStringList(val cue.Value) []string {
	if !val.Exists() {
		return nil
	}
	it, err := val.List()
	if err != nil {
		return nil
	}
	out := make([]string, 0)
	for it.Next() {
		s, _ := it.Value().String()
		s = strings.TrimSpace(strings.Trim(s, "\""))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ExtractNotificationPolicies parses #NotificationPolicies from infra.
func (n *Normalizer) ExtractNotificationPolicies(val cue.Value) (*NotificationPoliciesDef, error) {
	v := val.LookupPath(cue.ParsePath("#NotificationPolicies"))
	if !v.Exists() {
		return nil, nil
	}

	enabled := true
	if b, err := v.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
		enabled = b
	}
	if !enabled {
		return nil, nil
	}

	def := &NotificationPoliciesDef{
		Enabled: true,
		Rules:   []NotificationPolicyRule{},
	}

	rulesVal := v.LookupPath(cue.ParsePath("rules"))
	if !rulesVal.Exists() {
		return def, nil
	}

	list, err := rulesVal.List()
	if err != nil {
		return def, nil
	}
	for list.Next() {
		rv := list.Value()
		rule := NotificationPolicyRule{
			Enabled:  true,
			Event:    strings.TrimSpace(getString(rv, "event")),
			Type:     strings.TrimSpace(getString(rv, "type")),
			Audience: strings.TrimSpace(getString(rv, "audience")),
			Channels: parseStringList(rv.LookupPath(cue.ParsePath("channels"))),
			Template: strings.TrimSpace(getString(rv, "template")),
			MuteKey:  strings.TrimSpace(getString(rv, "muteKey")),
		}
		if b, err := rv.LookupPath(cue.ParsePath("enabled")).Bool(); err == nil {
			rule.Enabled = b
		}
		def.Rules = append(def.Rules, rule)
	}

	return def, nil
}

// ExtractRepositories extracts repository definitions.
