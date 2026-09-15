package emitter

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// lineMarkerPrefix starts a comment the flow renderer puts before the code of
// a step: "//ang:line <cue file>:<line> <lines>". It is an ordinary comment, so
// it survives formatting; finalizeLineDirectives replaces it afterwards.
const lineMarkerPrefix = "//ang:line"

type lineMarker struct {
	file  string
	line  int
	lines int
}

func parseLineMarker(text string) (lineMarker, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(text), lineMarkerPrefix+" ")
	if !ok {
		return lineMarker{}, false
	}
	ref, count, ok := strings.Cut(strings.TrimSpace(rest), " ")
	if !ok {
		return lineMarker{}, false
	}
	colon := strings.LastIndex(ref, ":")
	if colon <= 0 {
		return lineMarker{}, false
	}
	line, err := strconv.Atoi(ref[colon+1:])
	if err != nil || line <= 0 {
		return lineMarker{}, false
	}
	lines, err := strconv.Atoi(count)
	if err != nil || lines <= 0 {
		return lineMarker{}, false
	}
	return lineMarker{file: ref[:colon], line: line, lines: lines}, true
}

// finalizeLineDirectives turns step markers in formatted Go into //line
// directives, so the compiler, go vet, gopls and panic traces report the CUE
// line a step is written on. The marked lines are mapped to CUE; the line after
// them gets a directive back to goPath with its real number. goPath is the
// file's place in the project (not a staging copy), relative to the working
// directory like the CUE file names in the markers; directive paths are
// relative to its directory, as Go reads them.
// Nothing may format the result again: gofmt keeps the directives in column 1,
// but a pass that moves lines would break the numbers.
func finalizeLineDirectives(src []byte, goPath string) []byte {
	text := string(src)
	if !strings.Contains(text, lineMarkerPrefix) {
		return src
	}
	goDir, err := filepath.Abs(filepath.Dir(goPath))
	if err != nil {
		goDir = ""
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+len(lines)/4)
	isMarker := func(i int) bool {
		return i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), lineMarkerPrefix)
	}
	for i := 0; i < len(lines); i++ {
		if !isMarker(i) {
			out = append(out, lines[i])
			continue
		}
		marker, ok := parseLineMarker(lines[i])
		cueRef := ""
		if ok && goDir != "" {
			if abs, err := filepath.Abs(filepath.FromSlash(marker.file)); err == nil {
				if rel, err := filepath.Rel(goDir, abs); err == nil {
					cueRef = filepath.ToSlash(rel)
				}
			}
		}
		// Comments and blank lines the step writes before its code stay unmapped.
		code := i + 1
		for code < len(lines) && !isMarker(code) && isBlankOrComment(lines[code]) {
			code++
		}
		out = append(out, lines[i+1:code]...)
		if cueRef == "" || code >= len(lines) || isMarker(code) {
			// A step without code of its own, or a marker that cannot be mapped.
			i = code - 1
			continue
		}
		out = append(out, fmt.Sprintf("//line %s:%d", cueRef, marker.line))
		end := code
		for end < len(lines) && end-code < marker.lines && !isMarker(end) {
			out = append(out, lines[end])
			end++
		}
		if end < len(lines) && !isMarker(end) {
			// The directive is line len(out)+1; the line after it is the next.
			out = append(out, fmt.Sprintf("//line %s:%d", filepath.Base(goPath), len(out)+2))
		}
		i = end - 1
	}
	return []byte(strings.Join(out, "\n"))
}

// sourceBackendPath is the place of a generated file in the project.
func (e *Emitter) sourceBackendPath(parts ...string) string {
	root := e.SourceBackendDir
	if strings.TrimSpace(root) == "" {
		root = e.OutputDir
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

func isBlankOrComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || (strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "//line "))
}
