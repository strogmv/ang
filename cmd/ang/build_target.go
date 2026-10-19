package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/sqlmigrate"
)

// generateTarget runs the emitters of one target into its (staged or dry-run)
// directories, applies the output, checks it and records the summary; a dry
// run also predicts the gates and collects the target's changes.
func (r *buildRun) generateTarget(td normalizer.TargetDef) bool {
	output, projectPath := r.output, r.projectPath
	intendedBackendDir := resolveBackendDirForTarget(r.effectiveMode, output.BackendDir, td, r.multiTarget)
	intendedFrontendDir := resolveFrontendDirForTarget(frontendDirOptionForTarget(output, td), intendedBackendDir, td, r.multiTarget)
	if !filepath.IsAbs(intendedBackendDir) {
		intendedBackendDir = filepath.Join(projectPath, intendedBackendDir)
	}
	if !filepath.IsAbs(intendedFrontendDir) {
		intendedFrontendDir = filepath.Join(projectPath, intendedFrontendDir)
	}
	backendDir := r.stagePath(intendedBackendDir)
	frontendDir := r.stagePath(intendedFrontendDir)
	if output.DryRun {
		safeName := safeTargetDirName(td.Name)
		backendDir = filepath.Join(r.dryRunTmpRoot, "backend", safeName)
		frontendDir = filepath.Join(r.dryRunTmpRoot, "frontend", safeName)
	} else {
		r.frontendTypecheckDirs = append(r.frontendTypecheckDirs, frontendDir)
	}
	r.intendedBackends = append(r.intendedBackends, intendedBackendDir)
	r.intendedFrontends = append(r.intendedFrontends, intendedFrontendDir)
	// Generation starts from empty directories, so everything under them
	// afterwards is exactly what this build produced.
	emitBackendDir, emitFrontendDir := backendDir, frontendDir
	if r.generationRoot != "" {
		emitBackendDir = filepath.Join(r.generationRoot, "backend", safeTargetDirName(td.Name))
		emitFrontendDir = filepath.Join(r.generationRoot, "frontend", safeTargetDirName(td.Name))
	}
	r.logText("Generating target %s (%s/%s/%s) -> %s", td.Name, td.Lang, td.Framework, td.DB, backendDir)
	if r.jsonLogs {
		r.event("emitters", td.Name, "start", "Target generation started")
	}

	em, ctx, ok := r.targetEmitter(td, emitBackendDir, emitFrontendDir, intendedBackendDir)
	if !ok {
		return false
	}
	caps, err := compiler.ResolveTargetCapabilities(td)
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, fmt.Sprintf("resolve capabilities for target=%s", td.Name), err)
		return false
	}
	// Infrastructure the project does not use is not generated: steps
	// that require a usage capability run only when it is granted here.
	for _, usage := range emitter.UsageCapabilities(ctx, em.IRSchema) {
		caps[compiler.Capability(usage)] = true
	}

	targetOutput := r.targetOutputOptions(td, emitBackendDir, emitFrontendDir)

	if r.in.infraContextPatch.NotificationMuting {
		ctx.NotificationMuting = true
	}

	registry, pluginNames, err := buildStepRegistry(buildStepRegistryInput{
		em:             em,
		irSchema:       em.IRSchema,
		ctx:            ctx,
		scenarios:      r.compiled.Normalized.Scenarios,
		cfgDef:         r.in.cfgDef,
		authDef:        r.in.authDef,
		sessionDef:     r.in.sessionDef,
		rbacDef:        r.in.rbacDef,
		infraValues:    r.in.infraValues,
		emailTemplates: r.in.emailTemplates,
		projectDef:     r.in.projectDef,
		targetOutput:   targetOutput,
		isMicroservice: r.isMicroservice,
	})
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "resolve target plugins", err)
		return false
	}
	var executeErr error
	if r.jsonLogs {
		var bufferedEvents []generator.StepEvent
		executeErr = withSuppressedStdout(func() error {
			return registry.Execute(td, caps, func(string, ...interface{}) {}, func(event generator.StepEvent) {
				bufferedEvents = append(bufferedEvents, event)
			})
		})
		for _, event := range bufferedEvents {
			r.logStepEvent(event)
		}
	} else {
		executeErr = registry.Execute(td, caps, func(format string, args ...interface{}) {
			r.logText(format, args...)
		}, r.logStepEvent)
	}
	if executeErr != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "run capability matrix steps", executeErr)
		return false
	}
	if r.generationRoot != "" {
		for _, out := range [][3]string{{emitBackendDir, backendDir, intendedBackendDir}, {emitFrontendDir, frontendDir, intendedFrontendDir}} {
			if err := applyGeneratedOutput(out[0], out[1], out[2], projectPath, r.producedFiles); err != nil {
				r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "apply generated output", err)
				return false
			}
		}
	}
	if strings.EqualFold(td.Lang, "go") {
		if err := emitter.ValidateGeneratedDI(backendDir, ctx, r.in.authDef); err != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, "validate generated dependency injection", err)
			return false
		}
	}
	if r.jsonLogs {
		r.event("emitters", td.Name, "ok", "Target generation finished")
	}

	if !output.DryRun && td.Lang == "go" && r.effectiveMode == "release" {
		if err := ensureReleaseGoModule(backendDir, r.goModule); err != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "ensure release go.mod", err)
			return false
		}
	}

	selfCheckStatus := "skipped"
	var selfCheckDetails []runtimePackageDir
	if !output.DryRun && strings.EqualFold(td.Lang, "go") {
		selfCheckRoot := projectPath
		if r.buildWorkspace != "" {
			selfCheckRoot = r.buildWorkspace
		}
		checkRes, err := runGoRuntimeSelfCheck(selfCheckRoot, backendDir, r.effectiveMode)
		if err != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "runtime source self-check", err)
			return false
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
	r.summaries = append(r.summaries, buildTargetSummary{
		Name:          td.Name,
		Lang:          td.Lang,
		Mode:          r.effectiveMode,
		Backend:       filepath.ToSlash(filepath.Clean(intendedBackendDir)),
		StagedBackend: filepath.ToSlash(filepath.Clean(backendDir)),
		Frontend:      filepath.ToSlash(filepath.Clean(intendedFrontendDir)),
		Plugins:       joinPluginNames(pluginNames),
		SelfCheck:     selfCheckStatus,
		Details:       selfCheckDetails,
	})

	if output.DryRun {
		return r.dryRunTarget(td, backendDir, frontendDir, intendedBackendDir, intendedFrontendDir)
	}
	return true
}

// targetEmitter configures the emitter and its generation context for one
// target: options, the target's copy of the IR (target.auth_default) and the
// infrastructure patches.
func (r *buildRun) targetEmitter(td normalizer.TargetDef, emitBackendDir, emitFrontendDir, intendedBackendDir string) (*emitter.Emitter, emitter.MainContext, bool) {
	em := emitter.New(emitBackendDir, emitFrontendDir, r.in.tmplDir)
	em.SourceBackendDir = intendedBackendDir
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
	em.FrontendAdminDir = r.output.FrontendAdminDir
	em.FrontendSDKSkip = td.FrontendSDKSkip
	if !emitter.ValidSQLIndexNames(td.SQLIndexNames) {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, fmt.Sprintf("target=%s", td.Name),
			fmt.Errorf("sql_index_names must be %q or %q, got %q", emitter.SQLIndexNamesTruncate, emitter.SQLIndexNamesHashed, td.SQLIndexNames))
		return nil, emitter.MainContext{}, false
	}
	if !sqlmigrate.ValidPostgresVersion(td.PostgresVersion) {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, fmt.Sprintf("target=%s", td.Name),
			fmt.Errorf("postgres_version must be a major version such as \"18\", got %q", td.PostgresVersion))
		return nil, emitter.MainContext{}, false
	}
	em.SQLIndexNames = td.SQLIndexNames
	em.PostgresVersion = td.PostgresVersion
	em.ServiceConfig = td.ServiceConfig
	em.OperationMetrics = td.OperationMetrics
	// target.auth_default: under "deny" this target emits a copy of the
	// schema in which every endpoint without auth needs a caller.
	irSchema, authCheck, authErr := compiler.ApplyAuthDefault(r.irSchema, td.AuthDefault)
	if authErr == nil {
		authErr = authCheck.Err()
	}
	if authErr != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterCapabilityResolve, fmt.Sprintf("target=%s", td.Name), authErr)
		return nil, emitter.MainContext{}, false
	}
	compiler.LatestDiagnostics = append(compiler.LatestDiagnostics, authCheck.Warnings...)
	if td.ServiceConfig {
		compiler.LatestDiagnostics = append(compiler.LatestDiagnostics, compiler.CheckConfigLoads(irSchema)...)
	}
	if r.in.projectDef != nil && strings.TrimSpace(r.in.projectDef.UIProvider) != "" {
		em.UIProviderPath = strings.TrimSpace(r.in.projectDef.UIProvider)
	}
	em.Version = compiler.Version
	em.InputHash = r.inputHash
	em.CompilerHash = r.compilerFingerprint
	em.GoModule = r.goModule
	em.IRSchema = irSchema
	em.WarningSink = func(w normalizer.Warning) {
		compiler.LatestDiagnostics = append(compiler.LatestDiagnostics, w)
	}

	ctx := em.AnalyzeContextFromIR(irSchema)
	ctx.HasScheduler = len(irSchema.Schedules) > 0
	ctx.ServerRoles = td.ServerRoles
	ctx.ShutdownTimeout = td.ShutdownTimeout
	ctx.InputHash = r.inputHash
	ctx.CompilerHash = r.compilerFingerprint
	ctx.ANGVersion = compiler.Version
	ctx.GoModule = r.goModule
	patch := r.in.infraContextPatch
	if patch.AuthService != "" {
		ctx.AuthService = patch.AuthService
	}
	if patch.AuthRefreshStore != "" {
		ctx.AuthRefreshStore = patch.AuthRefreshStore
	}
	if patch.ForceHasCache {
		ctx.HasCache = true
	}
	if patch.ForceHasSQL {
		ctx.HasSQL = true
	}
	if r.in.sessionDef != nil {
		ctx.HasSession = true
		ctx.SessionCookieName = r.in.sessionDef.CookieName
	}
	em.EnrichContextFromIR(&ctx, irSchema)
	return em, ctx, true
}

// targetOutputOptions are the build options as one target's steps see them:
// its emit directories and its (staged, per-target) external front outputs.
func (r *buildRun) targetOutputOptions(td normalizer.TargetDef, emitBackendDir, emitFrontendDir string) OutputOptions {
	output := r.output
	targetOutput := output
	targetOutput.BackendDir = emitBackendDir
	targetOutput.FrontendDir = emitFrontendDir
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
			targetOutput.FrontendAppDir = filepath.Join(r.projectPath, targetOutput.FrontendAppDir)
		}
	}
	if output.DryRun {
		targetOutput.FrontendAppDir = ""
		targetOutput.FrontendAdminAppDir = ""
		targetOutput.FrontendEnvPath = ""
	}
	if !output.DryRun && r.multiTarget && strings.TrimSpace(targetOutput.FrontendAppDir) != "" {
		targetOutput.FrontendAppDir = filepath.Join(targetOutput.FrontendAppDir, safeTargetDirName(td.Name))
	}
	if !output.DryRun && r.multiTarget && strings.TrimSpace(output.FrontendAdminAppDir) != "" {
		targetOutput.FrontendAdminAppDir = filepath.Join(output.FrontendAdminAppDir, safeTargetDirName(td.Name))
	}
	if !output.DryRun && r.multiTarget && strings.TrimSpace(output.FrontendEnvPath) != "" {
		targetOutput.FrontendEnvPath = filepath.Join(output.FrontendEnvPath, safeTargetDirName(td.Name), ".env.example")
	}
	if !output.DryRun {
		targetOutput.FrontendAppDir = r.stagePath(targetOutput.FrontendAppDir)
		targetOutput.FrontendAdminAppDir = r.stagePath(targetOutput.FrontendAdminAppDir)
		targetOutput.FrontendEnvPath = r.stagePath(targetOutput.FrontendEnvPath)
	}
	return targetOutput
}

// dryRunTarget predicts the gates of a real build for one target, applies the
// literal hoisting a build makes after them, and records the target's changes.
func (r *buildRun) dryRunTarget(td normalizer.TargetDef, backendDir, frontendDir, intendedBackendDir, intendedFrontendDir string) bool {
	output, projectPath := r.output, r.projectPath
	// A dry run that reports success for a build ang build would refuse misleads.
	if refusal := dryRunGates(intendedBackendDir, backendDir, resolveReleaseBaseline(output.ReleaseRef), output.AcceptContract, output.AcceptRemovals, r.logText); refusal != nil {
		r.fail(compiler.StageEmitters, refusal.code, refusal.op, refusal.err)
		return false
	}
	// The rewrite a build makes after its gates: compare what a
	// build would write, not the emitters' output before it.
	if strings.EqualFold(td.Lang, "go") {
		projectAbs, _ := filepath.Abs(projectPath)
		previousList, _, listErr := readGeneratedFiles(projectAbs)
		if listErr != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "read "+generatedFilesName, listErr)
			return false
		}
		owned := generatedTransactionPaths(projectPath, r.effectiveMode, []string{intendedBackendDir}, nil)
		kept := func(abs string) bool {
			return keptUngeneratedFile(projectAbs, abs, previousList, owned, r.partialBuild)
		}
		hoisted, hErr := dryRunHoist(backendDir, intendedBackendDir, projectPath, kept, output.DryRunDiff, r.logText)
		if hErr != nil {
			r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "hoist repeated literals", hErr)
			return false
		}
		r.dryManifest.Rewrites = append(r.dryManifest.Rewrites, hoisted.Rewrites...)
		r.dryManifest.Deletions = append(r.dryManifest.Deletions, hoisted.Deletions...)
	}
	backendChanges, err := collectDryRunChanges(backendDir, intendedBackendDir, projectPath, output.DryRunDiff)
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "collect dry-run backend changes", err)
		return false
	}
	frontendChanges, err := collectDryRunChanges(frontendDir, intendedFrontendDir, projectPath, output.DryRunDiff)
	if err != nil {
		r.fail(compiler.StageEmitters, compiler.ErrCodeEmitterStep, "collect dry-run frontend changes", err)
		return false
	}
	combined := append(backendChanges, frontendChanges...)
	r.dryManifest.Targets = append(r.dryManifest.Targets, dryRunTargetManifest{
		Target:   td.Name,
		Lang:     td.Lang,
		Backend:  filepath.ToSlash(filepath.Clean(intendedBackendDir)),
		Frontend: filepath.ToSlash(filepath.Clean(intendedFrontendDir)),
		Changes:  combined,
	})
	return true
}
