package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/expert"
	"github.com/strogmv/ang/compiler/paymentprovider"
	ppfacts "github.com/strogmv/ang/compiler/paymentprovider/facts"
)

type paymentProviderBuildResult struct {
	BuildErr     error
	TestErr      error
	Err          error
	Manifest     paymentprovider.BuildResult
	DryRun       bool
	GoTestStatus string
	GoTestCodes  []string
}

type ppExpertSession struct {
	runID           string
	scopeID         string
	factsBeforeHash string
	validated       ValidatedExpertReport
}

func validateExpertBuildOptions(opts OutputOptions) error {
	if err := validateExpertMode(opts.ExpertMode); err != nil {
		return err
	}
	switch opts.ExpertMode {
	case "shadow", "advise", "gate":
		if strings.TrimSpace(opts.ExpertBaseURL) == "" {
			return fmt.Errorf("%s mode requires --expert-base-url", opts.ExpertMode)
		}
	case "off":
		if strings.TrimSpace(opts.ExpertBaseURL) != "" || len(opts.ExpertPackIDs) > 0 {
			return fmt.Errorf("expert flags require --expert-mode shadow, advise, or gate")
		}
	}
	return nil
}

func runPaymentProviderBuild(projPath, cueRoot, tmplDir, dryRunRoot string, output OutputOptions) paymentProviderBuildResult {
	var session *ppExpertSession
	if expertModeUsesExpert(output.ExpertMode) {
		var expertErr error
		session, expertErr = beginPPExpertSession(projPath, cueRoot, output)
		if expertErr != nil && expertModeFailClosed(output.ExpertMode) {
			return paymentProviderBuildResult{Err: expertErr}
		}
		if output.ExpertMode == "gate" && session != nil && session.validated.ReportSchema != "" {
			if blocked, reason := expertGateBlocksBuild(session.validated); blocked {
				printPPExpertAdviceWithLabel(os.Stderr, session.validated, "gate")
				result := paymentProviderBuildResult{GoTestStatus: "skipped"}
				finalizePPExpertSession(session, projPath, cueRoot, output, result, "skipped")
				return paymentProviderBuildResult{Err: fmt.Errorf("expert gate blocked build: %s", reason)}
			}
		}
	}

	buildOpts := paymentprovider.BuildOptions{
		ProjectPath:  projPath,
		CueRoot:      cueRoot,
		TemplatesDir: tmplDir,
	}
	isDryRun := output.DryRun
	if isDryRun {
		buildOpts.OutputDir = filepath.Join(dryRunRoot, "payment-provider")
	}

	buildResult, buildErr := paymentprovider.BuildWithResult(buildOpts)
	result := paymentProviderBuildResult{
		BuildErr: buildErr,
		Manifest: buildResult,
		DryRun:   isDryRun,
	}

	if buildErr == nil && output.RunTests && !isDryRun {
		status, codes := runPaymentProviderGoTests(projPath)
		result.GoTestStatus = status
		result.GoTestCodes = codes
		if status == "failed" {
			result.TestErr = fmt.Errorf("payment provider go test failed")
		}
	} else {
		result.GoTestStatus = "skipped"
	}

	if result.BuildErr != nil {
		result.Err = result.BuildErr
	} else {
		result.Err = result.TestErr
	}

	if expertModeUsesExpert(output.ExpertMode) {
		buildStatus := "passed"
		if result.BuildErr != nil {
			buildStatus = "failed"
		}
		finalizePPExpertSession(session, projPath, cueRoot, output, result, buildStatus)
		if output.ExpertMode == "advise" || output.ExpertMode == "gate" {
			if session != nil && session.validated.ReportSchema != "" {
				printPPExpertAdviceWithLabel(os.Stderr, session.validated, output.ExpertMode)
			}
		}
		if output.ExpertMode == "advise" && session != nil && session.validated.ReportSchema != "" {
			if expertReportBlocked(session.validated) && result.Err == nil {
				result.Err = fmt.Errorf("expert report blocked")
			}
		}
	}
	return result
}

func beginPPExpertSession(projPath, cueRoot string, output OutputOptions) (*ppExpertSession, error) {
	runID, err := newOutcomeRunID()
	if err != nil {
		if expertModeFailClosed(output.ExpertMode) {
			return nil, fmt.Errorf("expert run id: %w", err)
		}
		expertShadowWarning("EXPERT_RUN_ID_FAILED: %v", err)
		return &ppExpertSession{}, nil
	}
	session := &ppExpertSession{runID: runID}

	envelope, err := ppfacts.Extract(ppfacts.ExtractOptions{
		ProjectPath: projPath,
		CueRoot:     cueRoot,
	})
	if err != nil {
		if expertModeFailClosed(output.ExpertMode) {
			return session, fmt.Errorf("extract facts before expert analyze: %w", err)
		}
		expertShadowWarning("EXPERT_FACTS_BEFORE_FAILED: %v", err)
		return session, nil
	}
	session.scopeID = envelope.ScopeID
	factsJSON, err := ppfacts.CanonicalJSON(envelope)
	if err != nil {
		if expertModeFailClosed(output.ExpertMode) {
			return session, fmt.Errorf("canonicalize facts before expert analyze: %w", err)
		}
		expertShadowWarning("EXPERT_FACTS_BEFORE_FAILED: %v", err)
		return session, nil
	}
	sum := sha256.Sum256(factsJSON)
	session.factsBeforeHash = hex.EncodeToString(sum[:])

	packIDs := output.ExpertPackIDs
	if len(packIDs) == 0 {
		packIDs = []string{"payment-provider.core"}
	}
	requestID, err := newExpertRequestID("pp.build")
	if err != nil {
		if expertModeFailClosed(output.ExpertMode) {
			return session, fmt.Errorf("expert request id: %w", err)
		}
		expertShadowWarning("EXPERT_ANALYZE_FAILED: %v", err)
		return session, nil
	}
	validated, err := Analyze(context.Background(), ExpertClientConfig{
		BaseURL: output.ExpertBaseURL,
		Timeout: 10 * time.Second,
	}, ExpertAnalyzeRequest{
		Schema:          expertRequestSchema,
		RequestID:       requestID,
		Goal:            "payment_provider.audit",
		CompilerVersion: compiler.Version,
		Facts:           factsJSON,
		PackIDs:         packIDs,
	}, session.factsBeforeHash, ExpertAnalyzeScope{
		ProjectPath: projPath,
		CueRoot:     cueRoot,
	})
	if err != nil {
		if expertModeFailClosed(output.ExpertMode) {
			return session, fmt.Errorf("expert analyze: %w", err)
		}
		expertShadowWarning("EXPERT_ANALYZE_FAILED: %v", err)
		return session, nil
	}
	session.validated = validated
	return session, nil
}

func finalizePPExpertSession(session *ppExpertSession, projPath, cueRoot string, output OutputOptions, result paymentProviderBuildResult, buildStatus string) {
	if session == nil {
		return
	}
	if buildStatus == "" {
		buildStatus = "passed"
		if result.BuildErr != nil {
			buildStatus = "failed"
		}
	}
	if session.factsBeforeHash == "" || session.validated.ReportHash == "" || session.scopeID == "" {
		if output.ExpertMode == "shadow" {
			expertShadowWarning("EXPERT_OUTCOME_NOT_RECORDED: missing verified facts/report")
		}
		return
	}

	verification := []ExpertVerification{{Check: "build", Status: buildStatus}}
	if result.GoTestStatus != "" {
		verification = append(verification, ExpertVerification{
			Check:  "go_test",
			Status: result.GoTestStatus,
			Codes:  append([]string(nil), result.GoTestCodes...),
		})
	}
	factsAfterHash := ""
	if result.BuildErr == nil && !result.DryRun {
		if after, err := ppfacts.Extract(ppfacts.ExtractOptions{
			ProjectPath: projPath,
			CueRoot:     cueRoot,
		}); err == nil {
			if hash, err := ppfacts.Hash(after); err == nil {
				factsAfterHash = hash
				verification = append(verification, ExpertVerification{Check: "post_facts", Status: "passed"})
			}
		}
	}

	finalStatus := outcomeFinalStatusFromValidated(buildStatus, result.GoTestStatus, session.validated)
	blockingCodes, unresolvedCodes := findingCodesFromValidated(session.validated)
	outcome := ExpertOutcomeRequest{
		Schema:                 "ang/expert-outcome/v1",
		RunID:                  session.runID,
		ScopeID:                session.scopeID,
		Goal:                   "payment_provider.audit",
		CompilerVersion:        compiler.Version,
		FactsBeforeHash:        session.factsBeforeHash,
		ReportHash:             session.validated.ReportHash,
		KnowledgeVersions:      knowledgeVersionsFromValidated(session.validated),
		ProposalDecisions:      expertProposalDecisions(session.validated, output.ExpertMode),
		Verification:           verification,
		FactsAfterHash:         factsAfterHash,
		BlockingFindingCodes:   blockingCodes,
		UnresolvedFindingCodes: unresolvedCodes,
		FinalStatus:            finalStatus,
	}
	if manifestHash, err := result.Manifest.ManifestHash(); err == nil && manifestHash != "" {
		outcome.OutputManifestHash = manifestHash
	}
	if err := RecordOutcome(context.Background(), ExpertClientConfig{
		BaseURL: output.ExpertBaseURL,
		Timeout: 10 * time.Second,
	}, outcome); err != nil {
		expertShadowWarning("EXPERT_OUTCOME_NOT_RECORDED: %v", err)
	}
}

func outcomeFinalStatusFromValidated(buildStatus, testStatus string, validated ValidatedExpertReport) string {
	if buildStatus == "failed" || testStatus == "failed" {
		return "failed"
	}
	if expertReportBlocked(validated) {
		return "blocked"
	}
	if expertReportHasAdvice(validated) {
		return "advice"
	}
	return "stable"
}

func outcomeFinalStatus(buildStatus, testStatus string, report *expert.Report) string {
	if buildStatus == "failed" || testStatus == "failed" {
		return "failed"
	}
	if report != nil && report.Status == expert.ReportBlocked {
		return "blocked"
	}
	if report != nil && (len(report.Findings) > 0 || len(report.Proposals) > 0) {
		return "advice"
	}
	return "stable"
}

func expertProposalDecisions(validated ValidatedExpertReport, mode string) []ExpertProposalDecision {
	reason := "shadow_mode"
	switch mode {
	case "advise":
		reason = "advise_mode"
	case "gate":
		reason = "gate_mode"
	}
	switch validated.ReportSchema {
	case expert.SchemaV2:
		if len(validated.ReportV2.Proposals) == 0 {
			return nil
		}
		decisions := make([]ExpertProposalDecision, 0, len(validated.ReportV2.Proposals))
		for _, proposal := range validated.ReportV2.Proposals {
			decisions = append(decisions, ExpertProposalDecision{
				ProposalID: proposal.ID,
				Decision:   "not_reviewed",
				ReasonCode: reason,
			})
		}
		return decisions
	default:
		return shadowProposalDecisions(&validated.Report)
	}
}

func shadowProposalDecisions(report *expert.Report) []ExpertProposalDecision {
	if report == nil || len(report.Proposals) == 0 {
		return nil
	}
	decisions := make([]ExpertProposalDecision, 0, len(report.Proposals))
	for _, proposal := range report.Proposals {
		decisions = append(decisions, ExpertProposalDecision{
			ProposalID: proposal.ID,
			Decision:   "not_reviewed",
			ReasonCode: "shadow_mode",
		})
	}
	return decisions
}

func findingCodesFromValidated(validated ValidatedExpertReport) (blocking, unresolved []string) {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		return findingCodesFromFindings(validated.ReportV2.Findings)
	default:
		return findingCodesFromReport(&validated.Report)
	}
}

func findingCodesFromFindings(findings []expert.Finding) (blocking, unresolved []string) {
	blockingSet := map[string]struct{}{}
	unresolvedSet := map[string]struct{}{}
	for _, finding := range findings {
		code := strings.TrimSpace(finding.Code)
		if code == "" {
			continue
		}
		if isBlockingFinding(finding) {
			blockingSet[code] = struct{}{}
			delete(unresolvedSet, code)
			continue
		}
		if _, blocked := blockingSet[code]; !blocked {
			unresolvedSet[code] = struct{}{}
		}
	}
	return sortedFindingCodes(blockingSet), sortedFindingCodes(unresolvedSet)
}

func knowledgeVersionsFromValidated(validated ValidatedExpertReport) []string {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		return append([]string(nil), validated.ReportV2.KnowledgeVersions...)
	default:
		return append([]string(nil), validated.Report.KnowledgeVersions...)
	}
}

func expertReportBlocked(validated ValidatedExpertReport) bool {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		return validated.ReportV2.Status == expert.ReportBlocked
	default:
		return validated.Report.Status == expert.ReportBlocked
	}
}

func expertReportHasAdvice(validated ValidatedExpertReport) bool {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		return len(validated.ReportV2.Findings) > 0 || len(validated.ReportV2.Proposals) > 0
	default:
		return len(validated.Report.Findings) > 0 || len(validated.Report.Proposals) > 0
	}
}

func findingCodesFromReport(report *expert.Report) (blocking, unresolved []string) {
	if report == nil {
		return nil, nil
	}
	return findingCodesFromFindings(report.Findings)
}

func isBlockingFinding(finding expert.Finding) bool {
	if finding.Status == expert.FindingConflict {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(finding.Severity)) {
	case "error", "critical":
		return true
	default:
		return false
	}
}

func sortedFindingCodes(codes map[string]struct{}) []string {
	if len(codes) == 0 {
		return nil
	}
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func expertShadowWarning(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

func newOutcomeRunID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "run-" + hex.EncodeToString(buf), nil
}
