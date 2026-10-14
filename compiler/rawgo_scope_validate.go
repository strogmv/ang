package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawgo"
)

const (
	// CodeImplResultShadowed: `impls: go:` code declares its own resp or err
	// and then returns naked or defers a function that reads it.
	CodeImplResultShadowed = "IMPL_RESULT_SHADOWED"
	// CodeLogicCallOuterErr: a logic.Call function literal uses err without
	// declaring it.
	CodeLogicCallOuterErr = "LOGIC_CALL_OUTER_ERR"
)

// emitRawGoScopeDiagnostics warns where Go written in CUE depends on a name
// the generated method declares around it (item 11 of
// docs/tech-debt-backlog.md). Every other name is legal and needs no check:
//
//   - `impls: go:` code runs as a block of its own inside the method, whose
//     named results are resp and err. The code may declare its own resp and
//     err; they shadow the results. Two places change meaning then: a naked
//     return in their scope (Go refuses it) and a deferred function literal
//     that reads or sets them (it sees the code's variable, not the returned
//     result). IMPL_RESULT_SHADOWED.
//   - A logic.Call function literal is called inside the generated flow, so
//     a name it does not declare binds to the flow's variables. For err that
//     is never what the author meant: the literal sets the error variable of
//     the generated method. LOGIC_CALL_OUTER_ERR.
func emitRawGoScopeDiagnostics(services []normalizer.Service, opts PipelineOptions) {
	for _, svc := range services {
		for _, m := range svc.Methods {
			op := svc.Name + "." + m.Name
			if m.Impl != nil && strings.TrimSpace(m.Impl.Code) != "" && !m.IsStreaming {
				uses, _ := rawgo.ShadowedResults(m.Impl.Code)
				for _, u := range uses {
					file, line := implCodeLine(m, u.Line)
					msg := fmt.Sprintf("%s: a naked return where the code's own %s (line %d of the code) shadows the operation's result", op, u.Name, u.DeclLine)
					hint := fmt.Sprintf("return the values explicitly: return resp, err (or give the code's variable another name than %s)", u.Name)
					if u.Deferred {
						msg = fmt.Sprintf("%s: the deferred func uses the code's own %s (declared on line %d of the code), not the %s the operation returns", op, u.Name, u.DeclLine, u.Name)
						hint = fmt.Sprintf("give the code's variable another name than %s, so the deferred func sees the operation's result", u.Name)
					}
					recordPipelineDiagnostic(normalizer.Warning{
						Kind: "impl", Code: CodeImplResultShadowed, Severity: "warn",
						Message: msg, Hint: hint, File: file, Line: line,
					}, opts)
				}
			}
			for _, step := range flatFlowSteps(m.Flow) {
				if step.Action != "logic.Call" {
					continue
				}
				if ref, _ := step.Args["funcRef"].(string); strings.TrimSpace(ref) != "" {
					continue
				}
				fn, _ := step.Args["func"].(string)
				fn = strings.TrimSpace(fn)
				if !strings.HasPrefix(strings.TrimLeft(fn, "( \t\n"), "func") {
					continue
				}
				uses, _ := rawgo.UndeclaredUses(fn, "err")
				for _, u := range uses {
					file, line := step.File, step.Line
					if text, ok := step.ArgText["func"]; ok && text.Line > 0 {
						file, line = text.File, text.Line+u.Line-1
					}
					recordPipelineDiagnostic(normalizer.Warning{
						Kind: "flow", Code: CodeLogicCallOuterErr, Severity: "warn",
						Message: op + ": the logic.Call func uses err without declaring it, so it reads and sets the error variable of the generated method",
						Hint:    "declare err in the func (var err error, or x, err := …)",
						File:    file, Line: line,
					}, opts)
				}
			}
		}
	}
}

// implCodeLine is the CUE place of line n (1-based) of an operation's
// `impls: go:` code.
func implCodeLine(m normalizer.Method, n int) (string, int) {
	if m.Impl.CodeText.Line > 0 {
		return m.Impl.CodeText.File, m.Impl.CodeText.Line + n - 1
	}
	if m.Impl.CodeSource != "" {
		return parseSourcePos(m.Impl.CodeSource)
	}
	return parseSourcePos(m.Source)
}
