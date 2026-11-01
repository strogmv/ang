// Package determinism guards generated simulation code against process-global RNGs.
package determinism

import (
	"fmt"
	"go/parser"
	"go/token"
	"strconv"
)

// CheckImports rejects aliases and dot/blank imports too, including rand/v2.
func CheckImports(name string, source []byte) error {
	f, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
	if err != nil {
		return err
	}
	for _, entry := range f.Imports {
		path, err := strconv.Unquote(entry.Path.Value)
		if err != nil {
			return err
		}
		if path == "math/rand" || path == "math/rand/v2" {
			return fmt.Errorf("%s: nondeterministic import %s; use behavior.CounterRandom(seed, actor, tick, stream)", name, path)
		}
	}
	return nil
}
