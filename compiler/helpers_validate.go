package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawgo"
)

// emitUnusedHelperDiagnostics warns about helpers no operation reaches. A
// func, type, var or const helper is still emitted (and compiled); a method
// helper is emitted only for the services that call it, so an unused one is
// never compiled at all.
func emitUnusedHelperDiagnostics(unused []normalizer.Helper, opts PipelineOptions) {
	for _, h := range unused {
		file, line := parseSourcePos(h.Source)
		msg := fmt.Sprintf("helper %s is not called by any operation", h.Name)
		if h.Kind == normalizer.HelperMethod {
			msg += "; a method helper no service calls is not generated, so not even compiled"
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "helper",
			Code:     "HELPER_UNUSED",
			Severity: "warn",
			Message:  msg,
			File:     file,
			Line:     line,
			Hint:     "call it from an operation's Go, or remove it from Helpers",
		}, opts)
	}
}

// validateHelperImports checks a helper's imports the way validateRawGoImports
// checks a logic.Call lambda's: a helper's file imports the service base
// packages and its own imports:, and goimports resolves the rest.
//   - an entry the code never uses is an error;
//   - `name.Sel` that two standard packages export (rand.Read: crypto/rand,
//     math/rand) is an error unless the helper imports name: goimports would
//     pick one.
func validateHelperImports(helpers []normalizer.Helper) error {
	base := map[string]bool{}
	for _, path := range rawgo.ServiceBaseImports("") {
		base[rawgo.PackageName(path)] = true
	}
	var problems []RawGoImportProblem
	for _, h := range helpers {
		refs, ok := rawgo.DeclRefs(h.Code)
		if !ok {
			continue // ExtractHelpers reported the syntax
		}
		file, line := splitSource(h.Source)
		at := func(refLine int) (string, int) {
			if h.CodeText.Line > 0 {
				return h.CodeText.File, h.CodeText.Line + refLine - 1
			}
			return file, line
		}
		op := "helper " + h.Name
		used := map[string]bool{}
		for _, r := range refs {
			used[r.Name] = true
		}
		available := map[string]bool{}
		for name := range base {
			available[name] = true
		}
		for _, entry := range h.Imports {
			name, path := importName(entry)
			available[name] = true
			if name == "_" || name == "." || used[name] {
				continue
			}
			problems = append(problems, RawGoImportProblem{
				File: file, Line: line, Operation: op,
				Message: fmt.Sprintf("import %q is not used by the code; remove it from imports", path),
			})
		}
		reported := map[string]bool{}
		for _, r := range refs {
			if available[r.Name] || reported[r.Name+"."+r.Sel] {
				continue
			}
			paths := rawgo.GuessedStdlib(r.Name, r.Sel)
			if len(paths) == 0 {
				continue
			}
			reported[r.Name+"."+r.Sel] = true
			set := map[string]bool{}
			for _, p := range paths {
				set[p] = true
			}
			f, l := at(r.Line)
			problems = append(problems, RawGoImportProblem{
				File: f, Line: l, Operation: op,
				Message: fmt.Sprintf("%s.%s exists in %s and nothing imports %s, so goimports would pick one; add imports: [\"<the one you mean>\"] to the helper",
					r.Name, r.Sel, strings.Join(quotedSorted(set), " and "), r.Name),
			})
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return &RawGoImportError{Problems: problems}
}
