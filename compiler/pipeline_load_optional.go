package compiler

import (
	"path/filepath"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/angir/parser"
)

// LoadOptionalDomain loads the CUE package in path. A folder without .cue
// files is absent (false, no error). A file that does not parse, or a package
// that does not evaluate, is an error: skipping the file used to drop its
// declarations from the generated code with only a note on stderr (item 1 of
// docs/tech-debt-backlog.md), and only go build of the output noticed.
func LoadOptionalDomain(p *parser.Parser, path string) (cue.Value, bool, error) {
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	if len(matches) == 0 {
		return cue.Value{}, false, nil
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		return cue.Value{}, false, err
	}
	return val, true, nil
}
