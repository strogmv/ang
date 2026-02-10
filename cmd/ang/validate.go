package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

func runValidate(args []string) {
	fmt.Println("Validating architecture...")
	projectPath := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectPath = args[0]
	}
	_, _, _, _, _, _, _, _, err := compiler.RunPipeline(projectPath)

	hasErrors := emitDiagnostics(os.Stderr, compiler.LatestDiagnostics)

	if err != nil {
		printStageFailure("Validation FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err)
		os.Exit(1)
	}
	if hasErrors {
		fmt.Println("Validation FAILED due to diagnostic errors.")
		os.Exit(1)
	}
	fmt.Println("Validation SUCCESSFUL.")
}

func emitDiagnostics(w io.Writer, diagnostics []normalizer.Warning) bool {
	hasErrors := false
	for _, d := range diagnostics {
		severity := "WARN"
		if d.Severity != "" {
			severity = strings.ToUpper(d.Severity)
		}
		if severity == "ERROR" {
			hasErrors = true
		}
		if d.Code != "" {
			fmt.Fprintf(w, "⚠️  %s [%s]: %s\n", severity, d.Code, d.Message)
		} else {
			fmt.Fprintf(w, "⚠️  %s: %s\n", severity, d.Message)
		}
		if d.File != "" {
			fmt.Fprintf(w, "   at %s:%d:%d\n", d.File, d.Line, d.Column)
		}
		if d.Hint != "" {
			fmt.Fprintf(w, "   💡 Hint: %s\n", d.Hint)
		}
	}
	return hasErrors
}
