package main

import (
	goscanner "go/scanner"
	gotoken "go/token"

	"cuelang.org/go/cue/literal"
)

// sameDecodedGoTokens reports whether two CUE multi-line string literals hold
// the same Go once their CUE escapes are decoded: the same tokens with the
// same literal text, whatever the whitespace and comments between them. A
// literal that does not decode, or decodes to Go that does not scan, never
// counts as the same.
func sameDecodedGoTokens(before, after string) bool {
	beforeGo, err := literal.Unquote(before)
	if err != nil {
		return false
	}
	afterGo, err := literal.Unquote(after)
	if err != nil {
		return false
	}
	beforeTokens, ok := scanGoTokens(beforeGo)
	if !ok {
		return false
	}
	afterTokens, ok := scanGoTokens(afterGo)
	if !ok || len(beforeTokens) != len(afterTokens) {
		return false
	}
	for i := range beforeTokens {
		if beforeTokens[i] != afterTokens[i] {
			return false
		}
	}
	return true
}

type goTokenText struct {
	tok gotoken.Token
	lit string
}

func scanGoTokens(src string) ([]goTokenText, bool) {
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	failed := false
	var s goscanner.Scanner
	s.Init(file, []byte(src), func(gotoken.Position, string) { failed = true }, 0)
	var tokens []goTokenText
	for {
		_, tok, lit := s.Scan()
		if tok == gotoken.EOF {
			break
		}
		if tok == gotoken.SEMICOLON {
			// Written ";" and one inserted at a line end are the same token.
			lit = ";"
		}
		tokens = append(tokens, goTokenText{tok: tok, lit: lit})
	}
	return tokens, !failed
}
