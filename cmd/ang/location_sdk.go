package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"github.com/strogmv/ang/compiler/locationsdk"
)

//go:embed location_sdk_preview.html
var locationSDKPreview string

type locationSDKBundle struct {
	Schema           string                      `json:"schema"`
	RuntimeConnected bool                        `json:"runtime_connected"`
	SourceHash       string                      `json:"source_hash"`
	Levels           []locationsdk.CompiledLevel `json:"levels"`
}

// Compile every requested layout before writing anything. A failed authoring
// edit must not replace the last valid preview with a partial collection.
func runLocationSDK(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("location-sdk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "generated JSON and interactive inspection directory")
	check := fs.Bool("check", false, "validate and compile without writing files")
	fingerprint := fs.String("fingerprint", "", "print one layout's candidate lock JSON to stdout without writing files or accepting it")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || (*fingerprint == "" && !*check && *out == "") || (*fingerprint != "" && (*check || *out != "")) {
		return errors.New("usage: ang location-sdk [-check] [-out directory] <CUE package directory>; or ang location-sdk -fingerprint <layout-id> <CUE package directory> (read-only candidate)")
	}
	dir, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	instances := load.Instances([]string{"."}, &load.Config{Dir: dir})
	if len(instances) != 1 {
		return errors.New("expected one CUE package")
	}
	v := cuecontext.New().BuildInstance(instances[0])
	if err = v.Err(); err != nil {
		return errors.New(cueerrors.Details(err, nil))
	}
	v = v.LookupPath(cue.ParsePath("sdk"))
	if err = v.Validate(cue.Concrete(true)); err != nil {
		return errors.New(cueerrors.Details(err, nil))
	}
	source, err := v.MarshalJSON()
	if err != nil {
		return err
	}
	var input locationsdk.Input
	decoder := json.NewDecoder(strings.NewReader(string(source)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		return fmt.Errorf("sdk input: %w", err)
	}
	if len(input.Layouts) == 0 {
		return errors.New("sdk.layouts must contain at least one layout")
	}
	if *fingerprint != "" {
		hash, fingerprintErr := locationsdk.LayoutFingerprint(input, *fingerprint)
		if fingerprintErr != nil {
			return reportLocationSDKValidation(fingerprintErr, stderr)
		}
		return json.NewEncoder(stdout).Encode(locationsdk.LayoutLock{LayoutID: *fingerprint, ExpectedHash: hash})
	}
	if err := locationsdk.ValidateLocks(input); err != nil {
		return reportLocationSDKValidation(err, stderr)
	}
	sort.Slice(input.Layouts, func(i, j int) bool { return input.Layouts[i].ID < input.Layouts[j].ID })
	hash := sha256.Sum256(source)
	bundle := locationSDKBundle{Schema: "ang.location-sdk.v1", SourceHash: hex.EncodeToString(hash[:])}
	seen := map[string]bool{}
	for _, layout := range input.Layouts {
		if seen[layout.ID] {
			return fmt.Errorf("duplicate layout ID %q", layout.ID)
		}
		seen[layout.ID] = true
		level, buildErr := locationsdk.Build(input.Catalog, layout)
		if buildErr != nil {
			var validation *locationsdk.ValidationError
			if errors.As(buildErr, &validation) {
				// Consumers can use codes/paths without parsing translated prose.
				if encodeErr := json.NewEncoder(stderr).Encode(struct {
					Layout      string                   `json:"layout"`
					Diagnostics []locationsdk.Diagnostic `json:"diagnostics"`
				}{layout.ID, validation.Diagnostics}); encodeErr != nil {
					return encodeErr
				}
			}
			return fmt.Errorf("layout %s: %w", layout.ID, buildErr)
		}
		bundle.Levels = append(bundle.Levels, level)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	if *check {
		_, err = fmt.Fprintf(stdout, "Validated %d location layouts; source %s\n", len(bundle.Levels), bundle.SourceHash)
		return err
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	// encoding/json escapes HTML-sensitive characters, including closing script
	// tags embedded in authored IDs/reasons; no source text becomes executable JS.
	html := strings.Replace(locationSDKPreview, "__LOCATION_SDK_DATA__", string(data), 1)
	for _, output := range []struct {
		name string
		data []byte
	}{
		{"compiled.json", append(data, '\n')}, {"index.html", []byte(html)},
	} {
		if err = writeLocationSDKFile(filepath.Join(*out, output.name), output.data); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "Compiled %d location layouts. Inspection: %s\n", len(bundle.Levels), filepath.Join(*out, "index.html"))
	return err
}

func reportLocationSDKValidation(err error, stderr io.Writer) error {
	var invalid *locationsdk.ValidationError
	if errors.As(err, &invalid) {
		if encodeErr := json.NewEncoder(stderr).Encode(struct {
			Diagnostics []locationsdk.Diagnostic `json:"diagnostics"`
		}{invalid.Diagnostics}); encodeErr != nil {
			return encodeErr
		}
	}
	return err
}

func writeLocationSDKFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".location-sdk-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0644); err == nil {
		_, err = f.Write(data)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}
