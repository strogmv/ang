package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/strogmv/ang/compiler/expert"
)

func printPPExpertAdvice(w io.Writer, validated ValidatedExpertReport) {
	printPPExpertAdviceWithLabel(w, validated, "")
}

func printPPExpertAdviceWithLabel(w io.Writer, validated ValidatedExpertReport, mode string) {
	label := "Advise"
	if mode == "gate" {
		label = "Gate"
	}
	switch validated.ReportSchema {
	case expert.SchemaV2:
		printPPExpertAdviceV2(w, validated.ReportV2, label)
	default:
		printPPExpertAdviceV1(w, validated.Report, label)
	}
}

func printPPExpertAdviceV1(w io.Writer, report expert.Report, label string) {
	fmt.Fprintf(w, "ANG payment-provider expert: %s\n", report.Goal)
	fmt.Fprintf(w, "Status: %s\n", report.Status)
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "- [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Code, finding.Summary)
	}
	for _, proposal := range report.Proposals {
		fmt.Fprintf(w, "Proposal %s (%s, requires approval)\n", proposal.ID, proposal.Risk)
		for _, change := range proposal.Changes {
			fmt.Fprintf(w, "  - %s %s (%s)\n", change.Op, change.File, change.CUEPath)
		}
	}
	fmt.Fprintf(w, "%s mode: no project files were modified.\n", label)
}

func printPPExpertAdviceV2(w io.Writer, report expert.ReportV2, label string) {
	fmt.Fprintf(w, "ANG payment-provider expert: %s\n", report.Goal)
	fmt.Fprintf(w, "Status: %s\n", report.Status)
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "- [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Code, finding.Summary)
	}
	for _, proposal := range report.Proposals {
		fmt.Fprintf(w, "Proposal %s (%s, requires approval)\n", proposal.ID, proposal.Risk)
		for _, change := range proposal.Changes {
			fmt.Fprintf(w, "  - %s %s/%s (%s)\n", change.Op, change.Target.Kind, change.Target.RelativePath, change.CUEPath)
		}
	}
	fmt.Fprintf(w, "%s mode: no project files were modified.\n", label)
}
