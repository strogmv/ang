package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestChannelTypeName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"in_app":     "InApp",
		"email":      "Email",
		"kafka.main": "KafkaMain",
		"nats-core":  "NatsCore",
		"":           "Channel",
	}
	for in, want := range cases {
		got := channelTypeName(in)
		if got != want {
			t.Fatalf("channelTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmitNotificationDispatchPortsNilConfigGeneratesDefault(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitNotificationDispatchPorts(nil); err != nil {
		t.Fatalf("expected nil config default generation, got error: %v", err)
	}
	path := filepath.Join(tmp, "internal", "port", "notification_dispatcher.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated dispatcher port: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, `NotificationInAppSink`) {
		t.Fatalf("generated dispatcher port missing default in_app sink")
	}
}

func TestEmitNotificationDispatcherRuntime(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.GoModule = "example.com/project"

	cfg := &ir.NotificationsConfig{
		Channels: &ir.NotificationChannels{
			DefaultChannels: []string{"email_primary", "email_fallback"},
			Channels: map[string]ir.NotificationChannelSpec{
				"email_primary":  {Enabled: true, Driver: "ses", Template: "password_reset"},
				"email_fallback": {Enabled: true, Driver: "smtp", Template: "password_reset"},
				"in_app":         {Enabled: true},
			},
		},
		Policies: &ir.NotificationPolicies{
			Rules: []ir.NotificationPolicyRule{
				{Enabled: true, Channels: []string{"email_primary", "email_fallback"}},
			},
		},
	}

	if err := em.EmitNotificationDispatcherRuntime(cfg); err != nil {
		t.Fatalf("EmitNotificationDispatcherRuntime returned error: %v", err)
	}

	path := filepath.Join(tmp, "internal", "adapter", "notifications", "dispatcher.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated dispatcher: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		`package notifications`,
		`"example.com/project/internal/port"`,
		`func NewDispatcher(cfg *config.Config) *Dispatcher`,
		`FailureRecorder`,
		`NotificationFailureRecorder`,
		`RetryEnqueuer`,
		`NotificationRetryEnqueuer`,
		`func classifyNotificationError(err error)`,
		`d.EmailPrimarySink = newEmailSink(cfg, "ses", "password_reset")`,
		`d.EmailFallbackSink = newEmailSink(cfg, "smtp", "password_reset")`,
		`case "email_primary":`,
		`case "email_fallback":`,
		`case "in_app":`,
		`channels = []string{`,
		`"email_primary"`,
		`"email_fallback"`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated dispatcher missing fragment %q", want)
		}
	}
}
