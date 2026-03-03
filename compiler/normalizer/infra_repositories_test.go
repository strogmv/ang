package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractRepositories_FromServicesEntities(t *testing.T) {
	t.Parallel()

	val := cuecontext.New().CompileString(`
package architecture

Services: {
	sandbox: {
		entities: ["Project", "ProjectFile"]
	}
}
`)

	n := New()
	repos, err := n.ExtractRepositories(val)
	if err != nil {
		t.Fatalf("ExtractRepositories: %v", err)
	}

	seen := map[string]bool{}
	for _, r := range repos {
		seen[r.Name] = true
	}
	for _, expected := range []string{"ProjectRepository", "ProjectFileRepository"} {
		if !seen[expected] {
			t.Fatalf("expected repository %s to be extracted, got %#v", expected, repos)
		}
	}
}

func TestExtractRepositories_DedupOwnsAndEntities(t *testing.T) {
	t.Parallel()

	val := cuecontext.New().CompileString(`
package architecture

Services: {
	sandbox: {
		owns:     ["Project"]
		entities: ["Project", "BuildLog"]
	}
}
`)

	n := New()
	repos, err := n.ExtractRepositories(val)
	if err != nil {
		t.Fatalf("ExtractRepositories: %v", err)
	}

	count := map[string]int{}
	for _, r := range repos {
		count[r.Name]++
	}

	if count["ProjectRepository"] != 1 {
		t.Fatalf("ProjectRepository expected once, got %d (%#v)", count["ProjectRepository"], repos)
	}
	if count["BuildLogRepository"] != 1 {
		t.Fatalf("BuildLogRepository expected once, got %d (%#v)", count["BuildLogRepository"], repos)
	}
}

