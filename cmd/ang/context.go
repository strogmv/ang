package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler"
)

// runContext generates a compact domain snapshot for AI context loading.
// Usage: ang context [service] [--format=md|yaml] [--all]
func runContext(args []string) {
	format := "md"
	filterService := ""
	showAll := false
	projectPath := "."

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case strings.HasPrefix(a, "--project="):
			projectPath = strings.TrimPrefix(a, "--project=")
		case a == "--all":
			showAll = true
		case !strings.HasPrefix(a, "--"):
			filterService = strings.ToLower(a)
		}
	}

	entities, services, endpoints, repos, events, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ang context: failed to parse project: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "md", "markdown":
		fmt.Print(compiler.RenderContextMarkdown(filterService, showAll, entities, services, endpoints, repos, events))
	default:
		fmt.Fprintf(os.Stderr, "ang context: unknown format %q (supported: md)\n", format)
		os.Exit(1)
	}
}
