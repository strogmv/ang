package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	explainpkg "github.com/strogmv/ang/internal/explain"
)

type explainInput = explainpkg.Input
type explainItem = explainpkg.Item
type explainEnvelope = explainpkg.Envelope

func explainAnyInput(input string) ([]explainItem, error) {
	return explainpkg.ExplainAnyInput(input)
}

func decodeDiagnostics(raw []byte) ([]explainInput, error) {
	return explainpkg.DecodeDiagnostics(raw)
}

func explainFromInput(in explainInput) explainItem {
	return explainpkg.ExplainFromInput(in)
}

func runExplain(args []string) {
	jsonOut := false
	var positional []string
	for _, a := range args {
		if strings.TrimSpace(a) == "--json" {
			jsonOut = true
			continue
		}
		positional = append(positional, a)
	}

	if len(positional) == 0 {
		fmt.Println("Usage: ang explain <CODE|error-json|path-to-json> [--json]")
		os.Exit(1)
	}

	rawInput := strings.TrimSpace(positional[0])
	items, err := explainAnyInput(rawInput)
	if err != nil {
		fmt.Printf("Explain FAILED: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Println("Explain FAILED: no diagnostics found in input")
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(explainEnvelope{
			Schema: "ang/explain/v2",
			Items:  items,
		})
		return
	}

	for i, it := range items {
		fmt.Printf("[%d] %s — %s\n", i+1, it.Code, it.Title)
		if it.Path != "" {
			fmt.Printf("Path: %s\n", it.Path)
		}
		fmt.Printf("What: %s\n", it.Description)
		if len(it.Expected) > 0 {
			fmt.Printf("Expected: %s\n", strings.Join(it.Expected, "; "))
		}
		if len(it.Found) > 0 {
			fmt.Printf("Found: %s\n", strings.Join(it.Found, "; "))
		}
		if it.Fix != "" {
			fmt.Printf("Fix: %s\n", it.Fix)
		}
		if it.Hint != "" {
			fmt.Printf("Hint: %s\n", it.Hint)
		}
		if it.ActionRef != "" {
			fmt.Printf("Action: %s  (see: ang actions --json | jq '.[] | select(.name==\"%s\")')\n", it.ActionRef, it.ActionRef)
		}
		if it.SchemaRef != "" {
			fmt.Printf("Rule: %s  (see: ang ops schema --json | jq '.rules[] | select(.code==\"%s\")')\n", it.SchemaRef, it.SchemaRef)
		}
		if it.DocAnchor != "" {
			fmt.Printf("Docs: %s\n", it.DocAnchor)
		}
		if i != len(items)-1 {
			fmt.Println()
		}
	}
}
