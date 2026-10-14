package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
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

// Item 23: cron: schedules get CronSpec and internal/scheduler/cron.go; a
// project without one gets neither (its scheduler.go is unchanged).
func TestEmitSchedulerCron(t *testing.T) {
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	if err := em.EmitScheduler([]ir.Schedule{
		{Name: "Morning", Service: "report", Action: "Send", Cron: "0 6 * * 1-5", Publish: "MorningRequested"},
		{Name: "Hourly", Service: "report", Action: "Sweep", Every: "1h", Publish: "HourlyRequested"},
	}); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(tmp, "internal", "scheduler", "scheduler.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`mustParseCron("0 6 * * 1-5")`, "go s.runCron(ctx, sc)", "Cron *CronSpec", "func (s *Scheduler) runCron("} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("scheduler.go misses %q:\n%s", want, src)
		}
	}
	if strings.Count(string(src), "mustParseCron(\"") != 1 {
		t.Fatalf("only the cron schedule gets a CronSpec:\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(tmp, "internal", "scheduler", "cron.go")); err != nil {
		t.Fatalf("cron.go: %v", err)
	}

	plain := t.TempDir()
	em = &Emitter{OutputDir: plain, GoModule: "example.com/project"}
	if err := em.EmitScheduler([]ir.Schedule{{Name: "Hourly", Service: "report", Action: "Sweep", Every: "1h", Publish: "HourlyRequested"}}); err != nil {
		t.Fatal(err)
	}
	src, err = os.ReadFile(filepath.Join(plain, "internal", "scheduler", "scheduler.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "Cron") {
		t.Fatalf("a project without cron schedules got cron code:\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(plain, "internal", "scheduler", "cron.go")); !os.IsNotExist(err) {
		t.Fatalf("cron.go without a cron schedule: %v", err)
	}
}

// The generated cron parser runs: Next across months, weekdays, steps,
// day-of-month OR day-of-week, a +05:30 zone and a DST change.
func TestGeneratedCronNext(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test on generated code")
	}
	dir := t.TempDir()
	cron, err := ReadTemplateByPath("templates/scheduler_cron.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	writeGenFile(t, filepath.Join(dir, "go.mod"), "module example.com/cron\n\ngo 1.22\n")
	writeGenFile(t, filepath.Join(dir, "cron.go"), string(cron))
	writeGenFile(t, filepath.Join(dir, "cron_test.go"), `package scheduler

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestNext(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		expr string
		from time.Time
		want time.Time
	}{
		{"0 6 * * *", time.Date(2026, 10, 14, 5, 59, 30, 0, time.UTC), time.Date(2026, 10, 14, 6, 0, 0, 0, time.UTC)},
		{"0 6 * * *", time.Date(2026, 10, 14, 6, 0, 0, 0, time.UTC), time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC)},
		{"*/15 * * * *", time.Date(2026, 10, 14, 6, 7, 0, 0, time.UTC), time.Date(2026, 10, 14, 6, 15, 0, 0, time.UTC)},
		{"0 9 * * 1-5", time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC), time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)}, // Friday -> Monday
		{"0 0 1 * *", time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"0 0 13 * 5", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}, // Friday the 16th: either day field
		{"30 2 * * 7", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 18, 2, 30, 0, 0, time.UTC)}, // 7 is Sunday
		{"0 6 * * *", time.Date(2026, 10, 14, 7, 0, 0, 0, kolkata), time.Date(2026, 10, 15, 6, 0, 0, 0, kolkata)},
		{"0 3 * * *", time.Date(2026, 3, 28, 12, 0, 0, 0, berlin), time.Date(2026, 3, 29, 3, 0, 0, 0, berlin)}, // the night the clocks go forward
		{"30 2 * * *", time.Date(2026, 3, 29, 1, 0, 0, 0, berlin), time.Date(2026, 3, 30, 2, 30, 0, 0, berlin)}, // 02:30 does not exist that night
		{"0 0 30 2 *", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Time{}},
	}
	for _, c := range cases {
		spec, err := ParseCron(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		if got := spec.Next(c.from); !got.Equal(c.want) {
			t.Errorf("%s from %s = %s, want %s", c.expr, c.from, got, c.want)
		}
	}
	for _, bad := range []string{"* * * *", "60 * * * *", "* * 0 * *", "*/0 * * * *"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("ParseCron(%q) accepted", bad)
		}
	}
}
`)
	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test of the generated cron.go: %v\n%s", err, out)
	} else if !strings.Contains(string(out), "--- PASS: TestNext") {
		t.Fatalf("generated cron test did not run:\n%s", out)
	}
}

func writeGenFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
