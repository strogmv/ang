package parser

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"fmt"
	"path/filepath"
)

// Parser loads and performs initial validation of CUE models.
type Parser struct {
	ctx *cue.Context
}

func New() *Parser {
	return &Parser{
		ctx: cuecontext.New(),
	}
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
	v := p.ctx.BuildInstance(bis[0])
	if v.Err() != nil {
		return cue.Value{}, v.Err()
	}

	return v, nil
}
