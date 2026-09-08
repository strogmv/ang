package templateset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

var errEmptyTemplatesDir = errors.New("templates_dir is empty")

// Options carries project-declared generation settings.
type Options struct {
	ModuleDirs []string
	Outputs    map[string]string
}

// GeneratedFile is one written output and its content hash.
type GeneratedFile struct {
	RelativePath string
	SHA256       string
}

// Emit renders opts.Outputs from templatesDir into outputDir. data is any
// template context; "{package}" in an output name is replaced with a
// PackageName field when data has one.
func Emit(templatesDir, outputDir string, data any, opts Options) ([]GeneratedFile, error) {
	if data == nil {
		return nil, fmt.Errorf("template data is nil")
	}
	if len(opts.Outputs) == 0 {
		return nil, fmt.Errorf("outputs map is empty")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir output: %w", err)
	}

	pkg := packageName(data)
	names := make([]string, 0, len(opts.Outputs))
	for name := range opts.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)

	files := make([]GeneratedFile, 0, len(names))
	for _, name := range names {
		outName := strings.ReplaceAll(opts.Outputs[name], "{package}", pkg)
		file, err := RenderFile(filepath.Join(templatesDir, name), filepath.Join(outputDir, outName), templatesDir, data, opts.ModuleDirs)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(outputDir, filepath.Join(outputDir, outName))
		if err != nil {
			return nil, fmt.Errorf("rel output path %s: %w", outName, err)
		}
		file.RelativePath = filepath.ToSlash(rel)
		files = append(files, file)
	}
	return files, nil
}

// RenderFile parses tmplPath together with the block libraries visible to it,
// formats the result, and writes it to outPath.
func RenderFile(tmplPath, outPath, templatesDir string, data any, moduleDirs []string) (GeneratedFile, error) {
	parsePaths := []string{tmplPath}
	for _, dir := range moduleDirs {
		shared, err := CollectTemplates(dir)
		if err != nil {
			return GeneratedFile{}, fmt.Errorf("collect module dir %s: %w", dir, err)
		}
		parsePaths = append(parsePaths, shared...)
	}
	own, err := CollectTemplates(filepath.Join(templatesDir, "modules"))
	if err == nil {
		parsePaths = append(parsePaths, own...)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).ParseFiles(parsePaths...)
	if err != nil {
		return GeneratedFile{}, fmt.Errorf("parse template %s: %w", filepath.Base(tmplPath), err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return GeneratedFile{}, fmt.Errorf("execute template %s: %w", filepath.Base(tmplPath), err)
	}
	formatted, err := imports.Process(outPath, buf.Bytes(), &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
	if err != nil {
		return GeneratedFile{}, fmt.Errorf("format generated %s: %w", filepath.Base(outPath), err)
	}
	if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
		return GeneratedFile{}, fmt.Errorf("write %s: %w", outPath, err)
	}
	return GeneratedFile{SHA256: HashContents(formatted)}, nil
}

// CollectTemplates walks a template library so blocks can be grouped in
// subdirectories instead of one flat list; order is stable so generation is
// reproducible.
func CollectTemplates(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".tmpl") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// HashContents returns a lowercase SHA-256 of data.
func HashContents(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func packageName(data any) string {
	if data == nil {
		return ""
	}
	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName("PackageName")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	}
	if v.Kind() == reflect.Map && v.Type().Key().Kind() == reflect.String {
		for _, key := range v.MapKeys() {
			if key.String() != "PackageName" {
				continue
			}
			val := v.MapIndex(key)
			if val.Kind() == reflect.Interface {
				val = val.Elem()
			}
			if val.IsValid() && val.Kind() == reflect.String {
				return val.String()
			}
		}
	}
	return ""
}
