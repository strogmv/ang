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
	if data == nil {
		return fmt.Errorf("template data is nil")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	for _, tf := range templateFiles {
		if tf.tmpl == "creds.go.tmpl" && (data.UseMacanP2P || data.SecretUseLabels) {
			continue
		}
		if tf.tmpl == "creds_macan.go.tmpl" && !data.UseMacanP2P && !data.SecretUseLabels {
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
			if data.RequestSigning == nil || data.RequestSigning.Format != "username_key_body_b64" {
				continue
			}
		}
		tmplPath := filepath.Join(templatesDir, tf.tmpl)
		raw, err := os.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("read template %s: %w", tf.tmpl, err)
		}
		tmpl, err := template.New(tf.tmpl).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", tf.tmpl, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute template %s: %w", tf.tmpl, err)
		}
		src := buf.Bytes()
		outPath := filepath.Join(outputDir, tf.output(data.PackageName))
		formatted, err := imports.Process(outPath, src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
		if err != nil {
			return fmt.Errorf("format generated %s: %w", tf.output(data.PackageName), err)
		}
		if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
	}
	return nil
}

func needsSignFile(data *TemplateData) bool {
	if data.UseMacanP2P {
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
