package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/expert"
	"github.com/strogmv/ang/compiler/facts"
)

// runAdvise is intentionally audit-only. It never writes the project and does
// not accept --apply; proposal application belongs to a later verified phase.
func runAdvise(args []string) {
	fs := flag.NewFlagSet("advise", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	goal := fs.String("goal", "project.audit", "advice goal (currently: project.audit)")
	asJSON := fs.Bool("json", false, "output ang/expert-report/v1 JSON")
	factsPath := fs.String("facts", "", "path to ang/facts/v1 JSON used as evidence")
	expertCommand := fs.String("expert-command", "", "path to a local Expert Runtime stdio executable (requires --facts)")
	expertURL := fs.String("expert-url", "", "URL of an Expert Runtime HTTP endpoint (requires --facts)")
	var expertArgs []string
	fs.Func("expert-arg", "argument passed to the local Expert Runtime process (repeatable)", func(value string) error {
		expertArgs = append(expertArgs, value)
		return nil
	})
	var packDirs []string
	fs.Func("pack", "CUE knowledge-pack directory (repeatable; requires --facts)", func(value string) error {
		packDirs = append(packDirs, value)
		return nil
	})
	var expertPackIDs []string
	fs.Func("expert-pack", "Expert Runtime pack ID (repeatable; used with --expert-command)", func(value string) error {
		expertPackIDs = append(expertPackIDs, value)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Advise FAILED: %v\n", err)
		return
	}
	if strings.TrimSpace(*goal) != "project.audit" {
		fmt.Printf("Advise FAILED: unsupported goal %q (supported: project.audit)\n", *goal)
		return
	}
	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}
	var report expert.Report
	var err error
	if strings.TrimSpace(*expertCommand) != "" && strings.TrimSpace(*expertURL) != "" {
		err = fmt.Errorf("--expert-command and --expert-url are mutually exclusive")
	} else if strings.TrimSpace(*expertCommand) != "" {
		report, err = buildExternalAdviceReport(projectPath, *goal, *factsPath, *expertCommand, expertArgs, expertPackIDs)
	} else if strings.TrimSpace(*expertURL) != "" {
		report, err = buildHTTPAdviceReport(projectPath, *goal, *factsPath, *expertURL, expertPackIDs)
	} else {
		report, err = buildAdviceReport(projectPath, *goal, *factsPath, packDirs)
	}
	if err != nil {
		fmt.Printf("Advise FAILED: %v\n", err)
		return
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "advise: encode report: %v\n", err)
		}
		return
	}
	printAdviceReport(report)
}

func buildAdviceReport(projectPath, goal, factsPath string, packDirs []string) (expert.Report, error) {
	var normalizedFacts []expert.Fact
	factsHash := ""
	if strings.TrimSpace(factsPath) != "" {
		envelope, err := loadFactsEnvelope(factsPath)
		if err != nil {
			return expert.Report{}, fmt.Errorf("load facts: %w", err)
		}
		factsHash, err = facts.Hash(*envelope)
		if err != nil {
			return expert.Report{}, fmt.Errorf("hash facts: %w", err)
		}
		normalizedFacts, _, err = expert.AdaptFacts(*envelope)
		if err != nil {
			return expert.Report{}, fmt.Errorf("adapt facts: %w", err)
		}
	} else if len(packDirs) > 0 {
		return expert.Report{}, fmt.Errorf("--pack requires --facts")
	}

	_, pipelineErr := compiler.RunSemanticPhases(projectPath)
	report := expert.Audit(expert.AuditInput{
		Goal:            goal,
		CompilerVersion: compiler.Version,
		FactsHash:       factsHash,
		Diagnostics:     append([]normalizer.Warning(nil), compiler.LatestDiagnostics...),
		PipelineError:   pipelineErr,
	})
	seenPacks := make(map[string]struct{}, len(packDirs))
	for _, dir := range packDirs {
		pack, err := expert.LoadKnowledgePack(dir)
		if err != nil {
			return expert.Report{}, fmt.Errorf("load knowledge pack %q: %w", dir, err)
		}
		packVersion, err := registerKnowledgePack(seenPacks, pack)
		if err != nil {
			return expert.Report{}, err
		}
		inferred, err := expert.Infer(normalizedFacts, pack)
		if err != nil {
			return expert.Report{}, fmt.Errorf("infer knowledge pack %q: %w", dir, err)
		}
		report.KnowledgeVersions = append(report.KnowledgeVersions, packVersion)
		report.Findings = append(report.Findings, inferred.Findings...)
		report.Trace = append(report.Trace, inferred.Trace...)
	}
	report.Status = expert.ReconcileReportStatus(report.Status, report.Findings)
	canonical, err := expert.Canonicalize(report)
	if err != nil {
		return expert.Report{}, fmt.Errorf("canonicalize advice report: %w", err)
	}
	if err := expert.ValidateReport(canonical); err != nil {
		return expert.Report{}, fmt.Errorf("validate advice report: %w", err)
	}
	return canonical, nil
}

func registerKnowledgePack(seen map[string]struct{}, pack expert.KnowledgePack) (string, error) {
	version := pack.Name + "@" + pack.Version
	if _, exists := seen[version]; exists {
		return "", fmt.Errorf("duplicate knowledge pack %q", version)
	}
	seen[version] = struct{}{}
	return version, nil
}

func printAdviceReport(report expert.Report) {
	renderAdviceReport(os.Stdout, report)
}

func renderAdviceReport(w io.Writer, report expert.Report) {
	fmt.Fprintf(w, "ANG expert advice: %s\n", report.Goal)
	fmt.Fprintf(w, "Status: %s\n", report.Status)
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "No compiler findings. No changes proposed.")
		return
	}
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "- [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Code, finding.Summary)
	}
	if hasConflictFinding(report.Findings) {
		fmt.Fprintln(w, "Decision blocked: resolve conflicting evidence or rules before acting.")
	}
	fmt.Fprintln(w, "No changes proposed: project.audit is read-only.")
}

func hasConflictFinding(findings []expert.Finding) bool {
	for _, finding := range findings {
		if finding.Status == expert.FindingConflict {
			return true
		}
	}
	return false
}
