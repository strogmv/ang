package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// GodotTheme is engine-native resource authoring, independent of any game skin.
// Property order is explicit because it also makes resource diffs reviewable.
type GodotTheme struct {
	ResourceName string               `json:"resource_name"`
	Target       string               `json:"target"`
	DefaultSkin  string               `json:"default_skin"`
	MinTextPX    int                  `json:"min_text_px"`
	Fonts        []GodotThemeAsset    `json:"fonts"`
	Textures     []GodotThemeAsset    `json:"textures"`
	Tokens       map[string]float64   `json:"tokens"`
	Skins        []GodotThemeSkin     `json:"skins"`
	Resources    []GodotThemeResource `json:"resources"`
	Properties   []GodotThemeProperty `json:"properties"`
	Contrast     []GodotThemeContrast `json:"contrast"`
}
type GodotThemeAsset struct {
	ID            string `json:"id"`
	File          string `json:"file"`
	SHA256        string `json:"sha256"`
	Role          string `json:"role,omitempty"`
	Family        string `json:"family,omitempty"`
	License       string `json:"license,omitempty"`
	LicenseFile   string `json:"license_file,omitempty"`
	LicenseSHA256 string `json:"license_sha256,omitempty"`
	Cyrillic      bool   `json:"cyrillic,omitempty"`
}
type GodotThemeSkin struct {
	ID    string            `json:"id"`
	Roles map[string]string `json:"roles"`
}
type GodotThemeResource struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Properties []GodotThemeProperty `json:"properties"`
}
type GodotThemeProperty struct {
	Name       string   `json:"name"`
	Color      string   `json:"color,omitempty"`
	Alpha      *float64 `json:"alpha,omitempty"`
	Token      string   `json:"token,omitempty"`
	Float      bool     `json:"float,omitempty"`
	External   string   `json:"external,omitempty"`
	Internal   string   `json:"internal,omitempty"`
	StringName string   `json:"string_name,omitempty"`
}
type GodotThemeContrast struct {
	Name       string `json:"name"`
	Foreground string `json:"foreground"`
	Background string `json:"background"`
}
type godotContrastResult struct {
	Skin   string  `json:"skin"`
	Pair   string  `json:"pair"`
	Ratio  float64 `json:"ratio"`
	WCAGAA bool    `json:"wcag_aa_4_5"`
}

var godotResourceID = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
var godotPropertyName = regexp.MustCompile(`^[A-Za-z0-9_/]+$`)

func godotRGB(value string) ([3]float64, error) {
	var rgb [3]float64
	if len(value) != 7 || value[0] != '#' {
		return rgb, fmt.Errorf("invalid sRGB colour %q", value)
	}
	raw, err := hex.DecodeString(value[1:])
	if err != nil {
		return rgb, err
	}
	for i := range rgb {
		rgb[i] = float64(raw[i]) / 255
	}
	return rgb, nil
}
func godotLuminance(rgb [3]float64) float64 {
	for i, c := range rgb {
		if c <= 0.04045 {
			rgb[i] = c / 12.92
		} else {
			rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
}
func godotContrast(a, b [3]float64) float64 {
	x, y := godotLuminance(a), godotLuminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

func (t GodotTheme) render(skin GodotThemeSkin) ([]byte, error) {
	ext, sub := map[string]bool{}, map[string]bool{}
	for _, assets := range [][]GodotThemeAsset{t.Fonts, t.Textures} {
		for _, a := range assets {
			if !godotResourceID.MatchString(a.ID) || ext[a.ID] {
				return nil, fmt.Errorf("invalid/duplicate external resource %s", a.ID)
			}
			ext[a.ID] = true
		}
	}
	for _, r := range t.Resources {
		if !godotResourceID.MatchString(r.ID) || sub[r.ID] {
			return nil, fmt.Errorf("invalid/duplicate subresource %s", r.ID)
		}
		sub[r.ID] = true
	}
	lines := []string{fmt.Sprintf("[gd_resource type=\"Theme\" load_steps=%d format=3]", len(ext)+len(sub)+1)}
	for _, a := range t.Fonts {
		lines = append(lines, fmt.Sprintf("[ext_resource type=\"FontFile\" path=%s id=%s]", strconv.Quote(a.File), strconv.Quote(a.ID)))
	}
	for _, a := range t.Textures {
		lines = append(lines, fmt.Sprintf("[ext_resource type=\"Texture2D\" path=%s id=%s]", strconv.Quote(a.File), strconv.Quote(a.ID)))
	}
	properties := func(props []GodotThemeProperty) error {
		names := map[string]bool{}
		for _, p := range props {
			if !godotPropertyName.MatchString(p.Name) || names[p.Name] {
				return fmt.Errorf("invalid/duplicate property %s", p.Name)
			}
			names[p.Name] = true
			count := 0
			for _, v := range []string{p.Color, p.Token, p.External, p.Internal, p.StringName} {
				if v != "" {
					count++
				}
			}
			if count != 1 {
				return fmt.Errorf("property %s needs exactly one typed value", p.Name)
			}
			var value string
			switch {
			case p.Color != "":
				color, ok := skin.Roles[p.Color]
				if !ok {
					return fmt.Errorf("skin %s has no colour role %s", skin.ID, p.Color)
				}
				rgb, err := godotRGB(color)
				if err != nil {
					return err
				}
				alpha := 1.0
				if p.Alpha != nil {
					alpha = *p.Alpha
				}
				if alpha < 0 || alpha > 1 {
					return fmt.Errorf("invalid alpha")
				}
				value = fmt.Sprintf("Color(%.5f, %.5f, %.5f, %.5f)", rgb[0], rgb[1], rgb[2], alpha)
			case p.Token != "":
				n, ok := t.Tokens[p.Token]
				if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
					return fmt.Errorf("unknown/invalid numeric token %s", p.Token)
				}
				if strings.HasSuffix(p.Name, "font_size") && n < float64(t.MinTextPX) {
					return fmt.Errorf("Steam Deck text %s = %g px is below %d px", p.Name, n, t.MinTextPX)
				}
				value = strconv.FormatFloat(n, 'f', -1, 64)
				if p.Float && !strings.Contains(value, ".") {
					value += ".0"
				}
			case p.External != "":
				if !ext[p.External] {
					return fmt.Errorf("unknown external resource %s", p.External)
				}
				value = "ExtResource(" + strconv.Quote(p.External) + ")"
			case p.Internal != "":
				if !sub[p.Internal] {
					return fmt.Errorf("unknown subresource %s", p.Internal)
				}
				value = "SubResource(" + strconv.Quote(p.Internal) + ")"
			default:
				value = "&" + strconv.Quote(p.StringName)
			}
			lines = append(lines, p.Name+"="+value)
		}
		return nil
	}
	for _, r := range t.Resources {
		switch r.Type {
		case "StyleBoxFlat", "StyleBoxTexture", "StyleBoxEmpty":
		default:
			return nil, fmt.Errorf("unsupported Theme resource %s", r.Type)
		}
		lines = append(lines, fmt.Sprintf("[sub_resource type=%s id=%s]", strconv.Quote(r.Type), strconv.Quote(r.ID)))
		if err := properties(r.Properties); err != nil {
			return nil, err
		}
	}
	lines = append(lines, "[resource]")
	if err := properties(t.Properties); err != nil {
		return nil, err
	}
	return []byte(strings.Join(lines, "\n\n") + "\n"), nil
}

func godotThemeAssetCheck(root, file, digest string) error {
	if !strings.HasPrefix(file, "res://") || strings.Contains(strings.TrimPrefix(file, "res://"), "..") {
		return fmt.Errorf("Theme asset must be a local res:// path: %s", file)
	}
	raw, err := os.ReadFile(filepath.Join(root, strings.TrimPrefix(file, "res://")))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if digest != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("Theme asset SHA mismatch: %s", file)
	}
	return nil
}

func writeGodotTheme(project string, t *GodotTheme) error {
	if t == nil {
		return nil
	}
	if !godotResourceID.MatchString(t.ResourceName) || t.Target == "" || filepath.IsAbs(t.Target) || t.MinTextPX < 12 || len(t.Skins) == 0 {
		return fmt.Errorf("Theme requires relative target, skins and Steam Deck minimum >=12 px")
	}
	root := filepath.Join(project, t.Target)
	for _, a := range t.Fonts {
		if a.Family == "" || a.Role == "" || a.License != "OFL-1.1" || !a.Cyrillic {
			return fmt.Errorf("Theme font needs family/role/OFL-1.1/Cyrillic declaration")
		}
		if err := godotThemeAssetCheck(root, a.File, a.SHA256); err != nil {
			return err
		}
		if err := godotThemeAssetCheck(root, a.LicenseFile, a.LicenseSHA256); err != nil {
			return err
		}
	}
	for _, a := range t.Textures {
		if err := godotThemeAssetCheck(root, a.File, a.SHA256); err != nil {
			return err
		}
	}
	files := map[string][]byte{}
	results := []godotContrastResult{}
	seen := map[string]bool{}
	for _, skin := range t.Skins {
		if !godotResourceID.MatchString(skin.ID) || seen[skin.ID] {
			return fmt.Errorf("invalid/duplicate skin %s", skin.ID)
		}
		seen[skin.ID] = true
		raw, err := t.render(skin)
		if err != nil {
			return err
		}
		files[skin.ID+".tres"] = raw
		if skin.ID == t.DefaultSkin {
			files[t.ResourceName+".tres"] = raw
		}
		for _, pair := range t.Contrast {
			a, err := godotRGB(skin.Roles[pair.Foreground])
			if err != nil {
				return err
			}
			b, err := godotRGB(skin.Roles[pair.Background])
			if err != nil {
				return err
			}
			ratio := godotContrast(a, b)
			results = append(results, godotContrastResult{Skin: skin.ID, Pair: pair.Name, Ratio: ratio, WCAGAA: ratio >= 4.5})
		}
	}
	if !seen[t.DefaultSkin] {
		return fmt.Errorf("default skin not declared")
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Skin != results[j].Skin {
			return results[i].Skin < results[j].Skin
		}
		return results[i].Pair < results[j].Pair
	})
	if err := os.MkdirAll(filepath.Join(root, "theme"), 0o755); err != nil {
		return err
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(root, "theme", name), raw, 0o644); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "generated"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "generated/theme-contrast.json"), append(raw, '\n'), 0o644)
}
