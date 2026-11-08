package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
)

func runGodotKitCatalog(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("godot-kit-catalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	registry := fs.String("registry", "", "CUE package containing locations")
	root := fs.String("game-root", "", "game root for registry-relative paths")
	out := fs.String("out", "", "directory for kit-catalog.md and JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || *registry == "" || *root == "" || *out == "" {
		return errors.New("usage: ang godot-kit-catalog -registry <CUE package> -game-root <root> -out <docs> <SDK CUE package>")
	}
	v, err := loadGodotCUE(fs.Arg(0))
	if err != nil {
		return err
	}
	var input locationsdk.Input
	if _, err = decodeGodotCUE(v, "sdk", &input); err != nil {
		return err
	}
	documentation := map[string]godot.KitDocumentation{}
	if d := v.LookupPath(cue.ParsePath("kit_documentation")); d.Exists() {
		if err = d.Decode(&documentation); err != nil {
			return err
		}
	}
	if err = locationsdk.ValidateLocks(input); err != nil {
		return err
	}
	v, err = loadGodotCUE(*registry)
	if err != nil {
		return err
	}
	var locations map[string]struct {
		LayoutID     string `json:"layout_id"`
		Presentation string `json:"presentation"`
		Models       string `json:"models"`
	}
	if err = v.LookupPath(cue.ParsePath("locations")).Decode(&locations); err != nil {
		return err
	}
	names := make([]string, 0, len(locations))
	for n := range locations {
		names = append(names, n)
	}
	sort.Strings(names)
	bindings := []godot.KitBinding{}
	for _, name := range names {
		row := locations[name]
		b := godot.KitBinding{Name: name, LayoutID: row.LayoutID, Models: map[string]string{}, Locators: map[string][]string{}, ObjectRoles: map[string]string{}, Sources: map[string]godot.KitSource{}, LocatorSpecs: map[string][]godot.KitLocator{}}
		p, err := loadGodotCUE(filepath.Join(*root, row.Presentation))
		if err != nil {
			return err
		}
		if _, err = decodeGodotCUE(p, "placement", &b.Placement); err != nil {
			return err
		}
		if missing := p.LookupPath(cue.ParsePath("missing_kit")); missing.Exists() {
			if err = missing.Decode(&b.Missing); err != nil {
				return err
			}
		}
		raw, err := os.ReadFile(filepath.Join(*root, row.Models))
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &b.Models); err != nil {
			return err
		}
		if f := p.LookupPath(cue.ParsePath("finishing")); f.Exists() {
			b.Finishing = new(godot.Finishing)
			if err = f.Decode(b.Finishing); err != nil {
				return err
			}
		}

		if f := p.LookupPath(cue.ParsePath("pipeline")); f.Exists() {
			raw, err := f.MarshalJSON()
			if err != nil {
				return err
			}
			var hints struct {
				Instances map[string]struct {
					Role  string `json:"material_role"`
					Asset string `json:"asset"`
					Basis string `json:"asset_basis"`
				} `json:"instances"`
				Locators []struct {
					Name  string `json:"name"`
					Kind  string `json:"kind"`
					Owner string `json:"instance_id"`
				} `json:"locators"`
				PortLocators []struct {
					Name   string `json:"name"`
					Owner  string `json:"instance_id"`
					PortID string `json:"port_id"`
					Kind   string `json:"kind"`
				} `json:"port_locators"`
			}
			if err = json.Unmarshal(raw, &hints); err != nil {
				return err
			}
			for id, i := range hints.Instances {
				if i.Role != "" {
					b.ObjectRoles[id] = i.Role
				}
				b.Sources[id] = godot.KitSource{Asset: i.Asset, Basis: i.Basis}
			}
			for _, l := range hints.Locators {
				b.Locators[l.Owner] = append(b.Locators[l.Owner], l.Name)
				b.LocatorSpecs[l.Owner] = append(b.LocatorSpecs[l.Owner], godot.KitLocator{Name: l.Name, Kind: l.Kind, SourceRef: "pipeline.locators"})
			}
			for _, l := range hints.PortLocators {
				b.LocatorSpecs[l.Owner] = append(b.LocatorSpecs[l.Owner], godot.KitLocator{Name: l.Name, Kind: l.Kind, PortID: l.PortID, SourceRef: "pipeline.port_locators"})
			}
		}
		bindings = append(bindings, b)
	}
	catalog, err := godot.BuildKitCatalog(input, bindings, filepath.Join(*root, "client-godot"), documentation)
	if err != nil {
		return err
	}
	if err = godot.WriteKitCatalog(*out, catalog); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Generated kit catalog: %d passports, %d unbound; %s\n", len(catalog.Modules), len(catalog.Missing), *out)
	return err
}
