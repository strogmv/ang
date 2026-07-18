package expert

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// AuditInput contains only already-computed compiler observations. Audit does
// not read CUE or the filesystem; that keeps it deterministic and testable.
type AuditInput struct {
	Goal            string
	CompilerVersion string
	FactsHash       string
	Diagnostics     []normalizer.Warning
	PipelineError   error
}

// Audit adapts existing compiler diagnostics into an audit-only expert report.
// It creates no rules and no proposals.
func Audit(input AuditInput) Report {
	goal := strings.TrimSpace(input.Goal)
	if goal == "" {
		goal = "project.audit"
	}
	report := Report{
		Schema:            SchemaV1,
		Goal:              goal,
		Status:            ReportNoChange,
		CompilerVersion:   input.CompilerVersion,
		FactsHash:         input.FactsHash,
		KnowledgeVersions: []string{},
		Findings:          []Finding{},
		Proposals:         []Proposal{},
		Trace:             []RuleTrace{},
		Verification:      []VerificationResult{},
		Diagnostics:       []Diagnostic{},
	}

	hasError := false
	seenFindings := make(map[string]struct{}, len(input.Diagnostics))
	for _, warning := range input.Diagnostics {
		severity := normalizedSeverity(warning.Severity)
		if severity == "error" {
			hasError = true
		}
		findingID := compilerDiagnosticID(warning)
		if _, exists := seenFindings[findingID]; exists {
			continue
		}
		seenFindings[findingID] = struct{}{}
		diagnostic := Diagnostic{
			Code:     fallback(warning.Code, "ANG_DIAGNOSTIC"),
			Severity: severity,
			Message:  fallback(warning.Message, "compiler diagnostic"),
			Origin:   "compiler",
			File:     warning.File,
			Line:     warning.Line,
			CUEPath:  warning.CUEPath,
		}
		report.Diagnostics = append(report.Diagnostics, diagnostic)
		report.Findings = append(report.Findings, Finding{
			ID:         findingID,
			Code:       diagnostic.Code,
			Severity:   diagnostic.Severity,
			Summary:    diagnostic.Message,
			Origin:     "compiler",
			Confidence: 1,
			Status:     FindingConfirmed,
		})
		report.Trace = append(report.Trace, RuleTrace{
			Origin: "compiler", RuleID: "compiler.diagnostic." + diagnostic.Code,
			Result: "matched", ProducedIDs: []string{findingID},
		})
	}
	if input.PipelineError != nil {
		report.Status = ReportFailed
		report.Diagnostics = append(report.Diagnostics, Diagnostic{
			Code: "COMPILER_SEMANTIC_PHASE_FAILED", Severity: "error", Message: input.PipelineError.Error(), Origin: "compiler",
		})
	} else if hasError {
		report.Status = ReportBlocked
	} else {
		report.Status = ReconcileReportStatus(report.Status, report.Findings)
	}
	canonical, err := Canonicalize(report)
	if err != nil {
		// Audit creates no RawMessage values, so this is an invariant failure.
		panic("expert: canonicalize audit report: " + err.Error())
	}
	return canonical
}

func compilerDiagnosticID(warning normalizer.Warning) string {
	// Deliberately exclude File: absolute developer paths must not affect an ID.
	content := strings.Join([]string{
		"compiler", warning.Code, normalizedSeverity(warning.Severity), warning.Op,
		warning.Action, warning.CUEPath, warning.Message,
	}, "\x00")
	sum := sha256.Sum256([]byte(content))
	return "finding.compiler." + hex.EncodeToString(sum[:12])
}

func normalizedSeverity(severity string) string {
	severity = strings.ToLower(strings.TrimSpace(severity))
	if severity == "" {
		return "error"
	}
	return severity
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
