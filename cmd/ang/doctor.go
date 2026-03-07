package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/doctor"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

func runDoctor(args []string) {
	if len(args) > 0 && strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		runDoctorStart(args[1:])
		return
	}

	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logFile := fs.String("log-file", "ang-build.log", "path to build log file")
	inlineLog := fs.String("log", "", "inline log text to analyze")
	fromStdin := fs.Bool("stdin", false, "read log from stdin")
	fix := fs.Bool("fix", false, "apply safe fixes for current project diagnostics (including ARCHITECTURE_VIOLATION)")
	projectPath := fs.String("project-path", ".", "path to project root for --fix")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Doctor FAILED: %v\n", err)
		os.Exit(1)
	}

	logText := strings.TrimSpace(*inlineLog)
	if logText == "" {
		useStdin := *fromStdin || isPipedStdin()
		if useStdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Printf("Doctor FAILED: read stdin: %v\n", err)
				os.Exit(1)
			}
			logText = string(b)
		}
	}
	if strings.TrimSpace(logText) == "" {
		b, err := os.ReadFile(*logFile)
		if err != nil {
			fmt.Printf("Doctor FAILED: cannot read %s (%v)\n", *logFile, err)
			fmt.Println("Provide --log, --stdin, or run with an existing ang-build.log")
			os.Exit(1)
		}
		logText = string(b)
	}

	resp := doctor.NewAnalyzer(".").Analyze(logText)
	if msg, risky := detectReleaseRootModuleMismatch("."); risky {
		resp.Summary = append(resp.Summary, "Output guard: "+msg)
	}
	if *fix {
		fixReport, fixErr := applyArchitectureDoctorFix(*projectPath)
		if fixErr != nil {
			fmt.Printf("Doctor FAILED: apply fix: %v\n", fixErr)
			os.Exit(1)
		}
		resp.Summary = append(resp.Summary, fixReport...)
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Printf("Doctor FAILED: marshal response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

func applyArchitectureDoctorFix(projectPath string) ([]string, error) {
	projectRoot := strings.TrimSpace(projectPath)
	if projectRoot == "" {
		projectRoot = "."
	}

	var diagnostics []normalizer.Warning
	_, err := compiler.RunSemanticPhasesWithOptions(projectRoot, compiler.PipelineOptions{
		ArchitectureMode: "strict",
		WarningSink: func(w normalizer.Warning) {
			diagnostics = append(diagnostics, w)
		},
	})
	if err != nil {
		return nil, err
	}

	reViolation := regexp.MustCompile(`Service '([^']+)' is not allowed to directly access entity '([^']+)'`)
	requested := make(map[string]normalizer.CrossServiceRule)
	for _, d := range diagnostics {
		if d.Code != "ARCHITECTURE_VIOLATION" {
			continue
		}
		m := reViolation.FindStringSubmatch(d.Message)
		if len(m) != 3 {
			continue
		}
		service := strings.TrimSpace(m[1])
		entity := strings.TrimSpace(m[2])
		if service == "" || entity == "" {
			continue
		}
		requested[strings.ToLower(service)+"|"+strings.ToLower(entity)] = normalizer.CrossServiceRule{Service: service, Entity: entity}
	}
	if len(requested) == 0 {
		return []string{"Doctor --fix: ARCHITECTURE_VIOLATION not detected, nothing to patch."}, nil
	}

	p := parser.New()
	n := normalizer.New()
	var projectDef *normalizer.ProjectDef
	if val, ok, loadErr := compiler.LoadOptionalDomain(p, filepath.Join(projectRoot, "cue/project")); loadErr == nil && ok {
		projectDef, _ = n.ExtractProject(val)
	}

	existing := make(map[string]struct{})
	if projectDef != nil {
		for _, r := range projectDef.AllowCrossService {
			k := strings.ToLower(strings.TrimSpace(r.Service)) + "|" + strings.ToLower(strings.TrimSpace(r.Entity))
			if k != "|" {
				existing[k] = struct{}{}
			}
		}
	}

	var newRules []normalizer.CrossServiceRule
	for key, rule := range requested {
		if _, ok := existing[key]; ok {
			continue
		}
		newRules = append(newRules, rule)
	}
	if len(newRules) == 0 {
		return []string{"Doctor --fix: архитектурные исключения уже добавлены, новых правил нет."}, nil
	}
	sort.Slice(newRules, func(i, j int) bool {
		li := strings.ToLower(newRules[i].Service + "|" + newRules[i].Entity)
		lj := strings.ToLower(newRules[j].Service + "|" + newRules[j].Entity)
		return li < lj
	})

	overridesPath := filepath.Join(projectRoot, "cue", "project", "architecture_overrides.cue")
	if err := os.MkdirAll(filepath.Dir(overridesPath), 0o755); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("package project\n\n")
	b.WriteString("#Project: {\n")
	b.WriteString("\tarchitecture: {\n")
	b.WriteString("\t\tallow_cross_service: [\n")
	for _, rule := range newRules {
		b.WriteString(fmt.Sprintf("\t\t\t{service: %q, entity: %q},\n", rule.Service, rule.Entity))
	}
	b.WriteString("\t\t]\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	if err := os.WriteFile(overridesPath, []byte(b.String()), 0o644); err != nil {
		return nil, err
	}

	return []string{
		fmt.Sprintf("Doctor --fix: added %d architecture override rule(s) in %s", len(newRules), filepath.ToSlash(overridesPath)),
		"Next: replace overrides with service.Call/read-model where possible and switch back to strict mode.",
	}, nil
}

func isPipedStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}
