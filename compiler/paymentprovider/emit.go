package paymentprovider

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

// A template set may lay its types out either as a single datatypes.go or as the
// model.go / secrets.go / status.go split; entries marked optional are emitted
// only when the set actually provides that template.
var templateFiles = []struct {
	tmpl      string
	output    func(pkg string) string
	whenMacan bool // emit only when TemplateData.UseMacanP2P
	optional  bool // skip silently when the template set omits this file
}{
	{"datatypes.go.tmpl", func(pkg string) string { return "datatypes.go" }, false, true},
	{"model.go.tmpl", func(pkg string) string { return "model.go" }, false, true},
	{"secrets.go.tmpl", func(pkg string) string { return "secrets.go" }, false, true},
	{"status.go.tmpl", func(pkg string) string { return "status.go" }, false, true},
	{"creds.go.tmpl", func(pkg string) string { return "creds.go" }, false, false},
	{"creds_macan.go.tmpl", func(pkg string) string { return "creds.go" }, false, false},
	// A provider that hosts its own pages (PIN/OTP forms) keeps them here; sets
	// without such pages simply omit the template.
	{"forms.go.tmpl", func(pkg string) string { return "forms.go" }, false, true},
	{"sign.go.tmpl", func(pkg string) string { return "sign.go" }, false, false},
	{"sign_test.go.tmpl", func(pkg string) string { return "sign_test.go" }, false, false},
	{"provider.go.tmpl", func(pkg string) string { return pkg + ".go" }, false, false},
	{"provider_macan.go.tmpl", func(pkg string) string { return pkg + "_macan.go" }, true, false},
	{"provider_test.go.tmpl", func(pkg string) string { return pkg + "_test.go" }, false, false},
}

// Emit writes generated provider files into outputDir.
func Emit(templatesDir, outputDir string, data *TemplateData, moduleDirs ...string) error {
	_, err := EmitWithResult(templatesDir, outputDir, data, moduleDirs...)
	return err
}

// EmitOptions carries project-declared generation settings.
type EmitOptions struct {
	// ModuleDirs are template libraries parsed before the set's own modules, so
	// a project can share blocks between sets; a block redefined by the set wins.
	ModuleDirs []string
	// Outputs replaces the built-in file layout with the project's own mapping
	// of template file to output file.
	Outputs map[string]string
}

// EmitWithResult writes generated files and returns the generator-owned manifest.
func EmitWithResult(templatesDir, outputDir string, data *TemplateData, moduleDirs ...string) ([]GeneratedFile, error) {
	return EmitWithOptions(templatesDir, outputDir, data, EmitOptions{ModuleDirs: moduleDirs})
}

// EmitWithOptions writes generated files and returns the generator-owned manifest.
func EmitWithOptions(templatesDir, outputDir string, data *TemplateData, opts EmitOptions) ([]GeneratedFile, error) {
	if data == nil {
		return nil, fmt.Errorf("template data is nil")
	}
	moduleDirs := opts.ModuleDirs
	if len(opts.Outputs) > 0 {
		return emitDeclaredOutputs(templatesDir, outputDir, data, opts)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir output: %w", err)
	}

	var files []GeneratedFile
	for _, tf := range templateFiles {
		if tf.tmpl == "creds.go.tmpl" && data.UseMacanP2P {
			continue
		}
		if tf.tmpl == "creds.go.tmpl" && data.CardEncryption != nil && data.CardEncryption.Enabled {
			continue
		}
		if tf.tmpl == "creds_macan.go.tmpl" && !data.UseMacanP2P {
			continue
		}
		if tf.whenMacan && !data.UseMacanP2P {
			continue
		}
		if tf.whenMacan && !data.HasCancel {
			continue
		}
		if tf.tmpl == "sign.go.tmpl" && data.SigningAlgorithm == "none" {
			// Still emit if salt or signature fields are used
			if !needsSignFile(data) {
				continue
			}
		}
		if tf.tmpl == "sign_test.go.tmpl" {
			emitSignTest := data.RequestSigning != nil && (data.RequestSigning.Format == "username_key_body_b64" || data.RequestSigning.Format == "hmac_timestamp_nonce")
			if data.CallbackSignature != nil && data.CallbackSignature.Format == "username_key_form_b64" {
				emitSignTest = true
			}
			if !emitSignTest {
				continue
			}
		}
		tmplPath := filepath.Join(templatesDir, tf.tmpl)
		if tf.optional {
			if _, statErr := os.Stat(tmplPath); os.IsNotExist(statErr) {
				continue
			}
		}
		parsePaths := []string{tmplPath}
		for _, dir := range moduleDirs {
			shared, walkErr := collectTemplates(dir)
			if walkErr != nil {
				return nil, fmt.Errorf("collect module dir %s: %w", dir, walkErr)
			}
			parsePaths = append(parsePaths, shared...)
		}
		if moduleFiles, globErr := filepath.Glob(filepath.Join(templatesDir, "modules", "*.tmpl")); globErr == nil {
			parsePaths = append(parsePaths, moduleFiles...)
		}
		tmpl, err := template.New(filepath.Base(tmplPath)).ParseFiles(parsePaths...)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", tf.tmpl, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("execute template %s: %w", tf.tmpl, err)
		}
		src := buf.Bytes()
		outPath := filepath.Join(outputDir, tf.output(data.PackageName))
		formatted, err := imports.Process(outPath, src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
		if err != nil {
			return nil, fmt.Errorf("format generated %s: %w", tf.output(data.PackageName), err)
		}
		if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", outPath, err)
		}
		rel, err := filepath.Rel(outputDir, outPath)
		if err != nil {
			return nil, fmt.Errorf("rel output path %s: %w", outPath, err)
		}
		files = append(files, GeneratedFile{
			RelativePath: filepath.ToSlash(rel),
			SHA256:       hashFileContents(formatted),
		})
	}
	return files, nil
}

// emitDeclaredOutputs generates exactly the files the project asked for. Unlike
// the built-in table it has no optional entries: a declared template that is
// missing is an error, so a misnamed file cannot be dropped in silence.
func emitDeclaredOutputs(templatesDir, outputDir string, data *TemplateData, opts EmitOptions) ([]GeneratedFile, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir output: %w", err)
	}

	names := make([]string, 0, len(opts.Outputs))
	for name := range opts.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)

	files := make([]GeneratedFile, 0, len(names))
	for _, name := range names {
		outName := strings.ReplaceAll(opts.Outputs[name], "{package}", data.PackageName)
		file, err := renderTemplate(filepath.Join(templatesDir, name), filepath.Join(outputDir, outName), templatesDir, data, opts.ModuleDirs)
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

// renderTemplate parses a template together with the block libraries visible to
// it, formats the result and writes it out.
func renderTemplate(tmplPath, outPath, templatesDir string, data *TemplateData, moduleDirs []string) (GeneratedFile, error) {
	parsePaths := []string{tmplPath}
	for _, dir := range moduleDirs {
		shared, err := collectTemplates(dir)
		if err != nil {
			return GeneratedFile{}, fmt.Errorf("collect module dir %s: %w", dir, err)
		}
		parsePaths = append(parsePaths, shared...)
	}
	own, err := collectTemplates(filepath.Join(templatesDir, "modules"))
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
	return GeneratedFile{SHA256: hashFileContents(formatted)}, nil
}

// collectTemplates walks a template library so blocks can be grouped in
// subdirectories instead of one flat list; order is stable so generation is
// reproducible.
func collectTemplates(dir string) ([]string, error) {
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

func needsSignFile(data *TemplateData) bool {
	if data.UseMacanP2P {
		return true
	}
	if data.CardEncryption != nil && data.CardEncryption.Enabled {
		return true
	}
	if data.CallbackSignature != nil {
		if data.CallbackSignature.Format == "rsa_pkcs1v15_body" {
			return true
		}
		return true
	}
	if data.RequestSigning != nil {
		return true
	}
	check := func(def *ResolvedRequestDef) bool {
		if def == nil {
			return false
		}
		return usesSalt(def.Fields)
	}
	return check(data.PayinRequest) || check(data.PayoutRequest) || check(data.P2PRequest)
}

// usesSalt reports whether any field, at any depth, is bound to the generated
// nonce — the generator needs it to decide on the crypto imports.
func usesSalt(fields []ResolvedField) bool {
	for _, f := range fields {
		if f.GoExpr == `generateSalt()` || usesSalt(f.Nested) {
			return true
		}
	}
	return false
}

// ResolveTemplatesDir resolves templates_dir relative to projectPath when not absolute.
func ResolveTemplatesDir(projectPath, templatesDir string) (string, error) {
	templatesDir = strings.TrimSpace(templatesDir)
	if templatesDir == "" {
		return "", fmt.Errorf("templates_dir is empty")
	}
	if filepath.IsAbs(templatesDir) {
		return filepath.Clean(templatesDir), nil
	}
	return filepath.Clean(filepath.Join(projectPath, templatesDir)), nil
}
