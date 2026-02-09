package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	"github.com/strogmv/ang/internal/mcp"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit()
	case "validate":
		runValidate(os.Args[2:])
	case "lint":
		runLint(os.Args[2:])
	case "build":
		runBuild(os.Args[2:])
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
	case "hash":
		runHash()
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
	fmt.Println("  ang init      Initialize a new ANG project structure")
	fmt.Println("  ang validate  Validate CUE models and architecture")
	fmt.Println("  ang lint      Perform deep semantic linting of flows and logic")
	fmt.Println("  ang build     Compile CUE intent into Go code and infra configs")
	fmt.Println("  ang db sync   Synchronize DB schema with CUE (requires DATABASE_URL)")
	fmt.Println("  ang migrate   Run migration diff/apply using Atlas")
	fmt.Println("  ang api-diff  Compare OpenAPI specs and recommend semver bump")
	fmt.Println("  ang contract-test  Run generated HTTP/WS contract tests")
	fmt.Println("  ang vet       Check architectural invariants and laws")
	fmt.Println("  ang vet logic  Audit embedded Go snippets for syntax errors")
	fmt.Println("  ang rbac actions  List all registered RBAC actions (service.method)")
	fmt.Println("  ang rbac inspect  Audit RBAC policies for holes and errors")
	fmt.Println("  ang events map    Visualize end-to-end event journey (Pub/Sub)")
	fmt.Println("  ang explain   Explain a lint code with examples")
	fmt.Println("  ang draw      Generate architecture diagrams (Mermaid)")
	fmt.Println("  ang hash      Show current project hash (CUE + Templates)")
}

func runHash() {
	inputHash, _ := calculateHash([]string{"cue"})
	compilerHash, _ := calculateHash([]string{"templates"})
	fmt.Printf("ANG Version:  %s\n", compiler.Version)
	fmt.Printf("Input Hash:   %s (cue/)\n", inputHash)
	fmt.Printf("Compiler Hash: %s (templates/)\n", compilerHash)
}

func calculateHash(dirs []string) (string, error) {
	h := sha256.New()
	var files []string
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
	}
	sort.Strings(files)
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		io.Copy(h, f)
		f.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func readGoModule() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func runInit() {
	fmt.Println("Initializing ANG project...")
	dirs := []string{
		"cue/domain",
		"cue/api",
		"cue/policies",
		"cue/invariants",
		"cue/architecture",
		"cue/repo",
		"cue/schema",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("Error creating directory %s: %v\n", dir, err)
			return
		}
	}

	moduleContent := `module: "github.com/strogmv/ang"
language: {
	version: "v0.9.0"
}
`
	modFile := "cue.mod/module.cue"
	if err := os.MkdirAll("cue.mod", 0755); err != nil {
		fmt.Printf("Error creating cue.mod: %v\n", err)
		return
	}
	if _, err := os.Stat(modFile); os.IsNotExist(err) {
		os.WriteFile(modFile, []byte(moduleContent), 0644)
	}

	fmt.Println("Project structure initialized successfully.")
}

func runValidate(args []string) {
	fmt.Println("Validating architecture...")
	projectPath := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
	}
	_, _, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)

	hasErrors := false
	for _, d := range compiler.LatestDiagnostics {
		severity := "WARN"
		if d.Severity != "" {
			severity = strings.ToUpper(d.Severity)
		}
		if severity == "ERROR" {
			hasErrors = true
		}
		fmt.Fprintf(os.Stderr, "⚠️  %s: %s\n", severity, d.Message)
		if d.File != "" {
			fmt.Fprintf(os.Stderr, "   at %s:%d:%d\n", d.File, d.Line, d.Column)
		}
		if d.Hint != "" {
			fmt.Fprintf(os.Stderr, "   💡 Hint: %s\n", d.Hint)
		}
	}

	if err != nil {
		fmt.Printf("Validation FAILED: %v\n", err)
		os.Exit(1)
	}
	if hasErrors {
		fmt.Println("Validation FAILED due to diagnostic errors.")
		os.Exit(1)
	}
	fmt.Println("Validation SUCCESSFUL.")
}

func runMigrate(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang migrate <diff|apply> [name]")
		os.Exit(1)
	}
	switch args[0] {
	case "diff":
		if len(args) < 2 {
			fmt.Println("Usage: ang migrate diff <name>")
			os.Exit(1)
		}
		name := args[1]
		if err := runAtlasDiff(name); err != nil {
			fmt.Printf("Migrate diff FAILED: %v\n", err)
			os.Exit(1)
		}
	case "apply":
		if err := runAtlasApply(); err != nil {
			fmt.Printf("Migrate apply FAILED: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown migrate command: %s\n", args[0])
		os.Exit(1)
	}
}

func runAPIDiff(args []string) {
	fs := flag.NewFlagSet("api-diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	basePath := fs.String("base", "api/openapi.base.yaml", "baseline OpenAPI spec")
	currentPath := fs.String("current", "api/openapi.yaml", "current OpenAPI spec")
	writeBase := fs.Bool("write-base", false, "overwrite base with current")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}

	if *writeBase {
		data, err := os.ReadFile(*currentPath)
		if err != nil {
			fmt.Printf("API diff FAILED: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*basePath, data, 0644); err != nil {
			fmt.Printf("API diff FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Baseline written to %s\n", *basePath)
		return
	}

	base, err := parseOpenAPI(*basePath)
	if err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}
	current, err := parseOpenAPI(*currentPath)
	if err != nil {
		fmt.Printf("API diff FAILED: %v\n", err)
		os.Exit(1)
	}

	report := diffOpenAPI(base, current)
	printAPIDiff(report)
}

func runContractTest() {
	cmd := exec.Command("go", "test", "-tags=contract", "./tests/contract/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Contract tests FAILED: %v\n", err)
		os.Exit(1)
	}
}

type explainEntry struct {
	Code        string
	Title       string
	Description string
	Example     string
}

func runExplain(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang explain <CODE>")
		os.Exit(1)
	}
	code := strings.ToUpper(strings.TrimSpace(args[0]))
	explanations := map[string]explainEntry{
		"MISSING_ID": {
			Code:        "MISSING_ID",
			Title:       "Missing ID assignment before repo.Save",
			Description: "When creating a new entity, an ID must be set before saving. ANG expects an explicit mapping.Assign to ensure deterministic identifiers.",
			Example:     "{action: \"mapping.Assign\", to: \"newItem.ID\", value: \"uuid.NewString()\"}",
		},
		"MISSING_CREATED_AT": {
			Code:        "MISSING_CREATED_AT",
			Title:       "Missing CreatedAt assignment before repo.Save",
			Description: "New entities should set CreatedAt to RFC3339 in UTC so storage is consistent and sortable.",
			Example:     "{action: \"mapping.Assign\", to: \"newItem.CreatedAt\", value: \"time.Now().UTC().Format(time.RFC3339)\"}",
		},
		"MISSING_OUTPUT": {
			Code:        "MISSING_OUTPUT",
			Title:       "Missing output variable in repo.Find/repo.List",
			Description: "Repo operations must declare an output variable to store the fetched entity or list.",
			Example:     "{action: \"repo.Find\", source: \"Item\", input: \"req.ItemID\", output: \"item\"}",
		},
		"MISSING_INPUT": {
			Code:        "MISSING_INPUT",
			Title:       "Missing input in repo or mapping step",
			Description: "Repo operations need an input (ID or filter) to identify what to fetch or save.",
			Example:     "{action: \"repo.Find\", source: \"Item\", input: \"req.ItemID\", output: \"item\"}",
		},
		"MISSING_SOURCE": {
			Code:        "MISSING_SOURCE",
			Title:       "Missing source entity in repo step",
			Description: "Repo operations must specify the source entity to select the correct repository.",
			Example:     "{action: \"repo.Save\", source: \"Item\", input: \"item\"}",
		},
		"MISSING_ENTITY": {
			Code:        "MISSING_ENTITY",
			Title:       "Missing entity in mapping.Map",
			Description: "mapping.Map requires an entity when creating new domain objects to infer the correct type.",
			Example:     "{action: \"mapping.Map\", output: \"newItem\", entity: \"Item\"}",
		},
	}

	if entry, ok := explanations[code]; ok {
		fmt.Printf("%s\n%s\n\nExample:\n%s\n", entry.Title, entry.Description, entry.Example)
		return
	}

	fmt.Printf("Unknown code: %s\n", code)
	os.Exit(1)
}

func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit structured JSON output")
	checkTestCov := fs.Bool("check-test-coverage", false, "check test coverage for endpoints")
	generateStubs := fs.Bool("generate-stubs", false, "generate stubs for missing tests")
	testDir := fs.String("test-dir", "tests", "directory containing test files")
	minCoverage := fs.Float64("min-coverage", 0, "minimum required coverage percentage (0-100)")
	verbose := fs.Bool("verbose", false, "show detailed output")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("\n❌ Lint FAILED: %v\n", err)
		os.Exit(1)
	}

	if *checkTestCov {
		runTestCoverageCheck(*testDir, *minCoverage, *verbose, *jsonOut, *generateStubs)
		return
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	if *jsonOut {
		var warnings []normalizer.Warning
		_, _, _, _, _, _, _, _, err := compiler.RunPipelineWithOptions(projectPath, compiler.PipelineOptions{
			WarningSink: func(w normalizer.Warning) { warnings = append(warnings, w) },
		})

		violations, lintErr := runCueLint(projectPath)

		report := lintReport{
			OK:       err == nil && lintErr == nil && len(violations) == 0,
			Warnings: warnings,
		}
		if err != nil {
			report.Errors = append(report.Errors, lintError{Message: err.Error()})
		}
		if lintErr != nil {
			report.Errors = append(report.Errors, lintError{Message: lintErr.Error()})
		}
		for _, v := range violations {
			report.Errors = append(report.Errors, v)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Lint FAILED: %v\n", encErr)
			os.Exit(1)
		}
		if err != nil || lintErr != nil || len(violations) > 0 {
			os.Exit(1)
		}
		return
	}

	fmt.Println("Linting intent...")
	_, _, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		fmt.Printf("\n❌ Lint FAILED: %v\n", err)
		os.Exit(1)
	}

	violations, lintErr := runCueLint(projectPath)
	if lintErr != nil {
		fmt.Printf("\n❌ Lint FAILED: %v\n", lintErr)
		os.Exit(1)
	}
	if len(violations) > 0 {
		fmt.Println("\n❌ Lint FAILED. Violations:")
		for _, v := range violations {
			fmt.Printf("  - %s\n", v.Message)
		}
		os.Exit(1)
	}

	fmt.Println("\n✅ Lint SUCCESSFUL.")
}

func runTestCoverageCheck(testDir string, minCoverage float64, verbose bool, jsonOut bool, generateStubs bool) {
	if !jsonOut {
		fmt.Println("Checking test coverage...")
	}

	_, _, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("\n❌ Test coverage check FAILED: %v\n", err)
		os.Exit(1)
	}

	report, err := checkTestCoverage(endpoints, testDir)
	if err != nil {
		fmt.Printf("\n❌ Test coverage check FAILED: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Test coverage check FAILED: %v\n", err)
			os.Exit(1)
		}
	} else {
		printTestCoverageReport(report, verbose)
	}

	if generateStubs && len(report.MissingTests) > 0 {
		fmt.Println("\nGenerating test stubs for missing endpoints...")
		em := emitter.New(".", "sdk", "templates")

		var missing []normalizer.Endpoint
		missingMap := make(map[string]bool)
		for _, m := range report.MissingTests {
			missingMap[m.Method+" "+m.Path] = true
		}
		for _, ep := range endpoints {
			if missingMap[ep.Method+" "+ep.Path] {
				missing = append(missing, ep)
			}
		}

		if err := em.EmitTestStubs(missing, "NEW-endpoint-stubs.test.ts"); err != nil {
			fmt.Printf("❌ Failed to generate test stubs: %v\n", err)
		}
	}

	if minCoverage > 0 && report.CoveragePercent < minCoverage {
		fmt.Printf("\n❌ Test coverage %.1f%% is below minimum required %.1f%%\n", report.CoveragePercent, minCoverage)
		os.Exit(1)
	}

	if len(report.MissingTests) > 0 {
		fmt.Printf("\n⚠️  %d endpoints without test coverage\n", len(report.MissingTests))
	} else {
		fmt.Println("\n✅ All endpoints have test coverage!")
	}
}

func runCueLint(projectPath string) ([]lintError, error) {
	p := parser.New()
	val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/lint"))
	if err != nil || !ok {
		return nil, err
	}
	if err := val.Validate(); err != nil {
		return nil, err
	}

	errVal := val.LookupPath(cue.ParsePath("validation_errors"))
	if !errVal.Exists() {
		return nil, nil
	}

	var violations []lintError
	iter, _ := errVal.Fields()
	for iter.Next() {
		violations = append(violations, lintError{
			Message: fmt.Sprintf("%s: %s", iter.Selector(), iter.Value()),
		})
	}
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Message < violations[j].Message
	})
	return violations, nil
}

func runOptionalMCPGeneration(projectPath string) error {
	root := projectPath
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	script := filepath.Join(root, "scripts", "gen_mcp_server.sh")
	if _, err := os.Stat(script); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mcp generation failed: %w", err)
	}
	return nil
}

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

		output, err := parseOutputOptions(args)
		if err != nil {
			fmt.Printf("Build FAILED: %v\n", err)
			return
		}

		entities, services, endpoints, repos, events, bizErrors, schedules, scenarios, err := compiler.RunPipeline(projectPath)
		if err != nil {
			fmt.Printf("Build FAILED during validation: %v\n", err)
			return
		}
		_ = scenarios

		p := parser.New()
		n := normalizer.New()

		var cfgDef *normalizer.ConfigDef
		var authDef *normalizer.AuthDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/infra"); err != nil {
			fmt.Printf("Build FAILED during config load: %v\n", err)
			return
		} else if ok {
			cfgDef, err = n.ExtractConfig(val)
			if err != nil {
				fmt.Printf("Build FAILED during config parse: %v\n", err)
				return
			}
			authDef, err = n.ExtractAuth(val)
			if err != nil {
				fmt.Printf("Build FAILED during auth parse: %v\n", err)
				return
			}
		}

		var rbacDef *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/rbac"); err != nil {
			fmt.Printf("Build FAILED during RBAC load: %v\n", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fmt.Printf("Build FAILED during RBAC parse: %v\n", err)
				return
			}
		} else if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/policies"); err != nil {
			fmt.Printf("Build FAILED during RBAC load: %v\n", err)
			return
		} else if ok {
			rbacDef, err = n.ExtractRBAC(val)
			if err != nil {
				fmt.Printf("Build FAILED during RBAC parse: %v\n", err)
				return
			}
		}

		var views []normalizer.ViewDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/views"); err != nil {
			fmt.Printf("Build FAILED during views load: %v\n", err)
			return
		} else if ok {
			views, err = n.ExtractViews(val)
			if err != nil {
				fmt.Printf("Build FAILED during views parse: %v\n", err)
				return
			}
		}

		var projectDef *normalizer.ProjectDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/project"); err != nil {
			fmt.Printf("Build FAILED during project load: %v\n", err)
			return
		} else if ok {
			projectDef, err = n.ExtractProject(val)
			if err != nil {
				fmt.Printf("Build FAILED during project parse: %v\n", err)
				return
			}
		}

		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/schema"); err == nil && ok {
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

		em := emitter.New(output.BackendDir, output.FrontendDir, "templates")
		em.FrontendAdminDir = output.FrontendAdminDir
		em.Version = compiler.Version
		em.InputHash = inputHash
		em.CompilerHash = compilerHash
		em.GoModule = goModule
		pythonSDKEnabled := strings.TrimSpace(os.Getenv("ANG_PY_SDK")) == "1"

		ctx := em.AnalyzeContext(services, entities, endpoints)
		ctx.HasScheduler = len(schedules) > 0
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
		for _, ev := range events {
			ent := normalizer.Entity{Name: ev.Name, Fields: ev.Fields}
			ctx.EventPayloads[ev.Name] = ent
		}

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
			fmt.Printf("Build FAILED during IR transformation: %v\n", err)
			return
		}

		entities = emitter.IREntitiesToNormalizer(irSchema.Entities)
		services = emitter.IRServicesToNormalizer(irSchema.Services)
		endpoints = emitter.IREndpointsToNormalizer(irSchema.Endpoints)
		repos = emitter.IRReposToNormalizer(irSchema.Repos)
		events = emitter.IREventsToNormalizer(irSchema.Events)
		bizErrors = emitter.IRErrorsToNormalizer(irSchema.Errors)
		schedules = emitter.IRSchedulesToNormalizer(irSchema.Schedules)

		if err := emitter.ValidateServiceDependencies(services); err != nil {
			fmt.Printf("Build FAILED due to service dependency validation: %v\n", err)
			return
		}

		services = emitter.OrderServicesByDependencies(services)
		ctx.Services = services
		ctx.Entities = entities

		for _, ep := range endpoints {
			if strings.ToUpper(ep.Method) == "WS" {
				ctx.WebSocketServices[ep.ServiceName] = true
				if ctx.WSEventMap[ep.ServiceName] == nil {
					ctx.WSEventMap[ep.ServiceName] = make(map[string]bool)
				}
				for _, msg := range ep.Messages {
					if msg != "" {
						ctx.WSEventMap[ep.ServiceName][msg] = true
					}
				}
				if ctx.WSRoomField[ep.ServiceName] == "" {
					param := ep.RoomParam
					if param == "" {
						param = firstPathParam(ep.Path)
					}
					if param != "" {
						ctx.WSRoomField[ep.ServiceName] = emitter.ExportName(param)
					}
				}
			}
		}

		projContent, _ := os.ReadFile("cue/project.cue")
		isMicroservice := strings.Contains(string(projContent), `build_strategy: "microservices"`)

		steps := []struct {
			name string
			fn   func() error
		}{
			{"Config", func() error { return em.EmitConfig(cfgDef) }},
			{"Logger", func() error { return em.EmitLogger() }},
			{"RBAC", func() error { return em.EmitRBAC(rbacDef) }},
			{"Domain Entities", func() error { return em.EmitDomain(irSchema.Entities) }},
			{"DTOs", func() error { return em.EmitDTO(irSchema.Entities) }},
			{"Service Ports", func() error { return em.EmitService(services) }},
			{"HTTP Handlers", func() error { return em.EmitHTTP(endpoints, services, events, authDef) }},
			{"Health Probes", func() error { return em.EmitHealth() }},
			{"Repository Ports", func() error { return em.EmitRepository(repos, entities) }}, {"Transaction Port", func() error { return em.EmitTransactionPort() }},
			{"Storage Port", func() error { return em.EmitStoragePort() }},
			{"S3 Client", func() error { return em.EmitS3Client() }},
			{"Postgres Repos", func() error { return em.EmitPostgresRepo(repos, entities) }},
			{"Postgres Common", func() error { return em.EmitPostgresCommon() }},
			{"Mongo Repos", func() error { return em.EmitMongoRepo(repos, entities) }},
			{"Mongo Common", func() error { return em.EmitMongoCommon(entities) }},
			{"SQL Schema", func() error { return em.EmitSQL(entities) }},
			{"Infra Configs", func() error { return em.EmitInfraConfigs() }},
			{"SQL Queries", func() error { return em.EmitSQLQueries(entities) }},
			{"Mongo Schemas", func() error { return em.EmitMongoSchema(entities) }},
			{"Repo Stubs", func() error { return em.EmitStubRepo(repos, entities) }},
			{"Redis Client", func() error { return em.EmitRedisClient() }},
			{"Auth Package", func() error { return em.EmitAuthPackage(authDef) }},
			{"Refresh Store Port", func() error { return em.EmitRefreshTokenStorePort() }},
			{"Refresh Store Memory", func() error { return em.EmitRefreshTokenStoreMemory() }},
			{"Refresh Store Redis", func() error { return em.EmitRefreshTokenStoreRedis() }},
			{"Refresh Store Postgres", func() error { return em.EmitRefreshTokenStorePostgres() }},
			{"Refresh Store Hybrid", func() error { return em.EmitRefreshTokenStoreHybrid() }},
			{"Mailer Port", func() error { return em.EmitMailerPort() }},
			{"SMTP Client", func() error { return em.EmitMailerAdapter() }},
			{"Events", func() error { return em.EmitEvents(events) }},
			{"Scheduler", func() error { return em.EmitScheduler(schedules) }},
			{"Publisher Interface", func() error { return em.EmitPublisherInterface(services, schedules) }},
			{"NATS Adapter", func() error { return em.EmitNatsAdapter(services, schedules) }},
			{"Metrics Middleware", func() error { return em.EmitMetrics() }},
			{"Logging Middleware", func() error { return em.EmitLoggingMiddleware() }},
			{"Errors", func() error { return em.EmitErrors(bizErrors) }},
			{"Views", func() error { return em.EmitViews(views) }},
			{"OpenAPI", func() error { return em.EmitOpenAPI(endpoints, services, bizErrors, projectDef) }},
			{"AsyncAPI", func() error { return em.EmitAsyncAPI(events, projectDef) }},
			{"Contract Tests", func() error { return em.EmitContractTests(endpoints, services) }},
			{"E2E Behavioral Tests", func() error { return em.EmitE2ETests(scenarios) }},
			{"Test Stubs", func() error {
				if output.TestStubs {
					report, err := checkTestCoverage(endpoints, "tests")
					if err != nil {
						return fmt.Errorf("check coverage: %w", err)
					}
					var missing []normalizer.Endpoint
					missingMap := make(map[string]bool)
					for _, m := range report.MissingTests {
						missingMap[m.Method+" "+m.Path] = true
					}
					for _, ep := range endpoints {
						if missingMap[ep.Method+" "+ep.Path] {
							missing = append(missing, ep)
						}
					}
					if len(missing) == 0 {
						fmt.Println("No missing tests found. Skipping stub generation.")
						return nil
					}
					return em.EmitTestStubs(missing, "NEW-endpoint-stubs.test.ts")
				}
				return nil
			}},
			{"Frontend SDK", func() error { return em.EmitFrontendSDK(entities, services, endpoints, events, bizErrors, rbacDef) }},
			{"Python SDK", func() error {
				if !pythonSDKEnabled {
					return nil
				}
				return em.EmitPythonSDK(endpoints)
			}},
			{"Frontend Components", func() error { return em.EmitFrontendComponents(services, endpoints, entities) }},
			{"Frontend Admin", func() error { return em.EmitFrontendAdmin(entities, services) }},
			{"Frontend SDK Copy", func() error { return copyFrontendSDK(output.FrontendDir, output.FrontendAppDir) }},
			{"Frontend Admin Copy", func() error { return copyFrontendAdmin(output.FrontendAdminDir, output.FrontendAdminAppDir) }},
			{"Frontend Env Example", func() error { return writeEnvExample(output) }},
			{"Tracing", func() error { return em.EmitTracing() }},
			{"System Manifest", func() error { return em.EmitManifest(irSchema) }},
			{"Service Impls", func() error { return em.EmitServiceImpl(services, entities, authDef) }},
			{"Cached Services", func() error { return em.EmitCachedService(services) }},
			{"K8s Manifests", func() error { return em.EmitK8s(services, isMicroservice) }},
			{"Server Main", func() error {
				if isMicroservice {
					return em.EmitMicroservices(services, ctx.WebSocketServices, authDef)
				}
				return em.EmitMain(ctx)
			}},
		}

		for _, step := range steps {
			if err := step.fn(); err != nil {
				fmt.Printf("Error during %s: %v\n", step.name, err)
				return
			}
		}

		if err := runOptionalMCPGeneration(projectPath); err != nil {
			fmt.Printf("Error during MCP Generation: %v\n", err)
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

func runVet(args []string) {
	if len(args) > 0 && args[0] == "logic" {
		runLogicVet()
		return
	}

	fmt.Println("Checking architectural invariants (ANG Law Enforcement)...")
	projectPath := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
	}
	entities, services, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		fmt.Printf("Vet FAILED (Parser error): %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Running CUE Policy Checks...")
	p := parser.New()
	val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/policies"))
	if err == nil && ok {
		if err := val.Validate(); err != nil {
			fmt.Printf("Policy Violation (Validate): %v\n", err)
			os.Exit(1)
		}

		errVal := val.LookupPath(cue.ParsePath("validation_errors"))
		if errVal.Exists() {
			fmt.Println("Policy Violations Found:")
			iter, _ := errVal.Fields()
			for iter.Next() {
				fmt.Printf("  - %s: %s\n", iter.Selector(), iter.Value())
			}
			os.Exit(1)
		}
		fmt.Println("CUE Policies Passed.")
	} else if err != nil {
		fmt.Printf("Policy Violation (Load): %v\n", err)
		os.Exit(1)
	}

	failed := false
	for _, e := range entities {
		if strings.HasSuffix(e.Name, "Response") || strings.HasSuffix(e.Name, "Request") {
			continue
		}
		hasID := false
		for _, f := range e.Fields {
			if f.Name == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			fmt.Printf("Violation: Entity '%s' has no 'id' field.\n", e.Name)
			failed = true
		}
	}

	for _, s := range services {
		if len(s.Methods) == 0 {
			fmt.Printf("Violation: Service '%s' is empty.\n", s.Name)
			failed = true
		}
	}

	if failed {
		fmt.Println("\nInvariants check FAILED.")
		os.Exit(1)
	}
	fmt.Println("All architectural laws are satisfied.")
}

func runDraw(args []string) {
	fmt.Println("Drawing architecture...")
	entities, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("Draw FAILED (Parser error): %v\n", err)
		os.Exit(1)
	}

	output, err := parseOutputOptions(args)
	if err != nil {
		fmt.Printf("Draw FAILED: %v\n", err)
		os.Exit(1)
	}

	em := emitter.New(output.BackendDir, output.FrontendDir, "templates")
	em.FrontendAdminDir = output.FrontendAdminDir
	ctx := em.AnalyzeContext(services, entities, endpoints)

	if err := em.EmitMermaid(ctx); err != nil {
		fmt.Printf("Draw FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Draw SUCCESSFUL.")
}

type OutputOptions struct {
	BackendDir          string
	FrontendDir         string
	FrontendAppDir      string
	FrontendEnvPath     string
	FrontendAdminDir    string
	FrontendAdminAppDir string
	TestStubs           bool
}

func parseOutputOptions(args []string) (OutputOptions, error) {
	fs := flag.NewFlagSet("ang", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	backendFlag := fs.String("backend", ".", "backend output root")
	frontendFlag := fs.String("frontend", "sdk", "frontend SDK output dir")
	frontendAppFlag := fs.String("frontend-app", "", "copy generated SDK into this app dir")
	frontendEnvFlag := fs.String("frontend-env", "", "write .env.example at this path")
	frontendAdminFlag := fs.String("frontend-admin", "", "frontend admin output dir")
	frontendAdminAppFlag := fs.String("frontend-admin-app", "", "copy generated admin into this app dir")
	testStubsFlag := fs.Bool("test-stubs", false, "generate vitest stubs for all endpoints")
	if err := fs.Parse(args); err != nil {
		return OutputOptions{}, err
	}

	backendDir := *backendFlag
	frontendDir := *frontendFlag
	if fs.NArg() >= 1 {
		backendDir = fs.Arg(0)
	}
	if fs.NArg() >= 2 {
		frontendDir = fs.Arg(1)
	}

	backendDir = normalizeBackendDir(backendDir)
	return OutputOptions{
		BackendDir:          backendDir,
		FrontendDir:         frontendDir,
		FrontendAppDir:      strings.TrimSpace(*frontendAppFlag),
		FrontendEnvPath:     strings.TrimSpace(*frontendEnvFlag),
		FrontendAdminDir:    strings.TrimSpace(*frontendAdminFlag),
		FrontendAdminAppDir: strings.TrimSpace(*frontendAdminAppFlag),
		TestStubs:           *testStubsFlag,
	}, nil
}

func normalizeBackendDir(path string) string {
	if path == "" {
		return "."
	}
	clean := filepath.Clean(path)
	if filepath.Base(clean) == "internal" {
		return filepath.Dir(clean)
	}
	return clean
}

func runAtlasDiff(name string) error {
	cmd := exec.Command("atlas", "migrate", "diff", name, "--env", "local", "--to", "file://db/schema/schema.sql", "--dir", "file://db/migrations")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	latest, err := latestMigrationFile("db/migrations")
	if err != nil {
		return err
	}
	if latest == "" {
		return nil
	}
	data, err := os.ReadFile(latest)
	if err != nil {
		return err
	}
	upper := strings.ToUpper(string(data))
	if strings.Contains(upper, "DROP TABLE") || strings.Contains(upper, "DROP COLUMN") {
		if os.Getenv("ALLOW_DROP") != "1" {
			return fmt.Errorf("destructive statements detected in %s (set ALLOW_DROP=1 to accept)", latest)
		}
	}
	return nil
}

func runAtlasApply() error {
	dbURL := os.Getenv("DB_URL")
	if strings.TrimSpace(dbURL) == "" {
		return fmt.Errorf("DB_URL is required")
	}
	cmd := exec.Command("atlas", "migrate", "apply", "--dir", "file://db/migrations", "--url", dbURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func latestMigrationFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	type info struct {
		path string
		mod  int64
	}
	var files []info
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		stat, err := entry.Info()
		if err != nil {
			return "", err
		}
		files = append(files, info{
			path: filepath.Join(dir, entry.Name()),
			mod:  stat.ModTime().UnixNano(),
		})
	}
	if len(files) == 0 {
		return "", nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	return files[0].path, nil
}

type apiDoc struct {
	Endpoints map[string]struct{}
	Schemas   map[string]apiSchema
}

type apiSchema struct{ Fields map[string]apiField }
type apiField struct {
	Type   string
	Format string
}
type apiDiffReport struct {
	Breaking  []string
	Additions []string
	Semver    string
}

func parseOpenAPI(path string) (apiDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return apiDoc{}, err
	}
	lines := strings.Split(string(data), "\n")
	doc := apiDoc{Endpoints: make(map[string]struct{}), Schemas: make(map[string]apiSchema)}
	section := ""
	var currentPath, currentMethod, currentSchema string
	_ = currentMethod
	var inSchemas, inProperties bool
	var currentField, currentFieldType, currentFieldFormat string
	var currentFieldIsArray bool
	var currentFieldArrayType string

	finalizeField := func() {
		if currentSchema == "" || currentField == "" {
			return
		}
		schema := doc.Schemas[currentSchema]
		if schema.Fields == nil {
			schema.Fields = make(map[string]apiField)
		}
		fieldType := currentFieldType
		if currentFieldIsArray {
			if currentFieldArrayType != "" {
				fieldType = "array:" + currentFieldArrayType
			} else {
				fieldType = "array"
			}
		}
		schema.Fields[currentField] = apiField{Type: fieldType, Format: currentFieldFormat}
		doc.Schemas[currentSchema] = schema
		currentField = ""
		currentFieldType = ""
		currentFieldFormat = ""
		currentFieldIsArray = false
		currentFieldArrayType = ""
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "paths:") {
			section = "paths"
			continue
		}
		if strings.HasPrefix(line, "components:") {
			section = "components"
			inSchemas = false
			continue
		}
		if section == "components" && strings.HasPrefix(line, "  schemas:") {
			inSchemas = true
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if section == "paths" {
			if strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") && indent == 2 {
				currentPath = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") && indent == 4 {
				method := strings.TrimSuffix(trimmed, ":")
				switch method {
				case "get", "post", "put", "patch", "delete":
					currentMethod = method
					if currentPath != "" {
						doc.Endpoints[method+" "+currentPath] = struct{}{}
					}
				default:
					currentMethod = ""
				}
				continue
			}
			continue
		}
		if section == "components" && inSchemas {
			if indent == 4 && strings.HasSuffix(trimmed, ":") && trimmed != "schemas:" {
				finalizeField()
				currentSchema = strings.TrimSuffix(trimmed, ":")
				doc.Schemas[currentSchema] = apiSchema{Fields: make(map[string]apiField)}
				inProperties = false
				continue
			}
			if indent == 6 && trimmed == "properties:" {
				inProperties = true
				continue
			}
			if inProperties && indent == 8 && strings.HasSuffix(trimmed, ":") {
				finalizeField()
				currentField = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if inProperties && indent == 10 && strings.HasPrefix(trimmed, "type:") {
				currentFieldType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				currentFieldIsArray = currentFieldType == "array"
				continue
			}
			if inProperties && indent == 10 && strings.HasPrefix(trimmed, "format:") {
				currentFieldFormat = strings.TrimSpace(strings.TrimPrefix(trimmed, "format:"))
				continue
			}
			if inProperties && indent == 10 && trimmed == "items:" {
				continue
			}
			if inProperties && indent == 12 && strings.HasPrefix(trimmed, "type:") && currentFieldIsArray {
				currentFieldArrayType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				continue
			}
			if indent <= 6 && inProperties {
				finalizeField()
				inProperties = false
			}
		}
	}
	finalizeField()
	return doc, nil
}

func diffOpenAPI(base, current apiDoc) apiDiffReport {
	report := apiDiffReport{}
	for ep := range base.Endpoints {
		if _, ok := current.Endpoints[ep]; !ok {
			report.Breaking = append(report.Breaking, "Removed endpoint: "+ep)
		}
	}
	for ep := range current.Endpoints {
		if _, ok := base.Endpoints[ep]; !ok {
			report.Additions = append(report.Additions, "Added endpoint: "+ep)
		}
	}
	for name, schema := range base.Schemas {
		currSchema, ok := current.Schemas[name]
		if !ok {
			report.Breaking = append(report.Breaking, "Removed schema: "+name)
			continue
		}
		for field, def := range schema.Fields {
			currField, ok := currSchema.Fields[field]
			if !ok {
				report.Breaking = append(report.Breaking, "Removed field: "+name+"."+field)
				continue
			}
			baseSig := def.Type + "|" + def.Format
			currSig := currField.Type + "|" + currField.Format
			if baseSig != currSig {
				report.Breaking = append(report.Breaking, "Changed field type: "+name+"."+field+" ("+baseSig+" -> "+currSig+")")
			}
		}
		for field := range currSchema.Fields {
			if _, ok := schema.Fields[field]; !ok {
				report.Additions = append(report.Additions, "Added field: "+name+"."+field)
			}
		}
	}
	for name := range current.Schemas {
		if _, ok := base.Schemas[name]; !ok {
			report.Additions = append(report.Additions, "Added schema: "+name)
		}
	}
	if len(report.Breaking) > 0 {
		report.Semver = "major"
	} else if len(report.Additions) > 0 {
		report.Semver = "minor"
	} else {
		report.Semver = "patch"
	}
	sort.Strings(report.Breaking)
	sort.Strings(report.Additions)
	return report
}

func printAPIDiff(report apiDiffReport) {
	fmt.Println("API Diff Report\n--------------")
	if len(report.Breaking) == 0 {
		fmt.Println("Breaking changes: none")
	} else {
		fmt.Println("Breaking changes:")
		for _, item := range report.Breaking {
			fmt.Println("  - " + item)
		}
	}
	if len(report.Additions) == 0 {
		fmt.Println("Additions: none")
	} else {
		fmt.Println("Additions:")
		for _, item := range report.Additions {
			fmt.Println("  - " + item)
		}
	}
	fmt.Printf("Recommended semver bump: %s\n", report.Semver)
}

func copyFrontendSDK(srcDir, appDir string) error {
	if strings.TrimSpace(appDir) == "" {
		return nil
	}
	targetDir := filepath.Join(appDir, filepath.Base(srcDir))
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("cleanup sdk target: %w", err)
	}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

func copyFrontendAdmin(srcDir, appDir string) error {
	if strings.TrimSpace(appDir) == "" || strings.TrimSpace(srcDir) == "" {
		return nil
	}
	targetDir := appDir
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

func writeEnvExample(opts OutputOptions) error {
	if strings.TrimSpace(opts.FrontendAppDir) == "" {
		return nil
	}
	envPath := strings.TrimSpace(opts.FrontendEnvPath)
	if envPath == "" {
		envPath = filepath.Join(opts.FrontendAppDir, ".env.example")
	}
	if _, err := os.Stat(envPath); err == nil {
		return nil
	}
	content := []byte("VITE_API_URL=http://localhost:8080\n")
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(envPath, content, 0644)
}

func firstPathParam(path string) string {
	params := pathParams(path)
	if len(params) == 0 {
		return ""
	}
	return params[0]
}

func pathParams(path string) []string {
	var params []string
	start := strings.Index(path, "{")
	for start != -1 {
		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}
		param := path[start+1 : start+end]
		if param != "" {
			params = append(params, param)
		}
		next := start + end + 1
		start = strings.Index(path[next:], "{")
		if start != -1 {
			start += next
		}
	}
	return params
}

type lintReport struct {
	OK       bool                 `json:"ok"`
	Warnings []normalizer.Warning `json:"warnings,omitempty"`
	Errors   []lintError          `json:"errors,omitempty"`
}
type lintError struct {
	Message string `json:"message"`
}

func runVersion() {
	fmt.Printf("ANG version %s (Schema v%s)\n", compiler.Version, compiler.SchemaVersion)
}

func runRBAC(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang rbac <actions|inspect>")
		return
	}

	cmd := args[0]
	_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	validActions := make(map[string]bool)
	for _, s := range services {
		for _, m := range s.Methods {
			validActions[strings.ToLower(s.Name)+"."+strings.ToLower(m.Name)] = true
		}
	}

	switch cmd {
	case "actions":
		fmt.Println("Registered RBAC Actions (Service.Method):")
		fmt.Println("----------------------------------------")
		for action := range validActions {
			fmt.Println(action)
		}

	case "inspect":
		fmt.Println("RBAC Security Audit:")
		fmt.Println("--------------------")

		p := parser.New()
		n := normalizer.New()
		var rbac *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/policies"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		} else if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/rbac"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		}

		if rbac == nil {
			fmt.Println("⚠️  No RBAC policies found in CUE (cue/policies or cue/rbac).")
			return
		}

		protected := make(map[string]bool)
		zombies := []string{}

		for action := range rbac.Permissions {
			if validActions[action] {
				protected[action] = true
			} else {
				zombies = append(zombies, action)
			}
		}

		fmt.Println("\n🔴 UNPROTECTED METHODS (Missing from policies.cue):")
		for action := range validActions {
			if !protected[action] {
				fmt.Printf("   - %s\n", action)
			}
		}

		if len(zombies) > 0 {
			fmt.Println("\n💀 ZOMBIE POLICIES (References non-existent methods):")
			for _, z := range zombies {
				fmt.Printf("   - %s\n", z)
			}
		}

		fmt.Printf("\n🟢 PROTECTED: %d methods\n", len(protected))

	default:
		fmt.Printf("Unknown RBAC command: %s\n", cmd)
	}
}

func runDB(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang db <sync|status>")
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DB_URL")
	}

	switch args[0] {
	case "status":
		if dbURL == "" {
			fmt.Println("Error: DATABASE_URL is not set. Cannot check drift.")
			return
		}
		fmt.Println("Checking database schema drift...")
		// atlas schema diff --from $DB_URL --to file://db/schema/schema.sql --dev-url "docker://postgres/15/dev"
		cmd := exec.Command("atlas", "schema", "diff",
			"--from", dbURL,
			"--to", "file://db/schema/schema.sql",
			"--dev-url", "docker://postgres/15/dev",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("\n❌ DRIFT DETECTED:\n%s\n", string(out))
			fmt.Println("💡 Hint: Run 'ang db sync' to apply these changes.")
			os.Exit(1)
		}
		if len(strings.TrimSpace(string(out))) == 0 || strings.Contains(string(out), "Schemas are in sync") {
			fmt.Println("✅ Database schema is in sync with CUE.")
		} else {
			fmt.Printf("\nPending changes:\n%s\n", string(out))
		}

	case "sync":
		if dbURL == "" {
			fmt.Println("Error: DATABASE_URL is not set.")
			return
		}
		fmt.Println("Synchronizing database schema with CUE...")
		cmd := exec.Command("atlas", "schema", "apply",
			"--url", dbURL,
			"--to", "file://db/schema/schema.sql",
			"--dev-url", "docker://postgres/15/dev",
			"--auto-approve",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("\nDB Sync FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✅ Database schema is now in sync with CUE.")
	default:
		fmt.Printf("Unknown DB command: %s\n", args[0])
	}
}

func runEvents(args []string) {

	if len(args) == 0 || args[0] != "map" {

		fmt.Println("Usage: ang events map")

		return

	}

	_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")

	if err != nil {

		fmt.Printf("Error: %v\n", err)

		return

	}

	fmt.Println("Event Flow Map (Event-Driven Architecture Audit):")

	fmt.Println("--------------------------------------------------")

	type Publisher struct {
		Service string

		Method string
	}

	type Subscriber struct {
		Service string

		Handler string
	}

	publishers := make(map[string][]Publisher)

	subscribers := make(map[string][]Subscriber)

	for _, s := range services {

		// 1. Collect Publishers from methods

		for _, m := range s.Methods {

			for _, p := range m.Publishes {

				publishers[p] = append(publishers[p], Publisher{Service: s.Name, Method: m.Name})

			}

		}

		// 2. Collect Subscribers from service definition

		for evt, handler := range s.Subscribes {

			subscribers[evt] = append(subscribers[evt], Subscriber{Service: s.Name, Handler: handler})

		}

	}

	// 3. Find unique event names

	allEvents := make(map[string]bool)

	for e := range publishers {
		allEvents[e] = true
	}

	for e := range subscribers {
		allEvents[e] = true
	}

	foundAny := false

	for evt := range allEvents {

		foundAny = true

		fmt.Printf("\n📢 Event: %s\n", evt)

		pubs := publishers[evt]

		if len(pubs) > 0 {

			fmt.Print("   Produced by:\n")

			for _, p := range pubs {

				fmt.Printf("     - %s.%s\n", p.Service, p.Method)

			}

		} else {

			fmt.Print("   ⚠️  PRODUCER MISSING (External or manual)\n")

		}

		subs := subscribers[evt]

		if len(subs) > 0 {

			fmt.Print("   Consumed by:\n")

			for _, s := range subs {

				fmt.Printf("     - %s (Handler: %s)\n", s.Service, s.Handler)

			}

		} else {

			fmt.Print("   ⚠️  DEAD END (No subscribers found)\n")

		}

	}

	if !foundAny {

		fmt.Println("No event publishers or subscribers found in current architecture.")

	}

}

func runLogicVet() {

	fmt.Println("Auditing embedded Go logic in CUE files...")

	_, _, _, _, _, _, _, _, _ = compiler.RunPipeline(".")

	found := false

	for _, d := range compiler.LatestDiagnostics {

		if d.Code == "GO_SYNTAX_ERROR" {

			fmt.Printf("\n❌ Logic Error at %s:%d:%d\n", d.File, d.Line, d.Column)

			fmt.Printf("   %s\n", d.Message)

			if d.Hint != "" {

				fmt.Printf("   💡 Hint: %s\n", d.Hint)

			}

			found = true

		}

	}

	if !found {

		fmt.Println("✅ All embedded logic blocks are syntactically valid.")

	} else {

		os.Exit(1)

	}

}
