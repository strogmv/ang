package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/schemavendor"
)

// buildRun is one run of `ang build` (one rebuild in watch mode) after the
// options are parsed and the guards passed: what the stages share and what the
// per-target loop (generateTarget) accumulates for the ends (finishDryRun,
// finishApply). Every stage reports its own failure and returns false.
type buildRun struct {
	output       OutputOptions
	projectPath  string
	schemaStatus schemavendor.Status
	started      time.Time

	jsonLogs     bool
	logText      func(format string, args ...any)
	logEvent     func(ev buildEvent)
	logStepEvent func(ev generator.StepEvent)

	in                  buildInputs
	compiled            compiler.CompileForEmitOutput
	irSchema            *ir.Schema
	inputHash           string
	templateHash        string
	compilerFingerprint string
	goModule            string
	postEmitDiagStart   int
	isMicroservice      bool
	effectiveMode       string
	selectedTargets     []normalizer.TargetDef
	multiTarget         bool
	// A build that skipped targets or parts did not produce their files, so
	// it must not treat them as no longer generated.
	partialBuild bool

	dryRunTmpRoot string
	dryManifest   dryRunManifest

	transaction    *buildTransaction
	buildWorkspace string
	generationRoot string
	// The committed OpenAPI contract and the repository interfaces of the
	// previous generation, by path and backend dir: the gates compare with them.
	contractBaselines map[string][]byte
	repoBaselines     map[string]repositoryMethods

	summaries             []buildTargetSummary
	frontendTypecheckDirs []string
	intendedBackends      []string
	intendedFrontends     []string
	producedFiles         map[string]struct{}
}

type buildTargetSummary struct {
	Name          string
	Lang          string
	Mode          string
	Backend       string
	StagedBackend string
	Frontend      string
	Plugins       string
	SelfCheck     string
	Details       []runtimePackageDir
}

func newBuildRun(output OutputOptions, projectPath string, schemaStatus schemavendor.Status) *buildRun {
	// Piped output defaults to JSON events; an explicit --log-format text
	// keeps text for tools and agents that read it.
	jsonLogs := output.LogFormat == "json" || output.PlanJSON || (!stdoutIsTerminal() && !output.LogFormatExplicit)
	r := &buildRun{
		output:       output,
		projectPath:  projectPath,
		schemaStatus: schemaStatus,
		started:      time.Now(),
		jsonLogs:     jsonLogs,
		dryManifest: dryRunManifest{
			Status: "dry_run",
			Notes: []string{
				"No output files were written to intended build directories.",
			},
		},
		contractBaselines: map[string][]byte{},
		repoBaselines:     map[string]repositoryMethods{},
		producedFiles:     map[string]struct{}{},
	}
	r.logText = func(format string, args ...any) {
		if !jsonLogs {
			fmt.Printf(format+"\n", args...)
		}
	}
	r.logEvent = func(ev buildEvent) {
		if jsonLogs {
			emitBuildEvent(ev)
		}
	}
	r.logStepEvent = func(ev generator.StepEvent) {
		if jsonLogs {
			emitBuildEvent(mapStepEvent(ev))
		}
	}
	return r
}

func (r *buildRun) fail(stage compiler.Stage, code, op string, err error) {
	printBootstrapGuidanceIfNeeded(r.projectPath, stage, code, err)
	printStageFailure("Build FAILED", stage, code, op, err)
}

// event logs a build event stamped now (JSON logs only).
func (r *buildRun) event(stage, target, status, message string) {
	r.logEvent(buildEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Stage:     stage,
		Target:    target,
		Status:    status,
		Message:   message,
	})
}

// prepareDryRunRoot creates the directory a dry run generates into; cleanup
// removes a temporary one (an explicit --dry-run-root is kept).
func (r *buildRun) prepareDryRunRoot() (cleanup func(), ok bool) {
	cleanup = func() {}
	if !r.output.DryRun {
		return cleanup, true
	}
	if strings.TrimSpace(r.output.DryRunRoot) != "" {
		r.dryRunTmpRoot = filepath.Clean(r.output.DryRunRoot)
		if err := os.RemoveAll(r.dryRunTmpRoot); err != nil && !os.IsNotExist(err) {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "reset dry-run temp dir", err)
			return cleanup, false
		}
		if err := os.MkdirAll(r.dryRunTmpRoot, 0o755); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "create dry-run temp dir", err)
			return cleanup, false
		}
		return cleanup, true
	}
	root, err := os.MkdirTemp("", "ang-dry-run-*")
	if err != nil {
		printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "create dry-run temp dir", err)
		return cleanup, false
	}
	r.dryRunTmpRoot = root
	return func() { os.RemoveAll(root) }, true
}

// compile loads the inputs, compiles the CUE for the emitters and resolves the
// build mode and the targets to generate.
func (r *buildRun) compile() bool {
	in, failure := loadBuildInputs(r.projectPath)
	if failure != nil {
		r.fail(failure.stage, failure.code, failure.op, failure.err)
		return false
	}
	r.in = in

	inputHash, err := calculateHash([]string{filepath.Join(r.projectPath, in.cueRoot)})
	if err != nil {
		r.fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "calculate CUE input hash", err)
		return false
	}
	r.inputHash = inputHash
	templateHash, err := calculateEmbeddedTemplateHash()
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "calculate embedded template hash", err)
		return false
	}
	r.templateHash = templateHash
	r.compilerFingerprint = compiler.BuildFingerprint()

	r.goModule = readGoModuleAt(r.projectPath)
	if r.goModule == "" {
		r.goModule = "github.com/strogmv/ang"
	}

	var projectDefVal normalizer.ProjectDef
	if in.projectDef != nil {
		projectDefVal = *in.projectDef
	}
	var cfgDefVal normalizer.ConfigDef
	if in.cfgDef != nil {
		cfgDefVal = *in.cfgDef
	}

	compiled, err := compiler.CompileForEmit(r.projectPath, compiler.PipelineOptions{CueRoot: in.cueRoot}, compiler.CompileForEmitOptions{
		Config:      cfgDefVal,
		Auth:        in.authDef,
		RBAC:        in.rbacDef,
		Views:       in.views,
		Project:     projectDefVal,
		InfraValues: in.infraValues,
		Templates:   in.templatesCatalog,
	})
	if err != nil {
		if ce, ok := err.(*compiler.ContractError); ok {
			r.fail(ce.Stage, ce.Code, ce.Op, ce.Err)
			return false
		}
		r.fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "compile for emit", err)
		return false
	}
	if reportBuildDiagnostics(r.jsonLogs, r.output.Verbose, compiler.LatestDiagnostics) {
		return false
	}
	r.postEmitDiagStart = len(compiler.LatestDiagnostics)
	r.compiled = compiled
	r.irSchema = compiled.IR

	r.isMicroservice = projectHasBuildStrategy(in.projectVal, "microservices")
	r.effectiveMode = resolveBuildMode(r.output.Mode, in.projectVal, r.output.BackendDirExplicit)
	if r.output.Mode == "" && r.effectiveMode == "in_place" {
		if hasDeprecatedOutputDirConfig(in.targetDefs) {
			fmt.Println("Warning: targets[].output_dir without explicit --mode/build.mode is deprecated.")
			fmt.Println("Migration: set build.mode: \"release\" (keep output_dir) or build.mode: \"in_place\" and use --backend-dir.")
		}
	}

	r.selectedTargets = filterTargets(in.targetDefs, r.output.TargetSelector)
	if len(r.selectedTargets) == 0 {
		r.logText("Build FAILED: no targets matched --target=%q", r.output.TargetSelector)
		r.logText("Available targets:")
		for _, td := range in.targetDefs {
			r.logText("  - %s (%s/%s/%s)", td.Name, td.Lang, td.Framework, td.DB)
		}
		return false
	}
	if err := validateBuildMode(r.effectiveMode, r.output, r.selectedTargets); err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "validate output mode", err)
		return false
	}
	r.multiTarget = len(r.selectedTargets) > 1
	r.partialBuild = r.output.SkipFrontend || r.output.SkipContractTests || len(r.selectedTargets) != len(in.targetDefs)
	return true
}

// beginTransaction (a real build only) records the gate baselines, snapshots
// the generated outputs and creates the staged workspace the emitters write to.
// rollback restores the outputs unless the transaction was committed.
func (r *buildRun) beginTransaction() (rollback func(), ok bool) {
	rollback = func() {}
	if r.output.DryRun {
		return rollback, true
	}
	backendDirs := make([]string, 0, len(r.selectedTargets))
	frontendDirs := make([]string, 0, len(r.selectedTargets))
	for _, td := range r.selectedTargets {
		backend := resolveBackendDirForTarget(r.effectiveMode, r.output.BackendDir, td, r.multiTarget)
		frontend := resolveFrontendDirForTarget(frontendDirOptionForTarget(r.output, td), backend, td, r.multiTarget)
		if !filepath.IsAbs(backend) {
			backend = filepath.Join(r.projectPath, backend)
		}
		if !filepath.IsAbs(frontend) {
			frontend = filepath.Join(r.projectPath, frontend)
		}
		backendDirs = append(backendDirs, backend)
		frontendDirs = append(frontendDirs, frontend)
		contractPath := filepath.Join(backend, "api", "openapi.yaml")
		if data, readErr := os.ReadFile(contractPath); readErr == nil {
			r.contractBaselines[contractPath] = data
		} else if !os.IsNotExist(readErr) {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "read previous OpenAPI contract", readErr)
			return rollback, false
		}
		if previousRepos, repoErr := collectRepositoryMethods(backend); repoErr != nil {
			r.logText("Warning: cannot read the previous repository interfaces in %s, the removed-methods check is skipped: %v", backend, repoErr)
		} else if len(previousRepos) > 0 {
			r.repoBaselines[backend] = previousRepos
		}

		frontendAppDir := strings.TrimSpace(r.output.FrontendAppDir)
		if frontendAppDir == "" {
			frontendAppDir = strings.TrimSpace(td.FrontendAppDir)
		}
		if envDir := strings.TrimSpace(os.ExpandEnv(os.Getenv("ANG_FRONTEND_APP_DIR"))); envDir != "" {
			frontendAppDir = envDir
		}
		for _, externalOutput := range []string{frontendAppDir, r.output.FrontendAdminAppDir, r.output.FrontendEnvPath} {
			externalOutput = strings.TrimSpace(os.ExpandEnv(externalOutput))
			if externalOutput == "" {
				continue
			}
			if !filepath.IsAbs(externalOutput) {
				externalOutput = filepath.Join(r.projectPath, externalOutput)
			}
			if r.multiTarget {
				externalOutput = filepath.Join(externalOutput, safeTargetDirName(td.Name))
			}
			frontendDirs = append(frontendDirs, externalOutput)
		}
	}
	transaction, err := beginBuildTransaction(generatedTransactionPaths(r.projectPath, r.effectiveMode, backendDirs, frontendDirs))
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "snapshot generated outputs", err)
		return rollback, false
	}
	r.transaction = transaction
	r.buildWorkspace, err = transaction.CreateWorkspace(r.projectPath)
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "create staged project workspace", err)
		return rollback, false
	}
	// Emitters write into empty directories here; their output is then
	// applied to the workspace. See ang-generated.txt.
	r.generationRoot = filepath.Join(filepath.Dir(r.buildWorkspace), "generated")
	transaction.SetConflictDir(r.projectPath, filepath.Join(r.projectPath, ".ang", "conflicts", time.Now().UTC().Format("20060102T150405Z")))
	if swept := transaction.Swept(); swept.Removed > 0 {
		noun := "directories"
		if swept.Removed == 1 {
			noun = "directory"
		}
		r.logText("Removed %d abandoned build scratch %s (%s).", swept.Removed, noun, formatScratchBytes(swept.Bytes))
	}
	for _, dir := range transaction.Swept().Kept {
		r.logText("Warning: %s holds project originals from a rollback that failed; it was kept, inspect it and delete it yourself.", dir)
	}
	return func() {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			printStageFailure("Build ROLLBACK FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "restore generated outputs", rollbackErr)
		}
	}, true
}

// stagePath maps a project path to where this build writes it: the staged
// workspace for paths inside the project, the transaction's staging otherwise;
// a dry run (no transaction) keeps the path.
func (r *buildRun) stagePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if r.transaction == nil {
		return path
	}
	if r.buildWorkspace != "" {
		projectAbs, _ := filepath.Abs(r.projectPath)
		pathAbs, _ := filepath.Abs(path)
		if rel, relErr := filepath.Rel(projectAbs, pathAbs); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.Join(r.buildWorkspace, rel)
		}
	}
	return r.transaction.StagePath(path)
}
