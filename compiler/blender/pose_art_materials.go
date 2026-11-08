package blender

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// ValidateArtMaterialPoseSources is the final-generation half of pose authority.
// Artists can select roles that the initial CUE package does not bind. The
// validated complete art report supplies actual usage, including original object
// names, so unused local palette data remains legal until selected for level art.
func ValidateArtMaterialPoseSources(profile PoseAuthority, packageDir string, presentation cue.Value, contract Contract, data []byte, sourceRoots ...cue.Value) error {
	if profile.PoseSource != "blend" {
		return nil
	}
	if _, err := ValidateReport(contract, data); err != nil {
		return err
	}
	var report struct {
		Components []struct {
			ID                   string            `json:"id"`
			AppliedMaterialRoles map[string]string `json:"applied_material_roles"`
			Objects              []struct {
				Name      string   `json:"name"`
				Materials []string `json:"materials"`
			} `json:"objects"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("art material pose sources: %w", err)
	}
	materialNames := map[string]string{}
	for role := range contract.Materials {
		materialNames[role] = role
		// This is the exact ANG material-binding protocol, not an exemption
		// based on a generated role-name prefix.
		materialNames["ANG_ROLE__"+role] = role
	}
	if bindings, err := presentation.LookupPath(cue.ParsePath("finishing.material_bindings")).Fields(); err == nil {
		for bindings.Next() {
			if role, e := bindings.Value().String(); e == nil {
				materialNames[bindings.Selector().Unquoted()] = role
			}
		}
	}
	usage := map[string]map[string]bool{}
	add := func(role, object, component string) {
		if role == "" {
			return
		}
		if object == "" {
			object = "<unnamed>"
		}
		if usage[role] == nil {
			usage[role] = map[string]bool{}
		}
		usage[role][fmt.Sprintf("%s (ang_id %s)", object, component)] = true
	}
	for _, component := range report.Components {
		for object, role := range component.AppliedMaterialRoles {
			add(role, object, component.ID)
		}
		for _, object := range component.Objects {
			for _, material := range object.Materials {
				add(materialNames[material], object.Name, component.ID)
			}
		}
	}
	generatedRoles := map[string]bool{}
	if contract.Architecture != nil {
		for _, role := range contract.Architecture.ConcreteRoles {
			generatedRoles[role] = true
		}
	}
	roles := []string{}
	for role := range usage {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	selected := map[string][]string{}
	for _, role := range roles {
		objects := []string{}
		for object := range usage[role] {
			objects = append(objects, object)
		}
		sort.Strings(objects)
		if _, ok := contract.Materials[role]; !ok {
			return fmt.Errorf("art material role %q on Blender/art object(s) %s is absent from the current contract", role, strings.Join(objects, ", "))
		}
		if !presentation.LookupPath(cue.MakePath(cue.Str("finishing"), cue.Str("materials"), cue.Str(role))).Exists() {
			if !generatedRoles[role] {
				return fmt.Errorf("art material role %q on Blender/art object(s) %s has no authored CUE material or compiler-resolved architecture scope", role, strings.Join(objects, ", "))
			}
			// The initial guard already checked every default, zone and object
			// concrete scope. Match the actual resolver map; never guess IDs.
			continue
		}
		selected[role] = objects
	}
	generated := profile.GeneratedCUE
	if !filepath.IsAbs(generated) {
		generated = filepath.Join(packageDir, generated)
	}
	abs, err := filepath.Abs(generated)
	if err != nil {
		return err
	}
	guard := poseSourceGuard{generated: filepath.Clean(abs), derived: map[string]bool{}, dependencies: map[string]bool{}}
	guard.roots = posePackageRoots(append(sourceRoots, presentation)...)
	return guard.checkFinishingPoses(presentation, selected)
}
