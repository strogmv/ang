package paymentprovider

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

var templateFiles = []struct {
	tmpl      string
	output    func(pkg string) string
	whenMacan bool // emit only when TemplateData.UseMacanP2P
}{
	{"datatypes.go.tmpl", func(pkg string) string { return "datatypes.go" }, false},
	{"creds.go.tmpl", func(pkg string) string { return "creds.go" }, false},
	{"creds_macan.go.tmpl", func(pkg string) string { return "creds.go" }, false},
	{"sign.go.tmpl", func(pkg string) string { return "sign.go" }, false},
	{"sign_test.go.tmpl", func(pkg string) string { return "sign_test.go" }, false},
	{"provider.go.tmpl", func(pkg string) string { return pkg + ".go" }, false},
	{"provider_macan.go.tmpl", func(pkg string) string { return pkg + "_macan.go" }, true},
	{"provider_test.go.tmpl", func(pkg string) string { return pkg + "_test.go" }, false},
}

// Emit writes generated provider files into outputDir.
func Emit(templatesDir, outputDir string, data *TemplateData) error {
	_, err := EmitWithResult(templatesDir, outputDir, data)
	return err
}

// EmitWithResult writes generated files and returns the generator-owned manifest.
func EmitWithResult(templatesDir, outputDir string, data *TemplateData) ([]GeneratedFile, error) {
	if data == nil {
		return nil, fmt.Errorf("template data is nil")
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
			emitSignTest := data.RequestSigning != nil && data.RequestSigning.Format == "username_key_body_b64"
			if data.CallbackSignature != nil && data.CallbackSignature.Format == "username_key_form_b64" {
				emitSignTest = true
			}
			if !emitSignTest {
				continue
			}
		}
		tmplPath := filepath.Join(templatesDir, tf.tmpl)
		parsePaths := []string{tmplPath}
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
		for _, f := range def.Fields {
			if f.GoExpr == `generateSalt()` {
				return true
			}
		}
		return false
	}
	return check(data.PayinRequest) || check(data.PayoutRequest) || check(data.P2PRequest)
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
