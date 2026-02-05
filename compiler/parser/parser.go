package parser

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"fmt"
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

// LoadDomain loads definitions from cue/domain.
func (p *Parser) LoadDomain(path string) (cue.Value, error) {
	bis := load.Instances([]string{"."}, &load.Config{
		Dir: path,
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
