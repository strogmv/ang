package emitter

import (
	"fmt"
	"go/format"
)

// formatGoStrict formats generated Go source and fails fast on syntax issues.
func formatGoStrict(src []byte, unit string) ([]byte, error) {
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("generated go is invalid (%s): %w", unit, err)
	}
	return out, nil
}
