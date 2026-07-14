package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/internal/mcp"
	"gopkg.in/yaml.v3"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n[ANG CRASHED] Unexpected error: %v\n", r)
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", debug.Stack())
			fmt.Fprintln(os.Stderr, "Please report: https://github.com/strogmv/ang/issues")
			fmt.Fprintln(os.Stderr, "Include your CUE files and this error message.")
			os.Exit(2)
		}
	}()

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]

	applyBuildProcessLimitsIfNeeded(cmd)

	switch cmd {
	case "init":
		runInit(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	case "lint":
		runLint(os.Args[2:])
	case "build":
		runBuild(os.Args[2:])
	case "up":
		runUp(os.Args[2:])
	case "first-run":
		runFirstRun(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "api-diff":
		runAPIDiff(os.Args[2:])
	case "db":
		runDB(os.Args[2:])
	case "contract-test":
		runContractTest()
	case "vet":
		runVet(os.Args[2:])
	case "explain":
		runExplain(os.Args[2:])
	case "draw":
		runDraw(os.Args[2:])
	case "rbac":
		runRBAC(os.Args[2:])
	case "events":
		runEvents(os.Args[2:])
	case "test":
		runTest(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "advise":
		runAdvise(os.Args[2:])
	case "smoke":
		runSmoke(os.Args[2:])
	case "config":
		runConfig(os.Args[2:])
	case "lsp":
		runLSP(os.Args[2:])
	case "flowfn":
		runFlowfn(os.Args[2:])
	case "hash":
		runHash(os.Args[2:])
	case "tips":
		runTips(os.Args[2:])
	case "ops":
		runOps(os.Args[2:])
	case "actions":
		runActions(os.Args[2:])
	case "patch":
		runPatch(os.Args[2:])
	case "fmt":
		runFmt(os.Args[2:])
	case "fix":
		runFix(os.Args[2:])
	case "context":
		runContext(os.Args[2:])
	case "template":
		runTemplate(os.Args[2:])
	case "extract":
		runExtract(os.Args[2:])
	case "import":
		runImport(os.Args[2:])
	case "openapi":
		runOpenAPI(os.Args[2:])
	case "sdk":
		runSDKBump(os.Args[2:])
	case "gen":
		runAIGen(os.Args[2:])
	case "pp":
		runPP(os.Args[2:])
	case "mcp":
		mcp.Run()
	case "version":
		runVersion()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("ANG — Architectural Normalized Generator v%s\n", compiler.Version)
	fmt.Println("\nUsage:")
	fmt.Println("  ang init [dir] [--template saas|ecommerce|marketplace] [--lang go] [--db postgres]")
	fmt.Println("  ang validate  Validate CUE models and architecture")
	fmt.Println("  ang lint      Perform deep semantic linting of flows and logic")
	fmt.Println("  ang build     Compile CUE intent into code and infra configs (--mode=in_place|release, --backend-dir, --dry-run, --run-tests, --skip-frontend, --skip-contract-tests, --log-format=json, --phase=all|plan|apply, --out-plan, --plan-file)")
	fmt.Println("  ang up        Local one-command bootstrap (doctor start + compose up + build + smoke; --frontend starts UI)")
	fmt.Println("  ang first-run Guided first launch (env bootstrap + infra/build + smoke)")
	fmt.Println("  ang status    Show local runtime/dev status (checks + health + compose)")
	fmt.Println("                Examples:")
	fmt.Println("                  ang build --mode=in_place --backend-dir .")
	fmt.Println("                  ang build --mode=release")
	fmt.Println("                Migration: projects using targets[].output_dir should set build.mode=\"release\" explicitly.")
	fmt.Println("  ang db sync   Synchronize DB schema with CUE (requires DATABASE_URL)")
	fmt.Println("  ang migrate   Run migration diff/apply using Atlas")
	fmt.Println("  ang api-diff  Compare OpenAPI specs and recommend semver bump")
	fmt.Println("  ang contract-test  Run generated HTTP/WS contract tests")
	fmt.Println("  ang test gen   Generate flow-derived test cases from CUE")
	fmt.Println("  ang vet       Check architectural invariants and laws")
	fmt.Println("  ang vet logic  Audit embedded Go snippets for syntax errors")
	fmt.Println("  ang rbac actions  List all registered RBAC actions (service.method)")
	fmt.Println("  ang rbac inspect  Audit RBAC policies for holes and errors")
	fmt.Println("  ang events map    Visualize end-to-end event journey (Pub/Sub)")
	fmt.Println("  ang doctor    Analyze build log and suggest concrete CUE fixes")
	fmt.Println("  ang doctor --code <CODE>  Show guidance for one diagnostic code")
	fmt.Println("  ang advise --goal project.audit [--json] [--expert-command PATH|--expert-url URL --expert-pack ID --facts FILE]  Read-only expert audit")
	fmt.Println("  ang doctor start  Preflight local startup checks (tools/env/compose/ports)")
	fmt.Println("  ang smoke     Check /health and /health/ready endpoints")
	fmt.Println("  ang tips      Beginner-friendly quick commands and recovery hints")
	fmt.Println("  ang ops schema  Machine-readable #Operation schema for AI (--json|--cue)")
	fmt.Println("  ang ops vet     Semantic validation of ops CUE files (--json)")
	fmt.Println("  ang actions   Print machine-readable flow action catalog (--json|--cue)")
	fmt.Println("  ang patch     Patch-first workflow: lint/plan/apply for structured CUE edits")
	fmt.Println("  ang fmt       Canonical format for CUE + flow alias normalization")
	fmt.Println("  ang fix       Rewrite deprecated flow aliases to canonical names")
	fmt.Println("  ang extract   Extract facts from Go/Java/OpenAPI/SQL for AI migration (--from=go|java|openapi|sql|auto, --out)")
	fmt.Println("  ang import java  Normalize Java sources into import IR and generate contract-layer CUE (--report --diff --update)")
	fmt.Println("  ang import openapi  Import OpenAPI spec into cue/api/http.cue + operations_*.cue + cue/domain/entities.cue (--report --diff --update)")
	fmt.Println("  ang template diff    Compare emitted project against current files and classify drift")
	fmt.Println("  ang template rebase  Plan/apply safe template deltas while preserving supported custom edits")
	fmt.Println("  ang config doctor  Validate runtime env against generated config schema")
	fmt.Println("  ang mcp       Run ANG MCP server over stdio")
	fmt.Println("  ang lsp --stdio  Run ANG language server (MVP diagnostics)")
	fmt.Println("  ang flowfn transpile  Parse flowfn and print transpiled flow array (--format cue-array|json)")
	fmt.Println("  ang flowfn validate   Validate flowfn and print diagnostics JSON")
	fmt.Println("  ang flowfn complete   Return completion items JSON (--line --character)")
	fmt.Println("  ang flowfn hover      Return hover JSON (--line --character)")
	fmt.Println("  ang explain   Explain CODE or error-json with fix hints (--json)")
	fmt.Println("  ang draw      Generate architecture diagrams (Mermaid)")
	fmt.Println("  ang hash      Show current project hash (CUE + Templates, or --artifacts)")
	fmt.Println("  ang openapi   Generate OpenAPI spec without full build (--out, --stdout)")
	fmt.Println("  ang sdk bump [patch|minor|major]  Bump version in cue/project/project.cue")
	fmt.Println("  ang sdk version                   Show current version from project.cue")
	fmt.Println("  ang gen       Generate CUE operations from ang/facts/v1 via AI (--facts, --service, --out, --dry-run)")
	fmt.Println("  ang pp schema sync|check|list  Payment-provider schema bundle")
	fmt.Println("  ang pp vet [path]              Semantic validation of provider CUE intent")
}

func runHash(args []string) {
	fs := flag.NewFlagSet("hash", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	artifacts := fs.Bool("artifacts", false, "print artifact hash manifest from .ang/cache/manifest.json")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Hash FAILED: %v\n", err)
		os.Exit(1)
	}
	if *artifacts {
		m, err := readArtifactHashManifest(".")
		if err != nil {
			fmt.Printf("Hash FAILED: read artifact manifest: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Schema Version:   %s\n", m.SchemaVersion)
		fmt.Printf("Compiler Version: %s\n", m.CompilerVersion)
		if strings.TrimSpace(m.CompilerFingerprint) != "" {
			fmt.Printf("Compiler Fingerprint: %s\n", m.CompilerFingerprint)
		}
		fmt.Printf("IR Version:       %s\n", m.IRVersion)
		if strings.TrimSpace(m.IRCanonicalVersion) != "" {
			fmt.Printf("IR Canonical:     %s\n", m.IRCanonicalVersion)
		}
		if strings.TrimSpace(m.InputHash) != "" {
			fmt.Printf("Input Hash:       %s\n", m.InputHash)
		}
		if strings.TrimSpace(m.TemplateHash) != "" {
			fmt.Printf("Template Hash:    %s\n", m.TemplateHash)
		}
		fmt.Printf("Artifacts:        %d\n", len(m.Artifacts))
		for _, a := range m.Artifacts {
			fmt.Printf("%s  %s\n", a.Hash, a.Path)
		}
		return
	}
	projCfg := loadProjectConfig(".")
	inputHash, err := calculateHash([]string{projCfg.CueRoot})
	if err != nil {
		fmt.Printf("Hash FAILED: calculate input hash: %v\n", err)
		os.Exit(1)
	}
	templateHash, err := calculateEmbeddedTemplateHash()
	if err != nil {
		fmt.Printf("Hash FAILED: calculate embedded template hash: %v\n", err)
		os.Exit(1)
	}
	compilerFingerprint := compiler.BuildFingerprint()
	fmt.Printf("ANG Version:  %s\n", compiler.Version)
	fmt.Printf("Input Hash:   %s (%s/)\n", inputHash, projCfg.CueRoot)
	fmt.Printf("Template Hash: %s (embedded templates)\n", templateHash)
	fmt.Printf("Compiler Fingerprint: %s (binary + IR ABI)\n", compilerFingerprint)
}

func calculateHash(dirs []string) (string, error) {
	h := sha256.New()
	type hashInput struct {
		path string
		key  string
	}
	var files []hashInput
	for dirIndex, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return "", fmt.Errorf("hash directory is empty")
		}
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, hashInput{path: path, key: fmt.Sprintf("%d/%s", dirIndex, filepath.ToSlash(rel))})
			return nil
		}); err != nil {
			return "", fmt.Errorf("walk hash directory %s: %w", dir, err)
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no files found in hash directories")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].key < files[j].key })
	for _, file := range files {
		f, err := os.Open(file.path)
		if err != nil {
			return "", fmt.Errorf("open hash input %s: %w", file.path, err)
		}
		_, _ = h.Write([]byte(file.key))
		_, _ = h.Write([]byte{0})
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("hash input %s: %w", file.path, err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("close hash input %s: %w", file.path, err)
		}
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func readGoModuleAt(projectPath string) string {
	base := strings.TrimSpace(projectPath)
	if base == "" {
		base = "."
	}
	data, err := os.ReadFile(filepath.Join(base, "go.mod"))
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}
	type angYAML struct {
		Go struct {
			Module string `yaml:"module"`
		} `yaml:"go"`
	}
	data, err = os.ReadFile(filepath.Join(base, "ang.yaml"))
	if err != nil {
		return ""
	}
	var cfg angYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Go.Module)
}

func readGoModule() string {
	return readGoModuleAt(".")
}

// ProjectConfig holds optional overrides loaded from ang.yaml.
type ProjectConfig struct {
	CueRoot      string // CUE intent directory (default: "cue", e.g. ".cue" for hidden)
	TemplatesDir string // Custom templates directory (default: "templates")
	SchemaDir    string // Shared schema directory (optional, e.g. "../.ang/schema")
}

func loadProjectConfig(projectPath string) ProjectConfig {
	base := strings.TrimSpace(projectPath)
	if base == "" {
		base = "."
	}
	type angYAMLFull struct {
		CueRoot      string `yaml:"cue_root"`
		TemplatesDir string `yaml:"templates_dir"`
		SchemaDir    string `yaml:"schema_dir"`
	}
	defaults := ProjectConfig{
		CueRoot:      compiler.DefaultCueRoot,
		TemplatesDir: "templates",
	}
	data, err := os.ReadFile(filepath.Join(base, "ang.yaml"))
	if err != nil {
		return defaults
	}
	var cfg angYAMLFull
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProjectConfig{}
	}
	pc := ProjectConfig{
		CueRoot:      strings.TrimSpace(cfg.CueRoot),
		TemplatesDir: strings.TrimSpace(cfg.TemplatesDir),
		SchemaDir:    strings.TrimSpace(cfg.SchemaDir),
	}
	if pc.CueRoot == "" {
		pc.CueRoot = compiler.DefaultCueRoot
	}
	if pc.TemplatesDir == "" {
		pc.TemplatesDir = "templates"
	}
	return pc
}

func runInit(args []string) {
	parseArgs := append([]string(nil), args...)
	targetDir := "."
	if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
		targetDir = parseArgs[0]
		parseArgs = parseArgs[1:]
	}

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	templateName := fs.String("template", "", "project template: saas|ecommerce|marketplace")
	lang := fs.String("lang", "go", "target language")
	db := fs.String("db", "postgres", "database backend")
	module := fs.String("module", "", "CUE module path (defaults to github.com/example/<dir>)")
	force := fs.Bool("force", false, "allow writing into a non-empty target directory")
	fun := fs.Bool("fun", false, "show fun onboarding banner after scaffold generation")
	if err := fs.Parse(parseArgs); err != nil {
		printCommandFailure("Init", err.Error(), "run `ang init --help`")
		os.Exit(1)
	}

	if fs.NArg() > 0 {
		targetDir = fs.Arg(0)
	}
	targetDir = filepath.Clean(targetDir)
	projectName := filepath.Base(targetDir)
	if projectName == "." || projectName == string(filepath.Separator) || strings.TrimSpace(projectName) == "" {
		projectName = "ang-app"
	}
	modulePath := strings.TrimSpace(*module)
	if modulePath == "" {
		modulePath = "github.com/example/" + sanitizeProjectName(projectName)
	}

	if strings.TrimSpace(*templateName) == "" {
		if err := initLegacyScaffold(targetDir, modulePath, *lang, *db); err != nil {
			printCommandFailure("Init", err.Error(), "retry in an empty directory or use `--force`")
			os.Exit(1)
		}
		fmt.Println("Minimal project scaffold initialized successfully.")
		printInitOnboarding(projectName, targetDir, false, isFunEnabled(*fun))
		return
	}

	opts := initTemplateOptions{
		TemplateName: strings.ToLower(strings.TrimSpace(*templateName)),
		TargetDir:    targetDir,
		ProjectName:  projectName,
		Lang:         strings.ToLower(strings.TrimSpace(*lang)),
		DB:           strings.ToLower(strings.TrimSpace(*db)),
		ModulePath:   modulePath,
		Force:        *force,
	}
	if err := initFromTemplate(opts); err != nil {
		printCommandFailure("Init", err.Error(), "retry in an empty directory or use `--force`")
		os.Exit(1)
	}
	fmt.Printf("Template %q initialized in %s\n", opts.TemplateName, targetDir)
	printInitOnboarding(projectName, targetDir, true, isFunEnabled(*fun))
}

func initLegacyScaffold(root, modulePath, lang, db string) error {
	if strings.TrimSpace(modulePath) == "" {
		modulePath = "github.com/example/" + sanitizeProjectName(filepath.Base(root))
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		lang = "go"
	}
	db = strings.ToLower(strings.TrimSpace(db))
	if db == "" {
		db = "postgres"
	}
	framework := defaultFrameworkForLang(lang)

	cr := compiler.DefaultCueRoot
	dirs := []string{
		filepath.Join(root, cr, "domain"),
		filepath.Join(root, cr, "api"),
		filepath.Join(root, cr, "policies"),
		filepath.Join(root, cr, "invariants"),
		filepath.Join(root, cr, "architecture"),
		filepath.Join(root, cr, "repo"),
		filepath.Join(root, cr, "schema"),
		filepath.Join(root, cr, "project"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	moduleContent := fmt.Sprintf(`module: %q
language: {
	version: "v0.9.0"
}
`, modulePath)
	projectContent := fmt.Sprintf(`package project

state: {
	target: {
		lang:      %q
		framework: %q
		db:        %q
	}

	targets: [{
		name:       %q
		lang:       %q
		framework:  %q
		db:         %q
		output_dir: "dist/release/%s-service"
	}]
}
`, lang, framework, db, lang, lang, framework, db, lang)
	domainContent := `package domain

#HealthSnapshot: {
	name: "HealthSnapshot"
	fields: {
		status:    {type: "string"}
		checkedAt: {type: "time"}
	}
}
`
	archContent := `package architecture

#Services: {
	system: {
		name:        "System"
		description: "Minimal bootstrap service"
		entities:    []
	}
}
`
	httpContent := `package api

HTTP: {
	Health: {
		method: "GET"
		path:   "/health"
	}
}
`
	opsContent := `package api

Health: {
	service: "system"
	output: {
		status: string
	}
	impl_steps: [
		{
			action: "mapping.Assign"
			to:     "resp.Status"
			value:  "\"ok\""
		},
	]
}
`
	repoContent := `package repo

Repositories: {}
`
	rbacContent := `package policies

#RBAC: {
	roles: {
		admin: ["health.read"]
	}
	permissions: {
		"health.read": "Read service health"
	}
}
`
	taskfileContent := `version: "3"

tasks:
  up:
    cmds:
      - ang up

  build:
    cmds:
      - ang build

  validate:
    cmds:
      - ang validate

  lint:
    cmds:
      - ang lint

  doctor:
    cmds:
      - ang doctor start
`
	goModContent := fmt.Sprintf("module %s\n\ngo %s\n", modulePath, detectRootGoVersion("go.mod"))
	goWorkContent := fmt.Sprintf("go %s\n\nuse .\n", detectRootGoVersion("go.mod"))
	runbookContent := `# Project runbook

## Normal development cycle

    ang validate
    ang build
    ang doctor --project-path .
    ang up --frontend

Edit intent under cue/. Never edit generated internal/, api/, sdk/,
db/schema/, or db/queries/ files directly. Use ang build --log-format json
for machine-readable diagnostics and ang doctor --code <CODE> for guidance.
Failed generation is staged and leaves the previous generated tree unchanged.
Breaking OpenAPI changes require explicit ang build --accept-contract.
`

	modFile := filepath.Join(root, "cue.mod", "module.cue")
	if err := os.MkdirAll(filepath.Dir(modFile), 0755); err != nil {
		return fmt.Errorf("create cue.mod: %w", err)
	}

	writeIfMissing := func(path string, content string) error {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check file %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write file %s: %w", path, err)
		}
		return nil
	}

	files := map[string]string{
		modFile:                                                 moduleContent,
		filepath.Join(root, "go.mod"):                           goModContent,
		filepath.Join(root, "go.work"):                          goWorkContent,
		filepath.Join(root, "Taskfile.yml"):                     taskfileContent,
		filepath.Join(root, "RUNBOOK.md"):                       runbookContent,
		filepath.Join(root, cr, "project", "project.cue"):       projectContent,
		filepath.Join(root, cr, "domain", "entities.cue"):       domainContent,
		filepath.Join(root, cr, "architecture", "services.cue"): archContent,
		filepath.Join(root, cr, "api", "http.cue"):              httpContent,
		filepath.Join(root, cr, "api", "operations.cue"):        opsContent,
		filepath.Join(root, cr, "repo", "repositories.cue"):     repoContent,
		filepath.Join(root, cr, "policies", "rbac.cue"):         rbacContent,
	}
	for path, content := range files {
		if err := writeIfMissing(path, content); err != nil {
			return err
		}
	}
	return nil
}
