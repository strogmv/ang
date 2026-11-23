package gamepoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
)

type marketFinish struct {
	Layout string           `json:"layout"`
	Grain  string           `json:"grain"`
	Shader string           `json:"shader"`
	Rules  []marketMeshRule `json:"rules"`
}

type marketMeshRule struct {
	Instances []string `json:"instances"`
	Mesh      string   `json:"mesh,omitempty"`
	Prefix    string   `json:"prefix,omitempty"`
	Role      string   `json:"role"`
	Reason    string   `json:"reason,omitempty"`
}

func readMarketFinish(world cue.Value, project string, c Compiled) (*marketFinish, error) {
	v := world.LookupPath(cue.ParsePath("market_finish"))
	if !v.Exists() {
		return nil, nil
	}
	var finish marketFinish
	if err := v.Decode(&finish); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	if c.LocationSDK != nil {
		for _, layout := range c.LocationSDK.Levels {
			if layout.ID == finish.Layout {
				for _, instance := range layout.Instances {
					ids[instance.ID] = true
				}
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("market_finish: unknown or empty layout %q", finish.Layout)
	}
	// The rendered composition can name a crane subassembly that has no separate navigation body.
	if source := v.LookupPath(cue.ParsePath("placement_package")); source.Exists() {
		rel, err := source.String()
		if err != nil || !filepath.IsLocal(rel) {
			return nil, fmt.Errorf("market_finish: invalid placement package")
		}
		pkg, err := cuePackage(filepath.Join(filepath.Dir(project), rel))
		if err != nil {
			return nil, err
		}
		layout, err := pkg.LookupPath(cue.ParsePath("placement.layout_id")).String()
		if err != nil || layout != finish.Layout {
			return nil, fmt.Errorf("market_finish: placement layout %q differs from %q", layout, finish.Layout)
		}
		rows, err := pkg.LookupPath(cue.ParsePath("placement.instances")).Fields()
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			ids[rows.Selector().Unquoted()] = true
		}
	}
	for _, r := range finish.Rules {
		if (r.Mesh == "") == (r.Prefix == "") || (r.Role != "hide" && r.Role != "signal") || (r.Role == "hide" && r.Reason == "") {
			return nil, fmt.Errorf("market_finish: invalid mesh rule %+v", r)
		}
		for _, id := range r.Instances {
			if !ids[id] {
				return nil, fmt.Errorf("market_finish: unknown instance %q in %s", id, finish.Layout)
			}
		}
	}
	for _, target := range c.GodotTargets {
		for _, res := range []string{finish.Grain, finish.Shader} {
			path, ok := strings.CutPrefix(res, "res://")
			if !ok || !filepath.IsLocal(path) {
				return nil, fmt.Errorf("market_finish: invalid resource %q", res)
			}
			if _, err := os.Stat(filepath.Join(project, target, path)); err != nil {
				return nil, err
			}
		}
	}
	return &finish, nil
}
