package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServices_OperationKindAndSideEffects(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		CreateUser: {
			service: "auth"
			primary_operation_kind: "create"
			capabilities: ["auth", "profile", "notify"]
			side_effects: [
				{kind: "send_email", channel: "email", template: "welcome_email"},
				{kind: "publish_event", event: "UserCreated"},
				{kind: "upload_file", target_field: "avatarURL"},
			]
			manual_required: false
			input: {
				email: string
			}
			output: {
				id: string
			}
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services, err := New().ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}
	if len(services) != 1 || len(services[0].Methods) != 1 {
		t.Fatalf("expected one service with one method, got %#v", services)
	}

	method := services[0].Methods[0]
	if method.PrimaryOperationKind != OperationKindCreate {
		t.Fatalf("PrimaryOperationKind=%q, want %q", method.PrimaryOperationKind, OperationKindCreate)
	}
	if len(method.Capabilities) != 3 {
		t.Fatalf("len(Capabilities)=%d, want 3", len(method.Capabilities))
	}
	if method.Capabilities[0] != CapabilityAuth || method.Capabilities[2] != CapabilityNotify {
		t.Fatalf("unexpected capabilities: %#v", method.Capabilities)
	}
	if len(method.SideEffects) != 3 {
		t.Fatalf("len(SideEffects)=%d, want 3", len(method.SideEffects))
	}
	if method.SideEffects[0].Kind != "notify.email" || method.SideEffects[0].Template != "welcome_email" {
		t.Fatalf("unexpected first side effect: %#v", method.SideEffects[0])
	}
	if method.SideEffects[2].TargetField != "photoURL" {
		t.Fatalf("TargetField=%q, want photoURL", method.SideEffects[2].TargetField)
	}
	if method.ManualRequired {
		t.Fatalf("ManualRequired=%v, want false", method.ManualRequired)
	}
}

func TestExtractServices_CanonicalizesNotifyEmailSideEffects(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		SendEmailToUser: {
			service: "auth"
			primary_operation_kind: "notify"
			side_effects: [
				{kind: "notify.email", channel: "email", template: "generic_email"},
				"send email",
			]
			input: {
				userID: string
				subject: string
				message: string
			}
			output: {
				notificationID: string
			}
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services, err := New().ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}
	method := services[0].Methods[0]
	if method.PrimaryOperationKind != OperationKindNotify {
		t.Fatalf("PrimaryOperationKind=%q, want %q", method.PrimaryOperationKind, OperationKindNotify)
	}
	if len(method.SideEffects) != 2 {
		t.Fatalf("len(SideEffects)=%d, want 2", len(method.SideEffects))
	}
	for i, effect := range method.SideEffects {
		if effect.Kind != "notify.email" {
			t.Fatalf("SideEffects[%d].Kind=%q, want notify.email", i, effect.Kind)
		}
	}
}

func TestExtractServices_NormalizesLegacyGetOneAndManualRequired(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		GetUser: {
			service: "auth"
			primary_operation_kind: "get_one"
			manual_required: true
			input: {
				id: string
			}
			output: {
				id: string
			}
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services, err := New().ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}
	method := services[0].Methods[0]
	if method.PrimaryOperationKind != OperationKindGet {
		t.Fatalf("PrimaryOperationKind=%q, want %q", method.PrimaryOperationKind, OperationKindGet)
	}
	if !method.ManualRequired {
		t.Fatalf("ManualRequired=%v, want true", method.ManualRequired)
	}
}
