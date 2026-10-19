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

func runBuild(args []string) error {
	watch := false
	for _, arg := range args {
		if arg == "-w" || arg == "--watch" {
			watch = true
			break
		}
	}

	projectPath := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
	}

	buildTask := func() (succeeded bool) {
		parseArgs := args
		if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
			parseArgs = parseArgs[1:]
		}
		output, err := parseOutputOptions(parseArgs)
		if err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "parse output options", err)
			return
		}
		projectPath := projectPath
		if output.ProjectDir != "" {
			projectPath = output.ProjectDir
		}
		// Guard before anything is written: the target must be an ANG project.
		if err := ensureGeneratableProject(projectPath, loadProjectConfig(projectPath).CueRoot); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "resolve project directory", err)
			return
		}
		// Guard before anything is generated: the binary must be the revision the
		// project pinned in ang.lock, or the committed API contract can silently
		// change. This runs before the plan/apply phases branch off, so they are
		// guarded too; warnings go to stderr so they never mix into the build event
		// stream written to stdout.
		if pin, pinned, err := readANGLock(projectPath); err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeANGLockMismatch, "read ang.lock", err)
			return
		} else if pinned {
			warnings, lockErr := checkANGLock(pin, runningCompilerRevision(), output.AllowLockMismatch)
			if lockErr != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeANGLockMismatch, "verify compiler revision against ang.lock", lockErr)
				return
			}
			for _, warning := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
		}
		// The project's vendored CUE schema is this binary's (the revision the
		// lock pins): a build writes it, a dry run compares and --check fails
		// on a difference (docs/schema.md).
		schemaStatus, err := syncSchemaForBuild(projectPath, loadProjectConfig(projectPath).CueRoot, output.DryRun)
		if err != nil {
			printStageFailure("Build FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "vendor the CUE schema", err)
			return
		}
		if schemaStatus.Applies && !schemaStatus.InSync() {
			if output.DryRun {
				if !output.Check {
					fmt.Fprintf(os.Stderr, "Warning [%s]: %s differs from this ANG's schema in %d files; ang build (or ang schema sync) rewrites it.\n", codeSchemaStale, schemaStatus.Dir, len(schemaStatus.Changes))
				}
			} else {
				fmt.Fprintf(os.Stderr, "Schema: %s synced with this ANG (%d files).\n", schemaStatus.Dir, len(schemaStatus.Changes))
			}
		}
		if output.Phase == "plan" || output.Phase == "apply" {
			phase := compiler.PhaseAll
			switch output.Phase {
			case "plan":
				phase = compiler.PhasePlan
			case "apply":
				phase = compiler.PhaseApply
			}
			p, err := compiler.RunWithOptions(projectPath, compiler.RunOptions{
				Phase:    phase,
				PlanFile: output.PlanFile,
				OutPlan:  output.OutPlan,
			})
			if err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "phase execution", err)
				return
			}
			if output.PlanJSON {
				data, err := json.MarshalIndent(p, "", "  ")
				if err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "marshal plan", err)
					return
				}
				fmt.Println(string(data))
			} else {
				fmt.Printf("Plan status: %s\n", p.Status)
				if output.OutPlan != "" {
					fmt.Printf("Plan written: %s\n", output.OutPlan)
				}
			}
			return true
		}
		return runBuildStages(newBuildRun(output, projectPath, schemaStatus))
	}

	if watch {
		watchCueRoot := loadProjectConfig(projectPath).CueRoot
		fmt.Printf("Live Mode: Watching for changes in %s/...\n", watchCueRoot)
		lastHash, _ := compiler.ComputeProjectHashWithRoot(projectPath, watchCueRoot)
		buildTask()
		for {
			time.Sleep(1 * time.Second)
			newHash, _ := compiler.ComputeProjectHashWithRoot(projectPath, watchCueRoot)
			if newHash != lastHash {
				lastHash = newHash
				buildTask()
			}
		}
	} else {
		if !buildTask() {
			return fmt.Errorf("build failed")
		}
	}
	return nil
}

func buildDoctorHint(projectPath string) (bool, string) {
	checks, err := collectStartupChecks(projectPath, true)
	if err != nil {
		return false, ""
	}
	needsDoctor := false
	for _, c := range checks {
		if c.Status != startupFail {
			continue
		}
		if strings.HasPrefix(c.Name, "tool:") || c.Name == "config-env" || c.Name == ".env.example" {
			needsDoctor = true
			break
		}
	}
	if !needsDoctor {
		return false, ""
	}
	preflightPath := filepath.Join(projectPath, "scripts", "preflight.sh")
	if _, err := os.Stat(preflightPath); err == nil {
		return true, "Hint: run `make doctor` (or `bash scripts/preflight.sh`) before first start to verify dependencies and .env."
	}
	return true, "Hint: run `ang doctor start` before first start to verify dependencies and .env."
}

// runBuildStages runs a build (or dry run, or --check) from the compile to the
// commit; each stage reports its own failure.
func runBuildStages(r *buildRun) bool {
	emitter.VerboseGenerated = r.output.Verbose && !r.jsonLogs
	r.logText("Compiling intent to Go...")
	if r.jsonLogs {
		r.event("build", "", "start", "Build started")
	}
	cleanupDryRun, ok := r.prepareDryRunRoot()
	defer cleanupDryRun()
	if !ok || !r.compile() {
		return false
	}
	rollback, ok := r.beginTransaction()
	defer rollback()
	if !ok {
		return false
	}
	for _, td := range r.selectedTargets {
		if !r.generateTarget(td) {
			return false
		}
	}
	if !r.syncGeneratedFileList() || !r.checkGates() {
		return false
	}
	if r.output.DryRun {
		return r.finishDryRun()
	}
	return r.finishStaged() && r.commit()
}
