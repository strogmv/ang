package main

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/expert"
)

func expertModeUsesExpert(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "shadow", "advise", "gate":
		return true
	default:
		return false
	}
}

func expertModeFailClosed(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "advise", "gate":
		return true
	default:
		return false
	}
}

func expertGateBlocksBuild(validated ValidatedExpertReport) (bool, string) {
	if expertReportBlocked(validated) {
		return true, "expert report blocked"
	}
	for _, finding := range findingsFromValidated(validated) {
		if isBlockingFinding(finding) {
			return true, fmt.Sprintf("blocking finding %s", finding.Code)
		}
	}
	for _, diagnostic := range diagnosticsFromValidated(validated) {
		if isGateBlockingDiagnostic(diagnostic) {
			return true, fmt.Sprintf("policy diagnostic %s", diagnostic.Code)
		}
	}
	return false, ""
}

func isGateBlockingDiagnostic(diagnostic expert.Diagnostic) bool {
	severity := strings.ToLower(strings.TrimSpace(diagnostic.Severity))
	if severity != "error" && severity != "critical" {
		return false
	}
	code := strings.ToUpper(strings.TrimSpace(diagnostic.Code))
	return code == "EXPERT_POLICY_BLOCK" || strings.HasPrefix(code, "VERIFICATION_FAILED")
}

func findingsFromValidated(validated ValidatedExpertReport) []expert.Finding {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		return validated.ReportV2.Findings
	default:
		return validated.Report.Findings
	}
}

func diagnosticsFromValidated(validated ValidatedExpertReport) []expert.Diagnostic {
	switch validated.ReportSchema {
	case expert.SchemaV2:
		out := make([]expert.Diagnostic, 0, len(validated.ReportV2.Diagnostics))
		for _, diagnostic := range validated.ReportV2.Diagnostics {
			out = append(out, expert.Diagnostic{
				Code:     diagnostic.Code,
				Severity: diagnostic.Severity,
				Message:  diagnostic.Message,
				Origin:   diagnostic.Origin,
				File:     diagnostic.File,
				Line:     diagnostic.Line,
				CUEPath:  diagnostic.CUEPath,
			})
		}
		return out
	default:
		return validated.Report.Diagnostics
	}
}
