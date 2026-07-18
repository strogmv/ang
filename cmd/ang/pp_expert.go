package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/expert"
	ppfacts "github.com/strogmv/ang/compiler/paymentprovider/facts"
)

func runPPExpert(args []string) {
	fs := flag.NewFlagSet("pp expert", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mode := fs.String("mode", "shadow", "expert integration mode (off|shadow|advise|gate)")
	baseURL := fs.String("expert-base-url", "", "Expert Runtime base URL (required for shadow)")
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	schemaDir := fs.String("schema-dir", "", "Shared schema directory override")
	asJSON := fs.Bool("json", false, "output ang/expert-report/v1 JSON")
	var packIDs []string
	fs.Func("expert-pack", "Expert Runtime pack ID (repeatable)", func(value string) error {
		packIDs = append(packIDs, value)
		return nil
	})
	projectPath, flagArgs := splitPPProjectPath(args)
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}
	if err := validateExpertMode(*mode); err != nil {
		fmt.Fprintf(os.Stderr, "pp expert FAILED: %v\n", err)
		os.Exit(1)
	}
	if *mode == "off" {
		fmt.Fprintln(os.Stderr, "pp expert: mode off, nothing to do")
		return
	}
	if strings.TrimSpace(*baseURL) == "" {
		fmt.Fprintln(os.Stderr, "pp expert FAILED: shadow/advise mode requires --expert-base-url")
		os.Exit(1)
	}
	validated, err := buildPPExpertValidated(context.Background(), ppExpertOptions{
		ProjectPath: projectPath,
		CueRoot:     *cueRoot,
		SchemaDir:   *schemaDir,
		BaseURL:     *baseURL,
		PackIDs:     packIDs,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp expert FAILED: %v\n", err)
		os.Exit(1)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		switch validated.ReportSchema {
		case expert.SchemaV2:
			if err := enc.Encode(validated.ReportV2); err != nil {
				fmt.Fprintf(os.Stderr, "pp expert FAILED: %v\n", err)
				os.Exit(1)
			}
		default:
			if err := enc.Encode(validated.Report); err != nil {
				fmt.Fprintf(os.Stderr, "pp expert FAILED: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}
	if *mode == "advise" || *mode == "gate" {
		printPPExpertAdviceWithLabel(os.Stdout, validated, *mode)
		if *mode == "gate" {
			if blocked, reason := expertGateBlocksBuild(validated); blocked {
				fmt.Fprintf(os.Stderr, "pp expert gate FAILED: %s\n", reason)
				os.Exit(1)
			}
		}
		return
	}
	printPPExpertReport(validatedReportV1(validated))
}

func validatedReportV1(validated ValidatedExpertReport) expert.Report {
	if validated.ReportSchema == expert.SchemaV2 {
		return expert.Report{
			Schema:            expert.SchemaV2,
			Goal:              validated.ReportV2.Goal,
			Status:            validated.ReportV2.Status,
			CompilerVersion:   validated.ReportV2.CompilerVersion,
			FactsHash:         validated.ReportV2.FactsHash,
			KnowledgeVersions: validated.ReportV2.KnowledgeVersions,
			Findings:          validated.ReportV2.Findings,
			Trace:             validated.ReportV2.Trace,
			Verification:      validated.ReportV2.Verification,
			Diagnostics:       validated.ReportV2.Diagnostics,
		}
	}
	return validated.Report
}

func buildPPExpertReport(ctx context.Context, opts ppExpertOptions) (expert.Report, error) {
	validated, err := buildPPExpertValidated(ctx, opts)
	if err != nil {
		return expert.Report{}, err
	}
	return validatedReportV1(validated), nil
}

func buildPPExpertValidated(ctx context.Context, opts ppExpertOptions) (ValidatedExpertReport, error) {
	envelope, err := ppfacts.Extract(ppfacts.ExtractOptions{
		ProjectPath: opts.ProjectPath,
		CueRoot:     opts.CueRoot,
		SchemaDir:   opts.SchemaDir,
	})
	if err != nil {
		return ValidatedExpertReport{}, fmt.Errorf("extract payment-provider facts: %w", err)
	}
	factsJSON, err := ppfacts.CanonicalJSON(envelope)
	if err != nil {
		return ValidatedExpertReport{}, fmt.Errorf("canonicalize payment-provider facts: %w", err)
	}
	sum := sha256.Sum256(factsJSON)
	factsHash := hex.EncodeToString(sum[:])
	requestID, err := newExpertRequestID("pp.expert")
	if err != nil {
		return ValidatedExpertReport{}, err
	}
	packIDs := opts.PackIDs
	if len(packIDs) == 0 {
		packIDs = []string{"payment-provider.core"}
	}
	validated, err := Analyze(ctx, ExpertClientConfig{
		BaseURL: opts.BaseURL,
		Timeout: 10 * time.Second,
	}, ExpertAnalyzeRequest{
		Schema:          expertRequestSchema,
		RequestID:       requestID,
		Goal:            "payment_provider.audit",
		CompilerVersion: compiler.Version,
		Facts:           factsJSON,
		PackIDs:         packIDs,
	}, factsHash, ExpertAnalyzeScope{
		ProjectPath: opts.ProjectPath,
		CueRoot:     opts.CueRoot,
	})
	if err != nil {
		return ValidatedExpertReport{}, err
	}
	return validated, nil
}

type ppExpertOptions struct {
	ProjectPath string
	CueRoot     string
	SchemaDir   string
	BaseURL     string
	PackIDs     []string
}

func printPPExpertReport(report expert.Report) {
	fmt.Printf("ANG payment-provider expert: %s\n", report.Goal)
	fmt.Printf("Status: %s\n", report.Status)
	if len(report.Findings) == 0 {
		fmt.Println("No expert findings.")
		return
	}
	for _, finding := range report.Findings {
		fmt.Printf("- [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Code, finding.Summary)
	}
	fmt.Println("Read-only audit: no project files were modified.")
}
