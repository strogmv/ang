package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestPublisherGeneratorsUseCanonicalGoEventName(t *testing.T) {
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	schedules := []ir.Schedule{{
		Name: "Escalate", Service: "tender", Action: "Escalate", Publish: "AwardSLAEscalationRequested", Every: "1h",
	}}
	if err := em.EmitPublisherInterface(nil, schedules); err != nil {
		t.Fatalf("EmitPublisherInterface failed: %v", err)
	}
	if err := em.EmitNatsAdapter(nil, schedules); err != nil {
		t.Fatalf("EmitNatsAdapter failed: %v", err)
	}

	for _, path := range []string{
		filepath.Join(tmp, "internal", "port", "publisher.go"),
		filepath.Join(tmp, "internal", "adapter", "events", "nats", "client.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(data)
		if !strings.Contains(source, "PublishAwardSlaEscalationRequested") {
			t.Fatalf("%s does not use canonical event method name:\n%s", path, source)
		}
		if strings.Contains(source, "PublishAwardSLAEscalationRequested") {
			t.Fatalf("%s leaked raw event spelling into a Go method name", path)
		}
	}
}
