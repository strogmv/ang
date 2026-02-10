package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
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
		runInit(os.Args[2:])
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
	case "test":
		runTest(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "lsp":
		runLSP(os.Args[2:])
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
	fmt.Println("  ang init [dir] --template saas|ecommerce|marketplace [--lang go] [--db postgres]")
	fmt.Println("  ang validate  Validate CUE models and architecture")
	fmt.Println("  ang lint      Perform deep semantic linting of flows and logic")
	fmt.Println("  ang build     Compile CUE intent into code and infra configs (--dry-run, --log-format=json)")
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
	fmt.Println("  ang lsp --stdio  Run ANG language server (MVP diagnostics)")
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
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Printf("Init FAILED: %v\n", err)
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

	if strings.TrimSpace(*templateName) == "" {
		if err := initLegacyScaffold(targetDir); err != nil {
			fmt.Printf("Init FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Project structure initialized successfully.")
		return
	}

	modulePath := strings.TrimSpace(*module)
	if modulePath == "" {
		modulePath = "github.com/example/" + sanitizeProjectName(projectName)
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
		fmt.Printf("Init FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Template %q initialized in %s\n", opts.TemplateName, targetDir)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", targetDir)
	fmt.Println("  docker compose up -d")
	fmt.Println("  ang build")
}

func initLegacyScaffold(root string) error {
	dirs := []string{
		filepath.Join(root, "cue", "domain"),
		filepath.Join(root, "cue", "api"),
		filepath.Join(root, "cue", "policies"),
		filepath.Join(root, "cue", "invariants"),
		filepath.Join(root, "cue", "architecture"),
		filepath.Join(root, "cue", "repo"),
		filepath.Join(root, "cue", "schema"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	moduleContent := `module: "github.com/strogmv/ang"
language: {
	version: "v0.9.0"
}
`
	modFile := filepath.Join(root, "cue.mod", "module.cue")
	if err := os.MkdirAll(filepath.Dir(modFile), 0755); err != nil {
		return fmt.Errorf("create cue.mod: %w", err)
	}
	if _, err := os.Stat(modFile); os.IsNotExist(err) {
		if err := os.WriteFile(modFile, []byte(moduleContent), 0644); err != nil {
			return fmt.Errorf("write module file: %w", err)
		}
	}
	return nil
}
