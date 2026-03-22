package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
)

// keep cue in scope for cue.Value return types
var _ = cue.Value{}

// Parser loads and performs initial validation of CUE models.
type Parser struct {
	ctx *cue.Context
}

func New() *Parser {
	return &Parser{
		ctx: cuecontext.New(),
	}
}

// FormatCUELocationError converts CUE error into human-readable advice with locations.
func FormatCUELocationError(err error) string {
	if err == nil {
		return ""
	}

	var msg strings.Builder
	errs := errors.Errors(err)

	for _, e := range errs {
		msg.WriteString(fmt.Sprintf("❌ CUE Error: %v\n", e))

		// Extract positions
		positions := errors.Positions(e)
		if len(positions) > 1 {
			msg.WriteString("   Conflict detected between these locations:\n")
			for i, p := range positions {
				msg.WriteString(fmt.Sprintf("   %d. %s\n", i+1, p.String()))
			}
			msg.WriteString("   💡 Suggestion: These values are incompatible. Check if one should be optional (?) or remove the duplicate definition.\n")
		}
	}

	if msg.Len() == 0 {
		return err.Error()
	}
	return msg.String()
}

// LoadDomain loads definitions from a specific path.
func (p *Parser) LoadDomain(path string) (cue.Value, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return cue.Value{}, err
	}
	bis := load.Instances([]string{"."}, &load.Config{
		Dir: absPath,
	})

	if len(bis) == 0 {
		return cue.Value{}, fmt.Errorf("no CUE files found in %s", path)
	}
	if bis[0].Err != nil {
		return cue.Value{}, bis[0].Err
	}

	// We take the first instance (usually a single package in the folder).
	// Note: BuildInstance already evaluates the CUE package and catches structural
	// errors. We intentionally do NOT call v.Validate(cue.All()) here because that
	// triggers a full recursive constraint walk over all optional fields, which on
	// large schemas (e.g. cue/schema/types.cue ×  many api/*.cue operations) can
	// spin all CPU cores for several seconds. v.Err() is sufficient to detect
	// evaluation errors; semantic/type violations are reported by the normalizer.
	v := p.ctx.BuildInstance(bis[0])
	if v.Err() != nil {
		return cue.Value{}, v.Err()
	}

	return v, nil
}
