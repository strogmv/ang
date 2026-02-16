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
	"github.com/strogmv/ang/compiler/generator"
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
		parseArgs := args
		if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
			parseArgs = parseArgs[1:]
		}
		output, err := parseOutputOptions(parseArgs)
		if err != nil {
			printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "parse output options", err)
			return
		}
		jsonLogs := output.LogFormat == "json"
		logText := func(format string, args ...any) {
			if !jsonLogs {
				fmt.Printf(format+"\n", args...)
			}
		}
		logEvent := func(ev buildEvent) {
			if jsonLogs {
				emitBuildEvent(ev)
			}
		}
		logStepEvent := func(ev generator.StepEvent) {
			if jsonLogs {
				emitBuildEvent(mapStepEvent(ev))
			}
		}
		logText("Compiling intent to Go...")
		if jsonLogs {
			logEvent(buildEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Stage:     "build",
				Status:    "start",
				Message:   "Build started",
			})
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
			if jsonLogs {
				logEvent(buildEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Stage:     string(stage),
					Status:    "error",
					Error:     fmt.Sprintf("%s: %s: %v", code, op, err),
				})
			}
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
		var emailTemplates []normalizer.EmailTemplateDef
		var templatesCatalog []normalizer.TemplateDef
		var infraValues map[string]any
		var infraContextPatch normalizer.InfraContextPatch
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/infra")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEInfraLoad, "load cue/infra", err)
			return
		} else if ok {
			infraRegistry := normalizer.NewInfraRegistry()
			infraValues, err = infraRegistry.ExtractAll(n, val)
			if err != nil {
				if infraErr, ok := err.(*normalizer.InfraExtractError); ok {
					code, op, unwrapErr := infraErr.FailParams()
					fail(compiler.StageCUE, code, op, unwrapErr)
					return
				}
				fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "extract infrastructure definitions", err)
				return
			}
			cfgDef = normalizer.InfraConfig(infraValues)
			authDef = normalizer.InfraAuth(infraValues)
			infraContextPatch = infraRegistry.BuildContextPatch(infraValues)
			templatesCatalog, err = n.ExtractTemplates(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "extract templates", err)
				return
			}
			templatesCatalog, err = resolveTemplates(projectPath, templatesCatalog)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "resolve templates", err)
				return
			}
			if len(templatesCatalog) > 0 {
				emailTemplates, err = templatesToEmail(templatesCatalog)
				if err != nil {
					fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "map templates to email templates", err)
					return
				}
			} else {
				emailTemplates, err = n.ExtractEmailTemplates(val)
				if err != nil {
					fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "extract email templates", err)
					return
				}
				emailTemplates, err = resolveEmailTemplates(projectPath, emailTemplates)
				if err != nil {
					fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "resolve email templates", err)
					return
				}
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
		compiler.AttachNotificationInfra(
			irSchema,
			normalizer.InfraNotificationChannels(infraValues),
			normalizer.InfraNotificationPolicies(infraValues),
		)
		compiler.AttachTemplates(irSchema, templatesCatalog)
		if err := compiler.ValidateIRSemantics(irSchema); err != nil {
			fail(compiler.StageIR, compiler.ErrCodeIRSemanticValidate, "validate IR semantics", err)
			return
		}

		if err := emitter.ValidateIRServiceDependencies(irSchema.Services); err != nil {
			fail(compiler.StageIR, compiler.ErrCodeIRServiceDependencies, "validate service dependencies", err)
			return
		}
		irSchema.Services = emitter.OrderIRServicesByDependencies(irSchema.Services)

		isMicroservice := projectHasBuildStrategy(projectVal, "microservices")
		if output.Mode == "" {
			if hasDeprecatedOutputDirConfig(targetDefs) {
				fmt.Println("Warning: targets[].output_dir without explicit --mode/build.mode is deprecated.")
				fmt.Println("Migration: set build.mode: \"release\" (keep output_dir) or build.mode: \"in_place\" and use --backend-dir.")
			}
		}

		selectedTargets := filterTargets(targetDefs, output.TargetSelector)
		if len(selectedTargets) == 0 {
			logText("Build FAILED: no targets matched --target=%q", output.TargetSelector)
			logText("Available targets:")
			for _, td := range targetDefs {
				logText("  - %s (%s/%s/%s)", td.Name, td.Lang, td.Framework, td.DB)
			}
			return
		}
		effectiveMode := resolveBuildMode(output.Mode, projectVal, output.BackendDirExplicit)
		if err := validateBuildMode(effectiveMode, output, selectedTargets); err != nil {
			fail(compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "validate output mode", err)
			return
		}

		multiTarget := len(selectedTargets) > 1
		type buildTargetSummary struct {
			Name      string
			Mode      string
			Backend   string
			Plugins   string
			SelfCheck string
			Details   []runtimePackageDir
		}
		summaries := make([]buildTargetSummary, 0, len(selectedTargets))
		frontendTypecheckDirs := make([]string, 0, len(selectedTargets))
		for _, td := range selectedTargets {
			intendedBackendDir := resolveBackendDirForTarget(effectiveMode, output.BackendDir, td, multiTarget)
			intendedFrontendDir := resolveFrontendDirForTarget(output.FrontendDir, intendedBackendDir, td, multiTarget)
			if !filepath.IsAbs(intendedBackendDir) {
				intendedBackendDir = filepath.Join(projectPath, intendedBackendDir)
			}
			if !filepath.IsAbs(intendedFrontendDir) {
				intendedFrontendDir = filepath.Join(projectPath, intendedFrontendDir)
			}
			backendDir := intendedBackendDir
			frontendDir := intendedFrontendDir
			if output.DryRun {
				safeName := safeTargetDirName(td.Name)
				backendDir = filepath.Join(dryRunTmpRoot, "backend", safeName)
				frontendDir = filepath.Join(dryRunTmpRoot, "frontend", safeName)
			} else {
				frontendTypecheckDirs = append(frontendTypecheckDirs, intendedFrontendDir)
			}
			logText("Generating target %s (%s/%s/%s) -> %s", td.Name, td.Lang, td.Framework, td.DB, backendDir)
			if jsonLogs {
				logEvent(buildEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Stage:     "emitters",
					Target:    td.Name,
					Status:    "start",
					Message:   "Target generation started",
				})
			}

			em := emitter.New(backendDir, frontendDir, "templates")
			em.FrontendAdminDir = output.FrontendAdminDir
			if projectDef != nil && strings.TrimSpace(projectDef.UIProvider) != "" {
				em.UIProviderPath = strings.TrimSpace(projectDef.UIProvider)
			}
			em.Version = compiler.Version
			em.InputHash = inputHash
			em.CompilerHash = compilerHash
			em.GoModule = goModule
			em.IRSchema = irSchema

			ctx := em.AnalyzeContextFromIR(irSchema)
			ctx.HasScheduler = len(irSchema.Schedules) > 0
			ctx.InputHash = inputHash
			ctx.CompilerHash = compilerHash
			ctx.ANGVersion = compiler.Version
			ctx.GoModule = goModule
			if infraContextPatch.AuthService != "" {
				ctx.AuthService = infraContextPatch.AuthService
			}
			if infraContextPatch.AuthRefreshStore != "" {
				ctx.AuthRefreshStore = infraContextPatch.AuthRefreshStore
			}
			if infraContextPatch.ForceHasCache {
				ctx.HasCache = true
			}
			if infraContextPatch.ForceHasSQL {
				ctx.HasSQL = true
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

			if infraContextPatch.NotificationMuting {
				ctx.NotificationMuting = true
			}

			registry, pluginNames, err := buildStepRegistry(buildStepRegistryInput{
				em:               em,
				irSchema:         irSchema,
				ctx:              ctx,
				scenarios:        scenarios,
				cfgDef:           cfgDef,
				authDef:          authDef,
				rbacDef:          rbacDef,
				infraValues:      infraValues,
				emailTemplates:   emailTemplates,
				projectDef:       projectDef,
				targetOutput:     targetOutput,
				pythonSDKEnabled: pythonSDKEnabled,
				isMicroservice:   isMicroservice,
			})
			if err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "resolve target plugins", err)
				return
			}
			if err := registry.Execute(td, caps, func(format string, args ...interface{}) {
				logText(format, args...)
			}, logStepEvent); err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "run capability matrix steps", err)
				return
			}
			if jsonLogs {
				logEvent(buildEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Stage:     "emitters",
					Target:    td.Name,
					Status:    "ok",
					Message:   "Target generation finished",
				})
			}

			if !output.DryRun && td.Lang == "go" && effectiveMode == "release" {
				if err := ensureReleaseGoModule(backendDir, goModule); err != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "ensure release go.mod", err)
					return
				}
			}

			selfCheckStatus := "skipped"
			var selfCheckDetails []runtimePackageDir
			if !output.DryRun && strings.EqualFold(td.Lang, "go") {
				checkRes, err := runGoRuntimeSelfCheck(projectPath, backendDir, effectiveMode)
				if err != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "runtime source self-check", err)
					return
				}
				selfCheckStatus = checkRes.Status
				selfCheckDetails = checkRes.Resolved
			}

			if len(em.MissingImpls) > 0 {
				fmt.Println("\n⚠️  MISSING IMPLEMENTATIONS (Blind Spots):")
				for _, m := range em.MissingImpls {
					fmt.Printf("   - %s.%s (at %s)\n", m.Service, m.Method, m.Source)
				}
			}
			summaries = append(summaries, buildTargetSummary{
				Name:      td.Name,
				Mode:      effectiveMode,
				Backend:   filepath.ToSlash(filepath.Clean(backendDir)),
				Plugins:   joinPluginNames(pluginNames),
				SelfCheck: selfCheckStatus,
				Details:   selfCheckDetails,
			})

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
			if err := runFrontendTypecheckGate(frontendTypecheckDirs); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "frontend typecheck gate", err)
				return
			}
		} else {
			dryManifest.OptionalStepsSkipped = []string{"runOptionalMCPGeneration"}
		}

		if output.DryRun {
			summarizeDryRunManifest(&dryManifest)
			printDryRunManifest(dryManifest)
			logText("\nBuild DRY-RUN SUCCESSFUL.")
			if jsonLogs {
				logEvent(buildEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Stage:     "build",
					Status:    "ok",
					Message:   "Dry-run build successful",
				})
			}
			return
		}

		logText("\nBuild SUCCESSFUL.")
		logText("Build Report:")
		for _, s := range summaries {
			logText("  - target=%s mode=%s backend=%s plugins=%s self-check=%s", s.Name, s.Mode, s.Backend, s.Plugins, s.SelfCheck)
			for _, d := range s.Details {
				logText("      %s -> %s", d.Package, d.Dir)
			}
		}
		if jsonLogs {
			logEvent(buildEvent{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Stage:     "build",
				Status:    "ok",
				Message:   "Build successful",
			})
		}
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
