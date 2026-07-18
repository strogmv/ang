package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler/paymentprovider"
)

func runPP(args []string) {
	if len(args) == 0 {
		printPPUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "schema":
		runPPSchema(args[1:])
	case "vet":
		runPPVet(args[1:])
	case "facts":
		runPPFacts(args[1:])
	case "expert":
		runPPExpert(args[1:])
	case "apply":
		runPPApply(args[1:])
	case "init":
		runPPInit(args[1:])
	case "brief":
		runPPBrief(args[1:])
	case "pack":
		runPPPack(args[1:])
	default:
		fmt.Printf("Unknown pp subcommand: %s\n", args[0])
		printPPUsage()
		os.Exit(1)
	}
}

func printPPUsage() {
	fmt.Println("Usage: ang pp <schema|vet|facts|expert|apply|init|brief|pack> ...")
	fmt.Println("  ang pp schema list              List bundled schema files shipped with ang")
	fmt.Println("  ang pp schema sync [path]       Copy bundled schema into project schema dir")
	fmt.Println("                                  (uses schema_dir from ang.yaml when set)")
	fmt.Println("                                  [--force] [--dry-run]")
	fmt.Println("  ang pp schema check [path]      Fail if local schema differs from ang bundle")
	fmt.Println("  ang pp vet [path]               Semantic validation of provider CUE intent")
	fmt.Println("  ang pp facts [path]             Extract canonical payment-provider facts JSON")
	fmt.Println("                                  [--cue-root .cue] [--schema-dir DIR] [--json]")
	fmt.Println("  ang pp expert [path]            Read-only payment-provider expert audit")
	fmt.Println("                                  [--mode off|shadow|advise|gate] [--expert-base-url URL]")
	fmt.Println("                                  [--expert-pack ID] [--json]")
	fmt.Println("  ang pp apply [path]             Sandbox-verify and optionally apply a proposal")
	fmt.Println("                                  --proposal ID --expert-base-url URL [--approve]")
	fmt.Println("                                  [--expert-pack ID] [--json]")
	fmt.Println("  ang pp init [path]              Scaffold provider workspace (ang.yaml, CUE skeleton)")
	fmt.Println("                                  --sid SID --label LABEL [--name NAME] [--package PKG]")
	fmt.Println("                                  [--module MODULE] [--ticket SUMMARY] [--knowledge ID] [--force]")
	fmt.Println("  ang pp brief [path]             Show integration brief from Expert knowledge [--json]")
	fmt.Println("  ang pp pack validate [path]     Validate Expert *.research.cue manifest")
	fmt.Println("")
	fmt.Println("Payment-provider schema (provider.cue, catalogs.cue, profiles.cue) is maintained in ang.")
	fmt.Println("Consumer repos keep provider intent (.cue/provider.cue) only.")
	fmt.Println("Schema: per-provider .cue/schema/ OR shared schema_dir in ang.yaml (monorepo).")
}

// splitPPProjectPath separates an optional leading project path from flag arguments.
func splitPPProjectPath(args []string) (projectPath string, flagArgs []string) {
	projectPath = "."
	flagArgs = args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
		flagArgs = args[1:]
	}
	return projectPath, flagArgs
}

func runPPSchema(args []string) {
	if len(args) == 0 {
		printPPUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		runPPSchemaList()
	case "sync":
		runPPSchemaSync(args[1:])
	case "check":
		runPPSchemaCheck(args[1:])
	default:
		fmt.Printf("Unknown pp schema subcommand: %s\n", args[0])
		printPPUsage()
		os.Exit(1)
	}
}

func runPPSchemaList() {
	files, err := paymentprovider.BundledSchemaFiles()
	if err != nil {
		fmt.Printf("Schema list FAILED: %v\n", err)
		os.Exit(1)
	}
	for _, f := range files {
		fmt.Println(f)
	}
}

func runPPSchemaSync(args []string) {
	fs := flag.NewFlagSet("pp schema sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "show files that would be updated without writing")
	force := fs.Bool("force", false, "overwrite local schema extensions with ang bundle")
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	res, err := paymentprovider.SyncSchema(paymentprovider.SchemaSyncOptions{
		ProjectPath: path,
		CueRoot:     *cueRoot,
		DryRun:      *dryRun,
		Force:       *force,
	})
	if err != nil {
		fmt.Printf("Schema sync FAILED: %v\n", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Printf("Dry run: target %s\n", res.TargetDir)
	}
	if len(res.Written) == 0 && len(res.Skipped) > 0 && len(res.Guarded) == 0 {
		fmt.Printf("Schema already up to date (%d files checked).\n", len(res.Skipped))
		return
	}
	if len(res.Guarded) > 0 {
		fmt.Println("Schema sync preserved local extensions:")
		for _, item := range res.Guarded {
			fmt.Printf("  - %s\n", item)
		}
	}
	if len(res.Written) > 0 {
		action := "Updated"
		if *dryRun {
			action = "Would update"
		}
		fmt.Printf("%s %d schema file(s) in %s:\n", action, len(res.Written), res.TargetDir)
		for _, f := range res.Written {
			fmt.Printf("  - %s\n", f)
		}
	}
}

func runPPSchemaCheck(args []string) {
	fs := flag.NewFlagSet("pp schema check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	drift, err := paymentprovider.CheckSchema(path, *cueRoot)
	if err != nil {
		fmt.Printf("Schema check FAILED: %v\n", err)
		os.Exit(1)
	}
	if len(drift) == 0 {
		fmt.Println("Schema is in sync with ang bundle.")
		return
	}
	fmt.Println("Schema drift detected:")
	for _, item := range drift {
		fmt.Printf("  - %s\n", item)
	}
	fmt.Println("")
	fmt.Println("Fix: ang pp schema sync " + strings.TrimSpace(path))
	os.Exit(1)
}

func runPPVet(args []string) {
	fs := flag.NewFlagSet("pp vet", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	jsonOut := fs.Bool("json", false, "print findings as JSON array")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	issues, err := paymentprovider.VetProject(path, *cueRoot)
	if err != nil {
		fmt.Printf("Vet FAILED: %v\n", err)
		os.Exit(1)
	}
	errors := 0
	warnings := 0
	for _, iss := range issues {
		if iss.Severity == "error" {
			errors++
		} else {
			warnings++
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(issues)
	} else if len(issues) == 0 {
		fmt.Println("Vet OK: no issues found.")
	} else {
		for _, iss := range issues {
			fmt.Printf("[%s] %s: %s\n", iss.Severity, iss.Code, iss.Message)
			if iss.Hint != "" {
				fmt.Printf("  hint: %s\n", iss.Hint)
			}
		}
		fmt.Printf("\nVet: %d error(s), %d warning(s)\n", errors, warnings)
	}
	if errors > 0 {
		os.Exit(1)
	}
}
