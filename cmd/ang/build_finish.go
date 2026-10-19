package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

// syncGeneratedFileList plans (dry run) or updates (build) ang-generated.txt
// after all targets ran: what the build owns and what it no longer generates.
func (r *buildRun) syncGeneratedFileList() bool {
	ownedPaths := generatedTransactionPaths(r.projectPath, r.effectiveMode, r.intendedBackends, r.intendedFrontends)
	if r.output.DryRun {
		if err := planGeneratedFiles(r.projectPath, ownedPaths, &r.dryManifest, r.partialBuild); err != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "plan "+generatedFilesName, err)
			return false
		}
		return true
	}
	if r.generationRoot == "" {
		return true
	}
	synced, err := syncGeneratedFiles(r.projectPath, r.stagePath, ownedPaths, r.producedFiles, r.partialBuild)
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "update "+generatedFilesName, err)
		return false
	}
	if synced.Created {
		r.logText("Wrote %s: from the next build on, files ANG stops generating are removed from the project.", generatedFilesName)
	}
	if len(synced.Removed) > 0 {
		r.logText("Removing %d file(s) ANG no longer generates:", len(synced.Removed))
		for _, rel := range synced.Removed {
			r.logText("  - %s", rel)
		}
		r.event("emitters", "", "ok", fmt.Sprintf("removing %d file(s) ANG no longer generates: %s", len(synced.Removed), strings.Join(synced.Removed, ", ")))
	}
	for _, rel := range synced.Outside {
		r.logText("Warning: %s is no longer generated but lies outside the directories ANG manages; remove it yourself.", rel)
	}
	return true
}

// checkGates reports the emitters' diagnostics and, in a real build, refuses
// a breaking contract change or removed repository methods unless accepted
// (a dry run checked them per target in dryRunTarget).
func (r *buildRun) checkGates() bool {
	newEmitterDiagnostics := compiler.LatestDiagnostics[r.postEmitDiagStart:]
	if reportBuildDiagnostics(r.jsonLogs, r.output.Verbose, newEmitterDiagnostics) {
		return false
	}
	if r.output.DryRun {
		return true
	}
	release := resolveReleaseBaseline(r.output.ReleaseRef)
	for contractPath, previous := range r.contractBaselines {
		current, readErr := os.ReadFile(r.stagePath(contractPath))
		if readErr != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "read generated OpenAPI contract", readErr)
			return false
		}
		if refusal := checkContractGate(contractPath, previous, current, release, r.output.AcceptContract, r.logText); refusal != nil {
			r.fail(compiler.StageEmitters, refusal.code, refusal.op, refusal.err)
			return false
		}
	}
	for backend, previous := range r.repoBaselines {
		if refusal := checkRepositoryGate(backend, previous, r.stagePath(backend), release, r.output.AcceptRemovals, r.logText); refusal != nil {
			r.fail(compiler.StageEmitters, refusal.code, refusal.op, refusal.err)
			return false
		}
	}
	return true
}

// finishStaged is the end of a real build before the commit: hash manifest,
// optional MCP/OpenAPI generation, front typecheck, literal hoisting, go
// verification and tests, all on the staged workspace.
func (r *buildRun) finishStaged() bool {
	manifestTargets := make([]artifactManifestTarget, 0, len(r.summaries))
	for _, s := range r.summaries {
		manifestTargets = append(manifestTargets, artifactManifestTarget{
			Mode:     s.Mode,
			Backend:  s.Backend,
			Frontend: s.Frontend,
		})
	}
	manifestRoot := r.projectPath
	if r.buildWorkspace != "" {
		manifestRoot = r.buildWorkspace
		for i := range manifestTargets {
			manifestTargets[i].Backend = r.stagePath(manifestTargets[i].Backend)
			manifestTargets[i].Frontend = r.stagePath(manifestTargets[i].Frontend)
		}
	}
	if err := writeArtifactHashManifest(manifestRoot, manifestTargets, r.irSchema.IRVersion, r.inputHash, r.templateHash, r.compilerFingerprint); err != nil {
		printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "write artifact hash manifest", err)
		return false
	}

	if err := runOptionalMCPGeneration(manifestRoot); err != nil {
		printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterMCPGen, "run optional MCP generation", err)
		return false
	}
	if r.output.WithOpenAPI {
		openapiPath := filepath.Join(manifestRoot, "api", "openapi.yaml")
		openapiEm := &emitter.Emitter{
			OutputDir: filepath.Join(r.projectPath, "api"),
			Version:   compiler.Version,
		}
		normalized := r.compiled.Normalized
		if oErr := openapiEm.EmitOpenAPIFromNormalizerTypes(
			normalized.Endpoints,
			normalized.Services,
			normalized.Errors,
			&normalized.Project,
			openapiPath,
		); oErr != nil {
			r.logText("Warning: openapi generation failed: %v", oErr)
		} else {
			r.logText("OpenAPI spec written to %s", openapiPath)
		}
	}
	if err := runFrontendTypecheckGate(r.frontendTypecheckDirs); err != nil {
		printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "frontend typecheck gate", err)
		return false
	}
	goBackends := make([]string, 0, len(r.summaries))
	for _, s := range r.summaries {
		if strings.EqualFold(strings.TrimSpace(s.Lang), "go") {
			goBackends = append(goBackends, s.StagedBackend)
		}
	}
	// A literal CUE pasted into many operations is declared once: the
	// service package compiles in a third of the time. A dry run does
	// the same per target before it compares (see hoistGoLiterals).
	for _, backend := range goBackends {
		if _, hErr := hoistGoLiterals(backend, backend, r.logText); hErr != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "hoist repeated literals", hErr)
			return false
		}
	}
	if !r.output.SkipGoVerify {
		if err := runGeneratedGoVerify(goBackends); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "post-build go verify", err)
			return false
		}
	}
	if r.output.RunTests {
		if err := runGeneratedGoTests(goBackends); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "post-build go tests", err)
			return false
		}
	}
	return true
}

// finishDryRun writes the report and diffs a dry run asked for and, with
// --check, fails when any generated file differs from the project.
func (r *buildRun) finishDryRun() bool {
	output := r.output
	r.dryManifest.OptionalStepsSkipped = []string{"runOptionalMCPGeneration"}
	summarizeDryRunManifest(&r.dryManifest)
	if strings.TrimSpace(output.DryRunReport) != "" {
		data, err := json.MarshalIndent(r.dryManifest, "", "  ")
		if err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "marshal dry-run report", err)
			return false
		}
		data = append(data, '\n')
		if err := os.MkdirAll(filepath.Dir(output.DryRunReport), 0o755); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "mkdir dry-run report dir", err)
			return false
		}
		if err := os.WriteFile(output.DryRunReport, data, 0o644); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "write dry-run report", err)
			return false
		}
	}
	diffsPrinted := false
	if output.DryRunDiff {
		diffs := dryRunDiffs(r.dryManifest)
		if output.DryRunDiffOut != "" {
			if err := writeDryRunDiffs(output.DryRunDiffOut, diffs); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "write dry-run diffs", err)
				return false
			}
			fmt.Fprintf(os.Stderr, "Wrote %d diff(s) to %s\n", len(diffs), output.DryRunDiffOut)
		} else if !output.Check {
			// With --check the diffs are printed after the failure list below.
			printDryRunDiffs(os.Stdout, diffs)
			diffsPrinted = true
		}
	}
	if output.Check {
		return r.finishCheck()
	}
	if !diffsPrinted {
		printDryRunManifest(r.dryManifest)
	}
	r.logText("\nBuild DRY-RUN SUCCESSFUL.")
	if r.jsonLogs {
		r.event("build", "", "ok", "Dry-run build successful")
	}
	return true
}

// finishCheck is the --check verdict: every generated file (and the vendored
// schema) must match the project.
func (r *buildRun) finishCheck() bool {
	output := r.output
	differences := dryRunDifferences(r.dryManifest, r.projectPath)
	for _, change := range r.schemaStatus.Changes {
		differences = append(differences, dryRunFileChange{Path: change.Path, Action: change.Action})
	}
	if len(differences) > 0 {
		fmt.Fprintf(os.Stderr, "Check FAILED: %d of %d generated files differ from the project:\n", len(differences), r.dryManifest.TotalGenerated)
		for _, difference := range differences {
			marker := "~"
			if difference.Action == "create" {
				marker = "+"
			} else if difference.Action == "delete" {
				marker = "-"
			}
			fmt.Fprintf(os.Stderr, "  %s %s\n", marker, difference.Path)
		}
		fmt.Fprintln(os.Stderr, "Run ang build and commit the result.")
		if output.DryRunDiff && output.DryRunDiffOut == "" {
			fmt.Fprintln(os.Stderr)
			printDryRunDiffs(os.Stderr, dryRunDiffs(r.dryManifest))
		}
		if r.jsonLogs {
			r.event("check", "", "error", fmt.Sprintf("%d of %d generated files differ from the project", len(differences), r.dryManifest.TotalGenerated))
		}
		return false
	}
	r.logText("Check passed: all %d generated files match the project.", r.dryManifest.TotalGenerated)
	if r.jsonLogs {
		r.event("check", "", "ok", fmt.Sprintf("all %d generated files match the project", r.dryManifest.TotalGenerated))
	}
	return true
}

// commit moves the staged outputs into the project and reports the build.
func (r *buildRun) commit() bool {
	if r.transaction != nil {
		if buildBeforeCommitHook != nil {
			buildBeforeCommitHook(r.projectPath)
		}
		if r.buildWorkspace != "" {
			if err := r.transaction.CaptureWorkspace(r.projectPath, r.buildWorkspace); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "capture staged generated outputs", err)
				return false
			}
		}
		if err := r.transaction.Commit(); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "commit generated outputs", err)
			return false
		}
		if conflicts := r.transaction.Conflicts(); len(conflicts) > 0 {
			r.logText("Warning: %d file(s) were edited while the build ran and were also regenerated. The generated version was kept; your version is saved in %s:", len(conflicts), r.transaction.ConflictDir())
			for _, path := range conflicts {
				r.logText("  - %s", path)
			}
		}
	}

	r.logText("\nBuild SUCCESSFUL in %s.", time.Since(r.started).Round(100*time.Millisecond))
	r.logText("Build Report:")
	for _, s := range r.summaries {
		r.logText("  - target=%s mode=%s backend=%s plugins=%s self-check=%s", s.Name, s.Mode, s.Backend, s.Plugins, s.SelfCheck)
		for _, d := range s.Details {
			r.logText("      %s -> %s", d.Package, d.Dir)
		}
	}
	if suggestDoctor, hint := buildDoctorHint(r.projectPath); suggestDoctor {
		r.logText("%s", hint)
	}
	if r.jsonLogs {
		r.event("build", "", "ok", "Build successful")
	}
	return true
}
