package godot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

type KitSource struct {
	Asset string `json:"source_asset,omitempty"`
	Basis string `json:"asset_basis,omitempty"`
}

type KitProposal struct {
	Passport map[string]any `json:"proposed_passport,omitempty"`
	ID       string         `json:"id"`
	Status   string         `json:"status"`
	Purpose  string         `json:"purpose"`
	Missing  string         `json:"missing"`
}

// KitDocumentation belongs to CUE authoring documentation, outside the SDK
// structural contract. Empty material_roles means unassigned, never inferred
// from a module name, legacy surface ID or an existing resource on disk.
type KitDocumentation struct {
	Purpose       string       `json:"purpose"`
	SourceRefs    []string     `json:"source_refs"`
	MaterialRoles []string     `json:"material_roles,omitempty"`
	Locators      []KitLocator `json:"locators,omitempty"`
}

type KitLocator struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"` // port/socket are semantic passport datums, not Blender empties.
	SourceRef  string `json:"source_ref"`
	LayoutID   string `json:"layout_id,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	PortID     string `json:"port_id,omitempty"`
}

type KitBinding struct {
	LocatorSpecs map[string][]KitLocator

	Missing     []KitProposal
	Sources     map[string]KitSource
	Name        string
	LayoutID    string
	Placement   placement.Authoring
	Models      map[string]string
	Finishing   *Finishing
	Locators    map[string][]string
	ObjectRoles map[string]string
}

type KitUse struct {
	LocatorBindings []KitLocator `json:"locator_bindings,omitempty"`
	SourceAsset     string       `json:"source_asset,omitempty"`
	AssetBasis      string       `json:"asset_basis,omitempty"`
	LayoutID        string       `json:"layout_id"`
	InstanceID      string       `json:"instance_id"`
	Purpose         string       `json:"purpose"`
	ModelRef        string       `json:"model_ref,omitempty"`
	Resource        string       `json:"resource,omitempty"`
	Bound           bool         `json:"bound"`
	MaterialRole    string       `json:"material_role,omitempty"`
	Locators        []string     `json:"locators"`
}

type KitEntry struct {
	Purpose         string       `json:"purpose"`
	PurposeSources  []string     `json:"purpose_sources"`
	MaterialRoles   []string     `json:"material_roles"`
	SurfaceIDs      []string     `json:"surface_ids"`
	Locators        []KitLocator `json:"locators"`
	MetadataMissing []string     `json:"metadata_missing"`

	ID           string                `json:"id"`
	GeometryID   string                `json:"geometry_id"`
	SizeMM       [3]int                `json:"size_mm"`
	Footprint    []locationsdk.Cell    `json:"footprint"`
	Blocked      []locationsdk.Cell    `json:"blocked"`
	UseClearance []locationsdk.Cell    `json:"use_clearance"`
	Maintenance  []locationsdk.Cell    `json:"maintenance_clearance"`
	Ports        []locationsdk.Port    `json:"ports"`
	Variants     []locationsdk.Variant `json:"variants"`
	Uses         []KitUse              `json:"uses"`
	Art          []KitUse              `json:"art_bindings"`
	Bound        bool                  `json:"bound"`
}

type KitCatalog struct {
	Proposals    []KitProposal            `json:"missing_kit_proposals"`
	Schema       string                   `json:"schema"`
	UnitMM       int                      `json:"unit_mm"`
	Modules      []KitEntry               `json:"modules"`
	Extras       []KitUse                 `json:"extra_art"`
	Materials    map[string]Material      `json:"materials"`
	Architecture map[string]*Architecture `json:"architecture"`
	Missing      []string                 `json:"unbound_modules"`
}

func BuildKitCatalog(input locationsdk.Input, bindings []KitBinding, clientRoot string, documentation ...map[string]KitDocumentation) (KitCatalog, error) {
	docs := map[string]KitDocumentation{}
	knownModules := map[string]bool{}
	for _, m := range input.Catalog.Modules {
		knownModules[m.ID] = true
	}
	for _, table := range documentation {
		for id, doc := range table {
			if !knownModules[id] {
				return KitCatalog{}, fmt.Errorf("kit_documentation[%q]: unknown SDK module; fix the CUE documentation key", id)
			}
			if strings.TrimSpace(doc.Purpose) == "" || len(doc.SourceRefs) == 0 {
				return KitCatalog{}, fmt.Errorf("kit_documentation[%q]: purpose and source_refs are required", id)
			}
			if _, duplicate := docs[id]; duplicate {
				return KitCatalog{}, fmt.Errorf("duplicate kit_documentation key %q", id)
			}
			docs[id] = doc
		}
	}
	out := KitCatalog{Schema: "ang.kit-catalog.v1", UnitMM: input.Catalog.UnitMM, Modules: []KitEntry{}, Extras: []KitUse{}, Materials: map[string]Material{}, Architecture: map[string]*Architecture{}, Missing: []string{}}
	geometry := map[string]locationsdk.Geometry{}
	for _, g := range input.Catalog.Geometries {
		geometry[g.ID] = g
	}
	layouts := map[string]locationsdk.Layout{}
	for _, l := range input.Layouts {
		layouts[l.ID] = l
	}
	exists := func(resource string) bool {
		if !strings.HasPrefix(resource, "res://") {
			return false
		}
		rel := strings.TrimPrefix(resource, "res://")
		if filepath.Clean(rel) != rel || filepath.IsAbs(rel) || strings.HasPrefix(rel, "../") {
			return false
		}
		info, err := os.Stat(filepath.Join(clientRoot, rel))
		return err == nil && !info.IsDir()
	}
	specs := map[string][]KitBinding{}
	for _, b := range bindings {
		out.Proposals = append(out.Proposals, b.Missing...)
		if _, ok := layouts[b.LayoutID]; !ok {
			return out, fmt.Errorf("kit catalog binding %q: unknown layout %q", b.Name, b.LayoutID)
		}
		specs[b.LayoutID] = append(specs[b.LayoutID], b)
		f, err := ResolveArchitecture(b.Finishing)
		if err != nil {
			return out, err
		}
		if f != nil {
			out.Architecture[b.Name] = f.Architecture
			for role, m := range f.Materials {
				out.Materials[b.Name+"/"+role] = m
			}
		}
		for _, e := range b.Placement.Extras {
			resource := b.Models[e.ModelRef]
			source := b.Sources[e.InstanceID]
			out.Extras = append(out.Extras, KitUse{SourceAsset: source.Asset, AssetBasis: source.Basis, LayoutID: b.LayoutID, InstanceID: e.InstanceID, Purpose: e.Reason, ModelRef: e.ModelRef, Resource: resource, Bound: exists(resource), MaterialRole: b.ObjectRoles[e.InstanceID], Locators: append([]string{}, b.Locators[e.InstanceID]...)})
		}
	}
	for _, module := range input.Catalog.Modules {
		g, ok := geometry[module.GeometryID]
		if !ok {
			return out, fmt.Errorf("kit catalog %s: unknown geometry %s", module.ID, module.GeometryID)
		}
		row := KitEntry{ID: module.ID, GeometryID: g.ID, SizeMM: [3]int{g.Width * out.UnitMM, g.Depth * out.UnitMM, g.Height * out.UnitMM}, Footprint: g.Footprint, Blocked: g.Blocked, UseClearance: g.UseClearance, Maintenance: g.MaintenanceClearance, Ports: g.Ports, Variants: module.Variants, Uses: []KitUse{}, Art: []KitUse{}}
		for _, b := range bindings {
			if ref := b.Placement.ModuleModels[module.ID]; ref != "" {
				resource := b.Models[ref]
				bound := exists(resource)
				row.Art = append(row.Art, KitUse{LayoutID: b.LayoutID, ModelRef: ref, Resource: resource, Bound: bound, Locators: []string{}})
				row.Bound = row.Bound || bound
			}
		}
		for _, l := range input.Layouts {
			for _, i := range l.Instances {
				if i.ModuleID != module.ID {
					continue
				}
				use := KitUse{LayoutID: l.ID, InstanceID: i.ID, Purpose: i.Reason, Locators: []string{}}
				for _, slot := range l.TextSlots {
					if slot.Instance == i.ID {
						name := slot.Anchor
						if name == "" {
							name = slot.ID
						}
						use.Locators = append(use.Locators, name)
						use.LocatorBindings = append(use.LocatorBindings, KitLocator{Name: name, Kind: "text", SourceRef: "sdk.layouts/" + l.ID + "/text_slots/" + slot.ID})
					}
				}
				bs := specs[l.ID]
				if len(bs) == 0 {
					row.Uses = append(row.Uses, use)
				}
				for _, b := range bs {
					u := use
					ref := module.ID
					if v, ok := b.Placement.ModuleModels[module.ID]; ok {
						ref = v
					}
					if v, ok := b.Placement.Instances[i.ID]; ok && v.ModelRef != nil {
						ref = *v.ModelRef
					}
					u.ModelRef = ref
					u.Resource = b.Models[ref]
					u.Bound = exists(u.Resource)
					u.MaterialRole = b.ObjectRoles[i.ID]
					source := b.Sources[i.ID]
					u.SourceAsset = source.Asset
					u.AssetBasis = source.Basis
					if u.SourceAsset == "" && (strings.HasSuffix(u.Resource, ".glb") || strings.HasSuffix(u.Resource, ".gltf")) {
						u.SourceAsset = u.Resource
						u.AssetBasis = "source_plan"
					}
					u.Locators = append(append([]string{}, use.Locators...), b.Locators[i.ID]...)
					sort.Strings(u.Locators)
					row.Uses = append(row.Uses, u)
					row.Bound = row.Bound || u.Bound
				}
			}
		}
		for i := range row.Art {
			for _, u := range row.Uses {
				if row.Art[i].ModelRef == u.ModelRef && u.SourceAsset != "" {
					row.Art[i].SourceAsset = u.SourceAsset
					row.Art[i].AssetBasis = u.AssetBasis
					break
				}
			}
		}
		fillKitMetadata(&row, g, docs[module.ID], bindings)
		if row.Purpose == "" {
			return out, fmt.Errorf("kit catalog %s: no authored purpose; add kit_documentation[%q] with source_refs or a layout instance reason", module.ID, module.ID)
		}
		if !row.Bound {
			out.Missing = append(out.Missing, module.ID)
		}
		sort.Slice(row.Uses, func(i, j int) bool {
			return row.Uses[i].LayoutID+"/"+row.Uses[i].InstanceID < row.Uses[j].LayoutID+"/"+row.Uses[j].InstanceID
		})
		out.Modules = append(out.Modules, row)
	}
	sort.Slice(out.Modules, func(i, j int) bool { return out.Modules[i].ID < out.Modules[j].ID })
	sort.Slice(out.Extras, func(i, j int) bool {
		return out.Extras[i].LayoutID+"/"+out.Extras[i].InstanceID < out.Extras[j].LayoutID+"/"+out.Extras[j].InstanceID
	})
	sort.Strings(out.Missing)
	return out, nil
}

// fillKitMetadata aggregates declarations only. SDK surface assignments are
// published separately from Godot material roles, and absent assignments remain
// explicit gaps even when old illustrated variants or bound art exist.
func fillKitMetadata(row *KitEntry, g locationsdk.Geometry, doc KitDocumentation, bindings []KitBinding) {
	row.Purpose = doc.Purpose
	row.PurposeSources = append([]string{}, doc.SourceRefs...)
	row.MaterialRoles = append([]string{}, doc.MaterialRoles...)
	row.SurfaceIDs = []string{}
	row.Locators = append([]KitLocator{}, doc.Locators...)
	row.MetadataMissing = []string{}
	reasons := []string{}
	for _, use := range row.Uses {
		if use.Purpose != "" {
			reasons = append(reasons, use.Purpose)
			if doc.Purpose == "" {
				row.PurposeSources = append(row.PurposeSources, "sdk.layouts/"+use.LayoutID+"/instances/"+use.InstanceID+"/reason")
			}
		}
		if use.MaterialRole != "" {
			row.MaterialRoles = append(row.MaterialRoles, use.MaterialRole)
		}
		typedNames := map[string]bool{}
		for _, locator := range use.LocatorBindings {
			locator.LayoutID = use.LayoutID
			locator.InstanceID = use.InstanceID
			row.Locators = append(row.Locators, locator)
			typedNames[locator.Name] = true
		}
		for _, b := range bindings {
			if b.LayoutID == use.LayoutID {
				for _, locator := range b.LocatorSpecs[use.InstanceID] {
					typedNames[locator.Name] = true
				}
			}
		}
		for _, name := range use.Locators {
			if name != "" && !typedNames[name] {
				row.Locators = append(row.Locators, KitLocator{Name: name, Kind: "authored", SourceRef: "binding.locators", LayoutID: use.LayoutID, InstanceID: use.InstanceID})
			}
		}
		for _, b := range bindings {
			if b.LayoutID == use.LayoutID {
				for _, locator := range b.LocatorSpecs[use.InstanceID] {
					locator.LayoutID = use.LayoutID
					locator.InstanceID = use.InstanceID
					row.Locators = append(row.Locators, locator)
				}
				if b.Finishing != nil && b.Finishing.ObjectRoles[use.InstanceID] != "" {
					row.MaterialRoles = append(row.MaterialRoles, b.Finishing.ObjectRoles[use.InstanceID])
				}
			}
		}
	}
	if row.Purpose == "" {
		row.Purpose = strings.Join(uniqueKitStrings(reasons), " ")
	}
	for _, variant := range row.Variants {
		if variant.SurfaceID != "" {
			row.SurfaceIDs = append(row.SurfaceIDs, variant.SurfaceID)
		}
	}
	for _, port := range g.Ports {
		row.Locators = append(row.Locators, KitLocator{Name: port.ID, Kind: "port", PortID: port.ID, SourceRef: "sdk.catalog.geometries/" + g.ID + "/ports/" + port.ID})
	}
	for _, socket := range g.Sockets {
		row.Locators = append(row.Locators, KitLocator{Name: socket.ID, Kind: "socket", SourceRef: "sdk.catalog.geometries/" + g.ID + "/sockets/" + socket.ID})
	}
	row.PurposeSources = uniqueKitStrings(row.PurposeSources)
	row.MaterialRoles = uniqueKitStrings(row.MaterialRoles)
	row.SurfaceIDs = uniqueKitStrings(row.SurfaceIDs)
	locators := map[string]KitLocator{}
	for _, l := range row.Locators {
		if l.Name != "" {
			key := l.LayoutID + "/" + l.InstanceID + "/" + l.Kind + "/" + l.Name
			locators[key] = l
		}
	}
	row.Locators = []KitLocator{}
	keys := []string{}
	for key := range locators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hasBlenderLocator := false
	for _, key := range keys {
		l := locators[key]
		row.Locators = append(row.Locators, l)
		if l.Kind != "port" && l.Kind != "socket" {
			hasBlenderLocator = true
		}
	}
	if len(row.MaterialRoles) == 0 {
		row.MetadataMissing = append(row.MetadataMissing, "godot_material_roles")
	}
	if !hasBlenderLocator {
		row.MetadataMissing = append(row.MetadataMissing, "blender_locators")
	}
}

func uniqueKitStrings(values []string) []string {
	seen := map[string]bool{}
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			seen[v] = true
		}
	}
	out := []string{}
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func kitMetadataNames(values []string) string {
	if len(values) == 0 {
		return "not assigned"
	}
	safe := strings.NewReplacer("|", "\\|", "\n", " ").Replace(strings.Join(values, "`, `"))
	return "`" + safe + "`"
}

func WriteKitCatalog(outDir string, c KitCatalog) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	var text strings.Builder
	fmt.Fprintln(&text, "# Generated location kit catalog\n\nCUE passports and explicit Godot art bindings. Bound means the referenced scene exists; it does not certify visual or engineering acceptance. Sizes and ports remain the source CUE contract.")
	fmt.Fprintln(&text, "| Module | Size mm | Blocked / footprint cells | Ports | Art | Uses |\n|---|---|---:|---:|---|---:|")
	clean := func(v string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(v) }
	for _, m := range c.Modules {
		bound := "unbound"
		if m.Bound {
			bound = "bound"
		}
		fmt.Fprintf(&text, "| `%s` | %d × %d × %d | %d / %d | %d | %s | %d |\n", clean(m.ID), m.SizeMM[0], m.SizeMM[1], m.SizeMM[2], len(m.Blocked), len(m.Footprint), len(m.Ports), bound, len(m.Uses))
	}
	fmt.Fprintln(&text, "\n## Purpose, approaches and locators\n\nFull footprint, blocked cells, port openings, use/maintenance clearance and per-module purpose/roles/locator provenance are in [kit-catalog.json](kit-catalog.json). Ports/sockets are semantic datums; only explicit CUE text/pipeline records declare Blender locators. Empty Godot roles are unassigned, never guessed from legacy illustrated surfaces.")
	for _, m := range c.Modules {
		fmt.Fprintf(&text, "### `%s`\n\n%s\n\n", clean(m.ID), clean(m.Purpose))
		fmt.Fprintf(&text, "Sources: `%s`. Godot material roles: %s. SDK surfaces: %s.\n\n", clean(strings.Join(m.PurposeSources, "`, `")), kitMetadataNames(m.MaterialRoles), kitMetadataNames(m.SurfaceIDs))
		if len(m.MetadataMissing) > 0 {
			fmt.Fprintf(&text, "Unassigned metadata: `%s`. No material or Blender empty is inferred from a passport port.\n\n", clean(strings.Join(m.MetadataMissing, "`, `")))
		}
		for _, l := range m.Locators {
			fmt.Fprintf(&text, "- Locator `%s` (%s): `%s`", clean(l.Name), clean(l.Kind), clean(l.SourceRef))
			if l.LayoutID != "" {
				fmt.Fprintf(&text, " in `%s/%s`", clean(l.LayoutID), clean(l.InstanceID))
			}
			fmt.Fprintln(&text)
		}
		if len(m.Locators) > 0 {
			fmt.Fprintln(&text)
		}
		for _, u := range m.Uses {
			fmt.Fprintf(&text, "- `%s/%s`: %s", clean(u.LayoutID), clean(u.InstanceID), clean(u.Purpose))
			if u.Resource != "" {
				fmt.Fprintf(&text, " Art `%s` (bound=%t).", clean(u.Resource), u.Bound)
			}
			if len(u.Locators) > 0 {
				fmt.Fprintf(&text, " Locators: `%s`.", clean(strings.Join(u.Locators, "`, `")))
			}
			fmt.Fprintln(&text)
		}
		fmt.Fprintln(&text)
	}
	if len(c.Proposals) > 0 {
		fmt.Fprintln(&text, "## Missing kit proposals")
		fmt.Fprintln(&text)
		for _, p := range c.Proposals {
			fmt.Fprintf(&text, "- `%s` (%s): %s Missing: %s\n", clean(p.ID), clean(p.Status), clean(p.Purpose), clean(p.Missing))
			if len(p.Passport) > 0 {
				raw, err := json.MarshalIndent(p.Passport, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintf(&text, "\n```json\n%s\n```\n\n", raw)
			}
		}
		fmt.Fprintln(&text)
	}
	for name, payload := range map[string][]byte{"kit-catalog.json": append(data, '\n'), "kit-catalog.md": []byte(text.String())} {
		if err := os.WriteFile(filepath.Join(outDir, name), payload, 0644); err != nil {
			return err
		}
	}
	return nil
}
