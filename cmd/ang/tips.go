package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runTips(args []string) {
	fs := flag.NewFlagSet("tips", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("project-path", ".", "project root path")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		printCommandFailure("Tips", err.Error(), "run `ang tips --help`")
		os.Exit(1)
	}

	root := filepath.Clean(strings.TrimSpace(*projectPath))
	if root == "" {
		root = "."
	}

	fmt.Println("ANG tips")
	fmt.Println("  1. First run: make doctor && ang up")
	fmt.Println("  2. Check health: ang smoke")
	fmt.Println("  3. Validate intent: ang validate && ang lint")
	fmt.Println("  4. Regenerate code: ang build")
	fmt.Println("  5. Reset local infra: make down && make up")
	fmt.Println("  6. Fun mode: ANG_FUN=1 ang up   (or pass --fun)")

	if _, err := os.Stat(filepath.Join(root, "scripts", "preflight.sh")); err == nil {
		fmt.Println("  7. Local preflight script: bash scripts/preflight.sh")
	}
}
