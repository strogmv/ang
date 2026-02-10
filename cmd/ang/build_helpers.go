package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/normalizer"
)

type OutputOptions struct {
	BackendDir          string
	FrontendDir         string
	FrontendAppDir      string
	FrontendAdminDir    string
	FrontendAdminAppDir string
	FrontendEnvPath     string
	TestStubs           bool
	TargetSelector      string
	DryRun              bool
	LogFormat           string
}

func parseOutputOptions(args []string) (OutputOptions, error) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	backendDir := fs.String("backend-dir", "internal", "Directory for generated backend code")
	frontendDir := fs.String("frontend-dir", "sdk", "Directory for generated frontend SDK (relative to backend-dir if not absolute)")
	frontendAppDir := fs.String("frontend-app-dir", "", "Directory to copy generated frontend SDK into app (optional)")
	frontendAdminDir := fs.String("frontend-admin-dir", "", "Directory containing frontend admin source templates")
	frontendAdminAppDir := fs.String("frontend-admin-app-dir", "", "Directory to copy generated frontend admin app into (optional)")
	frontendEnvPath := fs.String("frontend-env-path", "", "Path to write frontend .env.example (defaults to <frontend-app-dir>/.env.example)")
	testStubs := fs.Bool("test-stubs", false, "generate frontend test stubs")
	targetSelector := fs.String("target", "", "Build only selected target(s): name, lang, or lang/framework/db stack; comma-separated supported")
	dryRun := fs.Bool("dry-run", false, "preview generated file changes without writing to output directories")
	logFormat := fs.String("log-format", "text", "build log format: text|json")
	if err := fs.Parse(args); err != nil {
		return OutputOptions{}, err
	}

	opts := OutputOptions{
		BackendDir:          normalizeBackendDir(*backendDir),
		FrontendDir:         strings.TrimSpace(*frontendDir),
		FrontendAppDir:      strings.TrimSpace(*frontendAppDir),
		FrontendAdminDir:    strings.TrimSpace(*frontendAdminDir),
		FrontendAdminAppDir: strings.TrimSpace(*frontendAdminAppDir),
		FrontendEnvPath:     strings.TrimSpace(*frontendEnvPath),
		TestStubs:           *testStubs,
		TargetSelector:      strings.TrimSpace(*targetSelector),
		DryRun:              *dryRun,
		LogFormat:           strings.ToLower(strings.TrimSpace(*logFormat)),
	}
	if opts.FrontendDir == "" {
		opts.FrontendDir = "sdk"
	}
	if opts.LogFormat == "" {
		opts.LogFormat = "text"
	}
	if opts.LogFormat != "text" && opts.LogFormat != "json" {
		return OutputOptions{}, fmt.Errorf("invalid --log-format %q (expected text|json)", opts.LogFormat)
	}
	return opts, nil
}

func normalizeBackendDir(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		trimmed = "internal"
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(trimmed)
}

func projectHasBuildStrategy(projectVal cue.Value, expected string) bool {
	if !projectVal.Exists() {
		return false
	}
	check := func(path string) bool {
		v := projectVal.LookupPath(cue.ParsePath(path))
		if !v.Exists() {
			return false
		}
		s, err := v.String()
		if err != nil {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(s), expected)
	}
	return check("build.strategy") || check("Build.Strategy")
}

func filterTargets(targets []normalizer.TargetDef, selector string) []normalizer.TargetDef {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return targets
	}
	parts := strings.Split(selector, ",")
	var sels []string
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			sels = append(sels, part)
		}
	}
	if len(sels) == 0 {
		return targets
	}
	var out []normalizer.TargetDef
	for _, td := range targets {
		for _, sel := range sels {
			if targetMatchesSelector(td, sel) {
				out = append(out, td)
				break
			}
		}
	}
	return out
}

func targetMatchesSelector(td normalizer.TargetDef, selector string) bool {
	name := strings.ToLower(strings.TrimSpace(td.Name))
	lang := strings.ToLower(strings.TrimSpace(td.Lang))
	framework := strings.ToLower(strings.TrimSpace(td.Framework))
	db := strings.ToLower(strings.TrimSpace(td.DB))
	stack := strings.Trim(strings.Join([]string{lang, framework, db}, "/"), "/")
	if selector == name || selector == stack {
		return true
	}
	if selector == lang {
		return true
	}
	return false
}

func resolveBackendDirForTarget(baseBackendDir string, td normalizer.TargetDef, multiTarget bool) string {
	if v := strings.TrimSpace(td.OutputDir); v != "" {
		return normalizeBackendDir(v)
	}
	if multiTarget {
		return normalizeBackendDir(filepath.Join(baseBackendDir, safeTargetDirName(td.Name)))
	}
	return normalizeBackendDir(baseBackendDir)
}

func resolveFrontendDirForTarget(baseFrontendDir, backendDir string, td normalizer.TargetDef, multiTarget bool) string {
	trimmed := strings.TrimSpace(baseFrontendDir)
	if trimmed == "" {
		trimmed = "sdk"
	}
	if multiTarget {
		if filepath.IsAbs(trimmed) {
			return filepath.Join(trimmed, safeTargetDirName(td.Name))
		}
		return filepath.Join(backendDir, trimmed)
	}
	return trimmed
}

func safeTargetDirName(name string) string {
	v := strings.TrimSpace(strings.ToLower(name))
	if v == "" {
		return "target"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", ".", "-")
	v = replacer.Replace(v)
	v = strings.Trim(v, "-")
	if v == "" {
		return "target"
	}
	return v
}

func copyFrontendSDK(srcDir, appDir string) error {
	if strings.TrimSpace(appDir) == "" {
		return nil
	}
	targetDir := filepath.Join(appDir, filepath.Base(srcDir))
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("cleanup sdk target: %w", err)
	}
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

func copyFrontendAdmin(srcDir, appDir string) error {
	if strings.TrimSpace(appDir) == "" || strings.TrimSpace(srcDir) == "" {
		return nil
	}
	targetDir := appDir
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

func writeEnvExample(opts OutputOptions) error {
	if strings.TrimSpace(opts.FrontendAppDir) == "" {
		return nil
	}
	envPath := strings.TrimSpace(opts.FrontendEnvPath)
	if envPath == "" {
		envPath = filepath.Join(opts.FrontendAppDir, ".env.example")
	}
	if _, err := os.Stat(envPath); err == nil {
		return nil
	}
	content := []byte("VITE_API_URL=http://localhost:8080\n")
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(envPath, content, 0644)
}

func firstPathParam(path string) string {
	params := pathParams(path)
	if len(params) == 0 {
		return ""
	}
	return params[0]
}

func pathParams(path string) []string {
	var params []string
	start := strings.Index(path, "{")
	for start != -1 {
		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}
		param := path[start+1 : start+end]
		if param != "" {
			params = append(params, param)
		}
		next := start + end + 1
		start = strings.Index(path[next:], "{")
		if start != -1 {
			start += next
		}
	}
	return params
}
