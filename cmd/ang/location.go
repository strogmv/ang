package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationintent"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

const locationUsage = `usage: ang location <check|plan|scene|derive|still> -intent <CUE package> -id <level id|all> [flags] <SDK CUE package>

  check   derive and run every coherence rule and the SDK compiler; writes nothing
  plan    -out <dir>: top-down <short>.png and <short>.coherence.json (written even on failure)
  scene   -out <dir>: location.tscn, compiled.json, placements.json, finishing.json, derived.json
  derive  -world <content/world>: generated CUE of the level (layout, world data, Godot package)
  pockets -layout <layout id|all> <SDK CUE package>: enclosed floorless pockets of any SDK layout, hand-written included
  still   -client-root <client-godot> -launcher <scripts/location_still.py> -png <file>: scene, then one game-camera frame

Only the named level is derived and checked; other levels are never touched. With -id all every
intent is processed independently: one level's failure is reported and does not stop the others.`

// runLocation is the per-level entry of the intent layer. Nothing here reads
// or regenerates another level, the server world or any art.
func runLocation(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New(locationUsage)
	}
	mode := args[0]
	if mode == "pockets" {
		return runLocationPockets(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("location "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	intentDir := fs.String("intent", "", "CUE package with locations and defaults (content/world/location_intent)")
	id := fs.String("id", "", "level ID, or all")
	out := fs.String("out", "", "output directory (plan, scene)")
	world := fs.String("world", "", "game content/world directory (derive)")
	clientRoot := fs.String("client-root", "", "Godot client root (still)")
	launcher := fs.String("launcher", "", "still launcher script (still)")
	pngOut := fs.String("png", "", "frame file (still)")
	size := fs.Float64("size", 0, "still frame height in metres; 0 = the level camera")
	worldText := fs.Bool("world-text", false, "attach generated world lettering")
	focusAt := fs.String("focus", "", "still focus as x,y in layout metres; empty = the centre of the walkable floor")
	defer func() { stillFocus = "" }()
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || *intentDir == "" || *id == "" {
		return errors.New(locationUsage)
	}
	switch mode {
	case "check":
	case "plan", "scene":
		if *out == "" {
			return errors.New(locationUsage)
		}
	case "derive":
		if *world == "" {
			return errors.New(locationUsage)
		}
	case "still":
		if *clientRoot == "" || *launcher == "" || *pngOut == "" || *id == "all" {
			return errors.New(locationUsage)
		}
	default:
		return errors.New(locationUsage)
	}
	iv, err := loadGodotCUE(*intentDir)
	if err != nil {
		return err
	}
	var defaults locationintent.Defaults
	if _, err = decodeGodotCUE(iv, "defaults", &defaults); err != nil {
		return err
	}
	sv, err := loadGodotCUE(fs.Arg(0))
	if err != nil {
		return err
	}
	var catalog locationsdk.Catalog
	if _, err = decodeGodotCUE(sv, "sdk.catalog", &catalog); err != nil {
		return err
	}
	ids := []string{*id}
	if *id == "all" {
		ids = ids[:0]
		iter, e := iv.LookupPath(cue.ParsePath("locations")).Fields()
		if e != nil {
			return e
		}
		for iter.Next() {
			ids = append(ids, iter.Selector().Unquoted())
		}
		sort.Strings(ids)
	}
	failed := 0
	for _, level := range ids {
		stillFocus = *focusAt
		if err := runLocationLevel(mode, level, iv, catalog, defaults, *out, *world, *clientRoot, *launcher, *pngOut, *size, *worldText, stdout); err != nil {
			failed++
			fmt.Fprintf(stderr, "FAIL %s\n%v\n", level, err)
			continue
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d levels failed", failed, len(ids))
	}
	return nil
}

// stillFocus is the optional -focus of the still mode.
var stillFocus string

func runLocationLevel(mode, level string, iv cue.Value, catalog locationsdk.Catalog, defaults locationintent.Defaults, out, world, clientRoot, launcher, pngOut string, size float64, worldText bool, stdout io.Writer) error {
	value := iv.LookupPath(cue.MakePath(cue.Str("locations"), cue.Str(level)))
	if !value.Exists() {
		return fmt.Errorf("unknown intent level %q", level)
	}
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return errors.New(cueerrors.Details(err, nil))
	}
	raw, err := value.MarshalJSON()
	if err != nil {
		return err
	}
	var intent locationintent.Intent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&intent); err != nil {
		return fmt.Errorf("intent %s: %w", level, err)
	}
	d, diags := locationintent.Derive(intent, catalog, defaults)
	if mode == "plan" && d != nil {
		if err = os.MkdirAll(out, 0755); err != nil {
			return err
		}
		picture, e := locationintent.Plan(d, diags)
		if e != nil {
			return e
		}
		report, _ := json.MarshalIndent(map[string]any{"schema": "ang.location-coherence.v1", "level_id": level, "pass": len(diags) == 0, "diagnostics": diags, "summary": locationintent.Summary(d)}, "", "  ")
		if e = writeLocationSDKFile(filepath.Join(out, d.Short+".png"), picture); e != nil {
			return e
		}
		if e = writeLocationSDKFile(filepath.Join(out, d.Short+".coherence.json"), append(report, '\n')); e != nil {
			return e
		}
	}
	if len(diags) > 0 {
		return &locationintent.Error{Diagnostics: diags}
	}
	compiled, err := locationsdk.Build(d.Catalog, d.Layout)
	if err != nil {
		var validation *locationsdk.ValidationError
		if errors.As(err, &validation) {
			text, _ := json.Marshal(validation.Diagnostics)
			return fmt.Errorf("SDK compiler rejected the derived layout: %s", text)
		}
		return err
	}
	manifest, err := placement.Build(compiled, d.Authoring)
	if err != nil {
		return err
	}
	scene, err := godot.Render(compiled, manifest, godot.Options{Finishing: d.Finishing, Models: d.Models, Camera: d.Authoring.Camera, WorldText: worldText})
	if err != nil {
		return err
	}
	writeScene := func(dir string) error {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		finishing, err := godot.ResolveArchitectureForLevel(d.Finishing, compiled)
		if err != nil {
			return err
		}
		files := map[string]any{"compiled.json": locationSDKBundle{Schema: "ang.location-sdk.v1", Levels: []locationsdk.CompiledLevel{compiled}}, "placements.json": manifest, "finishing.json": finishing, "derived.json": d}
		for name, v := range files {
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return err
			}
			if err = writeLocationSDKFile(filepath.Join(dir, name), append(data, '\n')); err != nil {
				return err
			}
		}
		return writeLocationSDKFile(filepath.Join(dir, "location.tscn"), scene)
	}
	switch mode {
	case "scene":
		if err = writeScene(out); err != nil {
			return err
		}
	case "derive":
		files, e := locationintent.Files(d)
		if e != nil {
			return e
		}
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(world, filepath.FromSlash(name))
			if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
				return e
			}
			if e = writeLocationSDKFile(path, files[name]); e != nil {
				return e
			}
		}
	case "still":
		name := strings.NewReplacer(".", "_", "-", "_").Replace(d.Short)
		if err = writeScene(filepath.Join(clientRoot, "generated", "intent", name)); err != nil {
			return err
		}
		frame := d.Authoring.Camera.SizeM
		if size > 0 {
			frame = size
		}
		focus := d.Authoring.Camera.FocusPlanM
		if stillFocus != "" {
			var fx, fy float64
			if _, err := fmt.Sscanf(stillFocus, "%g,%g", &fx, &fy); err != nil {
				return fmt.Errorf("-focus wants x,y in metres: %w", err)
			}
			focus[0], focus[2] = fx, fy
		}
		command := exec.Command("python3", launcher, "--scene-res", "res://generated/intent/"+name+"/location.tscn", "--out", pngOut,
			"--size", fmt.Sprint(frame), "--focus", fmt.Sprintf("%g,%g,%g", focus[0], focus[1], focus[2]))
		command.Stdout, command.Stderr = stdout, os.Stderr
		if err = command.Run(); err != nil {
			return fmt.Errorf("still: %w", err)
		}
	}
	_, err = fmt.Fprintf(stdout, "OK %s %s\n", mode, locationintent.Summary(d))
	return err
}

// runLocationPockets applies the enclosed-pocket rule to ordinary SDK layouts.
func runLocationPockets(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("location pockets", flag.ContinueOnError)
	fs.SetOutput(stderr)
	layoutID := fs.String("layout", "all", "layout ID from sdk.layouts, or all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(locationUsage)
	}
	v, err := loadGodotCUE(fs.Arg(0))
	if err != nil {
		return err
	}
	var layouts []locationsdk.Layout
	if _, err = decodeGodotCUE(v, "sdk.layouts", &layouts); err != nil {
		return err
	}
	failed, found := 0, false
	for _, layout := range layouts {
		if *layoutID != "all" && layout.ID != *layoutID {
			continue
		}
		found = true
		diags := locationintent.LayoutPockets(layout)
		if len(diags) == 0 {
			fmt.Fprintf(stdout, "OK pockets %s\n", layout.ID)
			continue
		}
		failed++
		fmt.Fprintf(stderr, "FAIL %s\n%v\n", layout.ID, &locationintent.Error{Diagnostics: diags})
	}
	if !found {
		return fmt.Errorf("unknown layout %q", *layoutID)
	}
	if failed > 0 {
		return fmt.Errorf("%d layouts have enclosed pockets", failed)
	}
	return nil
}
