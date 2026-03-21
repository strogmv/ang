package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const aiGenModel = "claude-opus-4-6"

// runAIGen implements `ang gen --facts <file> [--service name] [--out dir] [--dry-run]`.
// It reads an ang/facts/v1 JSON, calls the Claude API, and writes CUE operations.
func runAIGen(args []string) {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	factsPath := fs.String("facts", "", "path to ang/facts/v1 JSON (required)")
	service := fs.String("service", "", "generate only operations for this service hint (empty = all)")
	outDir := fs.String("out", "cue/api", "output directory for generated CUE files")
	dryRun := fs.Bool("dry-run", false, "print generated CUE without writing files")
	model := fs.String("model", aiGenModel, "Claude model ID")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(*factsPath) == "" {
		fmt.Fprintln(os.Stderr, "gen: --facts <path> is required")
		fmt.Fprintln(os.Stderr, "  generate facts first:  ang extract ./src --from auto --out facts.json")
		os.Exit(1)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "gen: ANTHROPIC_API_KEY env var not set")
		os.Exit(1)
	}

	facts, err := loadFactsEnvelope(*factsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: load facts: %v\n", err)
		os.Exit(1)
	}
	if facts.Schema != "ang/facts/v1" {
		fmt.Fprintf(os.Stderr, "gen: expected schema ang/facts/v1, got %q\n", facts.Schema)
		os.Exit(1)
	}

	prompt := buildAIGenPrompt(*facts, *service)

	fmt.Printf("Calling %s to generate CUE from %d entities, %d operations...\n",
		*model, len(facts.Entities), len(facts.Operations))

	cueText, err := callClaudeAPI(apiKey, *model, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: API call failed: %v\n", err)
		os.Exit(1)
	}

	files := splitCUEOutput(cueText)
	if len(files) == 0 {
		fmt.Println("gen: no CUE blocks found in response, printing raw output:")
		fmt.Println(cueText)
		return
	}

	if *dryRun {
		for name, content := range files {
			fmt.Printf("// --- %s ---\n%s\n", name, content)
		}
		return
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen: mkdir %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	for name, content := range files {
		path := filepath.Join(*outDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gen: write %s: %v\n", path, err)
			continue
		}
		fmt.Printf("  wrote %s\n", path)
	}
	fmt.Printf("Done. Run `ang ops vet` to validate, `ang build` to compile.\n")
}

func buildAIGenPrompt(facts FactsEnvelope, serviceFilter string) string {
	var b strings.Builder

	b.WriteString(`You are generating ANG CUE intent from extracted facts.

## ANG CUE Operation Format

Each operation file is a CUE package with one or more operations:

` + "```cue" + `
package api

OperationName: {
    service: "servicename"        // lowercase, matches architecture service
    description: "what it does"
    input: {
        fieldName: string         // required field
        optionalField?: int       // optional field (note the ?)
    }
    output: {
        id: string
        createdAt: string
    }
    flow: [
        {action: "auth.RequireUser"},
        {action: "repo.Load", entity: "EntityName", by: {id: "input.id"}},
        {action: "repo.Save", entity: "entityVar"},
        {action: "mapping.Return", value: "entityVar"},
    ]
}
` + "```" + `

## CUE Type Mapping
- string → string
- int, int64 → int
- bool → bool
- float, float64 → float
- time.Time, Date, LocalDate → string
- UUID, uuid.UUID → string
- []T → [...T]
- optional field → fieldName?: type
- unknown/complex → string

## Common Flow Actions
- auth.RequireUser — enforce JWT auth
- auth.RequireRole(role: "admin") — RBAC check
- repo.Load(entity: "User", by: {id: "input.userId"}) — load from DB
- repo.Save(entity: "user") — persist entity
- repo.Delete(entity: "user") — delete entity
- mapping.Assign(target: "output.id", value: "entity.id") — map fields
- mapping.Return(value: "entity") — return value
- event.Publish(event: "UserCreated", payload: "user") — publish domain event
- logic.Validate(input: "input") — validate input

## Output Rules
1. Group operations by service into separate files named: <service>_operations.cue
2. Use lowercase service names matching the service_hint from facts
3. Output ONLY valid CUE — no markdown, no explanations, no comments except inside CUE
4. Wrap each file in: ` + "`" + `// FILE: <filename>.cue` + "`" + ` header line, then the CUE content
5. If http_method and http_path are known, add a comment: // @route POST /path

`)

	// Entities
	b.WriteString("## Entities (use these exact field names and types)\n\n")
	for _, e := range facts.Entities {
		b.WriteString(fmt.Sprintf("### %s\n", e.Name))
		for _, f := range e.Fields {
			t := f.CueTypeHint
			if t == "" {
				t = "string"
			}
			opt := ""
			if !f.Required {
				opt = "?"
			}
			b.WriteString(fmt.Sprintf("  %s%s: %s\n", f.Name, opt, t))
		}
		b.WriteString("\n")
	}

	// Operations to generate
	b.WriteString("## Operations to Generate\n\n")
	for _, op := range facts.Operations {
		if serviceFilter != "" && !strings.EqualFold(op.ServiceHint, serviceFilter) {
			continue
		}
		b.WriteString(fmt.Sprintf("### %s (service: %s)\n", op.Name, op.ServiceHint))
		if op.HTTPMethod != "" {
			b.WriteString(fmt.Sprintf("  route: %s %s\n", op.HTTPMethod, op.HTTPPath))
		}
		if op.Transactional {
			b.WriteString("  transactional: true\n")
		}
		if len(op.InputFields) > 0 {
			b.WriteString("  input:\n")
			for _, f := range op.InputFields {
				t := f.CueTypeHint
				if t == "" {
					t = "string"
				}
				opt := ""
				if !f.Required {
					opt = "?"
				}
				b.WriteString(fmt.Sprintf("    %s%s: %s\n", f.Name, opt, t))
			}
		}
		if len(op.OutputFields) > 0 {
			b.WriteString("  output:\n")
			for _, f := range op.OutputFields {
				t := f.CueTypeHint
				if t == "" {
					t = "string"
				}
				b.WriteString(fmt.Sprintf("    %s: %s\n", f.Name, t))
			}
		}
		b.WriteString("\n")
	}

	// Repositories available
	if len(facts.Repositories) > 0 {
		b.WriteString("## Available Repositories (use in flow repo.* actions)\n\n")
		for _, r := range facts.Repositories {
			b.WriteString(fmt.Sprintf("- %s: ", r.Entity))
			methods := make([]string, 0, len(r.Methods))
			for _, m := range r.Methods {
				methods = append(methods, m.Name)
			}
			b.WriteString(strings.Join(methods, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(`Generate CUE files now. Remember: output ONLY file blocks with // FILE: header, no other text.`)
	return b.String()
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func callClaudeAPI(apiKey, model, prompt string) (string, error) {
	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: 8192,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result claudeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode response: %w (status %d)", err, resp.StatusCode)
	}
	if result.Error != nil {
		return "", fmt.Errorf("%s: %s", result.Error.Type, result.Error.Message)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var text strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return text.String(), nil
}

var fileHeaderRe = regexp.MustCompile(`(?m)^//\s*FILE:\s*(\S+\.cue)`)

// splitCUEOutput splits AI response into a map of filename → CUE content.
// Expects blocks starting with "// FILE: name.cue".
func splitCUEOutput(text string) map[string]string {
	files := make(map[string]string)
	matches := fileHeaderRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		// Try to extract raw CUE if no file headers
		if strings.Contains(text, "package api") {
			files["generated_operations.cue"] = extractCUEBlock(text)
		}
		return files
	}

	for i, m := range matches {
		headerLine := text[m[0]:m[1]]
		nameMatch := fileHeaderRe.FindStringSubmatch(headerLine)
		if len(nameMatch) < 2 {
			continue
		}
		name := nameMatch[1]

		start := m[1] // after the header line
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		content := strings.TrimSpace(text[start:end])
		// Strip markdown code fences if present
		content = stripCodeFences(content)
		if content != "" {
			files[name] = content + "\n"
		}
	}
	return files
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = s[:strings.LastIndex(s, "```")]
	}
	return strings.TrimSpace(s)
}

func extractCUEBlock(text string) string {
	// Extract content between ``` fences if present
	if idx := strings.Index(text, "```cue"); idx >= 0 {
		text = text[idx+6:]
		if end := strings.Index(text, "```"); end >= 0 {
			return strings.TrimSpace(text[:end])
		}
	}
	// Otherwise return as-is up to reasonable length
	lines := strings.Split(text, "\n")
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" || len(out) > 0 {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
