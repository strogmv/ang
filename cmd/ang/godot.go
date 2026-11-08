package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"github.com/strogmv/ang/compiler/blender"
	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func runGodot(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("godot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "generated scene directory (client assets stay separate)")
	layoutID := fs.String("layout", "", "layout ID from sdk.layouts")
	models := fs.String("models", "", "client JSON map: model_ref -> res:// PackedScene")
	presentation := fs.String("placement", "", "optional separate CUE package containing placement authoring")
	attachmentFile := fs.String("attachments", "", "optional source-art model-local attachment JSON")
	worldText := fs.Bool("world-text", false, "attach generated world lettering (game-poc --godot-only)")
	blenderKitCatalog := fs.String("blender-kit-catalog", "", "published kit catalog JSON for Blender-first bootstrap (no SDK build)")
	blenderTools := fs.String("blender-tools", "", "emit Blender programs before a Blender-first layout exists")
	blenderOut := fs.String("blender", "", "emit the ID authoring contract and Blender start/export programs")
	clientRoot := fs.String("client-root", "", "client root used to resolve res:// Blender input assets")
	sourceBlend := fs.String("source-blend", "", "actual Blender input path for declared blend pose authority")
	artReport := fs.String("art-report", "", "validated ID exporter provenance for pipeline-authored levels")
	check := fs.Bool("check", false, "compile and resolve without writing outputs")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *blenderTools != "" && *blenderKitCatalog != "" {
		raw, err := os.ReadFile(*blenderKitCatalog)
		if err != nil {
			return err
		}
		var kit godot.KitCatalog
		if err = json.Unmarshal(raw, &kit); err != nil {
			return err
		}
		if *layoutID == "" {
			return errors.New("-blender-kit-catalog requires -layout for the new location ID")
		}
		contract, err := blender.KitContract(kit, *layoutID, *clientRoot)
		if err != nil {
			return err
		}
		files, err := blender.Files(contract)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(*blenderTools, 0755); err != nil {
			return err
		}
		for name, data := range files {
			if err = writeLocationSDKFile(filepath.Join(*blenderTools, name), data); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(stdout, "Generated Blender-first kit authoring for %s: %s\n", *layoutID, *blenderTools)
		return err
	}
	if *blenderTools != "" {
		if err := os.MkdirAll(*blenderTools, 0755); err != nil {
			return err
		}
		for name, data := range blender.ToolFiles() {
			if err := writeLocationSDKFile(filepath.Join(*blenderTools, name), data); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(stdout, "Generated ID Blender programs: %s\n", *blenderTools)
		return err
	}
	if fs.NArg() != 1 || *layoutID == "" || (!*check && *out == "" && *blenderOut == "") {
		return errors.New("usage: ang godot -layout <id> [-placement <CUE package>] [-models <JSON>] [-check] [-out <directory>] <CUE package>")
	}
	v, err := loadGodotCUE(fs.Arg(0))
	if err != nil {
		return err
	}
	sdkValue := v.LookupPath(cue.ParsePath("sdk.layouts"))
	sdkRoot := v
	var input locationsdk.Input
	source, err := decodeGodotCUE(v, "sdk", &input)
	if err != nil {
		return err
	}
	var selected *locationsdk.Layout
	for j := range input.Layouts {
		if input.Layouts[j].ID == *layoutID {
			if selected != nil {
				return fmt.Errorf("duplicate layout ID %q", *layoutID)
			}
			selected = &input.Layouts[j]
		}
	}
	if selected == nil {
		return fmt.Errorf("unknown layout %q", *layoutID)
	}
	if *presentation != "" {
		v, err = loadGodotCUE(*presentation)
		if err != nil {
			return err
		}
	}
	var authority blender.PoseAuthority
	if v.LookupPath(cue.ParsePath("authoring")).Exists() {
		if _, err = decodeGodotCUE(v, "authoring", &authority); err != nil {
			return err
		}
		packageDir := *presentation
		if packageDir == "" {
			packageDir = fs.Arg(0)
		}
		if err = blender.ValidatePoseAuthority(authority, packageDir, *sourceBlend); err != nil {
			return err
		}
		layouts, e := sdkValue.List()
		if e != nil {
			return e
		}
		for layouts.Next() {
			candidate := layouts.Value()
			id, _ := candidate.LookupPath(cue.ParsePath("id")).String()
			if id == *layoutID {
				if err = blender.ValidatePoseSources(authority, packageDir, candidate, v, sdkRoot); err != nil {
					return err
				}
				break
			}
		}
	}
	// Source authority and locks are checked before compiling any target.
	if err = locationsdk.ValidateLocks(input); err != nil {
		return reportLocationSDKValidation(err, stderr)
	}
	level, err := locationsdk.Build(input.Catalog, *selected)
	if err != nil {
		return reportLocationSDKValidation(err, stderr)
	}
	var authoring placement.Authoring
	if v.LookupPath(cue.ParsePath("placement")).Exists() {
		if _, err = decodeGodotCUE(v, "placement", &authoring); err != nil {
			return err
		}
	} else if *presentation != "" {
		return errors.New("placement CUE package must contain placement")
	}
	manifest, err := placement.Build(level, authoring)
	if err != nil {
		return err
	}
	modelMap := map[string]string{}
	if *models != "" {
		data, readErr := os.ReadFile(*models)
		if readErr != nil {
			return readErr
		}
		if err = json.Unmarshal(data, &modelMap); err != nil {
			return fmt.Errorf("model map: %w", err)
		}
	}
	var attachments godot.AttachmentCatalog
	if *attachmentFile != "" {
		raw, e := os.ReadFile(*attachmentFile)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &attachments); e != nil {
			return e
		}
		if e = attachments.Validate(); e != nil {
			return e
		}
	}
	var finishing *godot.Finishing
	if value := v.LookupPath(cue.ParsePath("finishing")); value.Exists() {
		finishing = new(godot.Finishing)
		if err = value.Decode(finishing); err != nil {
			return err
		}
	}
	packageDir := *presentation
	if packageDir == "" {
		packageDir = fs.Arg(0)
	}
	replacements, err := blender.GeneratedReplacements(authority, packageDir)
	if err != nil {
		return err
	}
	if len(replacements) > 0 {
		if finishing == nil {
			finishing = new(godot.Finishing)
		}
		finishing.Replacements = append(finishing.Replacements, replacements...)
	}
	finishing, err = godot.ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return err
	}
	var pipeline *blender.Pipeline
	if v.LookupPath(cue.ParsePath("pipeline")).Exists() {
		pipeline = new(blender.Pipeline)
		if _, err = decodeGodotCUE(v, "pipeline", pipeline); err != nil {
			return err
		}
	}
	if pipeline != nil || *blenderOut != "" || authority.PoseSource == "blend" {
		packageDir := *presentation
		if packageDir == "" {
			packageDir = fs.Arg(0)
		}
		contract, e := blender.Build(level, *selected, manifest, pipeline, finishing, modelMap, authoring.Camera, packageDir, *clientRoot, blender.CatalogContext{Catalog: input.Catalog, ModuleModels: authoring.ModuleModels, PoseSource: authority.PoseSource})
		if e != nil {
			return e
		}
		if (pipeline != nil || authority.PoseSource == "blend") && (*blenderOut == "" || (*out != "" && !*check)) {
			if *artReport == "" {
				return errors.New("pipeline: ID art validation required; emit -blender, run its export.py through the serial Blender wrapper, then pass -art-report <provenance.json>")
			}
			raw, e := os.ReadFile(*artReport)
			if e != nil {
				return e
			}
			warnings, e := blender.ValidateReport(contract, raw)
			if e != nil {
				return e
			}
			if e = blender.ValidateArtMaterialPoseSources(authority, packageDir, v, contract, raw, sdkRoot); e != nil {
				return e
			}
			for _, warning := range warnings {
				fmt.Fprintln(stderr, "WARNING:", warning)
			}
		}
		if *blenderOut != "" {
			files, e := blender.Files(contract)
			if e != nil {
				return e
			}
			if e = os.MkdirAll(*blenderOut, 0755); e != nil {
				return e
			}
			// Stable order also makes diagnostics and filesystem traces reproducible.
			for _, name := range []string{"authoring.json", "convention.json", "runtime.py", "start.py", "export.py", "assets.py", "import.py"} {
				if e = writeLocationSDKFile(filepath.Join(*blenderOut, name), files[name]); e != nil {
					return e
				}
			}
			if *out == "" {
				_, e = fmt.Fprintf(stdout, "Generated Blender authoring for %s: %s\n", level.ID, *blenderOut)
				return e
			}
		}
	}

	scene, err := godot.Render(level, manifest, godot.Options{Finishing: finishing, Attachments: attachments.Models, InstanceAttachments: attachments.Instances, WorldText: *worldText, Models: modelMap, Camera: authoring.Camera})
	if err != nil {
		return err
	}
	hash := sha256.Sum256(source)
	bundle := locationSDKBundle{Schema: "ang.location-sdk.v1", SourceHash: hex.EncodeToString(hash[:]), Levels: []locationsdk.CompiledLevel{level}}
	compiled, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	placements, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	finishData, err := json.MarshalIndent(finishing, "", "  ")
	if err != nil {
		return err
	}
	if *check {
		_, err = fmt.Fprintf(stdout, "Validated Godot layout %s: %d placements\n", level.ID, len(manifest.Instances))
		return err
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	for _, f := range []struct {
		name string
		data []byte
	}{
		{"finishing.json", append(finishData, '\n')}, {"compiled.json", append(compiled, '\n')}, {"placements.json", append(placements, '\n')}, {"location.tscn", scene},
	} {
		if err = writeLocationSDKFile(filepath.Join(*out, f.name), f.data); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "Generated Godot layout %s: %d placements; scene: %s\n", level.ID, len(manifest.Instances), filepath.Join(*out, "location.tscn"))
	return err
}

func loadGodotCUE(dir string) (cue.Value, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return cue.Value{}, err
	}
	instances := load.Instances([]string{"."}, &load.Config{Dir: abs})
	if len(instances) != 1 {
		return cue.Value{}, errors.New("expected one CUE package")
	}
	v := cuecontext.New().BuildInstance(instances[0])
	if err = v.Err(); err != nil {
		return v, errors.New(cueerrors.Details(err, nil))
	}
	return v, nil
}

func decodeGodotCUE(v cue.Value, field string, out any) ([]byte, error) {
	v = v.LookupPath(cue.ParsePath(field))
	if err := v.Validate(cue.Concrete(true)); err != nil {
		return nil, errors.New(cueerrors.Details(err, nil))
	}
	data, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return nil, fmt.Errorf("%s input: %w", field, err)
	}
	return data, nil
}
