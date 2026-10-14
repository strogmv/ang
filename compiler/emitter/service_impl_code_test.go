package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// Item 11 of docs/tech-debt-backlog.md: impls: go: code is a block of its
// own, copied verbatim. It used to be pasted into the method body and
// rewritten: `err :=` became `err =`, `l.` became `slog.` and `.UserId`
// became `.UserID`, inside string literals too.
func TestImplCodeIsABlockOfItsOwnAndVerbatim(t *testing.T) {
	code := "\n// look the user up\nerr := lookup(req.UserId)\nl := logger{}\nl.Info(\"$.data.userId\", err)\nreturn resp, nil\n"
	m := normalizer.Method{
		Name:   "Rename",
		Output: normalizer.Entity{Name: "RenameResponse"},
		Impl:   &normalizer.MethodImpl{Code: code, CodeText: normalizer.TextPos{File: "cue/api/user.cue", Line: 10}},
	}
	// The marker maps the code from its first line that is not a comment
	// (line 3 of the literal, which starts on line 10) to the CUE file;
	// finalizeLineDirectives skips the comment after it.
	want := "{\n" +
		"//ang:line cue/api/user.cue:12 4\n" +
		"// look the user up\n" +
		"err := lookup(req.UserId)\nl := logger{}\nl.Info(\"$.data.userId\", err)\nreturn resp, nil\n" +
		"}"
	if got := implCode(m); got != want {
		t.Fatalf("implCode:\n%s\nwant:\n%s", got, want)
	}

	m.Transactional = true
	tx := implCode(m)
	for _, wantPart := range []string{
		"err = s.txManager.WithTx(ctx, func(ctx context.Context) error {\n\tvar err error\n\tresp, err = func() (resp port.RenameResponse, err error) {\n{\n",
		"err := lookup(req.UserId)",
		"\n}\n\t}()\n\treturn err\n})\nreturn resp, err",
	} {
		if !strings.Contains(tx, wantPart) {
			t.Fatalf("transactional implCode lacks %q:\n%s", wantPart, tx)
		}
	}
	if strings.Contains(tx, "txErr") {
		t.Fatalf("the wrapper declares a name the code can see:\n%s", tx)
	}
}
