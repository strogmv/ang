package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// releaseRepo is a git work tree (a project in a subdirectory, as a backend
// can be) whose HEAD holds the released contract and port files.
func releaseRepo(t *testing.T, files map[string]string) (backend string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	backend = filepath.Join(root, "svc")
	for rel, content := range files {
		writeTestFile(t, filepath.Join(backend, rel), content)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "release"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = toolchainGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return backend
}

func quoteContract(responseFields ...string) string {
	var props strings.Builder
	for _, field := range responseFields {
		props.WriteString("        " + field + ": {type: string}\n")
	}
	return `openapi: 3.0.0
paths:
  /quotes:
    post:
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/QuoteResponse'
components:
  schemas:
    QuoteResponse:
      properties:
` + props.String()
}

func TestContractGateLetsUnreleasedChangesPass(t *testing.T) {
	backend := releaseRepo(t, map[string]string{"api/openapi.yaml": quoteContract("id")})
	contract := filepath.Join(backend, "api", "openapi.yaml")
	release := resolveReleaseBaseline("")
	logf := func(string, ...any) {}

	// The last build added `draft` (never committed); this one drops it again.
	previous, current := []byte(quoteContract("id", "draft")), []byte(quoteContract("id"))
	if refusal := checkContractGate(contract, previous, current, release, false, logf); refusal != nil {
		t.Fatalf("dropping a field HEAD never had must pass: %v", refusal.err)
	}
	// Dropping `id`, which HEAD has, is refused as before.
	refusal := checkContractGate(contract, previous, []byte(quoteContract("draft")), release, false, logf)
	if refusal == nil || !strings.Contains(refusal.err.Error(), "removed field QuoteResponse.id") || strings.Contains(refusal.err.Error(), "draft") {
		t.Fatalf("dropping a released field must be refused alone, got %v", refusal)
	}
	// --release-ref none: the previous generation is the baseline again.
	if refusal := checkContractGate(contract, previous, current, resolveReleaseBaseline(releaseRefNone), false, logf); refusal == nil {
		t.Fatal("without a release baseline, dropping draft must be refused")
	}
	// A revision without the contract released nothing.
	t.Setenv("ANG_RELEASE_REF", "HEAD")
	untracked := filepath.Join(backend, "api", "other.yaml")
	writeTestFile(t, untracked, quoteContract("id"))
	if refusal := checkContractGate(untracked, []byte(quoteContract("id")), []byte(quoteContract("x")), resolveReleaseBaseline(""), false, logf); refusal != nil {
		t.Fatalf("a contract HEAD does not have released nothing: %v", refusal.err)
	}
}

func TestContractGateWithoutGitKeepsThePreviousBaseline(t *testing.T) {
	dir := t.TempDir() // not a git work tree
	contract := filepath.Join(dir, "api", "openapi.yaml")
	writeTestFile(t, contract, quoteContract("id", "draft"))
	refusal := checkContractGate(contract, []byte(quoteContract("id", "draft")), []byte(quoteContract("id")), resolveReleaseBaseline(""), false, func(string, ...any) {})
	if refusal == nil {
		t.Fatal("outside git the previous generation is the baseline")
	}
}

const releasedPort = `package port

import "context"

type QuoteRepository interface {
	Save(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (string, error)
}
`

func TestRepositoryGateLetsUnreleasedFindersGo(t *testing.T) {
	backend := releaseRepo(t, map[string]string{"internal/port/quoterepository.go": releasedPort})
	release := resolveReleaseBaseline("")
	logf := func(string, ...any) {}
	previous := repositoryMethods{
		"QuoteRepository": {"Save": true, "FindByID": true, "ListDrafts": true},
		"DraftRepository": {"Save": true},
	}
	generated := t.TempDir()
	writeTestFile(t, filepath.Join(generated, "internal", "port", "quoterepository.go"), releasedPort)
	if refusal := checkRepositoryGate(backend, previous, generated, release, false, logf); refusal != nil {
		t.Fatalf("finders and interfaces HEAD never had may go: %v", refusal.err)
	}
	writeTestFile(t, filepath.Join(generated, "internal", "port", "quoterepository.go"), `package port

import "context"

type QuoteRepository interface {
	Save(ctx context.Context, id string) error
}
`)
	refusal := checkRepositoryGate(backend, previous, generated, release, false, logf)
	if refusal == nil || !strings.Contains(refusal.err.Error(), "port.QuoteRepository.FindByID") || strings.Contains(refusal.err.Error(), "ListDrafts") {
		t.Fatalf("a released finder must be refused alone, got %v", refusal)
	}
	if refusal := checkRepositoryGate(backend, previous, generated, release, true, logf); refusal != nil {
		t.Fatalf("--accept-removals accepts it: %v", refusal.err)
	}
}
