package compiler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawgo"
)

// RawGoImportProblem is one import of a raw Go block (`impls: go:` code or a
// logic.Call lambda) that is listed but not used, or used but not imported.
type RawGoImportProblem struct {
	File      string
	Line      int
	Operation string // Service.Method
	Message   string
}

func (p RawGoImportProblem) String() string {
	loc := p.File
	if p.Line > 0 {
		loc = fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return fmt.Sprintf("%s: %s: %s", loc, p.Operation, p.Message)
}

// RawGoImportError lists every import problem of the project's raw Go.
type RawGoImportError struct {
	Problems []RawGoImportProblem
}

func (e *RawGoImportError) Error() string {
	lines := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		lines = append(lines, p.String())
	}
	return strings.Join(lines, "\n")
}

// validateRawGoImports checks the imports of the Go written in CUE before any
// code is generated (item 4 of docs/tech-debt-backlog.md). A generated method
// file imports the template's base packages plus every `imports:` of its
// service, then goimports drops what is unused and looks up what is missing
// in the module cache. So an unused entry vanished without a word, and a
// missing one compiled only when goimports guessed right: rand.Read became
// math/rand where the code meant crypto/rand.
//
// An `impls: go:` block lists what its code uses beyond the base imports:
//   - an entry the code never uses is an error;
//   - a package the code uses that is neither a base import nor listed is an
//     error when its name is a standard package or a package some `imports:`
//     of the project names. Other names are left to the compiler.
//
// A logic.Call lambda may use the base imports and every import of its
// service, and may list its own in `imports:` on the step (they go into its
// method's file only):
//   - an entry the lambda never uses is an error;
//   - `name.Sel` that two standard packages export (rand.Read: crypto/rand,
//     math/rand) and the service does not import is an error: goimports
//     would pick one. A package goimports finds without a choice is not.
func validateRawGoImports(services []normalizer.Service) error {
	base := map[string]bool{}
	for _, path := range rawgo.ServiceBaseImports("") {
		base[rawgo.PackageName(path)] = true
	}
	// Package names some raw Go block of the project imports, with the paths.
	known := map[string]map[string]bool{}
	addKnown := func(entry string) {
		name, path := importName(entry)
		if known[name] == nil {
			known[name] = map[string]bool{}
		}
		known[name][path] = true
	}
	for _, svc := range services {
		for _, m := range svc.Methods {
			if m.Impl != nil {
				for _, entry := range m.Impl.Imports {
					addKnown(entry)
				}
			}
			for _, step := range flatFlowSteps(m.Flow) {
				for _, entry := range stepImports(step) {
					addKnown(entry)
				}
			}
		}
	}
	candidates := func(name string) []string {
		set := map[string]bool{}
		for _, path := range rawgo.StdlibPaths(name) {
			set[path] = true
		}
		for path := range known[name] {
			set[path] = true
		}
		return quotedSorted(set)
	}

	var problems []RawGoImportProblem
	for _, svc := range services {
		// What every method file of the service imports (see EmitServiceImpl).
		serviceImports := map[string]bool{}
		for name := range base {
			serviceImports[name] = true
		}
		for _, m := range svc.Methods {
			if m.Impl != nil {
				for _, entry := range m.Impl.Imports {
					name, _ := importName(entry)
					serviceImports[name] = true
				}
			}
			for _, step := range flatFlowSteps(m.Flow) {
				imports, _ := step.Args["_funcRefImports"].([]string)
				for _, path := range imports {
					serviceImports[rawgo.PackageName(path)] = true
				}
			}
		}

		for _, m := range svc.Methods {
			op := svc.Name + "." + m.Name
			if m.Impl != nil && m.Impl.CodeSource != "" && strings.TrimSpace(m.Impl.Code) != "" {
				problems = append(problems, checkImplImports(op, m.Impl, base, candidates)...)
			}
			problems = append(problems, checkLambdaImports(op, m, serviceImports)...)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return &RawGoImportError{Problems: problems}
}

func checkImplImports(op string, impl *normalizer.MethodImpl, base map[string]bool, candidates func(string) []string) []RawGoImportProblem {
	refs, ok := rawgo.FuncBodyRefs(impl.Code)
	if !ok {
		return nil // the normalizer and the Go compiler report syntax
	}
	codeFile, codeLine := splitSource(impl.CodeSource)
	used := map[string]bool{}
	for _, r := range refs {
		used[r.Name] = true
	}

	var problems []RawGoImportProblem
	declared := map[string]bool{}
	for i, entry := range impl.Imports {
		name, path := importName(entry)
		declared[name] = true
		if name == "_" || name == "." || used[name] {
			continue
		}
		file, line := codeFile, codeLine
		if i < len(impl.ImportSources) {
			file, line = splitSource(impl.ImportSources[i])
		}
		problems = append(problems, RawGoImportProblem{
			File: file, Line: line, Operation: op,
			Message: fmt.Sprintf("import %q is not used by the code; remove it from imports", path),
		})
	}

	reported := map[string]bool{}
	for _, r := range refs {
		if declared[r.Name] || base[r.Name] || reported[r.Name] {
			continue
		}
		paths := candidates(r.Name)
		if len(paths) == 0 {
			continue
		}
		reported[r.Name] = true
		file, line := codeFile, codeLine
		if impl.CodeText.Line > 0 {
			file, line = impl.CodeText.File, impl.CodeText.Line+r.Line-1
		}
		problems = append(problems, RawGoImportProblem{
			File: file, Line: line, Operation: op,
			Message: fmt.Sprintf("%s.%s uses package %s, which is not in imports; add %s", r.Name, r.Sel, r.Name, strings.Join(paths, " or ")),
		})
	}
	return problems
}

func checkLambdaImports(op string, m normalizer.Method, serviceImports map[string]bool) []RawGoImportProblem {
	// imports: of a logic.Call stays in its method's file (emitter
	// methodImports), so the method's steps add to the service's list.
	available := map[string]bool{}
	for name := range serviceImports {
		available[name] = true
	}
	for _, step := range flatFlowSteps(m.Flow) {
		for _, entry := range stepImports(step) {
			name, _ := importName(entry)
			available[name] = true
		}
	}
	var problems []RawGoImportProblem
	for _, step := range flatFlowSteps(m.Flow) {
		if step.Action != "logic.Call" {
			continue
		}
		if ref, _ := step.Args["funcRef"].(string); strings.TrimSpace(ref) != "" {
			continue // a sidecar's imports are read by the Go parser
		}
		fn, _ := step.Args["func"].(string)
		fn = strings.TrimSpace(fn)
		own := stepImports(step)
		if !strings.HasPrefix(strings.TrimLeft(fn, "( \t\n"), "func") {
			if len(own) > 0 {
				problems = append(problems, RawGoImportProblem{
					File: step.File, Line: step.Line, Operation: op,
					Message: "imports: on a logic.Call whose func is not a function literal; remove it",
				})
			}
			continue
		}
		refs, ok := rawgo.FuncLiteralRefs(fn)
		if !ok {
			continue
		}
		text, hasText := step.ArgText["func"]
		used := map[string]bool{}
		for _, r := range refs {
			used[r.Name] = true
		}
		for _, entry := range own {
			name, path := importName(entry)
			if name == "_" || name == "." || used[name] {
				continue
			}
			problems = append(problems, RawGoImportProblem{
				File: step.File, Line: step.Line, Operation: op,
				Message: fmt.Sprintf("logic.Call imports %q, which the func does not use; remove it from imports", path),
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
			file, line := step.File, step.Line
			if hasText && text.Line > 0 {
				file, line = text.File, text.Line+r.Line-1
			}
			set := map[string]bool{}
			for _, p := range paths {
				set[p] = true
			}
			problems = append(problems, RawGoImportProblem{
				File: file, Line: line, Operation: op,
				Message: fmt.Sprintf("%s.%s exists in %s and nothing imports %s, so goimports would pick one; add imports: [\"<the one you mean>\"] to this logic.Call",
					r.Name, r.Sel, strings.Join(quotedSorted(set), " and "), r.Name),
			})
		}
	}
	return problems
}

// stepImports reads `imports:` of a flow step (a list or one string).
func stepImports(step normalizer.FlowStep) []string {
	switch v := step.Args["imports"].(type) {
	case []string:
		return v
	case string:
		if strings.TrimSpace(v) != "" {
			return []string{v}
		}
	}
	return nil
}

func importName(entry string) (name, path string) {
	alias, path := rawgo.NormalizeImport(entry)
	if alias != "" {
		return alias, path
	}
	return rawgo.PackageName(path), path
}

func quotedSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, strconv.Quote(path))
	}
	sort.Strings(out)
	return out
}

// flatFlowSteps returns the steps of a flow and of every nested branch.
func flatFlowSteps(steps []normalizer.FlowStep) []normalizer.FlowStep {
	var out []normalizer.FlowStep
	var visit func([]normalizer.FlowStep)
	visit = func(steps []normalizer.FlowStep) {
		for _, step := range steps {
			out = append(out, step)
			for _, key := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if sub, ok := step.Args[key].([]normalizer.FlowStep); ok {
					visit(sub)
				}
			}
			for _, key := range []string{"_cases", "_branches"} {
				if groups, ok := step.Args[key].(map[string][]normalizer.FlowStep); ok {
					keys := make([]string, 0, len(groups))
					for k := range groups {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						visit(groups[k])
					}
				}
			}
		}
	}
	visit(steps)
	return out
}

// splitSource splits "file:line" (normalizer.formatPos).
func splitSource(source string) (string, int) {
	i := strings.LastIndex(source, ":")
	if i < 0 {
		return source, 0
	}
	line, err := strconv.Atoi(source[i+1:])
	if err != nil {
		return source, 0
	}
	return source[:i], line
}
