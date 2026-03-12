package lsp

import (
	"regexp"

	"github.com/strogmv/ang-ir/flowfn"
)

func FlowDiagnostics(text string, streaming bool) []Diagnostic {
	_, diags, err := flowfn.ParseValidateTranspileWithOptions(text, flowfn.ValidateOptions{
		InStreamingMethod: streaming,
	})
	if err != nil {
		if parsed := parseLineColumnError(err.Error()); parsed != nil {
			return []Diagnostic{*parsed}
		}
		return []Diagnostic{{
			Range:    Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 1}},
			Severity: 1,
			Code:     "E_FLOW_PARSE",
			Message:  err.Error(),
			Source:   "ang-flowfn",
		}}
	}
	out := make([]Diagnostic, 0, len(diags))
	for _, d := range diags {
		line := max(d.Line-1, 0)
		col := max(d.Column-1, 0)
		out = append(out, Diagnostic{
			Range:    Range{Start: Position{Line: line, Character: col}, End: Position{Line: line, Character: col + 1}},
			Severity: 1,
			Code:     d.Code,
			Message:  d.Message,
			Source:   "ang-flowfn",
		})
	}
	return out
}

var lineColRe = regexp.MustCompile(`at\s+(\d+):(\d+)`)

func parseLineColumnError(msg string) *Diagnostic {
	m := lineColRe.FindStringSubmatch(msg)
	if len(m) != 3 {
		return nil
	}
	line := atoiSafe(m[1]) - 1
	col := atoiSafe(m[2]) - 1
	return &Diagnostic{
		Range:    Range{Start: Position{Line: max(line, 0), Character: max(col, 0)}, End: Position{Line: max(line, 0), Character: max(col, 0) + 1}},
		Severity: 1,
		Code:     "E_FLOW_PARSE",
		Message:  msg,
		Source:   "ang-flowfn",
	}
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
