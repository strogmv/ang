package gamepoc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/wounds"
	woundsRuntime "github.com/strogmv/ang/compiler/wounds/runtime"
)

func woundsFixture(t *testing.T) woundsRuntime.Model {
	t.Helper()
	raw, err := os.ReadFile("../wounds/testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var m woundsRuntime.Model
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for i := range m.Scenarios {
		m.Scenarios[i].RiskRuns = min(m.Scenarios[i].RiskRuns, 20)
	}
	return m
}

// The remembered table equals wounds.Report byte for byte, and is replayed when a text it read, the model, or the
// switches ANGELS_NO_CACHE / ANGELS_FULL say so.
func TestWoundsReportMemo(t *testing.T) {
	t.Setenv("ANGELS_PIPELINE_CACHE", t.TempDir())
	t.Setenv("ANGELS_NO_CACHE", "")
	t.Setenv("ANGELS_FULL", "")
	m := woundsFixture(t)
	texts := map[string]string{}
	calls := 0
	text := func(key string) string {
		calls++
		if v, ok := texts[key]; ok {
			return v
		}
		return "«" + key + "»"
	}
	want, err := wounds.Report(m, text)
	if err != nil {
		t.Fatal(err)
	}
	first, err := reportWounds(m, text)
	if err != nil || !bytes.Equal(first, want) {
		t.Fatalf("fresh table differs from wounds.Report (err %v)", err)
	}
	entries, _ := filepath.Glob(filepath.Join(os.Getenv("ANGELS_PIPELINE_CACHE"), "ang-wounds-report", "*.json"))
	if len(entries) != 1 {
		t.Fatalf("%d memo entries", len(entries))
	}
	again, err := reportWounds(m, text)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatal("remembered table differs")
	}
	// A text the table shows changes: the table is replayed with it.
	var entry woundsReportEntry
	raw, _ := os.ReadFile(entries[0])
	_ = json.Unmarshal(raw, &entry)
	var shown string
	for key := range entry.Texts {
		if strings.HasPrefix(key, "wounds.scenario.") {
			shown = key
			break
		}
	}
	if shown == "" {
		t.Fatal("the memo recorded no scenario text")
	}
	texts[shown] = "новое имя"
	changed, _ := reportWounds(m, text)
	if bytes.Equal(changed, want) || !bytes.Contains(changed, []byte("новое имя")) {
		t.Fatal("a changed text did not replay the table")
	}
	fresh, _ := wounds.Report(m, text)
	if !bytes.Equal(changed, fresh) {
		t.Fatal("replayed table differs from wounds.Report")
	}
	// The model changes: another entry.
	m.Scenarios = m.Scenarios[1:]
	smaller, _ := reportWounds(m, text)
	if bytes.Equal(smaller, changed) {
		t.Fatal("a changed model reused the old table")
	}
	// A planted entry is returned as is (the memo is read) except with ANGELS_FULL=1 or ANGELS_NO_CACHE=1.
	path := woundsReportPath(m)
	var planted woundsReportEntry
	raw, _ = os.ReadFile(path)
	_ = json.Unmarshal(raw, &planted)
	planted.Report = []byte("planted")
	raw, _ = json.Marshal(planted)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := reportWounds(m, text); string(got) != "planted" {
		t.Fatal("the memo was not read")
	}
	for _, name := range []string{"ANGELS_FULL", "ANGELS_NO_CACHE"} {
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, "1")
		if got, _ := reportWounds(m, text); string(got) == "planted" || !bytes.Equal(got, smaller) {
			t.Fatalf("%s=1 used the memo", name)
		}
		t.Setenv(name, "")
	}
	_ = calls
}
