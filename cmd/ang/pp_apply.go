package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler/expert"
	"github.com/strogmv/ang/compiler/paymentprovider"
)

func runPPApply(args []string) {
	fs := flag.NewFlagSet("pp apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	proposalID := fs.String("proposal", "", "Expert proposal ID to verify or apply (required)")
	baseURL := fs.String("expert-base-url", "", "Expert Runtime base URL (required)")
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	schemaDir := fs.String("schema-dir", "", "Shared schema directory override")
	approve := fs.Bool("approve", false, "write verified proposal changes to the project")
	asJSON := fs.Bool("json", false, "output verification result as JSON")
	var packIDs []string
	fs.Func("expert-pack", "Expert Runtime pack ID (repeatable)", func(value string) error {
		packIDs = append(packIDs, value)
		return nil
	})
	projectPath, flagArgs := splitPPProjectPath(args)
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}
	if strings.TrimSpace(*proposalID) == "" {
		fmt.Fprintln(os.Stderr, "pp apply FAILED: --proposal is required")
		os.Exit(1)
	}
	if strings.TrimSpace(*baseURL) == "" {
		fmt.Fprintln(os.Stderr, "pp apply FAILED: --expert-base-url is required")
		os.Exit(1)
	}
	result, err := verifyPPProposal(context.Background(), ppApplyOptions{
		ProjectPath: projectPath,
		CueRoot:     *cueRoot,
		SchemaDir:   *schemaDir,
		BaseURL:     *baseURL,
		PackIDs:     packIDs,
		ProposalID:  *proposalID,
		Approve:     *approve,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp apply FAILED: %v\n", err)
		os.Exit(1)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	} else {
		printPPApplyResult(os.Stdout, result)
	}
	if !result.Verified {
		os.Exit(1)
	}
}

type ppApplyOptions struct {
	ProjectPath string
	CueRoot     string
	SchemaDir   string
	BaseURL     string
	PackIDs     []string
	ProposalID  string
	Approve     bool
}

type ppApplyResult struct {
	ProposalID     string   `json:"proposal_id"`
	Verified       bool     `json:"verified"`
	Applied          bool     `json:"applied"`
	OutcomeRecorded  bool     `json:"outcome_recorded,omitempty"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	BuildStatus    string   `json:"build_status,omitempty"`
	VetStatus      string   `json:"vet_status,omitempty"`
	BlockingReason string   `json:"blocking_reason,omitempty"`
}

func verifyPPProposal(ctx context.Context, opts ppApplyOptions) (ppApplyResult, error) {
	validated, err := buildPPExpertValidated(ctx, ppExpertOptions{
		ProjectPath: opts.ProjectPath,
		CueRoot:     opts.CueRoot,
		SchemaDir:   opts.SchemaDir,
		BaseURL:     opts.BaseURL,
		PackIDs:     opts.PackIDs,
	})
	if err != nil {
		return ppApplyResult{}, err
	}
	if validated.ReportSchema != expert.SchemaV2 {
		return ppApplyResult{}, fmt.Errorf("proposal apply requires ang/expert-report/v2")
	}
	if err := expert.ValidateReportV2Scope(opts.ProjectPath, opts.CueRoot, validated.ReportV2); err != nil {
		return ppApplyResult{}, fmt.Errorf("report scope: %w", err)
	}
	proposal, err := expert.FindProposalV2(validated.ReportV2, opts.ProposalID)
	if err != nil {
		return ppApplyResult{}, err
	}
	sandbox, err := os.MkdirTemp("", "ang-pp-apply-*")
	if err != nil {
		return ppApplyResult{}, err
	}
	defer os.RemoveAll(sandbox)

	changedFiles, err := stageProposalChanges(opts, sandbox, proposal)
	if err != nil {
		return ppApplyResult{}, err
	}
	vetStatus, buildStatus, verifyErr := verifySandboxProject(opts, sandbox)
	result := ppApplyResult{
		ProposalID:   opts.ProposalID,
		Verified:     verifyErr == nil,
		ChangedFiles: changedFiles,
		BuildStatus:  buildStatus,
		VetStatus:    vetStatus,
	}
	if verifyErr != nil {
		result.BlockingReason = verifyErr.Error()
		return result, nil
	}
	if !opts.Approve {
		return result, nil
	}
	applied, err := commitProposalChanges(opts, proposal)
	if err != nil {
		return ppApplyResult{}, err
	}
	result.Applied = applied
	if applied {
		if err := recordPPApplyOutcome(ctx, opts, validated, opts.ProposalID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: EXPERT_OUTCOME_NOT_RECORDED: %v\n", err)
		} else {
			result.OutcomeRecorded = true
		}
	}
	return result, nil
}

func stageProposalChanges(opts ppApplyOptions, sandbox string, proposal expert.ProposalV2) ([]string, error) {
	if err := copyProviderTreeForSandbox(opts, sandbox); err != nil {
		return nil, err
	}
	byFile := map[string][]expert.ChangeV2{}
	order := make([]string, 0)
	for _, change := range proposal.Changes {
		rel := filepath.ToSlash(strings.TrimSpace(change.Target.RelativePath))
		if _, ok := byFile[rel]; !ok {
			order = append(order, rel)
		}
		byFile[rel] = append(byFile[rel], change)
	}
	changed := make([]string, 0, len(order))
	for _, rel := range order {
		path := filepath.Join(sandbox, opts.CueRoot, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		for _, change := range byFile[rel] {
			if err := expert.VerifyChangeBeforeHash(content, change); err != nil {
				return nil, err
			}
			content, err = expert.ApplyChangeV2(content, change, false)
			if err != nil {
				return nil, fmt.Errorf("stage %s: %w", rel, err)
			}
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", rel, err)
		}
		changed = append(changed, rel)
	}
	return changed, nil
}

func copyProviderTreeForSandbox(opts ppApplyOptions, sandbox string) error {
	pc := paymentprovider.LoadProjectConfig(opts.ProjectPath)
	cueRoot := opts.CueRoot
	if cueRoot == "" {
		cueRoot = pc.CueRoot
	}
	if err := copyDir(filepath.Join(opts.ProjectPath, cueRoot), filepath.Join(sandbox, cueRoot)); err != nil {
		return fmt.Errorf("copy cue root: %w", err)
	}
	localCfg := map[string]string{
		"cue_root":      cueRoot,
		"templates_dir": pc.TemplatesDir,
		"schema_dir":    pc.SchemaDir,
	}
	if strings.TrimSpace(opts.SchemaDir) != "" {
		localCfg["schema_dir"] = opts.SchemaDir
	}
	if strings.TrimSpace(pc.SchemaDir) != "" {
		schemaDir := localCfg["schema_dir"]
		resolved, err := paymentprovider.ResolvePath(opts.ProjectPath, schemaDir)
		if err != nil {
			return err
		}
		localSchema := filepath.Join(sandbox, ".ang", "schema")
		if err := copyDir(resolved, localSchema); err != nil {
			return fmt.Errorf("copy schema dir: %w", err)
		}
		localCfg["schema_dir"] = ".ang/schema"
	}
	if strings.TrimSpace(pc.TemplatesDir) != "" {
		resolved, err := paymentprovider.ResolvePath(opts.ProjectPath, pc.TemplatesDir)
		if err != nil {
			return err
		}
		localTemplates := filepath.Join(sandbox, ".ang", "templates")
		if err := copyDir(resolved, localTemplates); err != nil {
			return fmt.Errorf("copy templates dir: %w", err)
		}
		localCfg["templates_dir"] = ".ang/templates"
	}
	if err := writeSandboxAngYAML(sandbox, localCfg); err != nil {
		return err
	}
	return nil
}

func writeSandboxAngYAML(sandbox string, cfg map[string]string) error {
	lines := make([]string, 0, len(cfg))
	if v := strings.TrimSpace(cfg["cue_root"]); v != "" {
		lines = append(lines, fmt.Sprintf("cue_root: %q", v))
	}
	if v := strings.TrimSpace(cfg["templates_dir"]); v != "" {
		lines = append(lines, fmt.Sprintf("templates_dir: %q", v))
	}
	if v := strings.TrimSpace(cfg["schema_dir"]); v != "" {
		lines = append(lines, fmt.Sprintf("schema_dir: %q", v))
	}
	if len(lines) == 0 {
		return nil
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(filepath.Join(sandbox, "ang.yaml"), []byte(content), 0o644)
}

func verifySandboxProject(opts ppApplyOptions, sandbox string) (vetStatus, buildStatus string, err error) {
	issues, vetErr := paymentprovider.VetProject(sandbox, opts.CueRoot)
	if vetErr != nil {
		return "failed", "skipped", vetErr
	}
	for _, issue := range issues {
		if issue.Severity == "error" {
			return "failed", "skipped", fmt.Errorf("pp vet error %s: %s", issue.Code, issue.Message)
		}
	}
	vetStatus = "passed"
	pc := paymentprovider.LoadProjectConfig(sandbox)
	if _, buildErr := paymentprovider.BuildWithResult(paymentprovider.BuildOptions{
		ProjectPath:  sandbox,
		CueRoot:      opts.CueRoot,
		SchemaDir:    opts.SchemaDir,
		TemplatesDir: pc.TemplatesDir,
		OutputDir:    filepath.Join(sandbox, "generated"),
	}); buildErr != nil {
		return vetStatus, "failed", buildErr
	}
	return vetStatus, "passed", nil
}

func commitProposalChanges(opts ppApplyOptions, proposal expert.ProposalV2) (bool, error) {
	byFile := map[string][]expert.ChangeV2{}
	order := make([]string, 0)
	for _, change := range proposal.Changes {
		rel := filepath.ToSlash(strings.TrimSpace(change.Target.RelativePath))
		if _, ok := byFile[rel]; !ok {
			order = append(order, rel)
		}
		byFile[rel] = append(byFile[rel], change)
	}
	applied := false
	for _, rel := range order {
		path := filepath.Join(opts.ProjectPath, opts.CueRoot, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return applied, fmt.Errorf("read %s: %w", rel, err)
		}
		for _, change := range byFile[rel] {
			if err := expert.VerifyChangeBeforeHash(content, change); err != nil {
				return applied, err
			}
			content, err = expert.ApplyChangeV2(content, change, false)
			if err != nil {
				return applied, fmt.Errorf("apply %s: %w", rel, err)
			}
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return applied, fmt.Errorf("write %s: %w", rel, err)
		}
		applied = true
	}
	return applied, nil
}

func printPPApplyResult(w io.Writer, result ppApplyResult) {
	fmt.Fprintf(w, "ANG payment-provider apply: %s\n", result.ProposalID)
	fmt.Fprintf(w, "Verified: %t\n", result.Verified)
	if result.VetStatus != "" {
		fmt.Fprintf(w, "Vet: %s\n", result.VetStatus)
	}
	if result.BuildStatus != "" {
		fmt.Fprintf(w, "Build: %s\n", result.BuildStatus)
	}
	if len(result.ChangedFiles) > 0 {
		fmt.Fprintf(w, "Changed files (sandbox): %s\n", strings.Join(result.ChangedFiles, ", "))
	}
	if result.Applied {
		fmt.Fprintln(w, "Applied: project CUE intent updated.")
		if result.OutcomeRecorded {
			fmt.Fprintln(w, "Outcome: recorded in Expert Runtime.")
		}
	} else if result.Verified {
		fmt.Fprintln(w, "Sandbox verified. Re-run with --approve to write changes.")
	}
	if result.BlockingReason != "" {
		fmt.Fprintf(w, "Blocked: %s\n", result.BlockingReason)
	}
}
