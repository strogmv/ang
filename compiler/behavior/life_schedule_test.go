package behavior

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Run the emitted tests against the emitted kernel, rather than a handwritten
// duplicate of its timetable arithmetic. The scratch package has no game host.
func TestGeneratedLifeRailScheduleAndPopulationQueue(t *testing.T) {
	files, err := Render([]byte(`{"actions":{},"tasks":{}}`), []byte(`{"cases":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module rail_schedule_proof\n\ngo 1.25\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "TestLife|TestPopulation", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated rail schedule regressions: %v\n%s", err, out)
	}
}
