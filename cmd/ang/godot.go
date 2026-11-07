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
	check := fs.Bool("check", false, "compile and resolve without writing outputs")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || *layoutID == "" || (!*check && *out == "") {
		return errors.New("usage: ang godot -layout <id> [-placement <CUE package>] [-models <JSON>] [-check] [-out <directory>] <CUE package>")
	}
	v, err := loadGodotCUE(fs.Arg(0))
	if err != nil {
		return err
	}
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
	// Validate locks before generating a target, just like location-sdk.
	if err = locationsdk.ValidateLocks(input); err != nil {
		return reportLocationSDKValidation(err, stderr)
	}
	level, err := locationsdk.Build(input.Catalog, *selected)
	if err != nil {
		return reportLocationSDKValidation(err, stderr)
	}
	if *presentation != "" {
		v, err = loadGodotCUE(*presentation)
		if err != nil {
			return err
		}
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
	scene, err := godot.Render(level, manifest, godot.Options{Models: modelMap, Camera: authoring.Camera})
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
		{"compiled.json", append(compiled, '\n')}, {"placements.json", append(placements, '\n')}, {"location.tscn", scene},
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
