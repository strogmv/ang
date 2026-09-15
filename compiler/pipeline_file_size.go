package compiler

import (
	"strings"

	cueast "cuelang.org/go/cue/ast"
	cueparser "cuelang.org/go/cue/parser"
)

// cueLinesOutsideMultilineStrings counts the lines of a CUE file that are CUE:
// lines inside """…""" and #"""…"""# strings (embedded Go, SQL, templates)
// are left out. A file that does not parse is counted as a whole.
func cueLinesOutsideMultilineStrings(filename string, content []byte) (cueLines, totalLines int) {
	totalLines = strings.Count(string(content), "\n") + 1
	file, err := cueparser.ParseFile(filename, content, cueparser.ParseComments)
	if err != nil {
		return totalLines, totalLines
	}
	inStrings := 0
	cueast.Walk(file, func(node cueast.Node) bool {
		var start, end int
		switch value := node.(type) {
		case *cueast.BasicLit:
			start = value.Pos().Offset()
			end = start + len(value.Value)
		case *cueast.Interpolation:
			start = value.Pos().Offset()
			end = value.End().Offset()
		default:
			return true
		}
		if start < 0 || end > len(content) || end <= start {
			return false
		}
		literal := string(content[start:end])
		if strings.HasPrefix(strings.TrimLeft(literal, "#"), `"""`) {
			// The opening and closing quote lines are CUE; everything between
			// them is the string.
			if n := strings.Count(literal, "\n"); n > 1 {
				inStrings += n - 1
			}
		}
		return false
	}, nil)
	return totalLines - inStrings, totalLines
}
