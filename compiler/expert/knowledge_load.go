package expert

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// LoadKnowledgePack reads one CUE package directory containing a top-level
// `pack` value constrained by schema.#ExpertKnowledgePack.
func LoadKnowledgePack(dir string) (KnowledgePack, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return KnowledgePack{}, fmt.Errorf("resolve knowledge pack directory: %w", err)
	}
	instances := load.Instances([]string{"."}, &load.Config{Dir: dir})
	if len(instances) != 1 {
		return KnowledgePack{}, fmt.Errorf("load knowledge pack from %s: expected one CUE instance, got %d", dir, len(instances))
	}
	if err := instances[0].Err; err != nil {
		return KnowledgePack{}, fmt.Errorf("load knowledge pack from %s: %w", dir, err)
	}
	value := cuecontext.New().BuildInstance(instances[0])
	if err := value.Err(); err != nil {
		return KnowledgePack{}, fmt.Errorf("build knowledge pack from %s: %w", dir, err)
	}
	packValue := value.LookupPath(cue.ParsePath("pack"))
	if !packValue.Exists() {
		return KnowledgePack{}, fmt.Errorf("knowledge pack in %s is missing top-level \"pack\"", dir)
	}
	if err := packValue.Validate(cue.Concrete(true)); err != nil {
		return KnowledgePack{}, fmt.Errorf("validate knowledge pack in %s: %w", dir, err)
	}
	var pack KnowledgePack
	if err := packValue.Decode(&pack); err != nil {
		return KnowledgePack{}, fmt.Errorf("decode knowledge pack in %s: %w", dir, err)
	}
	if err := ValidateKnowledgePack(pack); err != nil {
		return KnowledgePack{}, err
	}
	return pack, nil
}
