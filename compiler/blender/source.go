package blender

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/parser"
	"github.com/strogmv/ang/compiler/godot"
)

type PoseAuthority struct {
	PoseSource   string `json:"pose_source"`
	GeneratedCUE string `json:"generated_cue,omitempty"`
	Blend        string `json:"blend,omitempty"`
}

// ValidatePoseAuthority keeps the declared source unique. Blender-first input is
// a generated geometry package, never a second set of matching hand poses.
func ValidatePoseAuthority(profile PoseAuthority, packageDir, sourceBlend string) error {
	if profile.PoseSource == "" || profile.PoseSource == "cue" {
		return nil
	}
	if profile.PoseSource != "blend" {
		return fmt.Errorf("authoring.pose_source: choose cue or blend")
	}
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(packageDir, path)
	}
	if profile.GeneratedCUE == "" {
		return fmt.Errorf("authoring.generated_cue: Blender pose source requires its generated geometry CUE file")
	}
	generated, err := filepath.Abs(resolve(profile.GeneratedCUE))
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(generated)
	if err != nil {
		return fmt.Errorf("authoring.generated_cue: run ANG import.py through the serial Blender wrapper before generation: %w", err)
	}
	if !strings.HasPrefix(string(raw), "// Generated from the ID-tagged Blender source by ANG; do not hand-edit.") {
		return fmt.Errorf("authoring.generated_cue %s: missing ANG source provenance; do not substitute hand CUE poses", generated)
	}
	value := cuecontext.New().CompileBytes(raw, cue.Filename(generated))
	if err = value.Err(); err != nil {
		return fmt.Errorf("authoring.generated_cue: %w", err)
	}
	kind, err := value.LookupPath(cue.ParsePath("generated_geometry.source_kind")).String()
	if err != nil || kind != "blend" {
		return fmt.Errorf("authoring.generated_cue: generated_geometry.source_kind must be blend")
	}
	expected, err := value.LookupPath(cue.ParsePath("generated_geometry.source_sha256")).String()
	if err != nil {
		return fmt.Errorf("authoring.generated_cue: source_sha256 required")
	}
	blend := sourceBlend
	if blend == "" {
		blend = profile.Blend
		if blend != "" {
			blend = resolve(blend)
		}
	}
	if blend == "" {
		return fmt.Errorf("authoring.blend: Blender pose source requires the .blend input (or -source-blend execution path)")
	}
	source, err := os.ReadFile(blend)
	if err != nil {
		return fmt.Errorf("authoring.blend: %w", err)
	}
	digest := sha256.Sum256(source)
	if expected != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("authoring.generated_cue: source .blend SHA changed; rerun Blender import before generation")
	}
	entries, err := os.ReadDir(filepath.Dir(generated))
	if err != nil {
		return err
	}
	poseFields := map[string]bool{"floor": true, "instances": true, "obstacles": true, "extras": true, "locators": true, "width": true, "depth": true, "elevation": true, "source_kind": true}
	for _, entry := range entries {
		path := filepath.Join(filepath.Dir(generated), entry.Name())
		if entry.IsDir() || !strings.HasSuffix(path, ".cue") || path == generated {
			continue
		}
		tree, e := parser.ParseFile(path, nil, parser.ParseComments)
		if e != nil {
			return e
		}
		forbidden := false
		ast.Walk(tree, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok {
				return true
			}
			label, _, _ := ast.LabelName(field.Label)
			if label != "generated_geometry" {
				return true
			}
			object, ok := field.Value.(*ast.StructLit)
			if !ok {
				return true
			}
			for _, decl := range object.Elts {
				f, ok := decl.(*ast.Field)
				if ok {
					name, _, _ := ast.LabelName(f.Label)
					forbidden = forbidden || poseFields[name]
				}
			}
			return !forbidden
		}, nil)
		if forbidden {
			return fmt.Errorf("authoring.pose_source blend: hand CUE pose owner %s still defines generated_geometry; move that pose file aside, retaining semantic CUE", path)
		}
	}
	return nil
}

// GeneratedReplacements reads only rendering ownership from the already
// validated Blender geometry artifact. The ordinary finishing validator still
// checks that every surface exists and its replacement owner has bound art.
func GeneratedReplacements(profile PoseAuthority, packageDir string) ([]godot.Replacement, error) {
	if profile.PoseSource != "blend" {
		return nil, nil
	}
	if profile.GeneratedCUE == "" {
		return nil, fmt.Errorf("authoring.generated_cue required for Blender rendering ownership")
	}
	path := profile.GeneratedCUE
	if !filepath.IsAbs(path) {
		path = filepath.Join(packageDir, path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value := cuecontext.New().CompileBytes(raw, cue.Filename(path))
	if err = value.Err(); err != nil {
		return nil, err
	}
	field := value.LookupPath(cue.ParsePath("generated_geometry.replacements"))
	if !field.Exists() {
		return nil, nil
	}
	var replacements []godot.Replacement
	if err = field.Decode(&replacements); err != nil {
		return nil, fmt.Errorf("generated_geometry.replacements: %w", err)
	}
	return replacements, nil
}
