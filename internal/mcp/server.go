package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

var (
	projectLocks sync.Map // map[string]*sync.Mutex
)

func getLock(path string) *sync.Mutex {
	lock, _ := projectLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// validateCuePath проверяет, что путь ведет в папку cue/ и файл имеет расширение .cue
func validateCuePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	cueDir := filepath.Join(cwd, "cue")
	if !strings.HasPrefix(abs, cueDir) {
		return fmt.Errorf("access denied: only files in /cue/ directory are modifiable")
	}
	if filepath.Ext(path) != ".cue" {
		return fmt.Errorf("access denied: only .cue files can be modified")
	}
	return nil
}

// validateReadPath проверяет, что путь находится внутри воркспейса
func validateReadPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(abs, cwd) {
		return fmt.Errorf("access denied: path %s is outside of workspace", path)
	}
	return nil
}

func Run() {
	s := server.NewMCPServer(
		"ANG MCP Server",
		compiler.Version,
		server.WithLogging(),
	)

	// --- CUE TOOLS (RW) ---

	s.AddTool(mcp.NewTool("cue_read",
		mcp.WithDescription("Read a CUE file from /cue directory"),
		mcp.WithString("path", mcp.Description("Path to .cue file"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		if err := validateReadPath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error reading file: %v", err)), nil
		}
		return mcp.NewToolResultText(string(content)), nil
	})

	s.AddTool(mcp.NewTool("cue_apply_patch",
		mcp.WithDescription("Update a CUE file. ONLY allowed for files in /cue directory."),
		mcp.WithString("path", mcp.Description("Path to .cue file"), mcp.Required()),
		mcp.WithString("content", mcp.Description("Full new content of the file"), mcp.Required()),
		mcp.WithBoolean("dry_run", mcp.Description("If true, only validate changes")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		content := mcp.ParseString(request, "content", "")
		dryRun := mcp.ParseBoolean(request, "dry_run", true)

		if err := validateCuePath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		if dryRun {
			return mcp.NewToolResultText("Validation successful (Dry-run)"), nil
		}

		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error writing file: %v", err)), nil
		}
		return mcp.NewToolResultText("File updated successfully"), nil
	})

	s.AddTool(mcp.NewTool("cue_fmt",
		mcp.WithDescription("Format CUE files in a directory"),
		mcp.WithString("path", mcp.Description("Path to directory or file"), mcp.DefaultString("cue/")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "cue/")
		cmd := exec.Command("cue", "fmt", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("cue fmt failed: %s\n%v", string(out), err)), nil
		}
		return mcp.NewToolResultText("Formatting complete"), nil
	})

	// --- GENERATION (ANG) ---

	s.AddTool(mcp.NewTool("ang_generate",
		mcp.WithDescription("Generate Go code and artifacts from CUE intent. This is the ONLY way to update the codebase."),
		mcp.WithString("project_path", mcp.Description("Path to project root"), mcp.DefaultString(".")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "project_path", ".")
		if err := validateReadPath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}

		lock := getLock(path)
		lock.Lock()
		defer lock.Unlock()

		executable, err := os.Executable()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Failed to find executable: %v", err)), nil
		}

		// Запускаем билд
		cmd := exec.Command(executable, "build")
		cmd.Dir = path
		output, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Generation failed:\n%s\nError: %v", string(output), err)), nil
		}

		// Получаем манифест изменений (из git status, так как мы знаем, что пишет только ang)
		diffCmd := exec.Command("git", "status", "--porcelain")
		diffCmd.Dir = path
		diffOut, _ := diffCmd.Output()

		res := map[string]interface{}{
			"status":  "success",
			"summary": string(output),
			"changes": strings.Split(string(diffOut), "\n"),
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- CODE READING (RO) ---

	s.AddTool(mcp.NewTool("repo_read_file",
		mcp.WithDescription("Read a generated file (Go, SQL, YAML, etc). READ-ONLY."),
		mcp.WithString("path", mcp.Description("Path to file"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := mcp.ParseString(request, "path", "")
		if err := validateReadPath(path); err != nil {
			return mcp.NewToolResultText(err.Error()), nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Error reading file: %v", err)), nil
		}
		return mcp.NewToolResultText(string(content)), nil
	})

	s.AddTool(mcp.NewTool("repo_diff",
		mcp.WithDescription("Get diff of generated code changes. Very token-efficient."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cmd := exec.Command("git", "diff")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("git diff failed: %v", err)), nil
		}
		return mcp.NewToolResultText(string(out)), nil
	})

	// --- RUNTIME & TESTS ---

	s.AddTool(mcp.NewTool("run_tests",
		mcp.WithDescription("Run unit and integration tests"),
		mcp.WithString("target", mcp.Description("Package or directory"), mcp.DefaultString("./...")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		target := mcp.ParseString(request, "target", "./...")
		cmd := exec.Command("go", "test", "-v", target)
		out, err := cmd.CombinedOutput()
		
		res := map[string]interface{}{
			"success": err == nil,
			"output":  string(out),
		}
		if err != nil {
			res["error"] = err.Error()
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- MIGRATIONS (Stage 25) ---

	s.AddTool(mcp.NewTool("ang_migrate_diff",
		mcp.WithDescription("Calculate difference between CUE intent and current migrations"),
		mcp.WithString("name", mcp.Description("Name of the new migration"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		
		// 1. Run ang build to update schema.sql
		buildCmd := exec.Command("os.Executable", "build") // We will use os.Executable() logic
		if exe, err := os.Executable(); err == nil {
			buildCmd = exec.Command(exe, "build")
		}
		_ = buildCmd.Run()

		// 2. Run atlas migrate diff
		// Note: Requires atlas installed on host
		cmd := exec.Command("atlas", "migrate", "diff", name,
			"--env", "local",
			"--to", "file://db/schema/schema.sql",
			"--dir", "file://db/migrations")
		
		out, err := cmd.CombinedOutput()
		
		res := map[string]interface{}{
			"success": err == nil,
			"output":  string(out),
		}

		if err == nil {
			// Check for destructive changes in the latest file
			files, _ := filepath.Glob("db/migrations/*.sql")
			if len(files) > 0 {
				latest := files[len(files)-1]
				content, _ := os.ReadFile(latest)
				if strings.Contains(string(content), "DROP ") {
					res["warning"] = "Destructive change detected: " + latest
					res["is_destructive"] = true
				}
				res["file"] = latest
				res["plan"] = string(content)
			}
		} else {
			res["error"] = err.Error()
		}

		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	s.AddTool(mcp.NewTool("ang_migrate_apply",
		mcp.WithDescription("Apply pending migrations to the database"),
		mcp.WithBoolean("dry_run", mcp.Description("If true, only show what will be executed")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dryRun := mcp.ParseBoolean(request, "dry_run", true)
		
		args := []string{"migrate", "apply", "--env", "local", "--dir", "file://db/migrations"}
		if dryRun {
			args = append(args, "--dry-run")
		}

		cmd := exec.Command("atlas", args...)
		out, err := cmd.CombinedOutput()

		res := map[string]interface{}{
			"success": err == nil,
			"output":  string(out),
			"dry_run": dryRun,
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- META & RESOURCES ---

	s.AddTool(mcp.NewTool("ang_capabilities",
		mcp.WithDescription("Get ANG compiler capabilities"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res := map[string]interface{}{
			"ang_version":    compiler.Version,
			"schema_version": compiler.SchemaVersion,
			"policy":         "Agent writes only CUE. ANG writes code. Agent reads code and runs tests.",
			"capabilities": []string{
				"ai_hints",
				"ai_patterns",
				"structured_diagnostics",
				"safe_apply",
				"intent_only_rw",
			},
			"zones": map[string]string{
				"/cue":  "Read/Write",
				"other": "Read-Only",
			},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	registerIRResources(s)
	
	// Diagnostics resource (RO)
	s.AddResource(mcp.NewResource("resource://ang/diagnostics/latest", "Latest Diagnostics",
		mcp.WithResourceDescription("Latest validation errors and warnings grouped by file"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		grouped := make(map[string][]normalizer.Warning)
		for _, w := range compiler.LatestDiagnostics {
			file := w.File
			if file == "" { file = "global" }
			grouped[file] = append(grouped[file], w)
		}
		jsonRes, _ := json.MarshalIndent(grouped, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/diagnostics/latest",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	// Resource: Contract Coverage
	s.AddResource(mcp.NewResource("resource://ang/coverage/contract", "Contract Coverage Report",
		mcp.WithResourceDescription("Analysis of CUE invariants coverage by generated tests"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		entities, services, endpoints, repos, events, bizErrors, schedules, err := compiler.RunPipeline(".")
		if err != nil {
			return nil, err
		}

		schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})

		totalMethods := 0
		coveredMethods := 0
		var uncovered []string

		for _, svc := range schema.Services {
			for _, m := range svc.Methods {
				totalMethods++
				covered := false
				if m.Metadata != nil {
					if _, ok := m.Metadata["testHints"]; ok {
						covered = true
					}
				}

				if covered {
					coveredMethods++
				} else {
					uncovered = append(uncovered, fmt.Sprintf("%s.%s", svc.Name, m.Name))
				}
			}
		}

		percentage := 0.0
		if totalMethods > 0 {
			percentage = float64(coveredMethods) / float64(totalMethods) * 100
		}

		res := map[string]interface{}{
			"total_entities": len(schema.Entities),
			"total_methods":  totalMethods,
			"covered":        coveredMethods,
			"percentage":     fmt.Sprintf("%.1f%%", percentage),
			"uncovered":      uncovered,
		}

		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/coverage/contract",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	// Resource: AI Hints
	s.AddResource(mcp.NewResource("resource://ang/ai_hints", "AI Agent Hints",
		mcp.WithResourceDescription("Context-optimized patterns and rules for AI agents"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		hints := map[string]interface{}{
			"core_rules": []string{
				"All entities must define a unique ID field.",
				"Use #Operation schema for all API endpoints.",
				"Service-to-service calls must use 'uses' list in architecture.",
			},
			"patterns": map[string]string{
				"repository_ops": "Use #Find, #Get, #List, #Save, #Delete standard actions.",
				"fsm":            "Define states and transitions explicitly in entity schema.",
				"logic_calls":    "Arguments for logic.Call must always be a list: [\"arg1\", \"arg2\"].",
			},
			"anti_patterns": []string{
				"Do not use raw strings for status fields; use @fsm or enums.",
				"Do not reference entities owned by other services directly.",
			},
		}
		jsonRes, _ := json.MarshalIndent(hints, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/ai_hints",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	// Resource: Transformers Catalog
	s.AddResource(mcp.NewResource("resource://ang/transformers", "Transformers Catalog",
		mcp.WithResourceDescription("List of available deterministic code transformers"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		catalog := []map[string]interface{}{
			{
				"name":        "tracing",
				"description": "Auto-injects OpenTelemetry spans into services and repositories.",
				"activation":  "Enabled by default if TracingProvider is configured.",
			},
			{
				"name":        "caching",
				"description": "Redis/Memory caching decorator for service methods.",
				"activation":  "Add 'cache: {ttl: \"5m\"}' to endpoint in HTTP block or @cache() attribute.",
			},
			{
				"name":        "validation",
				"description": "Injects go-validator tags based on @validate() attributes.",
				"activation":  "Add @validate(\"rule\") to entity fields.",
			},
			{
				"name":        "security",
				"description": "Field-level encryption and logging redaction.",
				"activation":  "Use @encrypt() or @redact() on sensitive fields.",
			},
		}
		jsonRes, _ := json.MarshalIndent(catalog, "", "  ")
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "resource://ang/transformers",
				MIMEType: "application/json",
				Text:     string(jsonRes),
			},
		}, nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func registerIRResources(s *server.MCPServer) {
	irBase := "resource://ang/ir"
	s.AddResource(mcp.NewResource(irBase, "Full IR", mcp.WithMIMEType("application/json")), 
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return readIRPart(request.Params.URI, func(schema *ir.Schema) interface{} { return schema })
		})
}

func readIRPart(uri string, selector func(*ir.Schema) interface{}) ([]mcp.ResourceContents, error) {
	entities, services, endpoints, repos, events, bizErrors, schedules, err := compiler.RunPipeline(".")
	if err != nil { return nil, err }
	schema := ir.ConvertFromNormalizer(entities, services, events, bizErrors, endpoints, repos, normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{})
	part := selector(schema)
	jsonRes, _ := json.MarshalIndent(part, "", "  ")
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI: uri, MIMEType: "application/json", Text: string(jsonRes),
		},
	}, nil
}
