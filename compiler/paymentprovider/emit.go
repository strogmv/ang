package paymentprovider

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler/templateset"
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
	if len(opts.Outputs) > 0 {
		files, err := templateset.Emit(templatesDir, outputDir, data, templateset.Options{
			ModuleDirs: opts.ModuleDirs,
			Outputs:    opts.Outputs,
		})
		if err != nil {
			return nil, err
		}
		return toGeneratedFiles(files), nil
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
		outPath := filepath.Join(outputDir, tf.output(data.PackageName))
		file, err := templateset.RenderFile(tmplPath, outPath, templatesDir, data, opts.ModuleDirs)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(outputDir, outPath)
		if err != nil {
			return nil, fmt.Errorf("rel output path %s: %w", outPath, err)
		}
		files = append(files, GeneratedFile{
			RelativePath: filepath.ToSlash(rel),
			SHA256:       file.SHA256,
		})
	}
	return files, nil
}

func toGeneratedFiles(in []templateset.GeneratedFile) []GeneratedFile {
	out := make([]GeneratedFile, len(in))
	for i, f := range in {
		out[i] = GeneratedFile{RelativePath: f.RelativePath, SHA256: f.SHA256}
	}
	return out
}

func needsSignFile(data *TemplateData) bool {
	if data.UseMacanP2P {
		return true
	}
	if data.CardEncryption != nil && data.CardEncryption.Enabled {
		return true
	}
	if data.CallbackSignature != nil {
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
	return templateset.ResolveTemplatesDir(projectPath, templatesDir)
}
