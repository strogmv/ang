package blender

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
)

// ValidatePoseSources checks the source expressions as well as the resulting
// geometry. Equal CUE constraints are still a second pose owner. In particular,
// decoding JSON first would lose the conjunctions and aliases that expose them.
// ValidatePoseAuthority must validate the artifact's source SHA before this call.
func ValidatePoseSources(profile PoseAuthority, packageDir string, layout, presentation cue.Value, sourceRoots ...cue.Value) error {
	if profile.PoseSource != "blend" {
		return nil
	}
	generated := profile.GeneratedCUE
	if !filepath.IsAbs(generated) {
		generated = filepath.Join(packageDir, generated)
	}
	generated, err := filepath.Abs(generated)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(generated)
	if err != nil {
		return err
	}
	geometry := cuecontext.New().CompileBytes(raw, cue.Filename(generated)).LookupPath(cue.ParsePath("generated_geometry"))
	if err = geometry.Err(); err != nil {
		return err
	}
	guard := poseSourceGuard{generated: filepath.Clean(generated), derived: map[string]bool{}, dependencies: map[string]bool{}}
	guard.roots = posePackageRoots(append(sourceRoots, presentation)...)
	if rows, e := presentation.LookupPath(cue.ParsePath("pipeline.instances")).Fields(); e == nil {
		for rows.Next() {
			if derived, _ := rows.Value().LookupPath(cue.ParsePath("derived")).Bool(); derived {
				guard.derived[rows.Selector().Unquoted()] = true
			}
		}
	}
	for _, field := range []string{"width", "depth", "elevation"} {
		actual, expected := layout.LookupPath(cue.MakePath(cue.Str(field))), geometry.LookupPath(cue.MakePath(cue.Str(field)))
		if err = guard.checkValue(actual, "sdk.layout."+field, false, 0); err != nil {
			return err
		}
		if !samePoseValue(actual, expected) {
			return guard.fail(actual, "sdk.layout."+field, "does not match the imported Blender geometry")
		}
	}
	for _, spec := range []struct {
		actual cue.Value
		name   string
		id     string
		fields []string
	}{
		{layout.LookupPath(cue.ParsePath("floor")), "floor", "id", []string{"rect"}},
		{layout.LookupPath(cue.ParsePath("instances")), "instances", "id", []string{"module_id", "at", "rotation"}},
		{layout.LookupPath(cue.ParsePath("obstacles")), "obstacles", "id", []string{"surface", "rect", "bottom_mm", "top_mm"}},
		{presentation.LookupPath(cue.ParsePath("placement.extras")), "extras", "instance_id", []string{"position_plan_m", "rotation_degrees"}},
		{presentation.LookupPath(cue.ParsePath("pipeline.locators")), "locators", "name", []string{"instance_id", "position_plan_m"}},
	} {
		path := "sdk.layout." + spec.name
		if spec.name == "extras" {
			path = "placement.extras"
		} else if spec.name == "locators" {
			path = "pipeline.locators"
		}
		if err = guard.checkRows(spec.actual, geometry.LookupPath(cue.MakePath(cue.Str(spec.name))), path, spec.id, spec.fields, spec.name == "instances"); err != nil {
			return err
		}
	}
	if lights, e := presentation.LookupPath(cue.ParsePath("finishing.lights")).List(); e == nil {
		for lights.Next() {
			light := lights.Value()
			id, _ := light.LookupPath(cue.ParsePath("id")).String()
			for _, field := range []string{"at_plan_m", "aim_plan_m"} {
				value := light.LookupPath(cue.MakePath(cue.Str(field)))
				if value.Exists() {
					if err = guard.checkValue(value, fmt.Sprintf("finishing.lights[%q].%s", id, field), true, 0); err != nil {
						return err
					}
				}
			}
		}
	}
	return guard.checkFinishingPoses(presentation)
}

type poseSourceGuard struct {
	generated    string
	derived      map[string]bool
	roots        []cue.Value
	dependencies map[string]bool
}

func (g poseSourceGuard) fail(v cue.Value, path, why string) error {
	return fmt.Errorf("authoring.pose_source blend: %s at %s %s; keep poses in the declared generated CUE artifact, and edit the ID-tagged Blender source", path, v.Pos(), why)
}

func (g poseSourceGuard) fromGenerated(v cue.Value) bool {
	path := v.Pos().Filename()
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	return err == nil && filepath.Clean(abs) == g.generated
}

// Schema bounds, open lists and enums constrain legal values without supplying
// poses. Concrete literals (including [] and zero) are deliberately not schemas.
func poseSchema(v cue.Value) bool {
	if !v.IsConcrete() {
		return true
	}
	n := v.Source()
	if f, ok := n.(*ast.Field); ok {
		n = f.Value
	}
	l, ok := n.(*ast.ListLit)
	if !ok {
		return false
	}
	if len(l.Elts) == 0 {
		return false
	}
	for _, element := range l.Elts {
		if _, ok := element.(*ast.Ellipsis); !ok {
			return false
		}
	}
	return true
}

// numberOrigin follows reference values, not identifier spellings. Each
// concrete conjunct must independently have an authorized source: x & alias(1)
// cannot inherit x's provenance merely because both values happen to be one.
func (g poseSourceGuard) numberOrigin(v cue.Value, offsets bool, depth int) (bool, error) {
	return g.numberOriginIn(v, offsets, depth, poseScope{})
}

func (g poseSourceGuard) numberOriginIn(v cue.Value, offsets bool, depth int, scope poseScope) (bool, error) {
	if depth > 80 {
		return false, g.fail(v, v.Path().String(), "has cyclic or unsupported pose provenance")
	}
	op, args := v.Expr()
	if op == cue.AndOp {
		found := false
		var unowned []cue.Value
		for _, arg := range args {
			if poseSchema(arg) {
				continue
			}
			owned, err := g.numberOriginIn(arg, offsets, depth+1, scope)
			if err != nil {
				return false, err
			}
			if !owned {
				unowned = append(unowned, arg)
			} else {
				found = true
			}
		}
		if found && len(unowned) != 0 {
			return false, g.fail(unowned[0], v.Path().String(), "adds a hand CUE pose constraint")
		}
		// Equal constraints on a constant unit, thickness or other policy
		// parameter are not a coordinate source. A containing expression must
		// still obtain an imported origin from another operand.
		return found, nil
	}
	if g.fromGenerated(v) {
		return true, nil
	}
	scope = g.scopeFor(v, scope)
	// Comprehension variables may not have a package ReferencePath. Dereference
	// retains their environment and reaches the actual imported row instead.
	if target := cue.Dereference(v); target.Err() == nil && target.Source() != v.Source() {
		return g.numberOriginIn(target, offsets, depth+1, scope)
	}
	if root, path := v.ReferencePath(); root.Exists() {
		target := root.LookupPath(path)
		if target.Exists() && target.Err() == nil {
			return g.numberOriginIn(target, offsets, depth+1, scope)
		}
	}
	if offsets && (op == cue.AddOp || op == cue.SubtractOp || op == cue.MultiplyOp || op == cue.FloatQuotientOp || op == cue.CallOp) {
		found := false
		for index, arg := range args {
			if op == cue.CallOp && index == 0 {
				continue // the builtin itself is not a geometry input
			}
			owned, err := g.numberOriginIn(arg, true, depth+1, scope)
			if err != nil {
				return false, err
			}
			found = found || owned
		}
		return found, nil
	}
	if offsets {
		// A builtin-produced scalar (for example one element of list.Range)
		// retains its call syntax but no longer has an Expr reference. Follow
		// that call's lexical comprehension bindings in the instantiated scope.
		if call, ok := v.Source().(*ast.CallExpr); ok {
			return g.syntaxOrigin(call, scope, depth+1, map[ast.Node]bool{})
		}
	}
	return false, nil
}

func (g poseSourceGuard) checkValue(v cue.Value, path string, offsets bool, depth int) error {
	if !v.Exists() || poseSchema(v) {
		return nil
	}
	if depth > 80 {
		return g.fail(v, path, "has cyclic or unsupported pose provenance")
	}
	if op, args := v.Expr(); op == cue.AndOp {
		for _, arg := range args {
			if err := g.checkValue(arg, path, offsets, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if root, reference := v.ReferencePath(); root.Exists() {
		if target := root.LookupPath(reference); target.Exists() && target.Err() == nil {
			return g.checkValue(target, path, offsets, depth+1)
		}
	}
	switch v.Kind() {
	case cue.IntKind, cue.FloatKind, cue.NumberKind:
		owned, err := g.numberOrigin(v, offsets, depth+1)
		if err != nil {
			return err
		}
		if !owned {
			return g.fail(v, path, "is a hand CUE pose (matching numbers are still a second owner)")
		}
	case cue.ListKind:
		items, err := v.List()
		if err != nil {
			return err
		}
		count := 0
		for items.Next() {
			if err = g.checkValue(items.Value(), fmt.Sprintf("%s[%d]", path, count), offsets, depth+1); err != nil {
				return err
			}
			count++
		}
		if count == 0 && !g.containerOrigin(v, depth+1) {
			return g.fail(v, path, "supplies a hand CUE empty pose list")
		}
	case cue.StructKind:
		fields, err := v.Fields()
		if err != nil {
			return err
		}
		for fields.Next() {
			if err = g.checkValue(fields.Value(), path+"."+fields.Selector().Unquoted(), offsets, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g poseSourceGuard) containerOrigin(v cue.Value, depth int) bool {
	if depth > 80 {
		return false
	}
	if root, path := v.ReferencePath(); root.Exists() {
		if target := root.LookupPath(path); target.Exists() && target.Err() == nil {
			return g.containerOrigin(target, depth+1)
		}
	}
	if g.fromGenerated(v) {
		return true
	}
	// Calls such as list.Concat may produce an empty list from authorized
	// empty lists. NoOp's sole argument is itself and must not be followed.
	op, args := v.Expr()
	if op != cue.NoOp {
		for _, arg := range args {
			if g.containerOrigin(arg, depth+1) {
				return true
			}
		}
	}
	if items, err := v.List(); err == nil {
		for items.Next() {
			if g.containerOrigin(items.Value(), depth+1) {
				return true
			}
		}
	}
	return false
}

func (g poseSourceGuard) rowSources(v cue.Value, path, idField string, fields []string, allowDerived bool, depth int) error {
	if !v.Exists() || poseSchema(v) {
		return nil
	}
	if depth > 80 {
		return g.fail(v, path, "has cyclic or unsupported pose provenance")
	}
	if op, args := v.Expr(); op == cue.AndOp {
		for _, arg := range args {
			if err := g.rowSources(arg, path, idField, fields, allowDerived, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if root, reference := v.ReferencePath(); root.Exists() {
		if target := root.LookupPath(reference); target.Exists() && target.Err() == nil {
			return g.rowSources(target, path, idField, fields, allowDerived, depth+1)
		}
	}
	items, err := v.List()
	if err != nil {
		return err
	}
	count := 0
	for items.Next() {
		row := items.Value()
		id, _ := row.LookupPath(cue.MakePath(cue.Str(idField))).String()
		if allowDerived && g.derived[id] {
			// Policy may choose a stocked kit and quarter rotation, but a flag
			// never authorizes literal coordinates. Both axes must depend on
			// the imported geometry, with every extra conjunct checked.
			for _, axis := range []string{"x", "y"} {
				coordinate := row.LookupPath(cue.MakePath(cue.Str("at"), cue.Str(axis)))
				if coordinate.Exists() && !poseSchema(coordinate) {
					owned, e := g.numberOrigin(coordinate, true, depth+1)
					if e != nil {
						return e
					}
					if !owned {
						return g.fail(coordinate, fmt.Sprintf("%s[%q].at.%s", path, id, axis), "declares derived:true but has no imported-geometry coordinate dependency")
					}
				}
			}
		} else {
			for _, field := range fields {
				if err = g.checkValue(row.LookupPath(cue.MakePath(cue.Str(field))), fmt.Sprintf("%s[%q].%s", path, id, field), false, depth+1); err != nil {
					return err
				}
			}
		}
		count++
	}
	if count == 0 && !g.containerOrigin(v, depth+1) {
		return g.fail(v, path, "supplies a hand CUE empty pose list")
	}
	return nil
}

func (g poseSourceGuard) checkRows(actual, expected cue.Value, path, idField string, fields []string, allowDerived bool) error {
	if err := g.rowSources(actual, path, idField, fields, allowDerived, 0); err != nil {
		return err
	}
	owned := map[string]cue.Value{}
	if rows, err := expected.List(); err == nil {
		for rows.Next() {
			row := rows.Value()
			id, _ := row.LookupPath(cue.MakePath(cue.Str(idField))).String()
			owned[id] = row
		}
	}
	if rows, err := actual.List(); err == nil {
		for rows.Next() {
			row := rows.Value()
			id, _ := row.LookupPath(cue.MakePath(cue.Str(idField))).String()
			original, ok := owned[id]
			if allowDerived && g.derived[id] && !ok {
				for _, axis := range []string{"x", "y"} {
					coordinate := row.LookupPath(cue.MakePath(cue.Str("at"), cue.Str(axis)))
					if !coordinate.Exists() || coordinate.Kind()&cue.NumberKind == 0 {
						return g.fail(row, fmt.Sprintf("%s[%q].at.%s", path, id, axis), "requires a concrete imported-geometry coordinate dependency")
					}
				}
				continue
			}
			if !ok {
				return g.fail(row, path+"["+fmt.Sprintf("%q", id)+"]", "is absent from the imported Blender geometry")
			}
			for _, field := range fields {
				a, b := row.LookupPath(cue.MakePath(cue.Str(field))), original.LookupPath(cue.MakePath(cue.Str(field)))
				if !samePoseValue(a, b) {
					return g.fail(a, fmt.Sprintf("%s[%q].%s", path, id, field), "does not match the imported Blender geometry")
				}
			}
			delete(owned, id)
		}
	}
	if len(owned) != 0 {
		return g.fail(actual, path, "omits imported Blender geometry IDs")
	}
	return nil
}

func samePoseValue(a, b cue.Value) bool {
	if !a.Exists() || !b.Exists() {
		return a.Exists() == b.Exists()
	}
	var left, right any
	x, xerr := a.MarshalJSON()
	y, yerr := b.MarshalJSON()
	return xerr == nil && yerr == nil && json.Unmarshal(x, &left) == nil && json.Unmarshal(y, &right) == nil && reflect.DeepEqual(left, right)
}
