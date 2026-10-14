package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Repositories that come from cue/repo finders used to be appended in the
// order of a Go map, so the IR listed them differently on every run. They are
// appended in entity-name order now.
func TestSemanticPhasesListRepositoriesInTheSameOrderEveryRun(t *testing.T) {
	base := copyMinimalProject(t)
	var domain, repo strings.Builder
	domain.WriteString("package domain\n\n")
	repo.WriteString("package repo\n\nRepositories: {\n")
	var want []string
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("Thing%02d", (i*7)%16)
		fmt.Fprintf(&domain, "%s: {\n\tfields: {\n\t\tid: string\n\t\tname: string\n\t}\n}\n\n", name)
		fmt.Fprintf(&repo, "\t%s: finders: [{name: \"ListAll\", returns: \"many\"}]\n", name)
		want = append(want, name+"Repository")
	}
	repo.WriteString("}\n")
	slices.Sort(want)
	if err := os.WriteFile(filepath.Join(base, "cue", "domain", "things.cue"), []byte(domain.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "cue", "repo", "things.cue"), []byte(repo.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	var first []string
	for run := 0; run < 5; run++ {
		out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		var names, things []string
		for _, r := range out.Repos {
			names = append(names, r.Name)
			if strings.HasPrefix(r.Name, "Thing") {
				things = append(things, r.Name)
			}
		}
		if !slices.Equal(things, want) {
			t.Fatalf("run %d: finder repositories %v, want them in name order %v", run, things, want)
		}
		if first == nil {
			first = names
		} else if !slices.Equal(first, names) {
			t.Fatalf("run %d listed repositories as %v, run 0 as %v", run, names, first)
		}
	}
}
