package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func emitFSMIntegrityDiagnostics(entities []normalizer.Entity, opts PipelineOptions) {
	seen := map[string]struct{}{}
	for _, e := range entities {
		if e.FSM == nil {
			continue
		}

		stateSet := make(map[string]struct{}, len(e.FSM.States))
		for _, s := range e.FSM.States {
			key := strings.TrimSpace(s)
			if key == "" {
				continue
			}
			stateSet[key] = struct{}{}
		}
		if len(stateSet) == 0 {
			continue
		}

		for from, toStates := range e.FSM.Transitions {
			fromState := strings.TrimSpace(from)
			if fromState != "" {
				if _, ok := stateSet[fromState]; !ok {
					file, line := parseSourcePos(e.Source)
					toPreview := ""
					if len(toStates) > 0 {
						toPreview = strings.TrimSpace(toStates[0])
					}
					diag := normalizer.Warning{
						Kind:     "architecture",
						Code:     "E_FSM_UNDEFINED_STATE",
						Severity: "error",
						Message: fmt.Sprintf(
							"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
							e.Name, fromState, toPreview, fromState,
						),
						File: file,
						Line: line,
						Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition source.", fromState),
					}
					key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						recordPipelineDiagnostic(diag, opts)
					}
				}
			}
			for _, to := range toStates {
				state := strings.TrimSpace(to)
				if state == "" {
					continue
				}
				if _, ok := stateSet[state]; ok {
					continue
				}
				file, line := parseSourcePos(e.Source)
				diag := normalizer.Warning{
					Kind:     "architecture",
					Code:     "E_FSM_UNDEFINED_STATE",
					Severity: "error",
					Message: fmt.Sprintf(
						"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
						e.Name, from, state, state,
					),
					File: file,
					Line: line,
					Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition target.", state),
				}
				key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					recordPipelineDiagnostic(diag, opts)
				}
			}
		}
	}
}

func recordPipelineDiagnostic(diag normalizer.Warning, opts PipelineOptions) {
	if opts.WarningSink != nil {
		opts.WarningSink(diag)
		return
	}
	LatestDiagnostics = append(LatestDiagnostics, diag)
}

func emitFileSizeDiagnostics(path string, opts PipelineOptions) {
	const lineLimit = 300
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	for _, filePath := range matches {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lineCount := strings.Count(string(content), "\n") + 1
		if lineCount > lineLimit {
			diag := normalizer.Warning{
				Kind:     "file-size",
				Code:     "LARGE_CUE_FILE",
				Severity: "warn",
				Message:  fmt.Sprintf("CUE file has %d lines (recommended limit: %d)", lineCount, lineLimit),
				File:     filePath,
				Line:     1,
				Hint:     "Split into multiple files in the same directory (CUE merges files with same package automatically)",
			}
			recordPipelineDiagnostic(diag, opts)
		}
	}
}
