package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runFirstRun(args []string) {
	fs := flag.NewFlagSet("first-run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("project-path", ".", "project root path")
	skipSmoke := fs.Bool("skip-smoke", false, "skip health smoke check")
	fun := fs.Bool("fun", false, "show fun launch banner")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		printCommandFailure("First-run", err.Error(), "run `ang first-run --help`")
		os.Exit(1)
	}

	root := filepath.Clean(strings.TrimSpace(*projectPath))
	if root == "" {
		root = "."
	}
	funMode := isFunEnabled(*fun)
	if funMode {
		printFunRocket()
	}

	progress := newUpProgress(3)

	progress.step("Environment bootstrap")
	cfgChecks, err := collectConfigStartupChecks(root)
	if err != nil {
		printCommandFailure("First-run", err.Error(), "")
		os.Exit(1)
	}
	hasFail := printStartupChecks(cfgChecks)
	if hasFail {
		printCommandFailure("First-run", "environment checks failed", "run `ang config doctor` and fix required keys")
		os.Exit(1)
	}

	progress.step("Dependencies + build")
	if err := runSelf(root, "up", "--skip-doctor", "--skip-smoke"); err != nil {
		printCommandFailure("First-run", fmt.Sprintf("up: %v", err), "run `ang up --skip-doctor --skip-smoke` directly to inspect logs")
		os.Exit(1)
	}

	progress.step("Readiness smoke")
	if !*skipSmoke {
		if err := runSelf(root, "smoke"); err != nil {
			printCommandFailure("First-run", fmt.Sprintf("smoke: %v", err), "run `ang smoke --base-url http://localhost:8080` after starting your server")
			os.Exit(1)
		}
	} else {
		fmt.Println("     skipped (--skip-smoke)")
	}

	fmt.Println("FIRST-RUN READY")
	fmt.Println("Tip: run `ang tips` for next steps.")
}
