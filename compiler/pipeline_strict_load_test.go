package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Item 1 of docs/tech-debt-backlog.md, second half: every CUE folder the parse
// phase reads must fail validate and build when one of its files does not
// parse. cue/events, cue/errors, cue/project, cue/projections and cue/policy
// used to drop the load error (their declarations vanished from the output),
// and cue/domain and cue/api skipped the file with only a stderr note.
func TestSemanticPhasesFailOnFileThatDoesNotParse(t *testing.T) {
	cases := []struct {
		folder string
		pkg    string
		code   string
	}{
		{"domain", "domain", ErrCodeCUEDomainLoad},
		{"api", "api", ErrCodeCUEAPILoad},
		{"events", "events", ErrCodeCUEEventsLoad},
		{"errors", "errors", ErrCodeCUEErrorsLoad},
		{"project", "project", ErrCodeCUEProjectLoad},
		{"projections", "projections", ErrCodeCUEProjectionsLoad},
		{"policy", "policy", ErrCodeCUEPolicyLoad},
		{"policies", "policies", ErrCodeCUEPolicyLoad},
	}
	for _, tc := range cases {
		t.Run(tc.folder, func(t *testing.T) {
			base := copyMinimalProject(t)
			dir := filepath.Join(base, "cue", tc.folder)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			// `\s` is not a CUE escape: the file does not parse; line 4.
			bad := "package " + tc.pkg + "\n\n#Broken: {\n\tpattern: \"a\\sb\"\n}\n"
			if err := os.WriteFile(filepath.Join(dir, "broken.cue"), []byte(bad), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
			if err == nil {
				t.Fatalf("cue/%s/broken.cue does not parse, but the semantic phases passed", tc.folder)
			}
			msg := err.Error()
			for _, want := range []string{"broken.cue:4:", "unknown escape sequence"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error does not mention %q:\n%s", want, msg)
				}
			}
			var ce *ContractError
			if !asContractError(err, &ce) || ce.Code != tc.code {
				t.Errorf("expected ContractError %s, got %#v", tc.code, err)
			}
		})
	}
}

// A folder whose files parse but do not evaluate (two files that disagree)
// must fail too, not be read as empty.
func TestSemanticPhasesFailOnConflictInOptionalFolder(t *testing.T) {
	cases := []struct {
		folder string
		code   string
	}{
		{"events", ErrCodeCUEEventsLoad},
		{"errors", ErrCodeCUEErrorsLoad},
		{"project", ErrCodeCUEProjectLoad},
		{"projections", ErrCodeCUEProjectionsLoad},
		{"policy", ErrCodeCUEPolicyLoad},
	}
	for _, tc := range cases {
		t.Run(tc.folder, func(t *testing.T) {
			base := copyMinimalProject(t)
			dir := filepath.Join(base, "cue", tc.folder)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			a := "package " + tc.folder + "\n\nmode: \"a\"\n"
			b := "package " + tc.folder + "\n\nmode: \"b\"\n"
			if err := os.WriteFile(filepath.Join(dir, "a.cue"), []byte(a), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "b.cue"), []byte(b), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
			if err == nil {
				t.Fatalf("cue/%s has conflicting values, but the semantic phases passed", tc.folder)
			}
			if !strings.Contains(err.Error(), "conflicting values") {
				t.Errorf("error does not mention the conflict:\n%v", err)
			}
			var ce *ContractError
			if !asContractError(err, &ce) || ce.Code != tc.code {
				t.Errorf("expected ContractError %s, got %#v", tc.code, err)
			}
		})
	}
}

// An event whose fields do not evaluate used to be dropped together with
// every other event of cue/events (ExtractEvents' error was ignored).
func TestSemanticPhasesFailOnUndecodableEvent(t *testing.T) {
	base := copyMinimalProject(t)
	dir := filepath.Join(base, "cue", "events")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package events

#UserCreated: {
	userId: string
}

#UserRenamed: {
	fields: {
		name: 1 & 2
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "events.cue"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err == nil {
		var names []string
		for _, e := range out.Events {
			names = append(names, e.Name)
		}
		t.Fatalf("an event with a conflicting field passed; events kept: %v", names)
	}
	if !strings.Contains(err.Error(), "conflicting values") {
		t.Errorf("error does not mention the conflict:\n%v", err)
	}
}

// The fixture without broken files still passes.
func TestSemanticPhasesLoadOptionalFolders(t *testing.T) {
	base := copyMinimalProject(t)
	files := map[string]string{
		"events/events.cue":           "package events\n\n#UserCreated: {\n\tuserId: string\n}\n",
		"errors/errors.cue":           "package errors\n\n#Errors: {\n\tUserNotFound: {code: 404, http: 404, msg: \"user not found\"}\n}\n",
		"project/project.cue":         "package project\n\nname: \"minimal\"\n",
		"projections/projections.cue": "package projections\n",
		"policy/policy.cue":           "package policy\n",
	}
	for rel, src := range files {
		path := filepath.Join(base, "cue", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Events) != 1 || out.Events[0].Name != "UserCreated" {
		t.Errorf("events: %#v", out.Events)
	}
	if len(out.Errors) != 1 || out.Errors[0].Name != "UserNotFound" {
		t.Errorf("errors: %#v", out.Errors)
	}
}

// A business error whose code is not a number used to be read as code 0.
func TestSemanticPhasesFailOnUndecodableErrorCode(t *testing.T) {
	base := copyMinimalProject(t)
	dir := filepath.Join(base, "cue", "errors")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package errors\n\n#Errors: {\n\tUserNotFound: {code: \"E404\", http: 404, msg: \"user not found\"}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "errors.cue"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err == nil {
		t.Fatalf("code: \"E404\" passed; errors: %#v", out.Errors)
	}
	for _, want := range []string{"UserNotFound", "code"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
	var ce *ContractError
	if !asContractError(err, &ce) || ce.Code != ErrCodeCUEErrorsLoad {
		t.Errorf("expected ContractError %s, got %#v", ErrCodeCUEErrorsLoad, err)
	}
}
