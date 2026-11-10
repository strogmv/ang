package godot

import (
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"text/template"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

//go:embed finishing_bindings.tscn.tmpl
var finishingBindingsTemplate string

func validateSceneManifest(level locationsdk.CompiledLevel, manifest placement.Manifest) error {
	if manifest.Schema != placement.Schema || manifest.WorldFrame != placement.WorldFrame || manifest.LayoutID != level.ID || manifest.StructuralHash != level.StructuralHash {
		return fmt.Errorf("godot: manifest does not match compiled level")
	}
	return nil
}

// RenderFinishingBindings lets another scene builder consume exactly the same
// CUE material resources, concrete shader and named/sentinel bindings as Render.
// It contains only the generic Finish adapter; it adds no geometry or lights.
// Instance its root directly under the location root, before that root's ready
// material fallback runs. The adapter resolves every imported mesh recursively.
func RenderFinishingBindings(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing) ([]byte, error) {
	if err := validateSceneManifest(level, manifest); err != nil {
		return nil, err
	}
	resolved, err := ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return nil, err
	}
	header, bindings, _, _, _, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("finishing-bindings").Parse(finishingBindingsTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	data := struct{ Header, Bindings, LayoutID string }{header, bindings, strconv.Quote(level.ID)}
	if err = tmpl.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
