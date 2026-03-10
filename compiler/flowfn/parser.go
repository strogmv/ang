package flowfn

import (
	"fmt"
	"strconv"
	"strings"
)

type parser struct {
	source string
	tokens []token
	pos    int
}

func Parse(source string) (Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return Program{}, err
	}
	p := &parser{source: source, tokens: tokens}
	return p.parseProgram(tokenEOF)
}

func ParseAndExpand(source string) (Program, error) {
	prog, err := Parse(source)
	if err != nil {
		return Program{}, err
	}
	return ExpandFragments(prog)
}

func (p *parser) parseProgram(stop tokenKind) (Program, error) {
	var nodes []Node
	for {
		p.skipSeparators()
		if p.peek().Kind == stop || p.peek().Kind == tokenEOF {
			if stop != tokenEOF && p.peek().Kind == stop {
				p.next()
			}
			return Program{Nodes: nodes}, nil
		}
		node, err := p.parseNode()
		if err != nil {
			return Program{}, err
		}
		nodes = append(nodes, node)
	}
}

func (p *parser) parseNode() (Node, error) {
	switch p.peek().Kind {
	case tokenKeywordIf:
		return p.parseIf()
	case tokenKeywordFor:
		return p.parseFor()
	case tokenKeywordTry:
		return p.parseTry()
	case tokenKeywordFragment:
		return p.parseFragment()
	case tokenKeywordUse:
		return p.parseUse()
	case tokenIdent:
		return p.parseCall()
	default:
		tok := p.peek()
		return nil, fmt.Errorf("unexpected token %q at %d:%d", tok.Literal, tok.Pos.Line, tok.Pos.Column)
	}
}

func (p *parser) parseCall() (Node, error) {
	actionTok := p.expect(tokenIdent)
	call := &CallNode{Pos: actionTok.Pos, Action: actionTok.Literal, Args: map[string]Value{}, Blocks: map[string][]Node{}}
	if p.match(tokenLParen) {
		args, err := p.parseArgs()
		if err != nil {
			return nil, err
		}
		call.Args = args
	}
	for {
		if p.match(tokenLBrace) {
			body, err := p.parseBlockContents()
			if err != nil {
				return nil, err
			}
			call.Blocks["do"] = body
			continue
		}
		if p.peek().Kind == tokenIdent && p.peekN(1).Kind == tokenLBrace {
			name := p.next().Literal
			p.next()
			body, err := p.parseBlockContents()
			if err != nil {
				return nil, err
			}
			call.Blocks[name] = body
			continue
		}
		break
	}
	if len(call.Blocks) == 0 {
		call.Blocks = nil
	}
	return call, nil
}

func (p *parser) parseArgs() (map[string]Value, error) {
	args := map[string]Value{}
	p.skipSeparators()
	if p.match(tokenRParen) {
		return args, nil
	}
	for {
		keyTok := p.expect(tokenIdent)
		if !p.match(tokenColon) {
			return nil, fmt.Errorf("expected ':' after %s at %d:%d", keyTok.Literal, keyTok.Pos.Line, keyTok.Pos.Column)
		}
		val, err := p.parseValue(tokenComma, tokenRParen)
		if err != nil {
			return nil, err
		}
		args[keyTok.Literal] = val
		if p.match(tokenComma) {
			continue
		}
		if p.match(tokenRParen) {
			return args, nil
		}
		tok := p.peek()
		return nil, fmt.Errorf("expected ',' or ')' at %d:%d, got %q", tok.Pos.Line, tok.Pos.Column, tok.Literal)
	}
}

func (p *parser) parseValue(delims ...tokenKind) (Value, error) {
	start := p.pos
	depthParen, depthBrace, depthBracket := 0, 0, 0
	for {
		tok := p.peek()
		if tok.Kind == tokenEOF {
			break
		}
		if depthParen == 0 && depthBrace == 0 && depthBracket == 0 && containsTokenKind(delims, tok.Kind) {
			break
		}
		switch tok.Kind {
		case tokenLParen:
			depthParen++
		case tokenRParen:
			if depthParen == 0 {
				goto done
			}
			depthParen--
		case tokenLBrace:
			depthBrace++
		case tokenRBrace:
			if depthBrace == 0 {
				goto done
			}
			depthBrace--
		case tokenLBracket:
			depthBracket++
		case tokenRBracket:
			if depthBracket == 0 {
				goto done
			}
			depthBracket--
		}
		p.next()
	}
done:
	if start == p.pos {
		tok := p.peek()
		return Value{}, fmt.Errorf("expected value at %d:%d", tok.Pos.Line, tok.Pos.Column)
	}
	toks := p.tokens[start:p.pos]
	raw := strings.TrimSpace(sliceText(p.source, toks[0].Pos.Offset, toks[len(toks)-1].End))
	if len(toks) == 1 {
		switch toks[0].Kind {
		case tokenString:
			s, err := strconv.Unquote(raw)
			if err != nil {
				return Value{}, err
			}
			return Value{Kind: ValueString, Raw: s}, nil
		case tokenInt:
			i, err := strconv.Atoi(raw)
			if err != nil {
				return Value{}, err
			}
			return Value{Kind: ValueInt, Int: i, Raw: raw}, nil
		case tokenBool:
			return Value{Kind: ValueBool, Bool: raw == "true", Raw: raw}, nil
		}
	}
	return Value{Kind: ValueExpr, Raw: raw}, nil
}

func (p *parser) parseIf() (Node, error) {
	start := p.expect(tokenKeywordIf)
	condition, err := p.captureUntil(tokenLBrace)
	if err != nil {
		return nil, err
	}
	if !p.match(tokenLBrace) {
		return nil, fmt.Errorf("expected '{' after if condition at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	thenBody, err := p.parseBlockContents()
	if err != nil {
		return nil, err
	}
	var elseBody []Node
	p.skipSeparators()
	if p.match(tokenKeywordElse) {
		if p.match(tokenKeywordIf) {
			p.pos--
			n, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			elseBody = []Node{n}
		} else {
			if !p.match(tokenLBrace) {
				return nil, fmt.Errorf("expected '{' after else at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
			}
			elseBody, err = p.parseBlockContents()
			if err != nil {
				return nil, err
			}
		}
	}
	return &IfNode{Pos: start.Pos, Condition: condition, Then: thenBody, Else: elseBody}, nil
}

func (p *parser) parseFor() (Node, error) {
	start := p.expect(tokenKeywordFor)
	aliasTok := p.expect(tokenIdent)
	if !p.match(tokenKeywordIn) {
		return nil, fmt.Errorf("expected 'in' after for alias at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	each, err := p.captureUntil(tokenLBrace)
	if err != nil {
		return nil, err
	}
	if !p.match(tokenLBrace) {
		return nil, fmt.Errorf("expected '{' after for expression at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	body, err := p.parseBlockContents()
	if err != nil {
		return nil, err
	}
	return &ForNode{Pos: start.Pos, Alias: aliasTok.Literal, Each: each, Do: body}, nil
}

func (p *parser) parseTry() (Node, error) {
	start := p.expect(tokenKeywordTry)
	if !p.match(tokenLBrace) {
		return nil, fmt.Errorf("expected '{' after try at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	doBody, err := p.parseBlockContents()
	if err != nil {
		return nil, err
	}
	p.skipSeparators()
	if !p.match(tokenKeywordCatch) {
		return nil, fmt.Errorf("expected catch after try block at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	if !p.match(tokenLBrace) {
		return nil, fmt.Errorf("expected '{' after catch at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	catchBody, err := p.parseBlockContents()
	if err != nil {
		return nil, err
	}
	return &TryNode{Pos: start.Pos, Do: doBody, Catch: catchBody}, nil
}

func (p *parser) parseFragment() (Node, error) {
	start := p.expect(tokenKeywordFragment)
	name := p.expect(tokenIdent)
	if !p.match(tokenLBrace) {
		return nil, fmt.Errorf("expected '{' after fragment name at %d:%d", p.peek().Pos.Line, p.peek().Pos.Column)
	}
	body, err := p.parseBlockContents()
	if err != nil {
		return nil, err
	}
	return &FragmentNode{Pos: start.Pos, Name: name.Literal, Body: body}, nil
}

func (p *parser) parseUse() (Node, error) {
	start := p.expect(tokenKeywordUse)
	name := p.expect(tokenIdent)
	return &UseNode{Pos: start.Pos, Name: name.Literal}, nil
}

func (p *parser) parseBlockContents() ([]Node, error) {
	prog, err := p.parseProgram(tokenRBrace)
	if err != nil {
		return nil, err
	}
	return prog.Nodes, nil
}

func (p *parser) captureUntil(stop tokenKind) (string, error) {
	start := p.pos
	depthParen, depthBracket := 0, 0
	for {
		tok := p.peek()
		if tok.Kind == tokenEOF {
			return "", fmt.Errorf("expected %s before EOF", stop)
		}
		if depthParen == 0 && depthBracket == 0 && tok.Kind == stop {
			break
		}
		switch tok.Kind {
		case tokenLParen:
			depthParen++
		case tokenRParen:
			if depthParen > 0 {
				depthParen--
			}
		case tokenLBracket:
			depthBracket++
		case tokenRBracket:
			if depthBracket > 0 {
				depthBracket--
			}
		}
		p.next()
	}
	if start == p.pos {
		return "", fmt.Errorf("expected expression before %s at %d:%d", stop, p.peek().Pos.Line, p.peek().Pos.Column)
	}
	toks := p.tokens[start:p.pos]
	return strings.TrimSpace(sliceText(p.source, toks[0].Pos.Offset, toks[len(toks)-1].End)), nil
}

func (p *parser) skipSeparators() {
	for {
		switch p.peek().Kind {
		case tokenNewline, tokenSemi:
			p.next()
		default:
			return
		}
	}
}

func (p *parser) expect(kind tokenKind) token {
	tok := p.peek()
	if tok.Kind != kind {
		panic(fmt.Sprintf("expected %s at %d:%d, got %s", kind, tok.Pos.Line, tok.Pos.Column, tok.Kind))
	}
	p.pos++
	return tok
}

func (p *parser) match(kind tokenKind) bool {
	if p.peek().Kind == kind {
		p.pos++
		return true
	}
	return false
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *parser) peekN(n int) token {
	idx := p.pos + n
	if idx >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[idx]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func containsTokenKind(items []tokenKind, target tokenKind) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func sliceText(src string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if start > end {
		return ""
	}
	return src[start:end]
}
