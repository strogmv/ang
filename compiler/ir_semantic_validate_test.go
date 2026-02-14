package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestValidateIRSemantics_OK(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{
				Name: "User",
				Fields: []ir.Field{
					{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
					{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}},
				},
			},
		},
		Services: []ir.Service{
			{
				Name: "Users",
				Methods: []ir.Method{
					{Name: "GetUser"},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/users/{id}", Service: "Users", RPC: "GetUser"},
		},
		Repos: []ir.Repository{
			{
				Name:   "UserRepository",
				Entity: "User",
				Finders: []ir.Finder{
					{
						Name: "FindByEmail",
						Where: []ir.WhereClause{
							{Field: "email", Param: "email", ParamType: "string"},
						},
						Returns: "one",
					},
				},
			},
		},
	}

	if err := ValidateIRSemantics(schema); err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownRPC(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Services: []ir.Service{
			{Name: "Users", Methods: []ir.Method{{Name: "GetUser"}}},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/users/{id}", Service: "Users", RPC: "UnknownRPC"},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "unknown RPC") {
		t.Fatalf("expected unknown RPC error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownFinderField(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{Name: "User", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
		},
		Repos: []ir.Repository{
			{
				Name:   "UserRepository",
				Entity: "User",
				Finders: []ir.Finder{
					{
						Name: "FindByEmail",
						Where: []ir.WhereClause{
							{Field: "email", Param: "email", ParamType: "string"},
						},
					},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "where field") {
		t.Fatalf("expected unknown finder field error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownEntityTypeRef(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{
				Name: "Order",
				Fields: []ir.Field{
					{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
					{Name: "user", Type: ir.TypeRef{Kind: ir.KindEntity, Name: "User"}},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "unknown entity type") {
		t.Fatalf("expected unknown entity type error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnServiceCycle(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Services: []ir.Service{
			{Name: "A", Uses: []string{"B"}},
			{Name: "B", Uses: []string{"A"}},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownNotificationTemplate(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "known_email"},
		},
		Notifications: &ir.NotificationsConfig{
			Channels: &ir.NotificationChannels{
				Channels: map[string]ir.NotificationChannelSpec{
					"email": {Enabled: true, Template: "missing_template"},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "unknown template") {
		t.Fatalf("expected unknown template error, got: %v", err)
	}
}

func TestValidateIRSemantics_OKWithKnownNotificationTemplates(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "known_email", Channel: "email", Subject: "s", Text: "t"},
			{ID: "known_in_app", Channel: "in_app", Body: "b"},
		},
		Notifications: &ir.NotificationsConfig{
			Channels: &ir.NotificationChannels{
				Channels: map[string]ir.NotificationChannelSpec{
					"email": {Enabled: true, Template: "known_email"},
				},
			},
			Policies: &ir.NotificationPolicies{
				Rules: []ir.NotificationPolicyRule{
					{Enabled: true, Template: "known_in_app"},
				},
			},
		},
	}
	if err := ValidateIRSemantics(schema); err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnNotificationTemplateChannelMismatch_ChannelSpec(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "email_tpl", Channel: "email"},
		},
		Notifications: &ir.NotificationsConfig{
			Channels: &ir.NotificationChannels{
				Channels: map[string]ir.NotificationChannelSpec{
					"in_app": {Enabled: true, Template: "email_tpl"},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "incompatible channel") {
		t.Fatalf("expected incompatible channel error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnNotificationTemplateChannelMismatch_PolicyRule(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "email_tpl", Channel: "email"},
		},
		Notifications: &ir.NotificationsConfig{
			Policies: &ir.NotificationPolicies{
				Rules: []ir.NotificationPolicyRule{
					{Enabled: true, Channels: []string{"nats"}, Template: "email_tpl"},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "incompatible channel") {
		t.Fatalf("expected incompatible channel error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnTemplateCatalogRules(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "email_bad_engine", Channel: "email", Engine: "json", Subject: "s", Body: "b"},
			{ID: "email_no_subject", Channel: "email", Engine: "go_template", Text: "hello"},
			{ID: "dup", Channel: "in_app", Engine: "plain", Body: "x"},
			{ID: "dup", Channel: "in_app", Engine: "plain", Body: "y"},
			{ID: "unsupported_engine", Channel: "nats", Engine: "liquid", Body: "{}"},
		},
	}

	err := ValidateIRSemantics(schema)
	if err == nil {
		t.Fatalf("expected validation errors, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"incompatible with channel",
		"requires non-empty subject",
		"is duplicated",
		"unsupported engine",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	}
}

func TestValidateIRSemantics_OKTemplateCatalogRules(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "email_ok", Channel: "email", Engine: "go_template", Subject: "s", Text: "t"},
			{ID: "in_app_ok", Channel: "in_app", Engine: "plain", Body: "b"},
			{ID: "nats_ok", Channel: "nats", Engine: "json", Body: `{"ok":true}`},
		},
	}
	if err := ValidateIRSemantics(schema); err != nil {
		t.Fatalf("expected valid template catalog, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnTemplateRequiredVarsRules(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{ID: "dup_var", Channel: "email", Engine: "go_template", Subject: "ok", Text: "{{.temporary_token}}", RequiredVars: []string{"temporary_token", "temporary_token"}},
			{ID: "invalid_var", Channel: "email", Engine: "go_template", Subject: "ok", Text: "{{.temporary_token}}", RequiredVars: []string{"temporary-token"}},
			{ID: "missing_usage", Channel: "email", Engine: "go_template", Subject: "ok", Text: "hello", RequiredVars: []string{"temporary_token"}},
			{ID: "overlap", Channel: "email", Engine: "go_template", Subject: "ok", Text: "{{.temporary_token}}", RequiredVars: []string{"temporary_token"}, OptionalVars: []string{"temporary_token"}},
		},
	}

	err := ValidateIRSemantics(schema)
	if err == nil {
		t.Fatalf("expected validation errors, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"duplicate requiredVars",
		"invalid requiredVars name",
		"does not reference",
		"both requiredVars and optionalVars",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	}
}

func TestValidateIRSemantics_FailsOnUIHintRules(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{
				Name: "User",
				Fields: []ir.Field{
					{
						Name:     "theme",
						Type:     ir.TypeRef{Kind: ir.KindString},
						Optional: true,
						UI: ir.FieldUI{
							Type:       "super-custom",
							Importance: "urgent",
							InputKind:  "secret_sauce",
							Intent:     "critical",
							Density:    "ultra",
							LabelMode:  "labeless",
							Surface:    "glass",
						},
					},
					{
						Name:     "avatar",
						Type:     ir.TypeRef{Kind: ir.KindString},
						Optional: true,
						UI: ir.FieldUI{
							Type: "custom",
						},
					},
					{
						Name:     "role",
						Type:     ir.TypeRef{Kind: ir.KindString},
						Optional: true,
						UI: ir.FieldUI{
							Type: "select",
						},
					},
					{
						Name:     "secret",
						Type:     ir.TypeRef{Kind: ir.KindString},
						Optional: false,
						UI: ir.FieldUI{
							Type:   "text",
							Hidden: true,
						},
					},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil {
		t.Fatalf("expected ui hint validation errors, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"[E_UI_UNKNOWN_TYPE]",
		"[E_UI_CUSTOM_COMPONENT_REQUIRED]",
		"[E_UI_SELECT_SOURCE_OR_OPTIONS_REQUIRED]",
		"[E_UI_HIDDEN_REQUIRED_CONFLICT]",
		"[E_UI_IMPORTANCE_INVALID]",
		"[E_UI_INPUT_KIND_INVALID]",
		"[E_UI_INTENT_INVALID]",
		"[E_UI_DENSITY_INVALID]",
		"[E_UI_LABEL_MODE_INVALID]",
		"[E_UI_SURFACE_INVALID]",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	}
}

func TestValidateIRSemantics_OKTemplateRequiredVarsRules(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Templates: []ir.Template{
			{
				ID:           "ok",
				Channel:      "email",
				Engine:       "go_template",
				Subject:      "Token {{.temporary_token}}",
				Text:         "TTL {{.ttl_minutes}}",
				RequiredVars: []string{"temporary_token", "ttl_minutes"},
				OptionalVars: []string{"company_name"},
			},
		},
	}
	if err := ValidateIRSemantics(schema); err != nil {
		t.Fatalf("expected valid required vars contract, got: %v", err)
	}
}
