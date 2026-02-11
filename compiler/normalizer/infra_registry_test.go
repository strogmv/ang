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
	for _, d := range defs {
		if d.Key != InfraKeyNotificationMuting {
			continue
		}
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
	}
	if !foundMuting {
		t.Fatalf("notification muting definition not found")
	}
}
