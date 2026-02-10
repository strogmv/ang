package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

func runBuild(args []string) {
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

	buildTask := func() {
		fmt.Println("Compiling intent to Go...")

		parseArgs := args
		if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
			parseArgs = parseArgs[1:]
		}
		output, err := parseOutputOptions(parseArgs)
		if err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "parse output options", err)
			return
		}
		var dryRunTmpRoot string
		dryManifest := dryRunManifest{
			Status: "dry_run",
			Notes: []string{
				"No output files were written to intended build directories.",
			},
		}
		if output.DryRun {
			dryRunTmpRoot, err = os.MkdirTemp("", "ang-dry-run-*")
			if err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "create dry-run temp dir", err)
				return
			}
			defer os.RemoveAll(dryRunTmpRoot)
		}

		fail := func(stage compiler.Stage, code, op string, err error) {
			printStageFailure("Build FAILED", stage, code, op, err)
		}

		entities, services, endpoints, repos, events, bizErrors, schedules, scenarios, err := compiler.RunPipeline(projectPath)
		if err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err)
			return
		}
		if emitDiagnostics(os.Stderr, compiler.LatestDiagnostics) {
			fmt.Println("Build FAILED due to diagnostic errors.")
			return
		}
		_ = scenarios

		p := parser.New()
		n := normalizer.New()

		var cfgDef *normalizer.ConfigDef
		var authDef *normalizer.AuthDef
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/infra")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEInfraLoad, "load cue/infra", err)
			return
		} else if ok {
			cfgDef, err = n.ExtractConfig(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "extract config", err)
				return
			}
			authDef, err = n.ExtractAuth(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEInfraAuthParse, "extract auth", err)
				return
			}
		}

		var rbacDef *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/rbac")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUERBACLoad, "load cue/rbac", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUERBACParse, "extract rbac", err)
				return
			}
		} else if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/policies")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEPoliciesLoad, "load cue/policies", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEPoliciesParse, "extract policies as rbac", err)
				return
			}
		}

		var views []normalizer.ViewDef
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/views")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEViewsLoad, "load cue/views", err)
			return
		} else if ok {
			views, err = n.ExtractViews(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEViewsParse, "extract views", err)
				return
			}
		}

		var projectDef *normalizer.ProjectDef
		var targetDefs []normalizer.TargetDef
		var projectVal cue.Value
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/project")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEProjectLoad, "load cue/project", err)
			return
		} else if ok {
			projectVal = val
			projectDef, err = n.ExtractProject(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEProjectParse, "extract project", err)
				return
			}
			targetDefs, err = n.ExtractTargets(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUETargetsParse, "extract targets", err)
				return
			}
		}
		if len(targetDefs) == 0 {
			targetDefs = []normalizer.TargetDef{{
				Name:      "default",
				Lang:      "go",
				Framework: "chi",
				DB:        "postgres",
				Cache:     "redis",
				Queue:     "nats",
				Storage:   "s3",
			}}
		}

		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/schema")); err == nil && ok {
			if err := n.LoadCodegenConfig(val); err != nil {
				fmt.Printf("Warning: failed to load codegen config: %v\n", err)
			}
		}

		inputHash, _ := calculateHash([]string{"cue"})
		compilerHash, _ := calculateHash([]string{"templates"})

		goModule := readGoModule()
		if goModule == "" {
			goModule = "github.com/strogmv/ang"
		}

		pythonSDKEnabled := strings.TrimSpace(os.Getenv("ANG_PY_SDK")) == "1"

		var projectDefVal normalizer.ProjectDef
		if projectDef != nil {
			projectDefVal = *projectDef
		}
		var cfgDefVal normalizer.ConfigDef
		if cfgDef != nil {
			cfgDefVal = *cfgDef
		}

		irSchema, err := compiler.ConvertAndTransform(
			entities, services, events, bizErrors, endpoints, repos,
			cfgDefVal, authDef, rbacDef, schedules, views, projectDefVal,
		)
		if err != nil {
			fail(compiler.StageIR, compiler.ErrCodeIRConvertTransform, "convert and transform", err)
			return
		}

		if err := emitter.ValidateIRServiceDependencies(irSchema.Services); err != nil {
			fail(compiler.StageIR, compiler.ErrCodeIRServiceDependencies, "validate service dependencies", err)
			return
		}
		irSchema.Services = emitter.OrderIRServicesByDependencies(irSchema.Services)

		isMicroservice := projectHasBuildStrategy(projectVal, "microservices")

		selectedTargets := filterTargets(targetDefs, output.TargetSelector)
		if len(selectedTargets) == 0 {
			fmt.Printf("Build FAILED: no targets matched --target=%q\n", output.TargetSelector)
			fmt.Println("Available targets:")
			for _, td := range targetDefs {
				fmt.Printf("  - %s (%s/%s/%s)\n", td.Name, td.Lang, td.Framework, td.DB)
			}
			return
		}

		multiTarget := len(selectedTargets) > 1
		for _, td := range selectedTargets {
			intendedBackendDir := resolveBackendDirForTarget(output.BackendDir, td, multiTarget)
			intendedFrontendDir := resolveFrontendDirForTarget(output.FrontendDir, intendedBackendDir, td, multiTarget)
			backendDir := intendedBackendDir
			frontendDir := intendedFrontendDir
			if output.DryRun {
				safeName := safeTargetDirName(td.Name)
				backendDir = filepath.Join(dryRunTmpRoot, "backend", safeName)
				frontendDir = filepath.Join(dryRunTmpRoot, "frontend", safeName)
			}
			fmt.Printf("Generating target %s (%s/%s/%s) -> %s\n", td.Name, td.Lang, td.Framework, td.DB, backendDir)

			em := emitter.New(backendDir, frontendDir, "templates")
			em.FrontendAdminDir = output.FrontendAdminDir
			em.Version = compiler.Version
			em.InputHash = inputHash
			em.CompilerHash = compilerHash
			em.GoModule = goModule

			ctx := em.AnalyzeContextFromIR(irSchema)
			ctx.HasScheduler = len(irSchema.Schedules) > 0
			ctx.InputHash = inputHash
			ctx.CompilerHash = compilerHash
			ctx.ANGVersion = compiler.Version
			ctx.GoModule = goModule
			if authDef != nil {
				ctx.AuthService = authDef.Service
				ctx.AuthRefreshStore = authDef.RefreshStore
				if strings.EqualFold(authDef.RefreshStore, "redis") || strings.EqualFold(authDef.RefreshStore, "hybrid") {
					ctx.HasCache = true
				}
				if strings.EqualFold(authDef.RefreshStore, "hybrid") {
					ctx.HasSQL = true
				}
			}
			em.EnrichContextFromIR(&ctx, irSchema)

			caps, err := compiler.ResolveTargetCapabilities(td)
			if err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, fmt.Sprintf("resolve capabilities for target=%s", td.Name), err)
				return
			}

			targetOutput := output
			targetOutput.BackendDir = backendDir
			targetOutput.FrontendDir = frontendDir
			if output.DryRun {
				targetOutput.FrontendAppDir = ""
				targetOutput.FrontendAdminAppDir = ""
				targetOutput.FrontendEnvPath = ""
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(output.FrontendAppDir) != "" {
				targetOutput.FrontendAppDir = filepath.Join(output.FrontendAppDir, safeTargetDirName(td.Name))
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(output.FrontendAdminAppDir) != "" {
				targetOutput.FrontendAdminAppDir = filepath.Join(output.FrontendAdminAppDir, safeTargetDirName(td.Name))
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(output.FrontendEnvPath) != "" {
				targetOutput.FrontendEnvPath = filepath.Join(output.FrontendEnvPath, safeTargetDirName(td.Name), ".env.example")
			}

			registry := buildStepRegistry(buildStepRegistryInput{
				em:               em,
				irSchema:         irSchema,
				ctx:              ctx,
				scenarios:        scenarios,
				cfgDef:           cfgDef,
				authDef:          authDef,
				rbacDef:          rbacDef,
				projectDef:       projectDef,
				targetOutput:     targetOutput,
				pythonSDKEnabled: pythonSDKEnabled,
				isMicroservice:   isMicroservice,
			})
			if err := registry.Execute(td, caps, func(format string, args ...interface{}) {
				fmt.Printf(format+"\n", args...)
			}); err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "run capability matrix steps", err)
				return
			}

			if output.DryRun {
				backendChanges, err := buildDryRunChanges(backendDir, intendedBackendDir)
				if err != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "collect dry-run backend changes", err)
					return
				}
				frontendChanges, err := buildDryRunChanges(frontendDir, intendedFrontendDir)
				if err != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "collect dry-run frontend changes", err)
					return
				}
				combined := append(backendChanges, frontendChanges...)
				dryManifest.Targets = append(dryManifest.Targets, dryRunTargetManifest{
					Target:   td.Name,
					Lang:     td.Lang,
					Backend:  filepath.ToSlash(filepath.Clean(intendedBackendDir)),
					Frontend: filepath.ToSlash(filepath.Clean(intendedFrontendDir)),
					Changes:  combined,
				})
			}
		}

		if !output.DryRun {
			if err := runOptionalMCPGeneration(projectPath); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterMCPGen, "run optional MCP generation", err)
				return
			}
		} else {
			dryManifest.OptionalStepsSkipped = []string{"runOptionalMCPGeneration"}
		}

		if output.DryRun {
			summarizeDryRunManifest(&dryManifest)
			printDryRunManifest(dryManifest)
			fmt.Println("\nBuild DRY-RUN SUCCESSFUL.")
			return
		}

		fmt.Println("\nBuild SUCCESSFUL.")
	}

	if watch {
		fmt.Println("Live Mode: Watching for changes in cue/...")
		lastHash, _ := compiler.ComputeProjectHash(projectPath)
		buildTask()
		for {
			time.Sleep(1 * time.Second)
			newHash, _ := compiler.ComputeProjectHash(projectPath)
			if newHash != lastHash {
				lastHash = newHash
				buildTask()
			}
		}
	} else {
		buildTask()
	}
}
