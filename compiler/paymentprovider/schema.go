package paymentprovider

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed schema/*.cue
var bundledSchema embed.FS

const bundledSchemaDir = "schema"

// BundledSchemaFiles returns canonical payment-provider schema file names
// shipped with ang (provider.cue, catalogs.cue, …).
func BundledSchemaFiles() ([]string, error) {
	entries, err := fs.ReadDir(bundledSchema, bundledSchemaDir)
	if err != nil {
		return nil, fmt.Errorf("read bundled schema: %w", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".cue") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ReadBundledSchemaFile returns the contents of a bundled schema file.
func ReadBundledSchemaFile(name string) ([]byte, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		return nil, fmt.Errorf("schema file name is required")
	}
	data, err := bundledSchema.ReadFile(filepath.Join(bundledSchemaDir, name))
	if err != nil {
		return nil, fmt.Errorf("read bundled schema %q: %w", name, err)
	}
	return data, nil
}

// SchemaSyncOptions configures copying ang's bundled schema into a provider project.
type SchemaSyncOptions struct {
	ProjectPath string
	CueRoot     string // defaults to ".cue"
	SchemaDir   string // if set in ang.yaml or here, sync to shared dir instead of cueRoot/schema
	DryRun      bool
}

// SchemaSyncResult describes files written or that would be written.
type SchemaSyncResult struct {
	TargetDir string
	Written   []string
	Skipped   []string
}

// schemaTargetDir resolves where schema files should live for a project.
func schemaTargetDir(projectPath, cueRoot, schemaDirOverride string) (string, error) {
	pc := LoadProjectConfig(projectPath)
	if cueRoot == "" {
		cueRoot = pc.CueRoot
	}
	if strings.TrimSpace(schemaDirOverride) != "" {
		return ResolvePath(projectPath, schemaDirOverride)
	}
	if pc.SchemaDir != "" {
		return ResolvePath(projectPath, pc.SchemaDir)
	}
	if cueRoot == "" {
		cueRoot = ".cue"
	}
	return filepath.Join(projectPath, cueRoot, "schema"), nil
}

// SyncSchema copies ang's bundled payment-provider schema into the project's schema directory.
func SyncSchema(opts SchemaSyncOptions) (*SchemaSyncResult, error) {
	projectPath := strings.TrimSpace(opts.ProjectPath)
	if projectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}
	targetDir, err := schemaTargetDir(projectPath, opts.CueRoot, opts.SchemaDir)
	if err != nil {
		return nil, err
	}
	result := &SchemaSyncResult{TargetDir: targetDir}

	files, err := BundledSchemaFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("bundled schema is empty")
	}

	if !opts.DryRun {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir schema dir: %w", err)
		}
	}

	for _, name := range files {
		src, err := ReadBundledSchemaFile(name)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(targetDir, name)
		if same, err := fileBytesEqual(dst, src); err != nil {
			return nil, err
		} else if same {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		if opts.DryRun {
			result.Written = append(result.Written, name)
			continue
		}
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", dst, err)
		}
		result.Written = append(result.Written, name)
	}
	return result, nil
}

// CheckSchema reports whether the project's schema matches ang's bundled schema.
func CheckSchema(projectPath, cueRoot string) ([]string, error) {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}
	targetDir, err := schemaTargetDir(projectPath, cueRoot, "")
	if err != nil {
		return nil, err
	}

	files, err := BundledSchemaFiles()
	if err != nil {
		return nil, err
	}
	var drift []string
	for _, name := range files {
		want, err := ReadBundledSchemaFile(name)
		if err != nil {
			return nil, err
		}
		gotPath := filepath.Join(targetDir, name)
		got, err := os.ReadFile(gotPath)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, name+": missing")
				continue
			}
			return nil, fmt.Errorf("read %s: %w", gotPath, err)
		}
		if !bytesEqual(got, want) {
			drift = append(drift, name+": differs from ang bundle")
		}
	}
	return drift, nil
}

func fileBytesEqual(path string, want []byte) (bool, error) {
	got, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	return bytesEqual(got, want), nil
}

func bytesEqual(a, b []byte) bool {
	return len(a) == len(b) && string(a) == string(b)
}
