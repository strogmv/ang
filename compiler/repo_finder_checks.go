package compiler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/strogmv/ang/angir/normalizer"
)

// RepoFinderCheck checks the finders of cue/repo against the repository code
// a target generates for them and returns one message per problem, each
// starting with the finder's file:line. The checks that need the generated
// method (its arguments, its scan) live with the emitter, which imports this
// package; it registers them here so ang validate and ang build both run
// them in the normalize phase.
type RepoFinderCheck func(repos []normalizer.Repository, entities []normalizer.Entity) []string

var (
	repoFinderChecksMu sync.Mutex
	repoFinderChecks   []RepoFinderCheck
)

// RegisterRepoFinderCheck adds a check run by every normalize phase.
func RegisterRepoFinderCheck(check RepoFinderCheck) {
	repoFinderChecksMu.Lock()
	defer repoFinderChecksMu.Unlock()
	repoFinderChecks = append(repoFinderChecks, check)
}

// RepoFinderError lists every finder whose generated method could not work.
type RepoFinderError struct {
	Problems []string
}

func (e *RepoFinderError) Error() string {
	return strings.Join(e.Problems, "\n")
}

func validateRepoFinders(repos []normalizer.Repository, entities []normalizer.Entity) error {
	repoFinderChecksMu.Lock()
	checks := append([]RepoFinderCheck(nil), repoFinderChecks...)
	repoFinderChecksMu.Unlock()
	var problems []string
	for _, check := range checks {
		problems = append(problems, check(repos, entities)...)
	}
	if len(problems) == 0 {
		return nil
	}
	return &RepoFinderError{Problems: problems}
}

// emitFinderPagingDiagnostics warns about the legacy pagination rule: a
// finder with limit: > 0, or with "paginate" in its name, takes
// (limit int, offset int) — whatever the number — unless paged: says
// otherwise. The generated code stays the same; paged: true says it plainly.
func emitFinderPagingDiagnostics(repos []normalizer.Repository, opts PipelineOptions) {
	for _, repo := range repos {
		for _, f := range repo.Finders {
			if f.Paged != nil {
				continue
			}
			var reason string
			switch {
			case f.Limit > 0:
				reason = fmt.Sprintf("limit: %d does not limit the query to %d rows; it adds the parameters (limit int, offset int)", f.Limit, f.Limit)
			case strings.Contains(strings.ToLower(f.Name), "paginate"):
				reason = `a name containing "paginate" adds the parameters (limit int, offset int)`
			default:
				continue
			}
			file, line := parseSourcePos(f.Source)
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "repository",
				Code:     "REPO_FINDER_PAGED_IMPLICIT",
				Severity: "warn",
				Message:  fmt.Sprintf("finder %s.%s: %s", repo.Entity, f.Name, reason),
				File:     file,
				Line:     line,
				Hint:     "write paged: true instead (same method, same SQL); paged: false turns the parameters off",
			}, opts)
		}
	}
}
