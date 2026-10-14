package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler"
)

const runCandidateDomain = `package domain

RunCandidate: {
	id:     string
	status: string
}

Ticket: {
	id:    string
	title: string
}
`

// A repository the operation's Go reaches (s.RunCandidateRepo) is a field and
// a constructor parameter of the service implementation even when no
// operation of the service lists its entity in sources or uses it in a
// repo.* step.
func TestPipelineInjectsRepositoriesTheCodeUses(t *testing.T) {
	outDir := emitFixtureProject(t, map[string]string{
		"domain/stats.cue": runCandidateDomain,
		"api/stats.cue": `package api

CountRuns: {
	service: "stats"
	input: {companyId: string}
	output: {count: int}
	impls: go: {
		code: """
			runs, err := s.RunCandidateRepo.ListAll(ctx)
			if err != nil {
				return resp, err
			}
			resp.Count = len(runs)
			return resp, nil
			"""
	}
}

CloseTicket: {
	service: "stats"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "logic.Call", func: "s.TicketRepo.Delete", args: ["ctx", "req.ID"]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
`,
	})
	matches, _ := filepath.Glob(filepath.Join(outDir, "internal", "service", "*.go"))
	var impl string
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		if strings.Contains(string(b), "type StatsImpl struct") {
			impl = readParsedGo(t, m)
		}
	}
	if impl == "" {
		t.Fatalf("no StatsImpl among %v", matches)
	}
	for _, want := range []string{
		"RunCandidateRepo port.RunCandidateRepository",
		"TicketRepo       port.TicketRepository",
		"runCandidateRepo port.RunCandidateRepository",
		"ticketRepo port.TicketRepository",
	} {
		if !strings.Contains(impl, want) {
			t.Errorf("StatsImpl lacks %q:\n%s", want, impl)
		}
	}
}

// Go that reaches a repository no entity has fails the build with the
// operation and the name, instead of an "undefined" from the Go compiler.
func TestPipelineRejectsRepositoryOfNoEntity(t *testing.T) {
	basePath := t.TempDir()
	for rel, text := range map[string]string{
		"domain/stats.cue": runCandidateDomain,
		"api/stats.cue": `package api

CountRuns: {
	service: "stats"
	input: {companyId: string}
	output: {count: int}
	impls: go: {
		code: """
			runs, _ := s.RunCandidatRepo.ListAll(ctx)
			resp.Count = len(runs)
			return resp, nil
			"""
	}
}
`,
	} {
		path := filepath.Join(basePath, "cue", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, _, _, _, _, _, _, _, _, err := compiler.RunPipeline(basePath)
	if err == nil {
		t.Fatal("RunPipeline accepted s.RunCandidatRepo, a repository of no entity")
	}
	for _, want := range []string{"CountRuns", "s.RunCandidatRepo", "RunCandidate", "stats.cue"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}
