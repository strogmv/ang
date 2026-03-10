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

		Handlers: {
			db:      {driver: "postgres"}
			ai:      {provider: "openai"}
			storage: {driver: "s3", bucket: "${S3_BUCKET}"}
			session: {driver: "cookie"}
			events:  {driver: "nats"}
			http:    {driver: "default"}
			cache:   {driver: "redis"}
			state:   {driver: "redis"}
		}

		TestHandlers: {
			db:      {driver: "stub"}
			ai:      {provider: "mock"}
			storage: {driver: "memory"}
			session: {driver: "memory"}
			events:  {driver: "memory"}
			http:    {driver: "mock"}
			cache:   {driver: "memory"}
			state:   {driver: "memory"}
		}

		Middleware: {
			db: [
				{type: "trace", level: "debug"},
				{type: "metrics", level: "info"},
			]
			ai: [
				{type: "retry", attempts: 3, backoff: "500ms", on: [429, 503]},
				{type: "timeout", duration: "30s"},
				{type: "log", level: "info"},
			]
			http: [
				{type: "retry", attempts: 2, backoff: "200ms"},
				{type: "timeout", duration: "10s"},
			]
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

	handlers, ok := out[InfraKeyEffectHandlers].(*EffectHandlersDef)
	if !ok || handlers == nil {
		t.Fatalf("expected %q to be parsed into *EffectHandlersDef", InfraKeyEffectHandlers)
	}
	if got := handlers.Bindings["db"].Driver; got != "postgres" {
		t.Fatalf("expected db driver postgres, got %q", got)
	}
	if got := handlers.Bindings["ai"].Provider; got != "openai" {
		t.Fatalf("expected ai provider openai, got %q", got)
	}
	if got := handlers.Bindings["storage"].Options["bucket"]; got != "${S3_BUCKET}" {
		t.Fatalf("expected storage bucket option, got %#v", got)
	}

	testHandlers, ok := out[InfraKeyEffectTestHandlers].(*EffectHandlersDef)
	if !ok || testHandlers == nil {
		t.Fatalf("expected %q to be parsed into *EffectHandlersDef", InfraKeyEffectTestHandlers)
	}
	if got := testHandlers.Bindings["db"].Driver; got != "stub" {
		t.Fatalf("expected test db driver stub, got %q", got)
	}
	if got := testHandlers.Bindings["ai"].Provider; got != "mock" {
		t.Fatalf("expected test ai provider mock, got %q", got)
	}

	middleware, ok := out[InfraKeyEffectMiddleware].(*EffectMiddlewareCatalogDef)
	if !ok || middleware == nil {
		t.Fatalf("expected %q to be parsed into *EffectMiddlewareCatalogDef", InfraKeyEffectMiddleware)
	}
	if got := len(middleware.Chains["ai"]); got != 3 {
		t.Fatalf("expected 3 ai middleware entries, got %d", got)
	}
	if got := middleware.Chains["ai"][0].Attempts; got != 3 {
		t.Fatalf("expected ai retry attempts 3, got %d", got)
	}
	if got := middleware.Chains["ai"][0].On; len(got) != 2 || got[0] != 429 || got[1] != 503 {
		t.Fatalf("unexpected ai retry status list: %#v", got)
	}
	if got := middleware.Chains["http"][1].Duration; got != "10s" {
		t.Fatalf("expected http timeout 10s, got %q", got)
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
	var foundHandlers bool
	var foundTestHandlers bool
	var foundMiddleware bool
	for _, d := range defs {
		if d.Key == InfraKeyEffectHandlers {
			foundHandlers = true
			if d.CUEPath != "Handlers" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Type != reflect.TypeOf(EffectHandlersDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
			continue
		}
		if d.Key == InfraKeyEffectTestHandlers {
			foundTestHandlers = true
			if d.CUEPath != "TestHandlers" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Type != reflect.TypeOf(EffectHandlersDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
			continue
		}
		if d.Key == InfraKeyEffectMiddleware {
			foundMiddleware = true
			if d.CUEPath != "Middleware" {
				t.Fatalf("unexpected CUEPath %q", d.CUEPath)
			}
			if d.Type != reflect.TypeOf(EffectMiddlewareCatalogDef{}) {
				t.Fatalf("unexpected type %v", d.Type)
			}
			continue
		}
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
	if !foundHandlers {
		t.Fatalf("effect handlers definition not found")
	}
	if !foundTestHandlers {
		t.Fatalf("effect test handlers definition not found")
	}
	if !foundMiddleware {
		t.Fatalf("effect middleware definition not found")
	}
	if !foundChannels {
		t.Fatalf("notification channels definition not found")
	}
	if !foundPolicies {
		t.Fatalf("notification policies definition not found")
	}
}
