package emitter

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMicroPlanFixture_SendEmailToUser(t *testing.T) {
	t.Parallel()

	got := runGeneratedMicroPlanFixture(t, "send_email_to_user_usecases.json", "empty_automata.json")
	op := requireFixtureOperation(t, got, "SendEmailToUser")
	assertStringField(t, op, "kind", "notify")
	assertStepKinds(t, op, []string{"session", "load", "notify_email", "reply", "reply"})
	assertCanonicalNotifyEffect(t, op)
}

func TestBuildMicroPlanFixture_CreateUserWithWelcomeEmail(t *testing.T) {
	t.Parallel()

	got := runGeneratedMicroPlanFixture(t, "create_user_welcome_email_usecases.json", "empty_automata.json")
	op := requireFixtureOperation(t, got, "CreateUser")
	assertStringField(t, op, "kind", "create")
	assertStepKinds(t, op, []string{"new", "set_id", "set_now", "set", "set", "hash_password", "set", "save", "notify_email", "reply", "reply"})
	assertCanonicalNotifyEffect(t, op)
}

func TestBuildMicroPlanFixture_ApproveRequestWithNotifyRequester(t *testing.T) {
	t.Parallel()

	got := runGeneratedMicroPlanFixture(t, "approve_request_notify_requester_usecases.json", "approve_request_notify_requester_automata.json")
	op := requireFixtureOperation(t, got, "ApproveRequest")
	assertStringField(t, op, "kind", "transition")
	assertStepKinds(t, op, []string{"session", "load", "guard_status", "transition", "save", "notify_email", "reply", "reply"})
	assertCanonicalNotifyEffect(t, op)
}

func runGeneratedMicroPlanFixture(t *testing.T, usecasesFixture, automataFixture string) map[string]any {
	t.Helper()

	usecasesRaw := mustReadFixture(t, usecasesFixture)
	automataRaw := mustReadFixture(t, automataFixture)
	snippet := renderPlanBuildMicroPlanCode(&flowRenderState{returnErrOnly: true}, "", "usecasesDoc", "automataDoc", "microPlanDoc", "=", "_micro", "_err")

	prog := "package main\n\n" +
		"import (\n" +
		"\t\"encoding/json\"\n" +
		"\t\"fmt\"\n" +
		"\t\"os\"\n" +
		"\t\"strings\"\n" +
		")\n\n" +
		"func main() {\n" +
		"\tusecasesDoc := json.RawMessage(" + quoteGoString(string(usecasesRaw)) + ")\n" +
		"\tautomataDoc := json.RawMessage(" + quoteGoString(string(automataRaw)) + ")\n" +
		"\tvar microPlanDoc map[string]any\n" +
		"\tif err := func() error {\n" +
		indentSnippet(snippet, "\t\t") + "\n" +
		"\t\treturn nil\n" +
		"\t}(); err != nil {\n" +
		"\t\tpanic(err)\n" +
		"\t}\n" +
		"\tenc := json.NewEncoder(os.Stdout)\n" +
		"\tenc.SetEscapeHTML(false)\n" +
		"\tif err := enc.Encode(microPlanDoc); err != nil {\n" +
		"\t\tpanic(err)\n" +
		"\t}\n" +
		"}\n"

	tmp := t.TempDir()
	mainPath := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(mainPath, []byte(prog), 0o644); err != nil {
		t.Fatalf("write temp main.go: %v", err)
	}

	cmd := exec.Command("go", "run", mainPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run generated micro plan failed: %v\n%s", err, out)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse generated micro plan output: %v\n%s", err, out)
	}
	return got
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "meta_plan", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func quoteGoString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func indentSnippet(s, indent string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = indent
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func requireFixtureOperation(t *testing.T, got map[string]any, name string) map[string]any {
	t.Helper()
	ops, ok := got["operations"].([]any)
	if !ok {
		t.Fatalf("operations missing or invalid: %#v", got["operations"])
	}
	for _, raw := range ops {
		op, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if opName, _ := op["name"].(string); opName == name {
			return op
		}
	}
	t.Fatalf("operation %q not found in %#v", name, ops)
	return nil
}

func assertStringField(t *testing.T, obj map[string]any, field, want string) {
	t.Helper()
	got, _ := obj[field].(string)
	if got != want {
		t.Fatalf("%s=%q, want %q", field, got, want)
	}
}

func assertCanonicalNotifyEffect(t *testing.T, op map[string]any) {
	t.Helper()
	effects, ok := op["side_effects"].([]any)
	if !ok || len(effects) == 0 {
		t.Fatalf("side_effects missing or empty: %#v", op["side_effects"])
	}
	first, ok := effects[0].(map[string]any)
	if !ok {
		t.Fatalf("first side effect invalid: %#v", effects[0])
	}
	kind, _ := first["kind"].(string)
	if kind != "notify.email" {
		t.Fatalf("side_effects[0].kind=%q, want notify.email", kind)
	}
}

func assertStepKinds(t *testing.T, op map[string]any, want []string) {
	t.Helper()
	steps, ok := op["steps"].([]any)
	if !ok {
		t.Fatalf("steps missing or invalid: %#v", op["steps"])
	}
	got := make([]string, 0, len(steps))
	for _, raw := range steps {
		step, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid step entry: %#v", raw)
		}
		got = append(got, asString(step["p"]))
	}
	if len(got) != len(want) {
		t.Fatalf("step count=%d want=%d steps=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step[%d]=%q want=%q full=%v", i, got[i], want[i], got)
		}
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
