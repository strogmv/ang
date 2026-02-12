package normalizer

import (
	"reflect"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestInfraRegistryExtractAll(t *testing.T) {
	t.Parallel()

	val := cuecontext.New().CompileString(`
		#AppConfig: {
			port: int
		}

		#Auth: {
			service: "auth_service"
			jwt: {
				alg: "HS256"
				tokens: {
					store: "redis"
				}
			}
		}

		#NotificationMuting: {
			enabled: true
			userEntity: "Account"
		}

		#NotificationChannels: {
			enabled: true
			defaultChannels: ["in_app", "email"]
			channels: {
				email: {
					enabled: true
					driver: "smtp"
					template: "default"
				}
				nats: {
					enabled: true
					subject: "notifications.>"
				}
			}
		}

		#NotificationPolicies: {
			enabled: true
			rules: [
				{
					event: "TenderCreated"
					type: "company_tender_created"
					audience: "subscribers"
					channels: ["in_app", "email"]
					template: "tender_created"
					muteKey: "company_tender_created"
				},
			]
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	reg := NewInfraRegistry()
	out, err := reg.ExtractAll(New(), val)
	if err != nil {
		t.Fatalf("extract all infra definitions: %v", err)
	}

	cfg, ok := out[InfraKeyConfig].(*ConfigDef)
	if !ok || cfg == nil {
		t.Fatalf("expected %q to be parsed into *ConfigDef", InfraKeyConfig)
	}
	if len(cfg.Fields) == 0 {
		t.Fatalf("expected app config fields to be parsed")
	}

	auth, ok := out[InfraKeyAuth].(*AuthDef)
	if !ok || auth == nil {
		t.Fatalf("expected %q to be parsed into *AuthDef", InfraKeyAuth)
	}
	if auth.Alg != "HS256" {
		t.Fatalf("expected auth alg HS256, got %q", auth.Alg)
	}

	muting, ok := out[InfraKeyNotificationMuting].(*NotificationMutingDef)
	if !ok || muting == nil {
		t.Fatalf("expected %q to be parsed into *NotificationMutingDef", InfraKeyNotificationMuting)
	}
	if !muting.Enabled {
		t.Fatalf("expected notification muting to be enabled")
	}
	if muting.UserEntity != "Account" {
		t.Fatalf("expected userEntity Account, got %q", muting.UserEntity)
	}

	channels, ok := out[InfraKeyNotificationChannels].(*NotificationChannelsDef)
	if !ok || channels == nil {
		t.Fatalf("expected %q to be parsed into *NotificationChannelsDef", InfraKeyNotificationChannels)
	}
	if !channels.Enabled {
		t.Fatalf("expected notification channels to be enabled")
	}
	if len(channels.DefaultChannels) != 2 || channels.DefaultChannels[0] != "in_app" || channels.DefaultChannels[1] != "email" {
		t.Fatalf("unexpected default channels: %#v", channels.DefaultChannels)
	}
	if channels.Channels["email"].Driver != "smtp" {
		t.Fatalf("expected email driver smtp, got %q", channels.Channels["email"].Driver)
	}
	if channels.Channels["nats"].Subject != "notifications.>" {
		t.Fatalf("expected nats subject notifications.>, got %q", channels.Channels["nats"].Subject)
	}

	policies, ok := out[InfraKeyNotificationPolicies].(*NotificationPoliciesDef)
	if !ok || policies == nil {
		t.Fatalf("expected %q to be parsed into *NotificationPoliciesDef", InfraKeyNotificationPolicies)
	}
	if !policies.Enabled {
		t.Fatalf("expected notification policies to be enabled")
	}
	if len(policies.Rules) != 1 {
		t.Fatalf("expected one policy rule, got %d", len(policies.Rules))
	}
	rule := policies.Rules[0]
	if rule.Event != "TenderCreated" || rule.Type != "company_tender_created" {
		t.Fatalf("unexpected policy rule identity: %+v", rule)
	}
	if len(rule.Channels) != 2 || rule.Channels[0] != "in_app" || rule.Channels[1] != "email" {
		t.Fatalf("unexpected policy channels: %#v", rule.Channels)
	}
	if rule.MuteKey != "company_tender_created" {
		t.Fatalf("expected muteKey company_tender_created, got %q", rule.MuteKey)
	}

	patch := reg.BuildContextPatch(out)
	if patch.AuthRefreshStore == "" {
		t.Fatalf("expected auth refresh store in context patch")
	}
	if patch.AuthService != "AuthService" {
		t.Fatalf("expected normalized auth service name, got %q", patch.AuthService)
	}
	if !patch.NotificationMuting {
		t.Fatalf("expected notification muting context hook to enable decorator")
	}

	stepsGo := reg.StepsForValues(InfraLanguageGo, out)
	if len(stepsGo) != 1 || stepsGo[0].Key != InfraKeyNotificationMuting {
		t.Fatalf("expected go infra step for %q", InfraKeyNotificationMuting)
	}
	stepsPy := reg.StepsForValues(InfraLanguagePython, out)
	if len(stepsPy) != 1 || stepsPy[0].Key != InfraKeyAuth {
		t.Fatalf("expected python infra step for %q", InfraKeyAuth)
	}
}

func TestInfraRegistryMetadata(t *testing.T) {
	t.Parallel()

	reg := NewInfraRegistry()
	defs := reg.defs
	if len(defs) < 3 {
		t.Fatalf("expected infra defs to be registered")
	}

	var foundMuting bool
	var foundChannels bool
	var foundPolicies bool
	for _, d := range defs {
		if d.Key == InfraKeyNotificationMuting {
			foundMuting = true
			if d.CUEPath != "#NotificationMuting" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Template != "notification_muting.tmpl" {
				t.Fatalf("unexpected template %q", d.Template)
			}
			if d.Type != reflect.TypeOf(NotificationMutingDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
			continue
		}
		if d.Key == InfraKeyNotificationChannels {
			foundChannels = true
			if d.CUEPath != "#NotificationChannels" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Template != "notification_channels" {
				t.Fatalf("unexpected template %q", d.Template)
			}
			if d.Type != reflect.TypeOf(NotificationChannelsDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
			continue
		}
		if d.Key == InfraKeyNotificationPolicies {
			foundPolicies = true
			if d.CUEPath != "#NotificationPolicies" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Template != "notification_policies" {
				t.Fatalf("unexpected template %q", d.Template)
			}
			if d.Type != reflect.TypeOf(NotificationPoliciesDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
		}
	}
	if !foundMuting {
		t.Fatalf("notification muting definition not found")
	}
	if !foundChannels {
		t.Fatalf("notification channels definition not found")
	}
	if !foundPolicies {
		t.Fatalf("notification policies definition not found")
	}
}
