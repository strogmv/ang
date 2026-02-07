package mcp

import (
	"bufio"
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
)

var (
	projectLocks sync.Map // map[string]*sync.Mutex
)

func getLock(path string) *sync.Mutex {
	lock, _ := projectLocks.LoadOrStore(path, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func validateCuePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	cwd, _ := os.Getwd()
	cueDir := filepath.Join(cwd, "cue")
	if !strings.HasPrefix(abs, cueDir) {
		return fmt.Errorf("access denied: only files in /cue/ directory are modifiable")
	}
	return nil
}

func validateReadPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil { return err }
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

	// --- NAVIGATION & SEARCH (Stage 27) ---

	s.AddTool(mcp.NewTool("ang_search",
		mcp.WithDescription("Hybrid semantic/symbol search across code and CUE intent"),
		mcp.WithString("query", mcp.Description("Natural language or exact query"), mcp.Required()),
		mcp.WithString("scope", mcp.Description("cue, code, or all"), mcp.DefaultString("all")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		scope := mcp.ParseString(request, "scope", "all")

		results := searchSymbols(query, scope)
		
		res := map[string]interface{}{
			"query":   query,
			"results": results,
			"limits":  map[string]int{"max_results": 8, "max_snippet_lines": 6},
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	s.AddTool(mcp.NewTool("repo_read_symbol",
		mcp.WithDescription("Read focused code snippet for a specific symbol"),
		mcp.WithString("symbol_id", mcp.Description("Symbol ID from search results"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := mcp.ParseString(request, "symbol_id", "")
		// Format expected: "file:line:symbol"
		parts := strings.Split(id, ":")
		if len(parts) < 2 {
			return mcp.NewToolResultText("Invalid symbol_id format"), nil
		}
		path := parts[0]
		lineStr := parts[1]
		
		content, _ := readHunk(path, lineStr, 50)
		return mcp.NewToolResultText(content), nil
	})

	s.AddTool(mcp.NewTool("find_refs",
		mcp.WithDescription("Find all usages of a specific symbol"),
		mcp.WithString("symbol", mcp.Description("Symbol name"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := mcp.ParseString(request, "symbol", "")
		cmd := exec.Command("grep", "-r", "-n", symbol, "internal/", "cue/")
		out, _ := cmd.CombinedOutput()
		return mcp.NewToolResultText(string(out)), nil
	})

	// --- CORE TOOLS ---

	s.AddTool(mcp.NewTool("ang_capabilities",
		mcp.WithDescription("Get ANG compiler capabilities"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res := map[string]interface{}{
			"ang_version":    compiler.Version,
			"capabilities":   []string{"hybrid_search", "symbol_navigation", "planning", "structured_diagnostics"},
			"policy":         "Agent writes only CUE. ANG writes code. Agent reads code and runs tests.",
		}
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	s.AddTool(mcp.NewTool("ang_plan",
		mcp.WithDescription("Propose a development plan"),
		mcp.WithString("goal", mcp.Description("Goal description"), mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		goal := mcp.ParseString(request, "goal", "")
		plan := []string{"1. Search relevant symbols via ang_search", "2. Analyze CUE via cue_read", "3. Propose changes"}
		res := map[string]interface{}{ "goal": goal, "steps": plan }
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.NewToolResultText(string(jsonRes)), nil
	})

	// --- CUE TOOLS ---

	s.AddTool(mcp.NewTool("cue_read", mcp.WithDescription("Read CUE"), mcp.WithString("path", mcp.Required())), 
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := mcp.ParseString(request, "path", "")
			data, _ := os.ReadFile(path)
			return mcp.NewToolResultText(string(data)), nil
		})

	s.AddTool(mcp.NewTool("run_preset", mcp.WithDescription("Run preset"), mcp.WithString("name", mcp.Required())),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := mcp.ParseString(request, "name", "")
			cmd := exec.Command("./ang_bin", name)
			if name == "unit" { cmd = exec.Command("go", "test", "./...") }
			out, _ := cmd.CombinedOutput()
			return mcp.NewToolResultText(string(out)), nil
		})

	registerResources(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

// --- HELPERS ---

type SearchResult struct {
	Rank     int      `json:"rank"`
	Kind     string   `json:"kind"`
	SymbolID string   `json:"symbol_id"`
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Snippet  []string `json:"snippet"`
}

func searchSymbols(query string, scope string) []SearchResult {
	var results []SearchResult
	// Search Go symbols
	if scope == "code" || scope == "all" {
		cmd := exec.Command("grep", "-r", "-n", "-E", "func |type |interface ", "internal/")
		out, _ := cmd.Output()
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		count := 0
		for scanner.Scan() && count < 5 {
			line := scanner.Text()
			parts := strings.Split(line, ":")
			if len(parts) < 3 { continue }
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				count++
				results = append(results, SearchResult{
					Rank: count, Kind: "code", SymbolID: parts[0] + ":" + parts[1] + ":" + parts[2],
					Path: parts[0], Title: strings.TrimSpace(parts[2]),
					Snippet: []string{strings.TrimSpace(parts[2])},
				})
			}
		}
	}
	// Search CUE symbols
	if scope == "cue" || scope == "all" {
		cmd := exec.Command("grep", "-r", "-n", "#", "cue/")
		out, _ := cmd.Output()
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		count := len(results)
		for scanner.Scan() && count < 10 {
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				count++
				parts := strings.Split(line, ":")
				results = append(results, SearchResult{
					Rank: count, Kind: "cue", SymbolID: line, Path: parts[0], Title: parts[2],
					Snippet: []string{parts[2]},
				})
			}
		}
	}
	return results
}

func readHunk(path string, lineStr string, window int) (string, error) {
	var line int
	fmt.Sscanf(lineStr, "%d", &line)
	f, _ := os.Open(path)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []string
	curr := 0
	for scanner.Scan() {
		curr++
		if curr >= line-5 && curr <= line+window {
			lines = append(lines, fmt.Sprintf("%d: %s", curr, scanner.Text()))
		}
		if curr > line+window { break }
	}
	return strings.Join(lines, "\n"), nil
}

func registerResources(s *server.MCPServer) {
	s.AddResource(mcp.NewResource("resource://ang/ai_contract", "AI Contract", mcp.WithMIMEType("application/json")),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			res := map[string]interface{}{"policy": "Agent writes only CUE", "workflow": "search -> plan -> patch -> build"}
			data, _ := json.MarshalIndent(res, "", "  ")
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "resource://ang/ai_contract", MIMEType: "application/json", Text: string(data)}}, nil
		})
}
