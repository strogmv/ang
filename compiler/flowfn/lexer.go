package flowfn

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind string

const (
	tokenEOF             tokenKind = "eof"
	tokenNewline         tokenKind = "newline"
	tokenIdent           tokenKind = "ident"
	tokenString          tokenKind = "string"
	tokenInt             tokenKind = "int"
	tokenBool            tokenKind = "bool"
	tokenLParen          tokenKind = "("
	tokenRParen          tokenKind = ")"
	tokenLBrace          tokenKind = "{"
	tokenRBrace          tokenKind = "}"
	tokenLBracket        tokenKind = "["
	tokenRBracket        tokenKind = "]"
	tokenComma           tokenKind = ","
	tokenColon           tokenKind = ":"
	tokenSemi            tokenKind = ";"
	tokenOperator        tokenKind = "operator"
	tokenKeywordIf       tokenKind = "if"
	tokenKeywordElse     tokenKind = "else"
	tokenKeywordFor      tokenKind = "for"
	tokenKeywordIn       tokenKind = "in"
	tokenKeywordTry      tokenKind = "try"
	tokenKeywordCatch    tokenKind = "catch"
	tokenKeywordFragment tokenKind = "fragment"
	tokenKeywordUse      tokenKind = "use"
)

type token struct {
	Kind    tokenKind
	Literal string
	Pos     Position
	End     int
}

type lexer struct {
	src     string
	runes   []rune
	offsets []int
	idx     int
	line    int
	col     int
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src, runes: []rune(src), line: 1, col: 1}
	l.offsets = make([]int, len(l.runes)+1)
	byteOffset := 0
	for i, r := range l.runes {
		l.offsets[i] = byteOffset
		byteOffset += len(string(r))
	}
	l.offsets[len(l.runes)] = byteOffset

	var toks []token
	for {
		tok, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		toks = append(toks, tok)
		if tok.Kind == tokenEOF {
			return toks, nil
		}
	}
}

func (l *lexer) nextToken() (token, error) {
	for !l.eof() {
		r := l.peek()
		if r == ' ' || r == '\t' || r == '\r' {
			l.advance()
			continue
		}
		if r == '/' && l.peekN(1) == '/' {
			for !l.eof() && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		break
	}
	if l.eof() {
		pos := Position{Offset: len(l.src), Line: l.line, Column: l.col}
		return token{Kind: tokenEOF, Pos: pos, End: len(l.src)}, nil
	}
	startIdx := l.idx
	start := Position{Offset: l.byteOffset(), Line: l.line, Column: l.col}
	r := l.peek()
	switch r {
	case '\n':
		l.advance()
		return token{Kind: tokenNewline, Literal: "\n", Pos: start, End: l.byteOffset()}, nil
	case '(':
		l.advance()
		return token{Kind: tokenLParen, Literal: "(", Pos: start, End: l.byteOffset()}, nil
	case ')':
		l.advance()
		return token{Kind: tokenRParen, Literal: ")", Pos: start, End: l.byteOffset()}, nil
	case '{':
		l.advance()
		return token{Kind: tokenLBrace, Literal: "{", Pos: start, End: l.byteOffset()}, nil
	case '}':
		l.advance()
		return token{Kind: tokenRBrace, Literal: "}", Pos: start, End: l.byteOffset()}, nil
	case '[':
		l.advance()
		return token{Kind: tokenLBracket, Literal: "[", Pos: start, End: l.byteOffset()}, nil
	case ']':
		l.advance()
		return token{Kind: tokenRBracket, Literal: "]", Pos: start, End: l.byteOffset()}, nil
	case ',':
		l.advance()
		return token{Kind: tokenComma, Literal: ",", Pos: start, End: l.byteOffset()}, nil
	case ':':
		l.advance()
		return token{Kind: tokenColon, Literal: ":", Pos: start, End: l.byteOffset()}, nil
	case ';':
		l.advance()
		return token{Kind: tokenSemi, Literal: ";", Pos: start, End: l.byteOffset()}, nil
	case '"', '\'':
		lit, err := l.scanString(r)
		if err != nil {
			return token{}, err
		}
		return token{Kind: tokenString, Literal: lit, Pos: start, End: l.byteOffset()}, nil
	}
	if unicode.IsDigit(r) {
		for !l.eof() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
		lit := l.src[start.Offset:l.byteOffset()]
		return token{Kind: tokenInt, Literal: lit, Pos: start, End: l.byteOffset()}, nil
	}
	if isIdentStart(r) {
		for !l.eof() && isIdentPart(l.peek()) {
			l.advance()
		}
		lit := l.src[start.Offset:l.byteOffset()]
		kind := classifyIdent(lit)
		return token{Kind: kind, Literal: lit, Pos: start, End: l.byteOffset()}, nil
	}
	for !l.eof() {
		r = l.peek()
		if unicode.IsSpace(r) || strings.ContainsRune("(){}[],:;", r) {
			break
		}
		if r == '/' && l.peekN(1) == '/' {
			break
		}
		if isIdentStart(r) || unicode.IsDigit(r) {
			break
		}
		l.advance()
	}
	lit := l.src[l.offsets[startIdx]:l.byteOffset()]
	return token{Kind: tokenOperator, Literal: lit, Pos: start, End: l.byteOffset()}, nil
}

func (l *lexer) scanString(quote rune) (string, error) {
	start := l.idx
	l.advance()
	escaped := false
	for !l.eof() {
		r := l.peek()
		l.advance()
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == quote {
			return l.src[l.offsets[start]:l.byteOffset()], nil
		}
	}
	return "", fmt.Errorf("unterminated string at %d:%d", l.line, l.col)
}

func (l *lexer) eof() bool  { return l.idx >= len(l.runes) }
func (l *lexer) peek() rune { return l.runes[l.idx] }
func (l *lexer) peekN(n int) rune {
	if l.idx+n >= len(l.runes) {
		return 0
	}
	return l.runes[l.idx+n]
}
func (l *lexer) advance() {
	if l.eof() {
		return
	}
	r := l.runes[l.idx]
	l.idx++
	if r == '\n' {
		l.line++
		l.col = 1
		return
	}
	l.col++
}
func (l *lexer) byteOffset() int { return l.offsets[l.idx] }

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || r == '#'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' || r == '#'
}

func classifyIdent(lit string) tokenKind {
	switch lit {
	case "if":
		return tokenKeywordIf
	case "else":
		return tokenKeywordElse
	case "for":
		return tokenKeywordFor
	case "in":
		return tokenKeywordIn
	case "try":
		return tokenKeywordTry
	case "catch":
		return tokenKeywordCatch
	case "fragment":
		return tokenKeywordFragment
	case "use":
		return tokenKeywordUse
	case "true", "false":
		return tokenBool
	default:
		return tokenIdent
	}
}
