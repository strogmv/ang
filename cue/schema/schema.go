// Package schema carries ANG's CUE schema (the .cue files of this directory)
// inside the ang binary, so a project gets exactly the schema of the ANG
// revision its ang.lock pins: ang build and ang schema sync vendor these files
// into the project's cue.mod/pkg/github.com/strogmv/ang/cue/schema
// (compiler/schemavendor, docs/schema.md).
package schema

import (
	"embed"
	"io/fs"
	"sort"
)

// ImportPath is the CUE import path of the schema package.
const ImportPath = "github.com/strogmv/ang/cue/schema"

//go:embed *.cue
var files embed.FS

// Files returns the schema's .cue files by name, as committed in ANG.
func Files() (map[string][]byte, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = data
	}
	return out, nil
}

// Names returns the schema's file names, sorted.
func Names() ([]string, error) {
	all, err := Files()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
