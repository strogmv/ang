package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

func runContractTest() {
	cmd := exec.Command("go", "test", "-tags=contract", "./tests/contract/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Contract tests FAILED: %v\n", err)
		os.Exit(1)
	}
}

func runVet(args []string) {
	if len(args) > 0 && args[0] == "logic" {
		runLogicVet()
		return
	}

	fmt.Println("Checking architectural invariants (ANG Law Enforcement)...")
	projectPath := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
	}
	entities, services, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		printStageFailure("Vet FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err)
		os.Exit(1)
	}

	fmt.Println("Running CUE Policy Checks...")
	p := parser.New()
	val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, "cue/policies"))
	if err == nil && ok {
		if err := val.Validate(); err != nil {
			printStageFailure("Policy Violation", compiler.StageCUE, compiler.ErrCodeCUEPolicyValidate, "validate cue/policies", err)
			os.Exit(1)
		}

		errVal := val.LookupPath(cue.ParsePath("validation_errors"))
		if errVal.Exists() {
			fmt.Println("Policy Violations Found:")
			iter, _ := errVal.Fields()
			for iter.Next() {
				fmt.Printf("  - %s: %s\n", iter.Selector(), iter.Value())
			}
			os.Exit(1)
		}
		fmt.Println("CUE Policies Passed.")
	} else if err != nil {
		printStageFailure("Policy Violation", compiler.StageCUE, compiler.ErrCodeCUEPolicyLoad, "load cue/policies", err)
		os.Exit(1)
	}

	failed := false
	for _, e := range entities {
		if strings.HasSuffix(e.Name, "Response") || strings.HasSuffix(e.Name, "Request") {
			continue
		}
		hasID := false
		for _, f := range e.Fields {
			if f.Name == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			fmt.Printf("Violation: Entity '%s' has no 'id' field.\n", e.Name)
			failed = true
		}
	}

	for _, s := range services {
		if len(s.Methods) == 0 {
			fmt.Printf("Violation: Service '%s' is empty.\n", s.Name)
			failed = true
		}
	}

	if failed {
		fmt.Println("\nInvariants check FAILED.")
		os.Exit(1)
	}
	fmt.Println("All architectural laws are satisfied.")
}

func runDraw(args []string) {
	fmt.Println("Drawing architecture...")
	entities, services, endpoints, repos, events, bizErrors, schedules, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("Draw FAILED (Parser error): %v\n", err)
		os.Exit(1)
	}
	irSchema, err := compiler.ConvertAndTransform(
		entities, services, events, bizErrors, endpoints, repos,
		normalizer.ConfigDef{}, nil, nil, schedules, nil, normalizer.ProjectDef{},
	)
	if err != nil {
		fmt.Printf("Draw FAILED (IR convert): %v\n", err)
		os.Exit(1)
	}
	output, err := parseOutputOptions(args)
	if err != nil {
		fmt.Printf("Draw FAILED: %v\n", err)
		os.Exit(1)
	}
	em := emitter.New(output.BackendDir, output.FrontendDir, "templates")
	em.FrontendAdminDir = output.FrontendAdminDir
	ctx := em.AnalyzeContextFromIR(irSchema)
	em.EnrichContextFromIR(&ctx, irSchema)
	if err := em.EmitMermaid(ctx); err != nil {
		fmt.Printf("Draw FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Draw SUCCESSFUL.")
}

func runVersion() {
	fmt.Printf("ANG version %s (Schema v%s)\n", compiler.Version, compiler.SchemaVersion)
}

func runRBAC(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang rbac <actions|inspect>")
		return
	}

	cmd := args[0]
	_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	validActions := make(map[string]bool)
	for _, s := range services {
		for _, m := range s.Methods {
			validActions[strings.ToLower(s.Name)+"."+strings.ToLower(m.Name)] = true
		}
	}

	switch cmd {
	case "actions":
		fmt.Println("Registered RBAC Actions (Service.Method):")
		fmt.Println("----------------------------------------")
		for action := range validActions {
			fmt.Println(action)
		}

	case "inspect":
		fmt.Println("RBAC Security Audit:")
		fmt.Println("--------------------")

		p := parser.New()
		n := normalizer.New()
		var rbac *normalizer.RBACDef
		if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/policies"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		} else if val, ok, err := compiler.LoadOptionalDomain(p, "./cue/rbac"); err == nil && ok {
			rbac, _ = n.ExtractRBAC(val)
		}

		if rbac == nil {
			fmt.Println("⚠️  No RBAC policies found in CUE (cue/policies or cue/rbac).")
			return
		}

		protected := make(map[string]bool)
		zombies := []string{}

		for action := range rbac.Permissions {
			if validActions[action] {
				protected[action] = true
			} else {
				zombies = append(zombies, action)
			}
		}

		fmt.Println("\n🔴 UNPROTECTED METHODS (Missing from policies.cue):")
		for action := range validActions {
			if !protected[action] {
				fmt.Printf("   - %s\n", action)
			}
		}

		if len(zombies) > 0 {
			fmt.Println("\n💀 ZOMBIE POLICIES (References non-existent methods):")
			for _, z := range zombies {
				fmt.Printf("   - %s\n", z)
			}
		}

		fmt.Printf("\n🟢 PROTECTED: %d methods\n", len(protected))

	default:
		fmt.Printf("Unknown RBAC command: %s\n", cmd)
	}
}

func runDB(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang db <sync|status>")
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DB_URL")
	}

	switch args[0] {
	case "status":
		if dbURL == "" {
			fmt.Println("Error: DATABASE_URL is not set. Cannot check drift.")
			return
		}
		fmt.Println("Checking database schema drift...")
		cmd := exec.Command("atlas", "schema", "diff",
			"--from", dbURL,
			"--to", "file://db/schema/schema.sql",
			"--dev-url", "docker://postgres/15/dev",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("\n❌ DRIFT DETECTED:\n%s\n", string(out))
			fmt.Println("💡 Hint: Run 'ang db sync' to apply these changes.")
			os.Exit(1)
		}
		if len(strings.TrimSpace(string(out))) == 0 || strings.Contains(string(out), "Schemas are in sync") {
			fmt.Println("✅ Database schema is in sync with CUE.")
		} else {
			fmt.Printf("\nPending changes:\n%s\n", string(out))
		}

	case "sync":
		if dbURL == "" {
			fmt.Println("Error: DATABASE_URL is not set.")
			return
		}
		fmt.Println("Synchronizing database schema with CUE...")
		cmd := exec.Command("atlas", "schema", "apply",
			"--url", dbURL,
			"--to", "file://db/schema/schema.sql",
			"--dev-url", "docker://postgres/15/dev",
			"--auto-approve",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("\nDB Sync FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✅ Database schema is now in sync with CUE.")
	default:
		fmt.Printf("Unknown DB command: %s\n", args[0])
	}
}

func runEvents(args []string) {
	if len(args) == 0 || args[0] != "map" {
		fmt.Println("Usage: ang events map")
		return
	}

	_, services, _, _, _, _, _, _, err := compiler.RunPipeline(".")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Event Flow Map (Event-Driven Architecture Audit):")
	fmt.Println("--------------------------------------------------")

	type Publisher struct {
		Service string
		Method  string
	}
	type Subscriber struct {
		Service string
		Handler string
	}

	publishers := make(map[string][]Publisher)
	subscribers := make(map[string][]Subscriber)

	for _, s := range services {
		for _, m := range s.Methods {
			for _, p := range m.Publishes {
				publishers[p] = append(publishers[p], Publisher{Service: s.Name, Method: m.Name})
			}
		}
		for evt, handler := range s.Subscribes {
			subscribers[evt] = append(subscribers[evt], Subscriber{Service: s.Name, Handler: handler})
		}
	}

	allEvents := make(map[string]bool)
	for e := range publishers {
		allEvents[e] = true
	}
	for e := range subscribers {
		allEvents[e] = true
	}

	foundAny := false
	for evt := range allEvents {
		foundAny = true
		fmt.Printf("\n📢 Event: %s\n", evt)

		pubs := publishers[evt]
		if len(pubs) > 0 {
			fmt.Print("   Produced by:\n")
			for _, p := range pubs {
				fmt.Printf("     - %s.%s\n", p.Service, p.Method)
			}
		} else {
			fmt.Print("   ⚠️  PRODUCER MISSING (External or manual)\n")
		}

		subs := subscribers[evt]
		if len(subs) > 0 {
			fmt.Print("   Consumed by:\n")
			for _, s := range subs {
				fmt.Printf("     - %s (Handler: %s)\n", s.Service, s.Handler)
			}
		} else {
			fmt.Print("   ⚠️  DEAD END (No subscribers found)\n")
		}
	}

	if !foundAny {
		fmt.Println("No event publishers or subscribers found in current architecture.")
	}
}

func runLogicVet() {
	fmt.Println("Auditing embedded Go logic in CUE files...")
	_, _, _, _, _, _, _, _, _ = compiler.RunPipeline(".")
	found := false
	for _, d := range compiler.LatestDiagnostics {
		if d.Code == "GO_SYNTAX_ERROR" {
			fmt.Printf("\n❌ Logic Error at %s:%d:%d\n", d.File, d.Line, d.Column)
			fmt.Printf("   %s\n", d.Message)
			found = true
		}
	}
	if found {
		os.Exit(1)
	}
	fmt.Println("✅ No Go syntax errors in embedded logic.")
}
