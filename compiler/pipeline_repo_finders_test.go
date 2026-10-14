package compiler

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// badFinderCUE holds a regex literal inside a finder's sql. `\s` is not a CUE
// escape, so the file does not parse; the error is on line 10.
const badFinderCUE = `package repo

Repositories: {
	Tender: {
		finders: [
			{
				name:    "SearchByTitle"
				returns: "many"
				sql: """
					SELECT id FROM tenders WHERE regexp_replace(title, '\s+', ' ') = $1
					"""
			},
		]
	}
}
`

// copyMinimalProject copies angir/testdata/minimal (one entity User with a
// custom finder List) into a temp dir and returns its path.
func copyMinimalProject(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "angir", "testdata", "minimal")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

// Item 1 of docs/tech-debt-backlog.md: an invalid string in one finder's sql
// used to drop finders silently while validate/build reported OK.
func TestSemanticPhasesFailOnInvalidEscapeInFinderSQL(t *testing.T) {
	base := copyMinimalProject(t)
	bad := filepath.Join(base, "cue", "repo", "search.cue")
	if err := os.WriteFile(bad, []byte(badFinderCUE), 0o644); err != nil {
		t.Fatalf("write bad finder: %v", err)
	}

	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err == nil {
		var finders []string
		for _, r := range out.Repos {
			for _, f := range r.Finders {
				finders = append(finders, r.Entity+"."+f.Name)
			}
		}
		t.Fatalf("expected a build error for the invalid escape, got none; finders kept: %v", finders)
	}
	msg := err.Error()
	for _, want := range []string{"search.cue:10:", "unknown escape sequence", `finder "SearchByTitle"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
	var ce *ContractError
	if !asContractError(err, &ce) || ce.Code != ErrCodeCUERepoLoad {
		t.Errorf("expected ContractError %s, got %#v", ErrCodeCUERepoLoad, err)
	}
}

// A finder field that exists but does not evaluate to a string (here a
// conflict) must fail the build instead of producing a finder without it.
func TestSemanticPhasesFailOnUndecodableFinderField(t *testing.T) {
	base := copyMinimalProject(t)
	conflict := `package repo

Repositories: {
	Tender: {
		finders: [{
			name:    "ByStatus"
			returns: "many"
			sql:     "SELECT 1" & "SELECT 2"
		}]
	}
}
`
	if err := os.WriteFile(filepath.Join(base, "cue", "repo", "status.cue"), []byte(conflict), 0o644); err != nil {
		t.Fatalf("write finder: %v", err)
	}
	_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err == nil {
		t.Fatalf("expected an error for a conflicting sql value")
	}
	msg := err.Error()
	for _, want := range []string{`finder "ByStatus"`, "status.cue:8:", "conflicting values"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
}

// The good path keeps working: the fixture's finder survives.
func TestSemanticPhasesKeepValidFinders(t *testing.T) {
	base := copyMinimalProject(t)
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range out.Repos {
		if r.Entity == "User" && len(r.Finders) == 1 && r.Finders[0].Name == "List" {
			return
		}
	}
	t.Fatalf("finder User.List missing: %#v", out.Repos)
}

func asContractError(err error, target **ContractError) bool {
	for err != nil {
		if ce, ok := err.(*ContractError); ok {
			*target = ce
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
