package emitter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
)

// implCode is an operation's impls: go: code the way its generated method
// runs it: as a block of its own. Around the block the method declares only
// s, ctx, req (chunks when streaming) and the named results resp and err
// (item 11 of docs/tech-debt-backlog.md). Inside it the code may declare any
// name, err and resp included, which then shadow the results, as a nested
// scope does in hand-written Go. The code is copied verbatim: nothing is
// rewritten (it used to turn `err :=` into `err =`, `l.` into `slog.` and
// `.UserId` into `.UserID`, inside strings too). Each code line keeps the one
// tab the template indents it by, as before, so a multi-line raw string
// keeps its bytes.
func implCode(m normalizer.Method) string {
	if m.Transactional {
		return transactionalImplCode(m)
	}
	return "{\n" + implCodeLines(m) + "\n}"
}

// implCodeLines is the code with a line marker for finalizeLineDirectives,
// so compiler errors, vet and panics name the CUE line of the code.
func implCodeLines(m normalizer.Method) string {
	code := strings.TrimRight(strings.TrimLeft(m.Impl.Code, "\n"), "\n")
	skipped := len(m.Impl.Code) - len(strings.TrimLeft(m.Impl.Code, "\n"))
	file, line := strings.TrimSpace(m.Impl.CodeText.File), m.Impl.CodeText.Line
	if file == "" || line <= 0 {
		return code
	}
	lead := leadingCommentLines(code)
	total := strings.Count(code, "\n") + 1
	if total <= lead {
		return code
	}
	return fmt.Sprintf("%s %s:%d %d\n%s", lineMarkerPrefix, filepath.ToSlash(filepath.Clean(file)), line+skipped+lead, total-lead, code)
}

// transactionalImplCode runs the code in s.txManager.WithTx: a closure with
// the method's named results, inside a transaction that commits when it
// returns a nil error and rolls back otherwise; ctx inside is the
// transaction's. The code is again a block of its own, and the wrapper
// declares nothing the code can see besides the closure's resp and err.
func transactionalImplCode(m normalizer.Method) string {
	respType := "error" // the signature of an operation without output
	eventHandler := len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == ""
	if m.Output.Name != "" && !eventHandler {
		respType = "port." + m.Output.Name
	}
	return fmt.Sprintf(`if s.txManager == nil {
	return resp, fmt.Errorf("transaction manager is not configured")
}
err = s.txManager.WithTx(ctx, func(ctx context.Context) error {
	var err error
	resp, err = func() (resp %s, err error) {
{
%s
}
	}()
	return err
})
return resp, err`, respType, implCodeLines(m))
}
