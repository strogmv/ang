package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler"
)

func formatStageFailure(prefix string, stage compiler.Stage, code, op string, err error) string {
	return fmt.Sprintf("%s: %v", prefix, compiler.WrapContractError(stage, code, op, err))
}

func printStageFailure(prefix string, stage compiler.Stage, code, op string, err error) {
	if !stdoutIsTerminal() {
		ev := buildEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Stage:     string(stage),
			Status:    "error",
			Code:      code,
			DocsURL:   compiler.DiagnosticDocsURL(code),
			Message:   op,
			Error:     err.Error(),
		}
		var selectorErr *compiler.DTOSelectorError
		if errors.As(err, &selectorErr) {
			ev.CUEFile = selectorErr.File
			ev.Line = selectorErr.Line
			if selectorErr.Suggestion != "" {
				ev.SuggestedFix = []normalizer.Fix{{
					Op: "replace", File: selectorErr.File,
					Before: selectorErr.Field, After: selectorErr.Suggestion,
					Rationale: "replace unknown DTO field with the closest generated Go field",
				}}
			}
		}
		emitBuildEvent(ev)
		return
	}
	fmt.Println(formatStageFailure(prefix, stage, code, op, err))
}
