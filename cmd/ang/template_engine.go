package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type templateDiffReport struct {
	Schema             string              `json:"schema"`
	ProjectPath        string              `json:"project_path"`
	GeneratedAtRoot    string              `json:"generated_root"`
	ManagedFiles       int                 `json:"managed_files"`
	Summary            templateDiffSummary `json:"summary"`
	Files              []templateDriftFile `json:"files"`
	Notes              []string            `json:"notes,omitempty"`
	ManifestCompatible bool                `json:"manifest_compatible"`
}

type templateDiffSummary struct {
	Total            int `json:"total"`
	SafeDrift        int `json:"safe_drift"`
	ConflictingDrift int `json:"conflicting_drift"`
	ManualEdits      int `json:"manual_edits"`
	Unchanged        int `json:"unchanged"`
	Creates          int `json:"creates"`
	Updates          int `json:"updates"`
	Deletes          int `json:"deletes"`
	AutoApplicable   int `json:"auto_applicable"`
}

type templateDriftFile struct {
	Path            string `json:"path"`
	Action          string `json:"action"`
	Classification  string `json:"classification"`
	Ownership       string `json:"ownership"`
	MergeStrategy   string `json:"merge_strategy,omitempty"`
	AutoApplicable  bool   `json:"auto_applicable"`
	Reason          string `json:"reason,omitempty"`
	PreviousHash    string `json:"previous_hash,omitempty"`
	CurrentHash     string `json:"current_hash,omitempty"`
	DesiredHash     string `json:"desired_hash,omitempty"`
	GeneratedSource string `json:"generated_source,omitempty"`
}

type templateRebasePlan struct {
	Schema      string                    `json:"schema"`
	ProjectPath string                    `json:"project_path"`
	Summary     templateRebasePlanSummary `json:"summary"`
	Steps       []templateRebasePlanStep  `json:"steps"`
	Skipped     []templateDriftFile       `json:"skipped,omitempty"`
}

type templateRebasePlanSummary struct {
	Total         int `json:"total"`
	ApplyCreate   int `json:"apply_create"`
	ApplyUpdate   int `json:"apply_update"`
	ApplyDelete   int `json:"apply_delete"`
	PreserveMerge int `json:"preserve_merge"`
	Skipped       int `json:"skipped"`
}

type templateRebasePlanStep struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	MergeStrategy string `json:"merge_strategy,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type templateRebaseApplyResult struct {
	Schema       string                   `json:"schema"`
	ProjectPath  string                   `json:"project_path"`
	ChangedFiles []string                 `json:"changed_files"`
	DeletedFiles []string                 `json:"deleted_files,omitempty"`
	Plan         []templateRebasePlanStep `json:"plan"`
	Skipped      []templateDriftFile      `json:"skipped,omitempty"`
}

type templateBuildSnapshot struct {
	ProjectPath string
	DryRunRoot  string
	Manifest    dryRunManifest
}

type templateGeneratedFile struct {
	Path        string
	SourcePath  string
	Action      string
	DesiredHash string
}

func runTemplate(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang template <diff|rebase> [project] [flags]")
		os.Exit(1)
	}
	switch strings.TrimSpace(args[0]) {
	case "diff":
		runTemplateDiff(args[1:])
	case "rebase":
		runTemplateRebase(args[1:])
	default:
		fmt.Printf("Template FAILED: unknown subcommand %q\n", args[0])
		fmt.Println("Usage: ang template <diff|rebase> [project] [flags]")
		os.Exit(1)
	}
}

func runTemplateDiff(args []string) {
	projectPath, opts, jsonOut, outPath := parseTemplateCommonFlags("template diff", args)
	snapshot, cleanup, err := buildTemplateSnapshot(projectPath, opts)
	if err != nil {
		fmt.Printf("Template diff FAILED: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	report, err := analyzeTemplateDiff(snapshot)
	if err != nil {
		fmt.Printf("Template diff FAILED: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		writeTemplateJSON(os.Stdout, report)
	} else {
		printTemplateDiffReport(report)
	}
	if strings.TrimSpace(outPath) != "" {
		if err := writeJSONFile(outPath, report); err != nil {
			fmt.Printf("Template diff FAILED: write report: %v\n", err)
			os.Exit(1)
		}
	}
	if report.Summary.ConflictingDrift > 0 {
		os.Exit(2)
	}
}

func runTemplateRebase(args []string) {
	fs := flag.NewFlagSet("template rebase", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit machine-readable output")
	planOnly := fs.Bool("plan", false, "print rebase plan without writing files")
	apply := fs.Bool("apply", false, "apply safe template deltas")
	outPlan := fs.String("out", "", "optional path to write rebase plan JSON")
	fromPlan := fs.String("from", "", "optional path to template diff/rebase plan JSON")
	target := fs.String("target", "", "restrict to selected build targets")
	mode := fs.String("mode", "", "build mode override: in_place|release")
	backendDir := fs.String("backend-dir", ".", "backend dir override")
	frontendDir := fs.String("frontend-dir", "sdk", "frontend dir override")
	skipFrontend := fs.Bool("skip-frontend", false, "skip frontend generation while planning/applying rebase")
	skipContractTests := fs.Bool("skip-contract-tests", false, "skip contract test generation while planning/applying rebase")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Template rebase FAILED: %v\n", err)
		os.Exit(1)
	}
	if !*planOnly && !*apply {
		*planOnly = true
	}
	if *planOnly && *apply {
		fmt.Println("Template rebase FAILED: choose either --plan or --apply")
		os.Exit(1)
	}
	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = filepath.Clean(fs.Arg(0))
	}
	opts := OutputOptions{
		BackendDir:        normalizeBackendDir(*backendDir),
		FrontendDir:       strings.TrimSpace(*frontendDir),
		TargetSelector:    strings.TrimSpace(*target),
		Mode:              strings.ToLower(strings.TrimSpace(*mode)),
		DryRun:            true,
		SkipFrontend:      *skipFrontend,
		SkipContractTests: *skipContractTests,
		SkipGoVerify:      true,
	}
	snapshot, cleanup, err := buildTemplateSnapshot(projectPath, opts)
	if err != nil {
		fmt.Printf("Template rebase FAILED: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	report, err := analyzeTemplateDiff(snapshot)
	if err != nil {
		fmt.Printf("Template rebase FAILED: %v\n", err)
		os.Exit(1)
	}
	plan, err := resolveTemplatePlan(snapshot.ProjectPath, report, strings.TrimSpace(*fromPlan))
	if err != nil {
		fmt.Printf("Template rebase FAILED: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(*outPlan) != "" {
		if err := writeJSONFile(*outPlan, plan); err != nil {
			fmt.Printf("Template rebase FAILED: write plan: %v\n", err)
			os.Exit(1)
		}
	}
	if *planOnly {
		if *jsonOut {
			writeTemplateJSON(os.Stdout, plan)
			return
		}
		printTemplateRebasePlan(plan)
		if plan.Summary.Skipped > 0 {
			os.Exit(2)
		}
		return
	}

	res, err := applyTemplateRebase(snapshot.ProjectPath, report, plan)
	if err != nil {
		fmt.Printf("Template rebase FAILED: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		writeTemplateJSON(os.Stdout, res)
		return
	}
	printTemplateRebaseResult(res)
	if len(res.Skipped) > 0 {
		os.Exit(2)
	}
}

func parseTemplateCommonFlags(name string, args []string) (string, OutputOptions, bool, string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit machine-readable output")
	outPath := fs.String("out", "", "optional path to write diff report JSON")
	target := fs.String("target", "", "restrict to selected build targets")
	mode := fs.String("mode", "", "build mode override: in_place|release")
	backendDir := fs.String("backend-dir", ".", "backend dir override")
	frontendDir := fs.String("frontend-dir", "sdk", "frontend dir override")
	skipFrontend := fs.Bool("skip-frontend", false, "skip frontend generation while diffing")
	skipContractTests := fs.Bool("skip-contract-tests", false, "skip contract test generation while diffing")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("%s FAILED: %v\n", strings.Title(name), err)
		os.Exit(1)
	}
	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = filepath.Clean(fs.Arg(0))
	}
	opts := OutputOptions{
		BackendDir:        normalizeBackendDir(*backendDir),
		FrontendDir:       strings.TrimSpace(*frontendDir),
		TargetSelector:    strings.TrimSpace(*target),
		Mode:              strings.ToLower(strings.TrimSpace(*mode)),
		DryRun:            true,
		SkipFrontend:      *skipFrontend,
		SkipContractTests: *skipContractTests,
		SkipGoVerify:      true,
	}
	return projectPath, opts, *jsonOut, strings.TrimSpace(*outPath)
}

func buildTemplateSnapshot(projectPath string, opts OutputOptions) (templateBuildSnapshot, func(), error) {
	tmpRoot, err := os.MkdirTemp("", "ang-template-evolution-*")
	if err != nil {
		return templateBuildSnapshot{}, nil, fmt.Errorf("create temp root: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpRoot) }
	reportPath := filepath.Join(tmpRoot, "dry-run-report.json")
	opts.DryRun = true
	opts.DryRunRoot = tmpRoot
	opts.DryRunReport = reportPath
	if err := runBuildDryRunSubprocess(projectPath, opts); err != nil {
		cleanup()
		return templateBuildSnapshot{}, nil, err
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		cleanup()
		return templateBuildSnapshot{}, nil, fmt.Errorf("read dry-run report: %w", err)
	}
	var man dryRunManifest
	if err := json.Unmarshal(data, &man); err != nil {
		cleanup()
		return templateBuildSnapshot{}, nil, fmt.Errorf("parse dry-run report: %w", err)
	}
	return templateBuildSnapshot{ProjectPath: filepath.Clean(projectPath), DryRunRoot: tmpRoot, Manifest: man}, cleanup, nil
}

func runBuildDryRunSubprocess(projectPath string, opts OutputOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	args := []string{"build"}
	if strings.TrimSpace(projectPath) != "" && strings.TrimSpace(projectPath) != "." {
		args = append(args, projectPath)
	}
	args = append(args, "--dry-run", "--skip-go-verify")
	if strings.TrimSpace(opts.DryRunRoot) != "" {
		args = append(args, "--dry-run-root", opts.DryRunRoot)
	}
	if strings.TrimSpace(opts.DryRunReport) != "" {
		args = append(args, "--dry-run-report", opts.DryRunReport)
	}
	if strings.TrimSpace(opts.TargetSelector) != "" {
		args = append(args, "--target", opts.TargetSelector)
	}
	if strings.TrimSpace(opts.Mode) != "" {
		args = append(args, "--mode", opts.Mode)
	}
	if strings.TrimSpace(opts.BackendDir) != "" {
		args = append(args, "--backend-dir", opts.BackendDir)
	}
	if strings.TrimSpace(opts.FrontendDir) != "" {
		args = append(args, "--frontend-dir", opts.FrontendDir)
	}
	if opts.SkipFrontend {
		args = append(args, "--skip-frontend")
	}
	if opts.SkipContractTests {
		args = append(args, "--skip-contract-tests")
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build dry-run subprocess failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func analyzeTemplateDiff(snapshot templateBuildSnapshot) (templateDiffReport, error) {
	generated, err := collectGeneratedTemplateFiles(snapshot.ProjectPath, snapshot.DryRunRoot, snapshot.Manifest)
	if err != nil {
		return templateDiffReport{}, err
	}
	previousManifest, manifestCompatible := readCompatibleArtifactManifest(snapshot.ProjectPath)
	previousByPath := map[string]string{}
	if manifestCompatible {
		for _, rec := range previousManifest.Artifacts {
			previousByPath[filepath.ToSlash(filepath.Clean(rec.Path))] = strings.TrimSpace(rec.Hash)
		}
	}
	existingManaged := make(map[string]templateGeneratedFile, len(generated))
	files := make([]templateDriftFile, 0, len(generated)+len(previousByPath))
	for _, gf := range generated {
		existingManaged[gf.Path] = gf
		files = append(files, classifyTemplateFile(snapshot.ProjectPath, gf, previousByPath[gf.Path], manifestCompatible))
	}
	if manifestCompatible {
		for path, prevHash := range previousByPath {
			if _, ok := existingManaged[path]; ok {
				continue
			}
			files = append(files, classifyTemplateDeletion(snapshot.ProjectPath, path, prevHash))
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Classification == files[j].Classification {
			if files[i].Action == files[j].Action {
				return files[i].Path < files[j].Path
			}
			return files[i].Action < files[j].Action
		}
		return files[i].Classification < files[j].Classification
	})
	report := templateDiffReport{
		Schema:             "ang/template-diff/v1",
		ProjectPath:        filepath.ToSlash(filepath.Clean(snapshot.ProjectPath)),
		GeneratedAtRoot:    filepath.ToSlash(filepath.Clean(snapshot.DryRunRoot)),
		ManagedFiles:       len(files),
		Files:              files,
		ManifestCompatible: manifestCompatible,
	}
	if !manifestCompatible {
		report.Notes = append(report.Notes, "artifact manifest missing or incompatible; drift classification falls back to generated headers and current content")
	}
	summarizeTemplateDiff(&report)
	return report, nil
}

func collectGeneratedTemplateFiles(projectPath, dryRunRoot string, man dryRunManifest) ([]templateGeneratedFile, error) {
	out := make([]templateGeneratedFile, 0)
	for _, target := range man.Targets {
		safeName := safeTargetDirName(target.Target)
		backendGenRoot := filepath.Join(dryRunRoot, "backend", safeName)
		frontendGenRoot := filepath.Join(dryRunRoot, "frontend", safeName)
		backendIntended := filepath.Clean(target.Backend)
		frontendIntended := filepath.Clean(target.Frontend)
		for _, ch := range target.Changes {
			intended := filepath.Clean(ch.Path)
			var sourcePath string
			switch {
			case pathHasRoot(intended, backendIntended):
				rel, err := filepath.Rel(backendIntended, intended)
				if err != nil {
					return nil, err
				}
				sourcePath = filepath.Join(backendGenRoot, rel)
			case pathHasRoot(intended, frontendIntended):
				rel, err := filepath.Rel(frontendIntended, intended)
				if err != nil {
					return nil, err
				}
				sourcePath = filepath.Join(frontendGenRoot, rel)
			default:
				continue
			}
			desiredHash := ""
			if data, err := os.ReadFile(sourcePath); err == nil {
				desiredHash = sha256Hex(data)
			}
			relToProject, err := filepath.Rel(projectPath, intended)
			if err != nil {
				relToProject = intended
			}
			out = append(out, templateGeneratedFile{
				Path:        filepath.ToSlash(filepath.Clean(relToProject)),
				SourcePath:  sourcePath,
				Action:      strings.ToLower(strings.TrimSpace(ch.Action)),
				DesiredHash: desiredHash,
			})
		}
	}
	return out, nil
}

func pathHasRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if root == "." {
		return true
	}
	if path == root {
		return true
	}
	prefix := root + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}

func classifyTemplateFile(projectPath string, gf templateGeneratedFile, previousHash string, manifestCompatible bool) templateDriftFile {
	absPath := filepath.Join(projectPath, filepath.FromSlash(gf.Path))
	desired, derr := os.ReadFile(gf.SourcePath)
	desiredHash := sha256Hex(desired)
	prevHash := strings.TrimSpace(previousHash)
	current, cerr := os.ReadFile(absPath)
	if errors.Is(cerr, os.ErrNotExist) {
		return templateDriftFile{
			Path:            gf.Path,
			Action:          "create",
			Classification:  "safe_drift",
			Ownership:       "generated",
			MergeStrategy:   "write_desired",
			AutoApplicable:  derr == nil,
			Reason:          "file does not exist locally",
			PreviousHash:    prevHash,
			DesiredHash:     desiredHash,
			GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath)),
		}
	}
	if cerr != nil {
		return templateDriftFile{Path: gf.Path, Action: gf.Action, Classification: "conflicting_drift", Ownership: "mixed", AutoApplicable: false, Reason: cerr.Error(), PreviousHash: prevHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
	}
	currentHash := sha256Hex(current)
	if bytes.Equal(current, desired) {
		return templateDriftFile{Path: gf.Path, Action: "unchanged", Classification: "unchanged", Ownership: classifyOwnership(current), AutoApplicable: false, PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
	}

	merged, mergeStrategy, mergeOK := tryTemplatePreserveMerge(gf.Path, string(desired), string(current))
	if mergeOK {
		mergedHash := sha256Hex([]byte(merged))
		if mergedHash != currentHash {
			return templateDriftFile{Path: gf.Path, Action: "update", Classification: "safe_drift", Ownership: "mixed", MergeStrategy: mergeStrategy, AutoApplicable: true, Reason: "custom blocks can be preserved while applying template delta", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: mergedHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
		}
		return templateDriftFile{Path: gf.Path, Action: "unchanged", Classification: "unchanged", Ownership: "mixed", MergeStrategy: mergeStrategy, AutoApplicable: false, Reason: "template delta only touches preserved custom blocks", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: mergedHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
	}

	if manifestCompatible && prevHash != "" {
		switch {
		case currentHash == prevHash:
			return templateDriftFile{Path: gf.Path, Action: "update", Classification: "safe_drift", Ownership: classifyOwnership(current), MergeStrategy: "write_desired", AutoApplicable: true, Reason: "file matches last generated artifact; update is safe", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
		case desiredHash == prevHash:
			return templateDriftFile{Path: gf.Path, Action: "update", Classification: "manual_edits", Ownership: "manual", AutoApplicable: false, Reason: "current file diverged from last generated artifact but template output did not change", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
		default:
			return templateDriftFile{Path: gf.Path, Action: "update", Classification: "conflicting_drift", Ownership: "mixed", AutoApplicable: false, Reason: "both local file and template output diverged from last generated artifact", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
		}
	}

	if hasANGGeneratedHeader(string(current)) {
		return templateDriftFile{Path: gf.Path, Action: "update", Classification: "conflicting_drift", Ownership: "generated", AutoApplicable: false, Reason: "generated file changed locally but no compatible manifest is available", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
	}
	return templateDriftFile{Path: gf.Path, Action: "update", Classification: "manual_edits", Ownership: "manual", AutoApplicable: false, Reason: "file is not tracked as generated; preserving manual edits", PreviousHash: prevHash, CurrentHash: currentHash, DesiredHash: desiredHash, GeneratedSource: filepath.ToSlash(filepath.Clean(gf.SourcePath))}
}

func classifyTemplateDeletion(projectPath, relPath, previousHash string) templateDriftFile {
	absPath := filepath.Join(projectPath, filepath.FromSlash(relPath))
	current, err := os.ReadFile(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return templateDriftFile{Path: relPath, Action: "unchanged", Classification: "unchanged", Ownership: "generated", AutoApplicable: false, PreviousHash: previousHash, Reason: "previously generated file already absent"}
	}
	if err != nil {
		return templateDriftFile{Path: relPath, Action: "delete", Classification: "conflicting_drift", Ownership: "mixed", AutoApplicable: false, PreviousHash: previousHash, Reason: err.Error()}
	}
	currentHash := sha256Hex(current)
	if currentHash == strings.TrimSpace(previousHash) {
		return templateDriftFile{Path: relPath, Action: "delete", Classification: "safe_drift", Ownership: "generated", MergeStrategy: "delete_file", AutoApplicable: true, PreviousHash: previousHash, CurrentHash: currentHash, Reason: "file is no longer emitted and still matches last generated artifact"}
	}
	return templateDriftFile{Path: relPath, Action: "delete", Classification: "conflicting_drift", Ownership: "mixed", AutoApplicable: false, PreviousHash: previousHash, CurrentHash: currentHash, Reason: "file is no longer emitted but contains local edits since last generated artifact"}
}

func readCompatibleArtifactManifest(projectPath string) (artifactHashManifest, bool) {
	m, err := readArtifactHashManifest(projectPath)
	if err != nil {
		return artifactHashManifest{}, false
	}
	return m, true
}

func buildTemplateRebasePlan(report templateDiffReport) templateRebasePlan {
	plan := templateRebasePlan{Schema: "ang/template-rebase-plan/v1", ProjectPath: report.ProjectPath}
	for _, f := range report.Files {
		if f.Classification == "unchanged" {
			continue
		}
		if f.AutoApplicable {
			plan.Steps = append(plan.Steps, templateRebasePlanStep{Path: f.Path, Action: f.Action, MergeStrategy: firstNonEmpty(f.MergeStrategy, "write_desired"), Reason: f.Reason})
			switch f.Action {
			case "create":
				plan.Summary.ApplyCreate++
			case "delete":
				plan.Summary.ApplyDelete++
			default:
				plan.Summary.ApplyUpdate++
				if strings.Contains(strings.ToLower(f.MergeStrategy), "custom") {
					plan.Summary.PreserveMerge++
				}
			}
			continue
		}
		plan.Skipped = append(plan.Skipped, f)
		plan.Summary.Skipped++
	}
	plan.Summary.Total = len(plan.Steps)
	sort.Slice(plan.Steps, func(i, j int) bool { return plan.Steps[i].Path < plan.Steps[j].Path })
	sort.Slice(plan.Skipped, func(i, j int) bool { return plan.Skipped[i].Path < plan.Skipped[j].Path })
	return plan
}

func resolveTemplatePlan(projectPath string, report templateDiffReport, fromPath string) (templateRebasePlan, error) {
	if strings.TrimSpace(fromPath) == "" {
		return buildTemplateRebasePlan(report), nil
	}
	data, err := os.ReadFile(fromPath)
	if err != nil {
		return templateRebasePlan{}, fmt.Errorf("read --from file: %w", err)
	}
	var probe struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return templateRebasePlan{}, fmt.Errorf("parse --from file: %w", err)
	}
	switch strings.TrimSpace(probe.Schema) {
	case "ang/template-rebase-plan/v1":
		var plan templateRebasePlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return templateRebasePlan{}, fmt.Errorf("parse rebase plan: %w", err)
		}
		plan.ProjectPath = filepath.ToSlash(filepath.Clean(projectPath))
		return plan, nil
	case "ang/template-diff/v1":
		var diff templateDiffReport
		if err := json.Unmarshal(data, &diff); err != nil {
			return templateRebasePlan{}, fmt.Errorf("parse diff report: %w", err)
		}
		return buildTemplateRebasePlan(diff), nil
	default:
		return templateRebasePlan{}, fmt.Errorf("unsupported --from schema %q", probe.Schema)
	}
}

func applyTemplateRebase(projectPath string, report templateDiffReport, plan templateRebasePlan) (templateRebaseApplyResult, error) {
	byPath := map[string]templateDriftFile{}
	for _, f := range report.Files {
		byPath[f.Path] = f
	}
	res := templateRebaseApplyResult{Schema: "ang/template-rebase-apply/v1", ProjectPath: report.ProjectPath, Plan: plan.Steps, Skipped: plan.Skipped}
	for _, step := range plan.Steps {
		f, ok := byPath[step.Path]
		if !ok {
			return res, fmt.Errorf("missing drift file for %s", step.Path)
		}
		absPath := filepath.Join(projectPath, filepath.FromSlash(step.Path))
		switch step.Action {
		case "delete":
			if err := os.Remove(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return res, fmt.Errorf("delete %s: %w", step.Path, err)
			}
			res.DeletedFiles = append(res.DeletedFiles, step.Path)
		default:
			desiredBytes, err := resolveDesiredBytesForApply(projectPath, f)
			if err != nil {
				return res, fmt.Errorf("resolve desired content for %s: %w", step.Path, err)
			}
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				return res, fmt.Errorf("mkdir for %s: %w", step.Path, err)
			}
			if err := os.WriteFile(absPath, desiredBytes, 0o644); err != nil {
				return res, fmt.Errorf("write %s: %w", step.Path, err)
			}
			res.ChangedFiles = append(res.ChangedFiles, step.Path)
		}
	}
	sort.Strings(res.ChangedFiles)
	sort.Strings(res.DeletedFiles)
	return res, nil
}

func resolveDesiredBytesForApply(projectPath string, f templateDriftFile) ([]byte, error) {
	src := strings.TrimSpace(f.GeneratedSource)
	if src == "" {
		return nil, fmt.Errorf("missing generated source")
	}
	desired, err := os.ReadFile(filepath.FromSlash(src))
	if err != nil {
		return nil, err
	}
	currentPath := filepath.Join(projectPath, filepath.FromSlash(f.Path))
	current, _ := os.ReadFile(currentPath)
	merged, strategy, ok := tryTemplatePreserveMerge(f.Path, string(desired), string(current))
	if ok && strategy == f.MergeStrategy {
		return []byte(merged), nil
	}
	return desired, nil
}

func hasANGGeneratedHeader(s string) bool {
	head := s
	if len(head) > 256 {
		head = head[:256]
	}
	checks := []string{
		"Code generated by ANG",
		"AUTO-GENERATED by ANG",
		"Generated by ANG",
		"Code generated by ang import openapi. DO NOT EDIT.",
	}
	for _, chk := range checks {
		if strings.Contains(head, chk) {
			return true
		}
	}
	return false
}

func classifyOwnership(content []byte) string {
	s := string(content)
	switch {
	case strings.Contains(s, "ANG:BEGIN_CUSTOM"):
		return "mixed"
	case hasANGGeneratedHeader(s):
		return "generated"
	default:
		return "manual"
	}
}

func summarizeTemplateDiff(report *templateDiffReport) {
	for _, f := range report.Files {
		report.Summary.Total++
		switch f.Classification {
		case "safe_drift":
			report.Summary.SafeDrift++
		case "conflicting_drift":
			report.Summary.ConflictingDrift++
		case "manual_edits":
			report.Summary.ManualEdits++
		default:
			report.Summary.Unchanged++
		}
		switch f.Action {
		case "create":
			report.Summary.Creates++
		case "delete":
			report.Summary.Deletes++
		case "update":
			report.Summary.Updates++
		}
		if f.AutoApplicable {
			report.Summary.AutoApplicable++
		}
	}
}

func sha256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeTemplateJSON(w *os.File, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func printTemplateDiffReport(report templateDiffReport) {
	fmt.Printf("Template diff: files=%d safe=%d conflicts=%d manual=%d unchanged=%d auto_applicable=%d\n", report.Summary.Total, report.Summary.SafeDrift, report.Summary.ConflictingDrift, report.Summary.ManualEdits, report.Summary.Unchanged, report.Summary.AutoApplicable)
	for _, f := range report.Files {
		if f.Classification == "unchanged" {
			continue
		}
		fmt.Printf("  - [%s] %s %s", f.Classification, f.Action, f.Path)
		if f.MergeStrategy != "" {
			fmt.Printf(" (%s)", f.MergeStrategy)
		}
		fmt.Println()
		if f.Reason != "" {
			fmt.Printf("      reason: %s\n", f.Reason)
		}
	}
}

func printTemplateRebasePlan(plan templateRebasePlan) {
	fmt.Printf("Template rebase plan: apply=%d skipped=%d create=%d update=%d delete=%d preserve=%d\n", plan.Summary.Total, plan.Summary.Skipped, plan.Summary.ApplyCreate, plan.Summary.ApplyUpdate, plan.Summary.ApplyDelete, plan.Summary.PreserveMerge)
	for _, step := range plan.Steps {
		fmt.Printf("  - %s %s", step.Action, step.Path)
		if step.MergeStrategy != "" {
			fmt.Printf(" (%s)", step.MergeStrategy)
		}
		fmt.Println()
	}
	for _, skip := range plan.Skipped {
		fmt.Printf("  - skip %s [%s]\n", skip.Path, skip.Classification)
	}
}

func printTemplateRebaseResult(res templateRebaseApplyResult) {
	fmt.Printf("Template rebase applied: changed=%d deleted=%d skipped=%d\n", len(res.ChangedFiles), len(res.DeletedFiles), len(res.Skipped))
	for _, p := range res.ChangedFiles {
		fmt.Printf("  - update %s\n", p)
	}
	for _, p := range res.DeletedFiles {
		fmt.Printf("  - delete %s\n", p)
	}
	for _, skip := range res.Skipped {
		fmt.Printf("  - skip %s [%s]\n", skip.Path, skip.Classification)
	}
}
