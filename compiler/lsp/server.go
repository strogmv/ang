package lsp

import (
	"regexp"
	"sort"
	"strings"
)

type Position struct {
	Line      int
	Character int
}

type Range struct {
	Start Position
	End   Position
}

type Diagnostic struct {
	Range    Range
	Severity int
	Code     string
	Message  string
	Source   string
}

type CompletionItem struct {
	Label      string
	Detail     string
	InsertText string
	Deprecated bool
	SortText   string
}

type Hover struct {
	Value string
	Range *Range
}

type CompletionContext struct {
	Tags   map[string]bool
	InTx   bool
	Inside string
}

var actionRegexp = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_.]*\b`)

func wordRangeAt(text string, pos Position) (string, *Range) {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", nil
	}
	line := lines[pos.Line]
	if pos.Character < 0 {
		pos.Character = 0
	}
	if pos.Character > len(line) {
		pos.Character = len(line)
	}
	matches := actionRegexp.FindAllStringIndex(line, -1)
	for _, m := range matches {
		if pos.Character < m[0] || pos.Character > m[1] {
			continue
		}
		return line[m[0]:m[1]], &Range{
			Start: Position{Line: pos.Line, Character: m[0]},
			End:   Position{Line: pos.Line, Character: m[1]},
		}
	}
	return "", nil
}

func sortCompletionItems(items []CompletionItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].SortText != items[j].SortText {
			return items[i].SortText < items[j].SortText
		}
		return items[i].Label < items[j].Label
	})
}
