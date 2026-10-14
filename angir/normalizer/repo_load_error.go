package normalizer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// DescribeRepoLoadError turns a CUE error from loading cue/repo into one line
// per problem: "file:line:col: message (finder "X")". A file of cue/repo that
// does not parse (an invalid escape such as `\s` inside sql: """…""") used to
// be skipped, and every finder declared in it vanished from the generated
// ports while validate and build said OK; the error must name where it is.
func DescribeRepoLoadError(err error) error {
	if err == nil {
		return nil
	}
	errs := cueerrors.Errors(err)
	if len(errs) == 0 {
		return err
	}
	lines := make([]string, 0, len(errs))
	seen := map[string]bool{}
	for _, e := range errs {
		format, args := e.Msg()
		msg := strings.TrimSpace(fmt.Sprintf(format, args...))
		if msg == "" {
			msg = e.Error()
		}
		positions := cueerrors.Positions(e)
		if len(positions) == 0 {
			if p := e.Position(); p.IsValid() {
				positions = []token.Pos{p}
			}
		}
		line := msg
		if len(positions) > 0 {
			p := positions[0]
			line = fmt.Sprintf("%s:%d:%d: %s", relToCwd(p.Filename()), p.Line(), p.Column(), msg)
			if name := finderNameAt(p.Filename(), p.Line()); name != "" {
				line += fmt.Sprintf(" (finder %q)", name)
			}
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

var finderNameLine = regexp.MustCompile(`^\s*"?name"?\s*:\s*"([^"]+)"`)

// finderNameAt reads the file and returns the nearest `name: "…"` at or above
// line: the finder the error sits in. A file that does not parse has no AST,
// so the text is the only place left to look.
func finderNameAt(file string, line int) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	name := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan() && n <= line; n++ {
		if m := finderNameLine.FindStringSubmatch(sc.Text()); m != nil {
			name = m[1]
		}
	}
	return name
}

func relToCwd(file string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

// finderString reads a string field of a finder. A missing or not yet concrete
// field is "" (it is optional); a field that exists but is an error (a
// conflict, a wrong type) is reported instead of being read as "".
func finderString(fv cue.Value, path string) (string, error) {
	v := fv.LookupPath(cue.ParsePath(path))
	if !v.Exists() {
		return "", nil
	}
	if d, ok := v.Default(); ok {
		v = d
	}
	if v.IncompleteKind() == cue.BottomKind {
		if err := v.Err(); err != nil {
			return "", err
		}
	}
	if !v.IsConcrete() {
		return "", nil
	}
	s, err := v.String()
	if err != nil {
		return "", err
	}
	return s, nil
}
