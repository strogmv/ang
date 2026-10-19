package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
)

// buildGateRefusal is a build that generated fine but must not be applied:
// the OpenAPI contract breaks without --accept-contract, or repository
// interfaces lose methods without --accept-removals.
type buildGateRefusal struct {
	code string
	op   string
	err  error
}

// contractGateError explains a refused contract change, one change per line.
func contractGateError(contractPath string, breaking []string) error {
	const shown = 30
	list := breaking
	more := ""
	if len(list) > shown {
		more = fmt.Sprintf("\n  … and %d more", len(list)-shown)
		list = list[:shown]
	}
	return fmt.Errorf(
		"this build makes %d breaking change(s) to the OpenAPI contract %s:\n  %s%s\nNothing was written. Review them, then rerun with --accept-contract to confirm",
		len(breaking), filepath.ToSlash(contractPath), strings.Join(list, "\n  "), more,
	)
}

// checkContractGate compares the previous contract with the one this build
// generated. Only what the release baseline has can break: the changes of this
// build that touch an operation, schema or field the release does not have
// pass. logf reports the summary line.
func checkContractGate(contractPath string, previous, current []byte, release releaseBaseline, accept bool, logf func(string, ...any)) *buildGateRefusal {
	diff, recoveredBaseline, err := diffOpenAPIContractsWithRecovery(previous, current)
	if err != nil {
		return &buildGateRefusal{code: compiler.ErrCodeEmitterStep, op: "compare OpenAPI contract", err: err}
	}
	if recoveredBaseline {
		logf("openapi: previous generated contract is invalid; replacing it with the valid regenerated contract")
	}
	breaking := diff.Breaking
	if len(breaking) > 0 {
		if released, present, ok := release.file(filepath.Dir(contractPath), filepath.Base(contractPath)); ok {
			if present {
				sinceRelease, _, relErr := diffOpenAPIContractsWithRecovery(released, current)
				if relErr == nil {
					breaking = contractChangesSinceRelease(diff.Breaking, sinceRelease.Breaking)
				}
			} else {
				breaking = nil
			}
			if unreleased := len(diff.Breaking) - len(breaking); unreleased > 0 {
				logf("openapi: %d change(s) touch only what %s does not have (unreleased), not gated", unreleased, release.ref)
			}
		}
	}
	logf("openapi: +%d operations, -%d operations, %d breaking", len(diff.AddedOperations), len(diff.RemovedOperations), len(breaking))
	if len(breaking) > 0 && !accept {
		messages := make([]string, 0, len(breaking))
		for _, change := range breaking {
			messages = append(messages, change.Message)
		}
		return &buildGateRefusal{code: compiler.ErrCodeContractBreaking, op: "OpenAPI breaking change gate", err: contractGateError(contractPath, messages)}
	}
	return nil
}

// checkRepositoryGate compares the repository interfaces of the previous
// generation with this build's. A method the release baseline does not have
// was never released and may go.
func checkRepositoryGate(backend string, previous repositoryMethods, generatedBackend string, release releaseBaseline, accept bool, logf func(string, ...any)) *buildGateRefusal {
	current, err := collectRepositoryMethods(generatedBackend)
	if err != nil {
		return &buildGateRefusal{code: compiler.ErrCodeEmitterStep, op: "read generated repository interfaces", err: err}
	}
	removed := diffRepositoryRemovals(previous, current)
	if len(removed) == 0 {
		return nil
	}
	if files, ok := release.tree(backend, "internal/port"); ok {
		released, parseErr := parseRepositoryMethods(filepath.Join(backend, "internal", "port"), files)
		if parseErr == nil {
			kept := removalsSinceRelease(removed, released)
			if unreleased := len(removed) - len(kept); unreleased > 0 {
				logf("repositories: %d removed method(s) %s does not have (unreleased), not gated", unreleased, release.ref)
			}
			removed = kept
		}
		if len(removed) == 0 {
			return nil
		}
	}
	if !accept {
		return &buildGateRefusal{code: compiler.ErrCodeEmitterStep, op: "repository removal gate", err: repositoryRemovalError(backend, removed)}
	}
	logf("repositories: %d method(s) removed, accepted by --accept-removals", len(removed))
	return nil
}

// dryRunGates runs the gates of a real build against a dry run's output, so
// `--dry-run` and `--check` refuse what `ang build` would refuse instead of
// reporting success. backend is the project's backend directory, generated
// the dry run's copy of it.
func dryRunGates(backend, generated string, release releaseBaseline, acceptContract, acceptRemovals bool, logf func(string, ...any)) *buildGateRefusal {
	contractPath := filepath.Join(backend, "api", "openapi.yaml")
	previous, err := os.ReadFile(contractPath)
	switch {
	case err == nil:
		current, readErr := os.ReadFile(filepath.Join(generated, "api", "openapi.yaml"))
		if readErr != nil && !os.IsNotExist(readErr) {
			return &buildGateRefusal{code: compiler.ErrCodeEmitterStep, op: "read generated OpenAPI contract", err: readErr}
		}
		if readErr == nil {
			if refusal := checkContractGate(contractPath, previous, current, release, acceptContract, logf); refusal != nil {
				return refusal
			}
		}
	case !os.IsNotExist(err):
		return &buildGateRefusal{code: compiler.ErrCodeEmitterStep, op: "read previous OpenAPI contract", err: err}
	}
	previousRepos, err := collectRepositoryMethods(backend)
	if err != nil {
		logf("Warning: cannot read the previous repository interfaces in %s, the removed-methods check is skipped: %v", backend, err)
		return nil
	}
	if len(previousRepos) == 0 {
		return nil
	}
	return checkRepositoryGate(backend, previousRepos, generated, release, acceptRemovals, logf)
}
