package main

import (
	"errors"
	"fmt"
	"strings"

	cueast "cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	cueparser "cuelang.org/go/cue/parser"
)

// In a """ string CUE decodes escapes, so Go inside it must be written with
// \\n for "\n", and "a\"b" cannot be written at all (\" becomes a bare quote).
// A raw #"""…"""# string keeps every byte, so the Go reads as it would in a .go
// file; interpolation is \#(…) there. toRawCUEString rewrites one multi-line
// string into that form without changing its value.

// cueStringSegments holds a string value as CUE evaluates it: decoded text
// fragments with the source text of the interpolated expressions between them.
type cueStringSegments struct {
	texts []string
	exprs []string
}

var errNotMultilineString = errors.New("not a multi-line string")

// decodeCUEStringSegments decodes a """ or #"""…"""# string the way the CUE
// compiler does: QuoteInfo.Unquote takes each fragment without its opening
// marker (the quotes, or the ')' that closes the previous interpolation) and
// with its closing one (the quotes, or the \( that opens the next).
func decodeCUEStringSegments(src []byte, value cueast.Expr) (cueStringSegments, error) {
	var out cueStringSegments
	switch v := value.(type) {
	case *cueast.BasicLit:
		if !strings.HasPrefix(strings.TrimLeft(v.Value, "#"), `"""`) {
			return out, errNotMultilineString
		}
		text, err := literal.Unquote(v.Value)
		if err != nil {
			return out, err
		}
		out.texts = []string{text}
		return out, nil
	case *cueast.Interpolation:
		first, ok1 := v.Elts[0].(*cueast.BasicLit)
		last, ok2 := v.Elts[len(v.Elts)-1].(*cueast.BasicLit)
		if !ok1 || !ok2 || !strings.HasPrefix(strings.TrimLeft(first.Value, "#"), `"""`) {
			return out, errNotMultilineString
		}
		quotes, start, _, err := literal.ParseQuotes(first.Value, last.Value)
		if err != nil {
			return out, err
		}
		for i, elt := range v.Elts {
			if i%2 == 1 {
				s, e := elt.Pos().Offset(), elt.End().Offset()
				if s < 0 || e > len(src) || e < s {
					return out, fmt.Errorf("interpolation %d has no source range", i/2)
				}
				out.exprs = append(out.exprs, string(src[s:e]))
				continue
			}
			lit, ok := elt.(*cueast.BasicLit)
			if !ok {
				return out, fmt.Errorf("fragment %d is %T", i, elt)
			}
			raw := lit.Value
			if i == 0 {
				raw = raw[start:]
			} else {
				raw = raw[1:]
			}
			text, err := quotes.Unquote(raw)
			if err != nil {
				return out, fmt.Errorf("fragment %d: %w", i, err)
			}
			out.texts = append(out.texts, text)
		}
		return out, nil
	}
	return out, errNotMultilineString
}

// toRawCUEString returns literal (a """ string, possibly interpolated) as a
// #"""…"""# string with the same value, indented with indent. ok is false when
// the conversion is not possible or not needed; reason says why when it is
// worth reporting.
func toRawCUEString(src []byte, value cueast.Expr, indent string) (rawLiteral string, ok bool, reason string) {
	start, end := value.Pos().Offset(), value.End().Offset()
	if start < 0 || end > len(src) || end <= start {
		return "", false, ""
	}
	original := string(src[start:end])
	if strings.HasPrefix(original, "#") {
		return "", false, "" // already raw
	}
	segments, err := decodeCUEStringSegments(src, value)
	if err != nil {
		if errors.Is(err, errNotMultilineString) {
			return "", false, ""
		}
		return "", false, "could not decode the string: " + err.Error()
	}
	var body strings.Builder
	for i, text := range segments.texts {
		if strings.Contains(text, `"""#`) || strings.Contains(text, `\#`) {
			return "", false, `the text contains """# or \#, which would end or interpolate a raw string`
		}
		body.WriteString(text)
		if i < len(segments.exprs) {
			body.WriteString(`\#(` + segments.exprs[i] + `)`)
		}
	}
	lines := strings.Split(body.String(), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	rawLiteral = "#\"\"\"\n" + strings.Join(lines, "\n") + "\n" + indent + "\"\"\"#"

	// Prove it: the new literal must decode to the same fragments and
	// expressions as the old one.
	check := "x: " + rawLiteral + "\n"
	file, err := cueparser.ParseFile("raw.cue", check)
	if err != nil {
		return "", false, "the raw string does not parse: " + err.Error()
	}
	field, _ := file.Decls[0].(*cueast.Field)
	if field == nil {
		return "", false, "the raw string does not parse as a field"
	}
	again, err := decodeCUEStringSegments([]byte(check), field.Value)
	if err != nil {
		return "", false, "the raw string does not decode: " + err.Error()
	}
	if strings.Join(again.texts, "\x00") != strings.Join(segments.texts, "\x00") || strings.Join(again.exprs, "\x00") != strings.Join(segments.exprs, "\x00") {
		return "", false, "the raw string would change the value"
	}
	return rawLiteral, true, ""
}
