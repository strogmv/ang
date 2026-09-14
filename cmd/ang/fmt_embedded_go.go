package main

import (
	"fmt"
	goast "go/ast"
	goformat "go/format"
	goparser "go/parser"
	goscanner "go/scanner"
	gotoken "go/token"
	"sort"
	"strconv"
	"strings"

	cueast "cuelang.org/go/cue/ast"
	cueparser "cuelang.org/go/cue/parser"
	cuetoken "cuelang.org/go/cue/token"
)

// Go written inside CUE ("""…""" strings) was never formatted: cue fmt treats
// it as text, and the generated file is gofmt'ed only after generation. ang
// fmt now formats the fields the generator reads as Go:
//
//   - func: an expression, the logic.Call function literal;
//   - code: statements, a whole method body;
//   - #Definitions holding a function literal, interpolated into other Go as
//     \(#Name)(args).
//
// Only a field whose value is a single multi-line string is touched; a value
// built from several strings and definitions is not Go on its own. CUE
// interpolations are masked with identifiers of the same length while gofmt
// runs. A fragment that does not parse, or whose formatting would add or
// remove lines, is left as it is and reported: generated files carry
// "// Source: file.cue:LINE" comments, so the line count of a block must not
// change.

type embeddedGoSkip struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// embeddedGoLineCountReason marks the common, harmless skip: gofmt would split
// a one-line block such as `if err != nil { return out, err }`.
const embeddedGoLineCountReason = "formatting would change the number of lines, which generated // Source: comments refer to"

type embeddedGoShape int

const (
	embeddedGoExpression embeddedGoShape = iota
	embeddedGoStatements
)

func embeddedGoShapeOf(label string) (embeddedGoShape, bool) {
	switch {
	case label == "func":
		return embeddedGoExpression, true
	case label == "code":
		return embeddedGoStatements, true
	case strings.HasPrefix(label, "#"):
		return embeddedGoExpression, true
	}
	return 0, false
}

// formatEmbeddedGo returns src with its embedded Go formatted, the number of
// fragments it changed and the fragments it had to leave alone. A file that
// does not parse as CUE is returned unchanged; cue fmt reports it.
func formatEmbeddedGo(filename string, src []byte) ([]byte, int, []embeddedGoSkip) {
	file, err := cueparser.ParseFile(filename, src, cueparser.ParseComments)
	if err != nil {
		return src, 0, nil
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	var skips []embeddedGoSkip
	cueast.Walk(file, func(node cueast.Node) bool {
		field, ok := node.(*cueast.Field)
		if !ok {
			return true
		}
		// A string with \( … ) parses as an Interpolation, not a BasicLit;
		// both are one literal in the source.
		var start, end int
		switch value := field.Value.(type) {
		case *cueast.BasicLit:
			if value.Kind != cuetoken.STRING {
				return true
			}
			start = value.Pos().Offset()
			end = start + len(value.Value)
		case *cueast.Interpolation:
			start = value.Pos().Offset()
			end = value.End().Offset()
		default:
			return true
		}
		if start < 0 || end > len(src) || end <= start {
			return true
		}
		literal := string(src[start:end])
		if !strings.HasPrefix(literal, `"""`) || !strings.HasSuffix(literal, `"""`) {
			return true
		}
		label, _, err := cueast.LabelName(field.Label)
		if err != nil {
			return true
		}
		shape, ok := embeddedGoShapeOf(label)
		if !ok {
			return true
		}
		next, reason := formatEmbeddedGoLiteral(literal, shape, strings.HasPrefix(label, "#"))
		switch {
		case reason != "":
			skips = append(skips, embeddedGoSkip{File: filename, Line: field.Value.Pos().Line(), Field: label, Reason: reason})
		case next != literal:
			edits = append(edits, edit{start: start, end: end, text: next})
		}
		return true
	}, nil)

	if len(edits) == 0 {
		return src, 0, skips
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), src...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out, len(edits), skips
}

// formatEmbeddedGoLiteral formats one """ literal. A non-empty reason means
// the fragment was left alone and should be reported. A definition that is
// not a function literal is not Go the generator reads: it is left alone
// silently.
func formatEmbeddedGoLiteral(value string, shape embeddedGoShape, definition bool) (string, string) {
	const quotes = `"""`
	skip := func(reason string) (string, string) {
		if definition {
			return value, ""
		}
		return value, reason
	}
	open := strings.IndexByte(value, '\n')
	closing := strings.LastIndexByte(value, '\n')
	if open != len(quotes) || closing <= open {
		return value, ""
	}
	indent := value[closing+1 : len(value)-len(quotes)]
	if strings.Trim(indent, " \t") != "" {
		return value, ""
	}
	lines := strings.Split(value[open+1:closing], "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, indent) {
			return skip("a line is indented less than the closing quotes")
		}
		lines[i] = line[len(indent):]
	}
	source := strings.Join(lines, "\n")

	masked, holes, ok := maskCueInterpolations(source)
	if !ok {
		return skip("an interpolation could not be masked")
	}
	if definition && !isGoFuncLiteral(masked) {
		return value, ""
	}

	var formatted string
	var reason string
	switch shape {
	case embeddedGoStatements:
		formatted, reason = gofmtStatements(masked)
	default:
		formatted, reason = gofmtExpression(masked)
	}
	if reason != "" {
		if strings.Contains(source, `\`) {
			// gofmt parses the CUE source text; \" there is still an escape,
			// so Go that is valid once decoded can fail here.
			reason += " (as written in CUE, before escapes such as \\\" are decoded)"
		}
		return skip(reason)
	}
	if strings.Count(formatted, "\n") != strings.Count(masked, "\n") {
		return skip(embeddedGoLineCountReason)
	}
	// gofmt saw the CUE source text, where \" and \\ are still escapes; the
	// generator reads the decoded string. Keep the result only if the decoded
	// Go is the same token for token.
	if !sameDecodedGoTokens(quotes+"\n"+masked+"\n"+quotes, quotes+"\n"+formatted+"\n"+quotes) {
		return skip("CUE escapes make the decoded Go read differently; formatting it could change a string")
	}
	for _, hole := range holes {
		if strings.Count(formatted, hole.mask) != 1 {
			return skip("an interpolation did not survive formatting")
		}
		formatted = strings.Replace(formatted, hole.mask, hole.original, 1)
	}

	out := strings.Split(formatted, "\n")
	for i, line := range out {
		if line != "" {
			out[i] = indent + line
		}
	}
	return quotes + "\n" + strings.Join(out, "\n") + "\n" + indent + quotes, ""
}

type cueInterpolationHole struct {
	mask, original string
}

// maskCueInterpolations replaces every \( … ) with an identifier of the same
// length, so gofmt aligns the text exactly as it will be read back.
func maskCueInterpolations(text string) (string, []cueInterpolationHole, bool) {
	var b strings.Builder
	var holes []cueInterpolationHole
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\\' || i+1 >= len(text) {
			b.WriteByte(c)
			continue
		}
		if text[i+1] != '(' {
			// An escape such as \\ or \n: keep both bytes together so \\( is
			// not taken for an interpolation.
			b.WriteByte(c)
			b.WriteByte(text[i+1])
			i++
			continue
		}
		depth := 0
		j := i + 1
		for ; j < len(text); j++ {
			if text[j] == '\n' {
				return "", nil, false
			}
			if text[j] == '(' {
				depth++
			} else if text[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(text) {
			return "", nil, false
		}
		original := text[i : j+1]
		mask := "_z" + strconv.Itoa(len(holes)) + "_"
		if len(mask) > len(original) {
			return "", nil, false
		}
		mask += strings.Repeat("_", len(original)-len(mask))
		holes = append(holes, cueInterpolationHole{mask: mask, original: original})
		b.WriteString(mask)
		i = j
	}
	masked := b.String()
	for _, hole := range holes {
		if strings.Count(masked, hole.mask) != 1 {
			return "", nil, false
		}
	}
	return masked, holes, true
}

func isGoFuncLiteral(text string) bool {
	expr, err := goparser.ParseExpr(text)
	if err != nil {
		return false
	}
	for {
		paren, ok := expr.(*goast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}
	_, ok := expr.(*goast.FuncLit)
	return ok
}

func gofmtExpression(text string) (string, string) {
	const prefix = "package p\n\nvar _ = "
	formatted, err := goformat.Source([]byte(prefix + text + "\n"))
	if err != nil {
		return "", "not a Go expression: " + firstErrorLine(err)
	}
	out := string(formatted)
	if !strings.HasPrefix(out, prefix) {
		return "", "gofmt moved text around the expression"
	}
	return strings.TrimSuffix(out[len(prefix):], "\n"), ""
}

func gofmtStatements(text string) (string, string) {
	const prefix = "package p\n\nfunc _() {\n"
	const suffix = "}\n"
	formatted, err := goformat.Source([]byte(prefix + text + "\n" + suffix))
	if err != nil {
		return "", "not Go statements: " + firstErrorLine(err)
	}
	out := string(formatted)
	if !strings.HasPrefix(out, prefix) || !strings.HasSuffix(out, "\n"+suffix) {
		return "", "gofmt moved text around the statements"
	}
	if spansLinesInsideToken(out) {
		// gofmt indents the body one tab; taking that tab back off would also
		// cut into a raw string or block comment spanning lines.
		return "", "statements hold a raw string or block comment spanning lines"
	}
	body := strings.Split(out[len(prefix):len(out)-len("\n"+suffix)], "\n")
	for i, line := range body {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			return "", "gofmt produced an unexpected indentation"
		}
		body[i] = line[1:]
	}
	return strings.Join(body, "\n"), ""
}

func spansLinesInsideToken(src string) bool {
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s goscanner.Scanner
	s.Init(file, []byte(src), nil, goscanner.ScanComments)
	for {
		_, tok, lit := s.Scan()
		switch tok {
		case gotoken.EOF:
			return false
		case gotoken.STRING, gotoken.COMMENT:
			if strings.Contains(lit, "\n") {
				return true
			}
		}
	}
}

func firstErrorLine(err error) string {
	msg := err.Error()
	if list, ok := err.(goscanner.ErrorList); ok && len(list) > 0 {
		msg = fmt.Sprintf("line %d: %s", list[0].Pos.Line-2, list[0].Msg)
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return msg
}
