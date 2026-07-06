package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang-ir/parser"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/paymentprovider"
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
			return
		}
		jsonLogs := output.LogFormat == "json" || output.PlanJSON || !stdoutIsTerminal()
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
			if strings.TrimSpace(output.DryRunRoot) != "" {
				dryRunTmpRoot = filepath.Clean(output.DryRunRoot)
				if err := os.RemoveAll(dryRunTmpRoot); err != nil && !os.IsNotExist(err) {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "reset dry-run temp dir", err)
					return
				}
				if err := os.MkdirAll(dryRunTmpRoot, 0o755); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "create dry-run temp dir", err)
					return
				}
			} else {
				dryRunTmpRoot, err = os.MkdirTemp("", "ang-dry-run-*")
				if err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "create dry-run temp dir", err)
					return
				}
				defer os.RemoveAll(dryRunTmpRoot)
			}
		}

		fail := func(stage compiler.Stage, code, op string, err error) {
			printBootstrapGuidanceIfNeeded(projectPath, stage, code, err)
			printStageFailure("Build FAILED", stage, code, op, err)
		}

		p := parser.New()
		n := normalizer.New()

		var cfgDef *normalizer.ConfigDef
		var authDef *normalizer.AuthDef
		var sessionDef *normalizer.SessionDef
		var emailTemplates []normalizer.EmailTemplateDef
		var templatesCatalog []normalizer.TemplateDef
		var infraValues map[string]any
		var infraContextPatch normalizer.InfraContextPatch
		projPath := resolvePaymentProviderProjectPath(projectPath, output)
		projCfg := loadProjectConfig(projPath)
		cueRoot := projCfg.CueRoot
		tmplDir := projCfg.TemplatesDir

		if paymentprovider.IsProject(projPath, cueRoot) {
			logText("Building payment provider from CUE intent...")
			if err := paymentprovider.Build(paymentprovider.BuildOptions{
				ProjectPath:  projPath,
				CueRoot:      cueRoot,
				TemplatesDir: tmplDir,
			}); err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "build payment provider", err)
				return
			}
			logText("\nBuild SUCCESSFUL (payment provider).")
			if jsonLogs {
				logEvent(buildEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					Stage:     "build",
					Status:    "ok",
					Message:   "Payment provider build successful",
				})
			}
			return
		}

		infraBundle, err := compiler.LoadInfraBundleWithRoot(projectPath, cueRoot)
		if err != nil {
			if ce, ok := err.(*compiler.ContractError); ok {
				fail(ce.Stage, ce.Code, ce.Op, ce.Err)
				return
			}
			fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "load infrastructure bundle", err)
			return
		}
		if infraBundle.Has {
			infraValues = infraBundle.Values
			cfgDef = infraBundle.Config
			authDef = infraBundle.Auth
			sessionDef = infraBundle.Session
			infraContextPatch = infraBundle.ContextPatch
			templatesCatalog = infraBundle.Templates
			emailTemplates = infraBundle.EmailTemplates
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
				emailTemplates, err = resolveEmailTemplates(projectPath, emailTemplates)
				if err != nil {
					fail(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "resolve email templates", err)
					return
				}
			}
		}

		var rbacDef *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "rbac")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUERBACLoad, "load "+cueRoot+"/rbac", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUERBACParse, "extract rbac", err)
				return
			}
		} else if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "policies")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEPoliciesLoad, "load "+cueRoot+"/policies", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fail(compiler.StageCUE, compiler.ErrCodeCUEPoliciesParse, "extract policies as rbac", err)
				return
			}
		}

		var views []normalizer.ViewDef
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "views")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEViewsLoad, "load "+cueRoot+"/views", err)
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
		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "project")); err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEProjectLoad, "load "+cueRoot+"/project", err)
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

		if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "schema")); err == nil && ok {
			if err := n.LoadCodegenConfig(val); err != nil {
				fmt.Printf("Warning: failed to load codegen config: %v\n", err)
			}
		}

		inputHash, err := calculateHash([]string{filepath.Join(projectPath, cueRoot)})
		if err != nil {
			fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "calculate CUE input hash", err)
			return
		}
		templateHash, err := calculateEmbeddedTemplateHash()
		if err != nil {
			fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "calculate embedded template hash", err)
			return
		}
		compilerFingerprint := compiler.BuildFingerprint()

		goModule := readGoModuleAt(projectPath)
		if goModule == "" {
			goModule = "github.com/strogmv/ang"
		}

		var projectDefVal normalizer.ProjectDef
		if projectDef != nil {
			projectDefVal = *projectDef
		}
		var cfgDefVal normalizer.ConfigDef
		if cfgDef != nil {
			cfgDefVal = *cfgDef
		}

		compiled, err := compiler.CompileForEmit(projectPath, compiler.PipelineOptions{CueRoot: cueRoot}, compiler.CompileForEmitOptions{
			Config:      cfgDefVal,
			Auth:        authDef,
			RBAC:        rbacDef,
			Views:       views,
			Project:     projectDefVal,
			InfraValues: infraValues,
			Templates:   templatesCatalog,
		})
		if err != nil {
			if ce, ok := err.(*compiler.ContractError); ok {
				fail(ce.Stage, ce.Code, ce.Op, ce.Err)
				return
			}
			fail(compiler.StageCUE, compiler.ErrCodeCUEPipeline, "compile for emit", err)
			return
		}
		hasDiagnosticErrors := false
		if jsonLogs {
			hasDiagnosticErrors = emitBuildDiagnostics(compiler.LatestDiagnostics)
		} else {
			hasDiagnosticErrors = emitDiagnostics(os.Stderr, compiler.LatestDiagnostics)
		}
		if hasDiagnosticErrors {
			fmt.Println("Build FAILED due to diagnostic errors.")
			return
		}
		postEmitDiagStart := len(compiler.LatestDiagnostics)

		irSchema := compiled.IR
		scenarios := compiled.Normalized.Scenarios

		isMicroservice := projectHasBuildStrategy(projectVal, "microservices")
		effectiveMode := resolveBuildMode(output.Mode, projectVal, output.BackendDirExplicit)
		if output.Mode == "" && effectiveMode == "in_place" {
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
		if err := validateBuildMode(effectiveMode, output, selectedTargets); err != nil {
			fail(compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "validate output mode", err)
			return
		}

		var transaction *buildTransaction
		var buildWorkspace string
		contractBaselines := map[string][]byte{}
		if !output.DryRun {
			backendDirs := make([]string, 0, len(selectedTargets))
			frontendDirs := make([]string, 0, len(selectedTargets))
			multiTarget := len(selectedTargets) > 1
			for _, td := range selectedTargets {
				backend := resolveBackendDirForTarget(effectiveMode, output.BackendDir, td, multiTarget)
				frontend := resolveFrontendDirForTarget(output.FrontendDir, backend, td, multiTarget)
				if !filepath.IsAbs(backend) {
					backend = filepath.Join(projectPath, backend)
				}
				if !filepath.IsAbs(frontend) {
					frontend = filepath.Join(projectPath, frontend)
				}
				backendDirs = append(backendDirs, backend)
				frontendDirs = append(frontendDirs, frontend)
				contractPath := filepath.Join(backend, "api", "openapi.yaml")
				if data, readErr := os.ReadFile(contractPath); readErr == nil {
					contractBaselines[contractPath] = data
				} else if !os.IsNotExist(readErr) {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "read previous OpenAPI contract", readErr)
					return
				}

				frontendAppDir := strings.TrimSpace(output.FrontendAppDir)
				if frontendAppDir == "" {
					frontendAppDir = strings.TrimSpace(td.FrontendAppDir)
				}
				if envDir := strings.TrimSpace(os.ExpandEnv(os.Getenv("ANG_FRONTEND_APP_DIR"))); envDir != "" {
					frontendAppDir = envDir
				}
				for _, externalOutput := range []string{frontendAppDir, output.FrontendAdminAppDir, output.FrontendEnvPath} {
					externalOutput = strings.TrimSpace(os.ExpandEnv(externalOutput))
					if externalOutput == "" {
						continue
					}
					if !filepath.IsAbs(externalOutput) {
						externalOutput = filepath.Join(projectPath, externalOutput)
					}
					if multiTarget {
						externalOutput = filepath.Join(externalOutput, safeTargetDirName(td.Name))
					}
					frontendDirs = append(frontendDirs, externalOutput)
				}
			}
			transaction, err = beginBuildTransaction(generatedTransactionPaths(projectPath, effectiveMode, backendDirs, frontendDirs))
			if err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "snapshot generated outputs", err)
				return
			}
			buildWorkspace, err = transaction.CreateWorkspace(projectPath)
			if err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "create staged project workspace", err)
				return
			}
			defer func() {
				if rollbackErr := transaction.Rollback(); rollbackErr != nil {
					printStageFailure("Build ROLLBACK FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "restore generated outputs", rollbackErr)
				}
			}()
		}

		multiTarget := len(selectedTargets) > 1
		stagePath := func(path string) string {
			if strings.TrimSpace(path) == "" {
				return ""
			}
			if transaction == nil {
				return path
			}
			if buildWorkspace != "" {
				projectAbs, _ := filepath.Abs(projectPath)
				pathAbs, _ := filepath.Abs(path)
				if rel, relErr := filepath.Rel(projectAbs, pathAbs); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return filepath.Join(buildWorkspace, rel)
				}
			}
			return transaction.StagePath(path)
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
			backendDir := stagePath(intendedBackendDir)
			frontendDir := stagePath(intendedFrontendDir)
			if output.DryRun {
				safeName := safeTargetDirName(td.Name)
				backendDir = filepath.Join(dryRunTmpRoot, "backend", safeName)
				frontendDir = filepath.Join(dryRunTmpRoot, "frontend", safeName)
			} else {
				frontendTypecheckDirs = append(frontendTypecheckDirs, frontendDir)
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

			em := emitter.New(backendDir, frontendDir, tmplDir)
			em.NatsWorkers = td.NatsWorkers
			if em.NatsWorkers <= 0 {
				em.NatsWorkers = 20
			}
			em.NatsPublishRetryAttempts = td.NatsPublishRetryAttempts
			if em.NatsPublishRetryAttempts <= 0 {
				em.NatsPublishRetryAttempts = 3
			}
			em.NatsPublishRetryDelayMS = td.NatsPublishRetryDelayMS
			if em.NatsPublishRetryDelayMS <= 0 {
				em.NatsPublishRetryDelayMS = 100
			}
			em.FrontendAdminDir = output.FrontendAdminDir
			if projectDef != nil && strings.TrimSpace(projectDef.UIProvider) != "" {
				em.UIProviderPath = strings.TrimSpace(projectDef.UIProvider)
			}
			em.Version = compiler.Version
			em.InputHash = inputHash
			em.CompilerHash = compilerFingerprint
			em.GoModule = goModule
			em.IRSchema = irSchema
			em.WarningSink = func(w normalizer.Warning) {
				compiler.LatestDiagnostics = append(compiler.LatestDiagnostics, w)
			}

			ctx := em.AnalyzeContextFromIR(irSchema)
			ctx.HasScheduler = len(irSchema.Schedules) > 0
			ctx.InputHash = inputHash
			ctx.CompilerHash = compilerFingerprint
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
			if sessionDef != nil {
				ctx.HasSession = true
				ctx.SessionCookieName = sessionDef.CookieName
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
			// CUE target.frontend_app_dir is used as default when CLI flag is not set.
			// ANG_FRONTEND_APP_DIR env var overrides both CLI flag and CUE value.
			if strings.TrimSpace(targetOutput.FrontendAppDir) == "" && strings.TrimSpace(td.FrontendAppDir) != "" {
				targetOutput.FrontendAppDir = td.FrontendAppDir
			}
			if envDir := strings.TrimSpace(os.ExpandEnv(os.Getenv("ANG_FRONTEND_APP_DIR"))); envDir != "" {
				targetOutput.FrontendAppDir = envDir
			}
			// Expand env vars in the path (e.g. $HOME/project/src/@sdk in project.cue).
			// Resolve relative paths against the project root.
			if strings.TrimSpace(targetOutput.FrontendAppDir) != "" {
				targetOutput.FrontendAppDir = strings.TrimSpace(os.ExpandEnv(targetOutput.FrontendAppDir))
				if !filepath.IsAbs(targetOutput.FrontendAppDir) {
					targetOutput.FrontendAppDir = filepath.Join(projectPath, targetOutput.FrontendAppDir)
				}
			}
			if output.DryRun {
				targetOutput.FrontendAppDir = ""
				targetOutput.FrontendAdminAppDir = ""
				targetOutput.FrontendEnvPath = ""
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(targetOutput.FrontendAppDir) != "" {
				targetOutput.FrontendAppDir = filepath.Join(targetOutput.FrontendAppDir, safeTargetDirName(td.Name))
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(output.FrontendAdminAppDir) != "" {
				targetOutput.FrontendAdminAppDir = filepath.Join(output.FrontendAdminAppDir, safeTargetDirName(td.Name))
			}
			if !output.DryRun && multiTarget && strings.TrimSpace(output.FrontendEnvPath) != "" {
				targetOutput.FrontendEnvPath = filepath.Join(output.FrontendEnvPath, safeTargetDirName(td.Name), ".env.example")
			}
			if !output.DryRun {
				targetOutput.FrontendAppDir = stagePath(targetOutput.FrontendAppDir)
				targetOutput.FrontendAdminAppDir = stagePath(targetOutput.FrontendAdminAppDir)
				targetOutput.FrontendEnvPath = stagePath(targetOutput.FrontendEnvPath)
			}

			if infraContextPatch.NotificationMuting {
				ctx.NotificationMuting = true
			}

			registry, pluginNames, err := buildStepRegistry(buildStepRegistryInput{
				em:             em,
				irSchema:       irSchema,
				ctx:            ctx,
				scenarios:      scenarios,
				cfgDef:         cfgDef,
				authDef:        authDef,
				sessionDef:     sessionDef,
				rbacDef:        rbacDef,
				infraValues:    infraValues,
				emailTemplates: emailTemplates,
				projectDef:     projectDef,
				targetOutput:   targetOutput,
				isMicroservice: isMicroservice,
			})
			if err != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "resolve target plugins", err)
				return
			}
			var executeErr error
			if jsonLogs {
				var bufferedEvents []generator.StepEvent
				executeErr = withSuppressedStdout(func() error {
					return registry.Execute(td, caps, func(string, ...interface{}) {}, func(event generator.StepEvent) {
						bufferedEvents = append(bufferedEvents, event)
					})
				})
				for _, event := range bufferedEvents {
					logStepEvent(event)
				}
			} else {
				executeErr = registry.Execute(td, caps, func(format string, args ...interface{}) {
					logText(format, args...)
				}, logStepEvent)
			}
			if executeErr != nil {
				fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "run capability matrix steps", executeErr)
				return
			}
			if strings.EqualFold(td.Lang, "go") {
				if err := emitter.ValidateGeneratedDI(backendDir, ctx, authDef); err != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, "validate generated dependency injection", err)
					return
				}
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
				selfCheckRoot := projectPath
				if buildWorkspace != "" {
					selfCheckRoot = buildWorkspace
				}
				checkRes, err := runGoRuntimeSelfCheck(selfCheckRoot, backendDir, effectiveMode)
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
				Name:          td.Name,
				Lang:          td.Lang,
				Mode:          effectiveMode,
				Backend:       filepath.ToSlash(filepath.Clean(intendedBackendDir)),
				StagedBackend: filepath.ToSlash(filepath.Clean(backendDir)),
				Frontend:      filepath.ToSlash(filepath.Clean(intendedFrontendDir)),
				Plugins:       joinPluginNames(pluginNames),
				SelfCheck:     selfCheckStatus,
				Details:       selfCheckDetails,
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

		newEmitterDiagnostics := compiler.LatestDiagnostics[postEmitDiagStart:]
		hasEmitterErrors := false
		if jsonLogs {
			hasEmitterErrors = emitBuildDiagnostics(newEmitterDiagnostics)
		} else {
			hasEmitterErrors = emitDiagnostics(os.Stderr, newEmitterDiagnostics)
		}
		if hasEmitterErrors {
			fmt.Println("Build FAILED due to diagnostic errors.")
			return
		}
		if !output.DryRun {
			for contractPath, previous := range contractBaselines {
				current, readErr := os.ReadFile(stagePath(contractPath))
				if readErr != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "read generated OpenAPI contract", readErr)
					return
				}
				diff, diffErr := diffOpenAPIContracts(previous, current)
				if diffErr != nil {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "compare OpenAPI contract", diffErr)
					return
				}
				logText("openapi: +%d operations, -%d operations, %d breaking", len(diff.AddedOperations), len(diff.RemovedOperations), len(diff.BreakingChanges))
				if len(diff.BreakingChanges) > 0 && !output.AcceptContract {
					fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "OpenAPI breaking change gate", fmt.Errorf("%s; rerun with --accept-contract to confirm", strings.Join(diff.BreakingChanges, "; ")))
					return
				}
			}
		}

		if !output.DryRun {
			manifestTargets := make([]artifactManifestTarget, 0, len(summaries))
			for _, s := range summaries {
				manifestTargets = append(manifestTargets, artifactManifestTarget{
					Mode:     s.Mode,
					Backend:  s.Backend,
					Frontend: s.Frontend,
				})
			}
			manifestRoot := projectPath
			if buildWorkspace != "" {
				manifestRoot = buildWorkspace
				for i := range manifestTargets {
					manifestTargets[i].Backend = stagePath(manifestTargets[i].Backend)
					manifestTargets[i].Frontend = stagePath(manifestTargets[i].Frontend)
				}
			}
			if err := writeArtifactHashManifest(manifestRoot, manifestTargets, irSchema.IRVersion, inputHash, templateHash, compilerFingerprint); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "write artifact hash manifest", err)
				return
			}

			if err := runOptionalMCPGeneration(manifestRoot); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterMCPGen, "run optional MCP generation", err)
				return
			}
			if output.WithOpenAPI {
				openapiPath := filepath.Join(manifestRoot, "api", "openapi.yaml")
				openapiEm := &emitter.Emitter{
					OutputDir: filepath.Join(projectPath, "api"),
					Version:   compiler.Version,
				}
				if oErr := openapiEm.EmitOpenAPIFromNormalizerTypes(
					compiled.Normalized.Endpoints,
					compiled.Normalized.Services,
					compiled.Normalized.Errors,
					&compiled.Normalized.Project,
					openapiPath,
				); oErr != nil {
					logText("Warning: openapi generation failed: %v", oErr)
				} else {
					logText("OpenAPI spec written to %s", openapiPath)
				}
			}
			if err := runFrontendTypecheckGate(frontendTypecheckDirs); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "frontend typecheck gate", err)
				return
			}
			goBackends := make([]string, 0, len(summaries))
			for _, s := range summaries {
				if strings.EqualFold(strings.TrimSpace(s.Lang), "go") {
					goBackends = append(goBackends, s.StagedBackend)
				}
			}
			if !output.SkipGoVerify {
				if err := runGeneratedGoVerify(goBackends); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "post-build go verify", err)
					return
				}
			}
			if output.RunTests {
				if err := runGeneratedGoTests(goBackends); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "post-build go tests", err)
					return
				}
			}
		} else {
			dryManifest.OptionalStepsSkipped = []string{"runOptionalMCPGeneration"}
		}

		if output.DryRun {
			summarizeDryRunManifest(&dryManifest)
			if strings.TrimSpace(output.DryRunReport) != "" {
				data, err := json.MarshalIndent(dryManifest, "", "  ")
				if err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "marshal dry-run report", err)
					return
				}
				data = append(data, '\n')
				if err := os.MkdirAll(filepath.Dir(output.DryRunReport), 0o755); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "mkdir dry-run report dir", err)
					return
				}
				if err := os.WriteFile(output.DryRunReport, data, 0o644); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterOptions, "write dry-run report", err)
					return
				}
			}
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
		if transaction != nil {
			if buildWorkspace != "" {
				if err := transaction.CaptureWorkspace(projectPath, buildWorkspace); err != nil {
					printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "capture staged generated outputs", err)
					return
				}
			}
			if err := transaction.Commit(); err != nil {
				printStageFailure("Build FAILED", compiler.StageEmitters, compiler.ErrCodeEmitterStep, "commit generated outputs", err)
				return
			}
		}

		logText("\nBuild SUCCESSFUL.")
		logText("Build Report:")
		for _, s := range summaries {
			logText("  - target=%s mode=%s backend=%s plugins=%s self-check=%s", s.Name, s.Mode, s.Backend, s.Plugins, s.SelfCheck)
			for _, d := range s.Details {
				logText("      %s -> %s", d.Package, d.Dir)
			}
		}
		if suggestDoctor, hint := buildDoctorHint(projectPath); suggestDoctor {
			logText("%s", hint)
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
		buildTask()
	}
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
