// Package sqltext reads the hand-written SQL of a repository finder well
// enough to check it against the Go ANG generates around it: which positional
// parameters ($1, $2, …) it uses and which columns its result has, in order.
//
// It is a lexer for the PostgreSQL dialect plus a walk over the top level of
// one statement, not a parser: string literals ('…', E'…'), quoted
// identifiers ("…"), dollar-quoted bodies ($tag$…$tag$), line and nested
// block comments are skipped, so a "$1" or a comma inside them never counts.
// Whatever it cannot read with certainty it reports as unknown, and the
// callers then check nothing rather than guess.
package sqltext

import (
	"fmt"
	"sort"
	"strings"
)

// Kind is the kind of a token.
type Kind int

const (
	Ident  Kind = iota // unquoted identifier or keyword (Text is lower case)
	QIdent             // "quoted identifier" (Text is the name, quotes removed)
	String             // string literal of any form
	Number             // numeric literal
	Param              // positional parameter $n (Param holds n)
	Punct              // one of ( ) [ ] , ; . :
	Op                 // operator: a run of + - * / < > = ~ ! @ # % ^ & | ` ? or ::
)

// Token is one lexical token of a statement.
type Token struct {
	Kind  Kind
	Text  string
	Start int // byte offset of the first byte in the source
	End   int // byte offset after the last byte
	Param int
}

// Tokens splits sql into tokens, skipping whitespace and comments.
func Tokens(sql string) ([]Token, error) {
	var out []Token
	i, n := 0, len(sql)
	for i < n {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			end, err := skipBlockComment(sql, i)
			if err != nil {
				return nil, err
			}
			i = end
		case c == '\'':
			end, err := skipString(sql, i, false)
			if err != nil {
				return nil, err
			}
			out = append(out, Token{Kind: String, Text: sql[i:end], Start: i, End: end})
			i = end
		case c == '"':
			end, name, err := readQuotedIdent(sql, i)
			if err != nil {
				return nil, err
			}
			out = append(out, Token{Kind: QIdent, Text: name, Start: i, End: end})
			i = end
		case c == '$':
			if i+1 < n && isDigit(sql[i+1]) {
				j := i + 1
				num := 0
				for j < n && isDigit(sql[j]) {
					num = num*10 + int(sql[j]-'0')
					j++
				}
				out = append(out, Token{Kind: Param, Text: sql[i:j], Start: i, End: j, Param: num})
				i = j
				continue
			}
			tagEnd := i + 1
			for tagEnd < n && isIdentPart(sql[tagEnd]) && sql[tagEnd] != '$' {
				tagEnd++
			}
			if tagEnd < n && sql[tagEnd] == '$' && (tagEnd == i+1 || isIdentStart(sql[i+1])) {
				tag := sql[i : tagEnd+1]
				closeAt := strings.Index(sql[tagEnd+1:], tag)
				if closeAt < 0 {
					return nil, fmt.Errorf("unterminated dollar-quoted string %s at offset %d", tag, i)
				}
				end := tagEnd + 1 + closeAt + len(tag)
				out = append(out, Token{Kind: String, Text: sql[i:end], Start: i, End: end})
				i = end
				continue
			}
			out = append(out, Token{Kind: Op, Text: "$", Start: i, End: i + 1})
			i++
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(sql[i+1])):
			j := i
			for j < n && (isDigit(sql[j]) || sql[j] == '.' || sql[j] == '_') {
				j++
			}
			if j < n && (sql[j] == 'e' || sql[j] == 'E') {
				k := j + 1
				if k < n && (sql[k] == '+' || sql[k] == '-') {
					k++
				}
				if k < n && isDigit(sql[k]) {
					j = k
					for j < n && isDigit(sql[j]) {
						j++
					}
				}
			}
			out = append(out, Token{Kind: Number, Text: sql[i:j], Start: i, End: j})
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentPart(sql[j]) {
				j++
			}
			word := sql[i:j]
			// E'…', B'…', X'…', N'…' and U&'…' are string literals.
			if j < n && sql[j] == '\'' && len(word) == 1 && strings.ContainsAny(word, "eEbBxXnN") {
				end, err := skipString(sql, j, word == "e" || word == "E")
				if err != nil {
					return nil, err
				}
				out = append(out, Token{Kind: String, Text: sql[i:end], Start: i, End: end})
				i = end
				continue
			}
			out = append(out, Token{Kind: Ident, Text: strings.ToLower(word), Start: i, End: j})
			i = j
		case c == ':' && i+1 < n && sql[i+1] == ':':
			out = append(out, Token{Kind: Op, Text: "::", Start: i, End: i + 2})
			i += 2
		case strings.IndexByte("()[],;.:", c) >= 0:
			out = append(out, Token{Kind: Punct, Text: string(c), Start: i, End: i + 1})
			i++
		case strings.IndexByte("+-*/<>=~!@#%^&|`?", c) >= 0:
			j := i + 1
			for j < n && strings.IndexByte("+-*/<>=~!@#%^&|`?", sql[j]) >= 0 {
				if (sql[j] == '-' && j+1 < n && sql[j+1] == '-') || (sql[j] == '/' && j+1 < n && sql[j+1] == '*') {
					break
				}
				j++
			}
			out = append(out, Token{Kind: Op, Text: sql[i:j], Start: i, End: j})
			i = j
		default:
			// Any other byte (non-ASCII letters are handled as identifiers above).
			out = append(out, Token{Kind: Op, Text: string(c), Start: i, End: i + 1})
			i++
		}
	}
	return out, nil
}

func skipBlockComment(sql string, start int) (int, error) {
	depth := 0
	i := start
	for i < len(sql) {
		if sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			depth++
			i += 2
			continue
		}
		if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
			depth--
			i += 2
			if depth == 0 {
				return i, nil
			}
			continue
		}
		i++
	}
	return 0, fmt.Errorf("unterminated /* comment at offset %d", start)
}

// skipString returns the offset after the string literal opening at start.
func skipString(sql string, start int, backslashEscapes bool) (int, error) {
	i := start + 1
	for i < len(sql) {
		switch sql[i] {
		case '\\':
			if backslashEscapes {
				i += 2
				continue
			}
		case '\'':
			if i+1 < len(sql) && sql[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1, nil
		}
		i++
	}
	return 0, fmt.Errorf("unterminated string literal at offset %d", start)
}

func readQuotedIdent(sql string, start int) (int, string, error) {
	var b strings.Builder
	i := start + 1
	for i < len(sql) {
		if sql[i] == '"' {
			if i+1 < len(sql) && sql[i+1] == '"' {
				b.WriteByte('"')
				i += 2
				continue
			}
			return i + 1, b.String(), nil
		}
		b.WriteByte(sql[i])
		i++
	}
	return 0, "", fmt.Errorf("unterminated quoted identifier at offset %d", start)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || isDigit(c) || c == '$' }

// Placeholders returns the distinct positional parameters ($n) of sql in
// ascending order.
func Placeholders(sql string) ([]int, error) {
	toks, err := Tokens(sql)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var out []int
	for _, t := range toks {
		if t.Kind == Param && !seen[t.Param] {
			seen[t.Param] = true
			out = append(out, t.Param)
		}
	}
	sort.Ints(out)
	return out, nil
}

// Column is one item of a result column list.
type Column struct {
	Text  string // the item as written
	Start int    // byte offsets of the item in the source
	End   int
	// Name is the output name when it can be read: the alias (AS x, or a
	// trailing x after a plain column) or the column of a plain reference
	// (t.col, col::type). Empty for any other expression.
	Name string
	// Plain is set for a plain column reference, optionally qualified and
	// cast: col, t.col, col::text, t.col::text AS x.
	Plain bool
	// Cast is the type of a trailing ::type of a plain reference, lower case.
	Cast string
	// Star is set for * or qualifier.*; Qualifier holds the qualifier.
	Star      bool
	Qualifier string
}

// ResultColumns returns the columns a statement returns: the select list of
// its top-level SELECT (after any WITH), or the RETURNING list of a top-level
// INSERT, UPDATE or DELETE. ok is false when the list cannot be read with
// certainty (a statement that starts with a parenthesis, no RETURNING, a
// lexing error); the caller then checks nothing.
func ResultColumns(sql string) (cols []Column, ok bool) {
	toks, err := Tokens(sql)
	if err != nil {
		return nil, false
	}
	start, end, found := resultListSpan(toks)
	if !found || start >= end {
		return nil, false
	}
	for _, item := range splitTopLevel(toks[start:end]) {
		if len(item) == 0 {
			return nil, false
		}
		cols = append(cols, describeColumn(sql, item))
	}
	return cols, true
}

// resultListSpan finds the token range [start, end) of the result list.
func resultListSpan(toks []Token) (int, int, bool) {
	depth := 0
	main := -1
	for i, t := range toks {
		if t.Kind == Punct && (t.Text == "(" || t.Text == "[") {
			depth++
			continue
		}
		if t.Kind == Punct && (t.Text == ")" || t.Text == "]") {
			depth--
			continue
		}
		if depth != 0 || t.Kind != Ident {
			continue
		}
		switch t.Text {
		case "select", "insert", "update", "delete":
			main = i
		case "with":
			if i == 0 {
				continue
			}
		default:
			if i == 0 {
				// A statement that starts with VALUES, TABLE, a parenthesis…
				return 0, 0, false
			}
			continue
		}
		break
	}
	if main < 0 {
		return 0, 0, false
	}
	if toks[main].Text == "select" {
		start := main + 1
		if start < len(toks) && toks[start].Kind == Ident && toks[start].Text == "all" {
			start++
		} else if start < len(toks) && toks[start].Kind == Ident && toks[start].Text == "distinct" {
			start++
			if start < len(toks) && toks[start].Kind == Ident && toks[start].Text == "on" {
				start++
				if start >= len(toks) || toks[start].Text != "(" {
					return 0, 0, false
				}
				d := 0
				for start < len(toks) {
					if toks[start].Kind == Punct && toks[start].Text == "(" {
						d++
					} else if toks[start].Kind == Punct && toks[start].Text == ")" {
						d--
						if d == 0 {
							start++
							break
						}
					}
					start++
				}
			}
		}
		end := scanUntil(toks, start, map[string]bool{
			"from": true, "into": true, "where": true, "group": true, "having": true, "window": true,
			"order": true, "limit": true, "offset": true, "fetch": true, "for": true,
			"union": true, "intersect": true, "except": true,
		})
		return start, end, true
	}
	// INSERT / UPDATE / DELETE: the RETURNING list, if any.
	depth = 0
	for i := main + 1; i < len(toks); i++ {
		t := toks[i]
		if t.Kind == Punct && (t.Text == "(" || t.Text == "[") {
			depth++
			continue
		}
		if t.Kind == Punct && (t.Text == ")" || t.Text == "]") {
			depth--
			continue
		}
		if depth == 0 && t.Kind == Ident && t.Text == "returning" {
			return i + 1, scanUntil(toks, i+1, nil), true
		}
	}
	return 0, 0, false
}

// scanUntil returns the index of the first top-level token at or after start
// that is ";" or one of the stop keywords, or len(toks).
func scanUntil(toks []Token, start int, stop map[string]bool) int {
	depth := 0
	for i := start; i < len(toks); i++ {
		t := toks[i]
		if t.Kind == Punct {
			switch t.Text {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
				if depth < 0 {
					return i
				}
			case ";":
				if depth == 0 {
					return i
				}
			}
			continue
		}
		if depth == 0 && t.Kind == Ident && stop[t.Text] {
			// a IS [NOT] DISTINCT FROM b is an expression, not the FROM clause.
			if t.Text == "from" && i > start && toks[i-1].Kind == Ident && toks[i-1].Text == "distinct" {
				continue
			}
			return i
		}
	}
	return len(toks)
}

func splitTopLevel(toks []Token) [][]Token {
	var out [][]Token
	depth := 0
	cur := []Token{}
	for _, t := range toks {
		if t.Kind == Punct {
			switch t.Text {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			case ",":
				if depth == 0 {
					out = append(out, cur)
					cur = []Token{}
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	return append(out, cur)
}

func describeColumn(sql string, item []Token) Column {
	col := Column{
		Start: item[0].Start,
		End:   item[len(item)-1].End,
	}
	col.Text = sql[col.Start:col.End]
	isName := func(t Token) bool { return t.Kind == Ident || t.Kind == QIdent }

	// * and qualifier.*
	if len(item) == 1 && item[0].Kind == Op && item[0].Text == "*" {
		col.Star = true
		return col
	}
	if len(item) == 3 && isName(item[0]) && item[1].Text == "." && item[2].Kind == Op && item[2].Text == "*" {
		col.Star = true
		col.Qualifier = item[0].Text
		return col
	}

	// Explicit alias: … AS name.
	body := item
	alias := ""
	if n := len(item); n >= 3 && isName(item[n-1]) && item[n-2].Kind == Ident && item[n-2].Text == "as" {
		alias = item[n-1].Text
		body = item[:n-2]
	}

	// A plain reference: name(.name)* [::type [(n[,m])] [[]]] [alias].
	i := 0
	last := ""
	if i < len(body) && isName(body[i]) {
		last = body[i].Text
		i++
		for i+1 < len(body) && body[i].Text == "." && isName(body[i+1]) {
			last = body[i+1].Text
			i += 2
		}
		cast := ""
		if i < len(body) && body[i].Kind == Op && body[i].Text == "::" {
			j := i + 1
			var parts []string
			for j < len(body) && body[j].Kind == Ident && isTypeWord(body, j, len(parts)) {
				parts = append(parts, body[j].Text)
				j++
			}
			if len(parts) > 0 {
				if j < len(body) && body[j].Text == "(" {
					k := j + 1
					for k < len(body) && body[k].Text != ")" {
						k++
					}
					if k < len(body) {
						j = k + 1
					}
				}
				for j+1 < len(body) && body[j].Text == "[" && body[j+1].Text == "]" {
					parts[len(parts)-1] += "[]"
					j += 2
				}
				cast = strings.Join(parts, " ")
				i = j
			}
		}
		// A trailing bare word after a plain reference is an implicit alias.
		if alias == "" && i == len(body)-1 && isName(body[i]) && !reservedAfterColumn[body[i].Text] {
			alias = body[i].Text
			i++
		}
		if i == len(body) {
			col.Plain = true
			col.Cast = cast
			col.Name = last
		}
	}
	if alias != "" {
		col.Name = alias
	}
	return col
}

// isTypeWord accepts the words of a multi-word type name after ::
// (double precision, timestamp with time zone, character varying …).
func isTypeWord(body []Token, j, seen int) bool {
	if seen == 0 {
		return true
	}
	switch body[j].Text {
	case "precision", "varying", "with", "without", "time", "zone":
		return true
	}
	return false
}

var reservedAfterColumn = map[string]bool{
	"from": true, "as": true, "and": true, "or": true, "is": true, "not": true, "in": true,
	"like": true, "ilike": true, "between": true, "end": true, "null": true, "collate": true,
}

// QuoteIdent quotes an identifier when it is not a plain lower-case name.
func QuoteIdent(name string) string {
	plain := name != ""
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c == '_' || (c >= 'a' && c <= 'z') || (i > 0 && isDigit(c))) {
			plain = false
			break
		}
	}
	if plain {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
