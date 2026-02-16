package emitter

import (
	"fmt"
	"golang.org/x/tools/imports"
)

// formatGoStrict formats generated Go source and fails fast on syntax issues.
func formatGoStrict(src []byte, unit string) ([]byte, error) {
	out, err := imports.Process(unit, src, nil)
	if err != nil {
		return nil, fmt.Errorf("generated go is invalid (%s): %w", unit, err)
	}
	return out, nil
}
