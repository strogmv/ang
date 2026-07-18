package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitSchedulerSupportsDailyClockTime(t *testing.T) {
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	if err := em.EmitScheduler([]ir.Schedule{{
		Name: "Nightly", Service: "report", Action: "Rebuild", At: "02:00", Every: "24h",
	}}); err != nil {
		t.Fatalf("EmitScheduler failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "internal", "scheduler", "scheduler.go"))
	if err != nil {
		t.Fatalf("read generated scheduler: %v", err)
	}
	source := string(data)
	for _, want := range []string{
		`time.Parse("15:04", v)`,
		`os.Getenv("SCHEDULER_TIMEZONE")`,
		`time.Now().In(schedulerLocation())`,
		`next.Add(24 * time.Hour)`,
		`mustParseTime("02:00")`,
		`_ "time/tzdata"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated scheduler missing %q:\n%s", want, source)
		}
	}
}
