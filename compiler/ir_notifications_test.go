package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestAttachNotificationInfra(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{}
	channels := &normalizer.NotificationChannelsDef{
		Enabled:         true,
		DefaultChannels: []string{"in_app", "email"},
		Channels: map[string]normalizer.NotificationChannelConfig{
			"email": {Enabled: true, Driver: "smtp", Template: "default"},
			"nats":  {Enabled: true, Subject: "notifications.>"},
		},
	}
	policies := &normalizer.NotificationPoliciesDef{
		Enabled: true,
		Rules: []normalizer.NotificationPolicyRule{
			{
				Enabled:  true,
				Event:    "TenderCreated",
				Type:     "company_tender_created",
				Audience: "subscribers",
				Channels: []string{"in_app", "email"},
				Template: "tender_created",
				MuteKey:  "company_tender_created",
			},
		},
	}

	AttachNotificationInfra(schema, channels, policies)

	if schema.Notifications == nil {
		t.Fatalf("expected notifications to be attached")
	}
	if schema.Notifications.Channels == nil || !schema.Notifications.Channels.Enabled {
		t.Fatalf("expected channels to be attached and enabled")
	}
	if schema.Notifications.Channels.Channels["email"].Driver != "smtp" {
		t.Fatalf("expected email driver smtp, got %q", schema.Notifications.Channels.Channels["email"].Driver)
	}
	if schema.Notifications.Policies == nil || len(schema.Notifications.Policies.Rules) != 1 {
		t.Fatalf("expected one policy rule in IR")
	}
	if schema.Notifications.Policies.Rules[0].Event != "TenderCreated" {
		t.Fatalf("unexpected policy event: %q", schema.Notifications.Policies.Rules[0].Event)
	}
}
