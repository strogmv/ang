package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler/paymentprovider"
	"github.com/strogmv/ang/compiler/paymentprovider/integration"
)

func runPPInit(args []string) {
	projectPath, flagArgs := splitPPProjectPath(args)
	fs := flag.NewFlagSet("pp init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sid := fs.String("sid", "", "Provider SID (e.g. mx6)")
	label := fs.String("label", "", "Provider label (e.g. MX-6)")
	name := fs.String("name", "", "Provider display name")
	pkg := fs.String("package", "", "Go package name (default: directory name)")
	module := fs.String("module", "", "CUE module (default: transferty.local/<package>)")
	ticket := fs.String("ticket", "", "One-line PM ticket summary")
	knowledge := fs.String("knowledge", "", "Expert knowledge id (knowledge/data/<id>.json; default: sid)")
	force := fs.Bool("force", false, "Overwrite existing scaffold files")
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(2)
	}
	if strings.TrimSpace(*sid) == "" {
		fmt.Fprintln(os.Stderr, "pp init: --sid is required")
		os.Exit(2)
	}
	if strings.TrimSpace(*label) == "" {
		fmt.Fprintln(os.Stderr, "pp init: --label is required")
		os.Exit(2)
	}
	if projectPath == "." {
		wd, err := os.Getwd()
		if err == nil {
			projectPath = wd
		}
	}
	if *pkg == "" {
		*pkg = filepathBase(projectPath)
	}
	result, err := integration.InitProject(integration.InitOptions{
		ProjectPath:   projectPath,
		SID:           *sid,
		Label:         *label,
		Name:          *name,
		PackageName:   *pkg,
		Module:        *module,
		TicketSummary: *ticket,
		KnowledgeID:   *knowledge,
		Force:         *force,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp init FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Integration workspace ready at %s\n", projectPath)
	if len(result.Created) > 0 {
		fmt.Println("Created:")
		for _, f := range result.Created {
			fmt.Printf("  + %s\n", f)
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Println("Skipped (already exist; use --force):")
		for _, f := range result.Skipped {
			fmt.Printf("  - %s\n", f)
		}
	}
	fmt.Println("\nNext: add ticket/API knowledge to deal/expert/knowledge/data/, refine .cue/provider.cue, run ang pp vet")
}

func runPPBrief(args []string) {
	projectPath, flagArgs := splitPPProjectPath(args)
	fs := flag.NewFlagSet("pp brief", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "Print brief as JSON")
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(2)
	}
	cfg := paymentprovider.LoadProjectConfig(projectPath)
	brief, err := integration.ResolveBrief(projectPath, cfg.ExpertRoot, cfg.ExpertKnowledgeID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp brief FAILED: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		printBriefJSON(brief)
		return
	}
	printBriefSummary(brief)
}

func runPPPack(args []string) {
	if len(args) == 0 {
		printPPUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "validate":
		runPPPackValidate(args[1:])
	default:
		fmt.Printf("Unknown pp pack subcommand: %s\n", args[0])
		printPPUsage()
		os.Exit(1)
	}
}

func runPPPackValidate(args []string) {
	path := "integration/expert.pack.cue"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		path = args[0]
	}
	if err := integration.ValidateExpertPack(path); err != nil {
		fmt.Fprintf(os.Stderr, "pp pack validate FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pack OK: %s\n", path)
}

func filepathJoin(base, rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	return strings.TrimRight(base, "/") + "/" + rel
}

func filepathBase(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func printBriefSummary(b integration.Brief) {
	fmt.Printf("Provider: %s (%s) package=%s\n", b.Provider.Label, b.Provider.Code, b.Provider.PackageName)
	fmt.Printf("Integration: type=%s flow=%s implementation=%s status=%s\n",
		b.Integration.Type, b.Integration.Flow, b.Implementation, b.Status)
	if len(b.Methods) > 0 {
		fmt.Printf("Methods: %s\n", strings.Join(b.Methods, ", "))
	}
	if len(b.Operations) > 0 {
		fmt.Printf("Operations: %s\n", strings.Join(b.Operations, ", "))
	}
	if len(b.Currencies) > 0 {
		fmt.Printf("Currencies: %s\n", strings.Join(b.Currencies, ", "))
	}
	if b.References.Ticket != "" {
		fmt.Printf("Ticket: %s\n", b.References.Ticket)
	}
	if b.References.ExpertKnowledge != "" {
		fmt.Printf("Expert knowledge: knowledge/data/%s.json\n", b.References.ExpertKnowledge)
	} else if b.References.APIDoc != "" {
		fmt.Printf("API doc: %s\n", b.References.APIDoc)
	}
}

func printBriefJSON(b integration.Brief) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp brief FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
