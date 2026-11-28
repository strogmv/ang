package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// GodotWorldText is a resolved presentation contract. All vectors use placement's
// world frame; source-local node names remain attachment references, never IDs
// interpreted as a visibility or gameplay rule.
type GodotWorldText struct {
	Schema     string               `json:"schema"`
	WorldFrame string               `json:"world_frame"`
	Fonts      []WorldTextFontRef   `json:"fonts"`
	Slots      []GodotWorldTextSlot `json:"slots"`
}
type GodotWorldTextSlot struct {
	WorldTextSlotDrawn
	Frame   GodotTextFrame         `json:"frame"`
	Backing *locationsdk.TextPlate `json:"backing,omitempty"`
}
type GodotTextFrame struct {
	Center placement.Vec3 `json:"center"`
	Right  placement.Vec3 `json:"right"`
	Up     placement.Vec3 `json:"up"`
	Normal placement.Vec3 `json:"normal"`
}

func transformTextVector(t placement.Transform, p placement.Vec3) placement.Vec3 {
	b := t.BasisColumns()
	var out placement.Vec3
	for i := range 3 {
		for j := range 3 {
			out[i] += b[j][i] * p[j]
		}
	}
	return out
}
func textFaceVector(face string) placement.Vec3 {
	switch face {
	case "north":
		return placement.Vec3{0, 0, -1}
	case "south":
		return placement.Vec3{0, 0, 1}
	case "east":
		return placement.Vec3{1, 0, 0}
	default:
		return placement.Vec3{-1, 0, 0}
	}
}

func godotTextFrame(level locationsdk.RuntimeLevel, s WorldTextSlotDrawn) (GodotTextFrame, error) {
	floor := float64(level.Elevation*level.UnitMM) / 1000
	t := placement.PlanTransform(placement.Vec3{0, floor, 0}, 0)
	local := placement.Vec3{float64(s.CenterMM.X) / 1000, float64(s.CenterMM.Z) / 1000, float64(s.CenterMM.Y) / 1000}
	n := textFaceVector(s.Face)
	if s.Vista != "" {
		found := false
		for _, v := range level.Vistas {
			if v.ID != s.Vista {
				continue
			}
			found = true
			unit := float64(level.UnitMM) / 1000
			o := placement.Vec3{float64(v.At.X) * unit, 0, float64(v.At.Y) * unit}
			f, r := textFaceVector(string(v.Direction)), placement.Vec3{1, 0, 0}
			switch v.Direction {
			case locationsdk.South:
				o[2] += unit
			case locationsdk.East:
				o[0] += unit
				r = placement.Vec3{0, 0, 1}
			case locationsdk.West:
				r = placement.Vec3{0, 0, 1}
			}
			inset := float64(max(0, v.Inset)) * unit
			length := float64(v.LengthMM)/1000 + inset
			w := float64(v.Width) * unit
			a := math.Max(2, length-w-1.5)
			b := 0.03
			n = r
			if v.End == "turn_right" {
				b = w - 0.03
				for j := range n {
					n[j] = -n[j]
				}
			}
			for j := range local {
				local[j] = o[j] + f[j]*(a-inset) + r[j]*b
			}
			local[1] = 2.9
		}
		if !found {
			return GodotTextFrame{}, fmt.Errorf("world text: missing vista %s", s.Vista)
		}
	} else if s.Instance != "" {
		found := false
		for _, i := range level.Instances {
			if i.ID == s.Instance {
				t = placement.Pose(i.At, i.Width, i.Depth, floor, i.Rotation)
				found = true
				break
			}
		}
		if !found {
			return GodotTextFrame{}, fmt.Errorf("world text: missing instance %s", s.Instance)
		}
	}
	// Source reading right = cross(up,-normal); placement owns the reflection.
	right := placement.Vec3{-n[2], 0, n[0]}
	c := transformTextVector(t, local)
	for j := range c {
		c[j] += t.Position[j]
	}
	return GodotTextFrame{c, transformTextVector(t, right), transformTextVector(t, placement.Vec3{0, 1, 0}), transformTextVector(t, n)}, nil
}

func godotWorldText(c Compiled) (GodotWorldText, error) {
	out := GodotWorldText{Schema: "ang.godot.world_text.v1", WorldFrame: placement.WorldFrame, Slots: []GodotWorldTextSlot{}}
	if c.WorldTextDrawn == nil {
		return out, nil
	}
	out.Fonts = c.WorldTextDrawn.Fonts
	for _, s := range c.WorldTextDrawn.Slots {
		found := false
		for _, l := range c.LocationSDK.Levels {
			if l.ID != s.LevelID {
				continue
			}
			found = true
			f, err := godotTextFrame(l, s)
			if err != nil {
				return out, err
			}
			drawn := GodotWorldTextSlot{WorldTextSlotDrawn: s, Frame: f}
			// The vista builder in the reference client supplies this enamelled
			// wall plate, separately from explicit text-slot railing plates.
			if s.Vista != "" {
				drawn.Backing = &locationsdk.TextPlate{DepthMM: 30, Color: "#D1CCB8"}
			}
			out.Slots = append(out.Slots, drawn)
		}
		if !found {
			return out, fmt.Errorf("world text: missing level %s", s.LevelID)
		}
	}
	return out, nil
}

type worldTextAsset struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
type worldTextAssetManifest struct {
	Schema string           `json:"schema"`
	Files  []worldTextAsset `json:"files"`
}

// Validate the producer's hashes BEFORE delivery; consumers verify the same
// manifest. Assets are generated, font TTFs remain art-source inputs.
func worldTextAssets(project string, c WorldTextCatalog) (map[string][]byte, error) {
	files := map[string][]byte{}
	if len(c.Fonts) == 0 {
		return files, nil
	}
	root := filepath.Join(project, c.MetricsDir)
	raw, err := os.ReadFile(filepath.Join(root, "assets-manifest.json"))
	if err != nil {
		return nil, err
	}
	var m worldTextAssetManifest
	if err = json.Unmarshal(raw, &m); err != nil || m.Schema != "ang.world_text.assets.v1" {
		return nil, fmt.Errorf("invalid world-text asset manifest")
	}
	for _, a := range m.Files {
		if a.File == "" || filepath.IsAbs(a.File) || filepath.ToSlash(filepath.Clean(a.File)) != a.File || a.File == ".." || len(a.File) > 3 && a.File[:3] == "../" {
			return nil, fmt.Errorf("invalid world text asset path %q", a.File)
		}
		if _, ok := files[a.File]; ok {
			return nil, fmt.Errorf("duplicate world text asset %s", a.File)
		}
		b, e := os.ReadFile(filepath.Join(root, a.File))
		if e != nil {
			return nil, e
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != a.SHA256 {
			return nil, fmt.Errorf("world text asset SHA mismatch: %s", a.File)
		}
		files[a.File] = b
	}
	for _, f := range c.Fonts {
		for _, ext := range []string{".json", ".sdf.bytes"} {
			if _, ok := files[filepath.Base(f.Metrics)+ext]; !ok {
				return nil, fmt.Errorf("world text manifest missing %s%s", f.Style, ext)
			}
		}
	}
	if _, ok := files["fonts-manifest.json"]; !ok {
		return nil, fmt.Errorf("world text fonts manifest missing")
	}
	var fonts struct {
		Fonts []struct {
			LicenseFile string `json:"license_file"`
		}
	}
	if err = json.Unmarshal(files["fonts-manifest.json"], &fonts); err != nil {
		return nil, err
	}
	if len(fonts.Fonts) != len(c.Fonts) {
		return nil, fmt.Errorf("world text font licence count mismatch")
	}
	for _, f := range fonts.Fonts {
		if _, ok := files[f.LicenseFile]; !ok || f.LicenseFile == "" {
			return nil, fmt.Errorf("world text OFL licence missing")
		}
	}
	files["assets-manifest.json"] = raw
	return files, nil
}
func writeGodotWorldText(project string, c Compiled) error {
	if len(c.GodotTargets) == 0 {
		return nil
	}
	runtime, err := godotWorldText(c)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(runtime, "", " ")
	if err != nil {
		return err
	}
	assets, err := worldTextAssets(project, c.WorldText)
	if err != nil {
		return err
	}
	for _, target := range c.GodotTargets {
		dir := filepath.Join(project, target, "generated/world_text")
		for file, b := range assets {
			path := filepath.Join(dir, file)
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err = writeGeneratedFile(path, b, 0644); err != nil {
				return err
			}
		}
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err = writeGeneratedFile(filepath.Join(dir, "runtime.json"), payload, 0644); err != nil {
			return err
		}
		for _, file := range []string{"world_text.gd", "world_text.gdshader", "world_text_deformation.gdshaderinc"} {
			b, e := sources.ReadFile("templates/godot_" + file + ".tmpl")
			if e != nil {
				return e
			}
			if err = writeGeneratedFile(filepath.Join(dir, file), b, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// GenerateGodot refreshes only the Godot presentation from live CUE; it never
// reads a cached world snapshot or writes server/legacy engine outputs.
func GenerateGodot(project string) error {
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	endInventory := beginGeneratedOutputs()
	defer endInventory()
	compiled, def, err := readCompiled(project)
	if err != nil {
		return err
	}
	if err = writeGodotScreens(project, compiled, def); err != nil {
		return err
	}
	compiled.WorldTextDrawn, err = compileWorldText(project, compiled)
	if err != nil {
		return err
	}
	if err = writeGodotTranslations(project, compiled.GodotTargets, compiled.Localization); err != nil {
		return err
	}
	if err = writeGodotTheme(project, compiled.GodotTheme); err != nil {
		return err
	}
	if err := writeGodotWorldText(project, compiled); err != nil {
		return err
	}
	return finishGeneratedOutputs(project)
}
