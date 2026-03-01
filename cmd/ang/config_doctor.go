package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type configEnvField struct {
	Key      string
	Default  string
	Required bool
}

func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang config doctor [--config-path internal/config/config.go] [--env-file .env] [--example-file .env.example]")
		os.Exit(1)
	}
	switch args[0] {
	case "doctor":
		runConfigDoctor(args[1:])
	default:
		fmt.Printf("Unknown config command: %s\n", args[0])
		fmt.Println("Usage: ang config doctor [--config-path internal/config/config.go] [--env-file .env] [--example-file .env.example]")
		os.Exit(1)
	}
}

func runConfigDoctor(args []string) {
	fs := flag.NewFlagSet("config doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config-path", filepath.Join("internal", "config", "config.go"), "path to generated config.go")
	envPath := fs.String("env-file", ".env", "path to runtime .env file")
	examplePath := fs.String("example-file", ".env.example", "path to env example file")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Printf("Config doctor FAILED: %v\n", err)
		os.Exit(1)
	}

	fields, err := parseConfigEnvFields(*configPath)
	if err != nil {
		fmt.Printf("Config doctor FAILED: %v\n", err)
		os.Exit(1)
	}
	envValues, envFound, err := readEnvFile(*envPath)
	if err != nil {
		fmt.Printf("Config doctor FAILED: %v\n", err)
		os.Exit(1)
	}
	exampleValues, exampleFound, err := readEnvFile(*examplePath)
	if err != nil {
		fmt.Printf("Config doctor FAILED: %v\n", err)
		os.Exit(1)
	}
	exampleKeys := make(map[string]struct{}, len(exampleValues))
	for k := range exampleValues {
		exampleKeys[k] = struct{}{}
	}

	missing, warnings := evaluateConfig(fields, envValues, exampleKeys, os.Getenv)

	fmt.Println("Config doctor report")
	fmt.Printf("  Config schema: %s\n", *configPath)
	if envFound {
		fmt.Printf("  Runtime env:   %s\n", *envPath)
	} else {
		fmt.Printf("  Runtime env:   %s (not found; using process env only)\n", *envPath)
	}
	if exampleFound {
		fmt.Printf("  Env example:   %s\n", *examplePath)
	} else {
		fmt.Printf("  Env example:   %s (not found)\n", *examplePath)
	}
	fmt.Printf("  Keys in schema: %d\n", len(fields))

	if len(warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	if len(missing) > 0 {
		fmt.Println("Missing required config values:")
		for _, m := range missing {
			fmt.Printf("  - %s\n", m)
		}
		fmt.Println("Config doctor FAILED")
		os.Exit(1)
	}

	fmt.Println("Config doctor OK")
}

func parseConfigEnvFields(path string) ([]configEnvField, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var out []configEnvField
	foundStruct := false
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || ts.Name.Name != "Config" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			foundStruct = true
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tagRaw := strings.Trim(field.Tag.Value, "`")
				tag := reflect.StructTag(tagRaw)
				key := strings.TrimSpace(tag.Get("env"))
				if key == "" {
					continue
				}
				required := strings.EqualFold(strings.TrimSpace(tag.Get("env-required")), "true")
				out = append(out, configEnvField{
					Key:      key,
					Default:  tag.Get("env-default"),
					Required: required,
				})
			}
		}
	}
	if !foundStruct {
		return nil, fmt.Errorf("Config struct not found in %s", path)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func readEnvFile(path string) (map[string]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	out := make(map[string]string)
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out, true, nil
}

func evaluateConfig(fields []configEnvField, envValues map[string]string, exampleKeys map[string]struct{}, getenv func(string) string) ([]string, []string) {
	exists := make(map[string]configEnvField, len(fields))
	for _, f := range fields {
		exists[f.Key] = f
	}
	resolve := func(key string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		if v := strings.TrimSpace(envValues[key]); v != "" {
			return v
		}
		if f, ok := exists[key]; ok && strings.TrimSpace(f.Default) != "" {
			return strings.TrimSpace(f.Default)
		}
		return ""
	}
	resolveRaw := func(key string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		if v := strings.TrimSpace(envValues[key]); v != "" {
			return v
		}
		return ""
	}

	seenMissing := map[string]bool{}
	var missing []string
	addMissing := func(msg string) {
		if seenMissing[msg] {
			return
		}
		seenMissing[msg] = true
		missing = append(missing, msg)
	}

	for _, f := range fields {
		if !f.Required {
			continue
		}
		if resolve(f.Key) == "" {
			addMissing(fmt.Sprintf("%s (required by config schema)", f.Key))
		}
	}

	jwtAlg := strings.ToUpper(strings.TrimSpace(resolve("JWT_ALG")))
	if jwtAlg == "" {
		jwtAlg = "HS256"
	}
	switch jwtAlg {
	case "RS256", "ES256":
		if resolve("JWT_PUBLIC_KEY") == "" {
			addMissing(fmt.Sprintf("JWT_PUBLIC_KEY (required for %s)", jwtAlg))
		}
	case "HS256":
		if resolve("JWT_PRIVATE_KEY") == "" {
			addMissing("JWT_PRIVATE_KEY (required for HS256)")
		}
	}

	emailProvider := strings.ToLower(strings.TrimSpace(resolve("EMAIL_PROVIDER")))
	if emailProvider == "" {
		emailProvider = "noop"
	}
	switch emailProvider {
	case "smtp":
		if resolve("SMTP_HOST") == "" {
			addMissing("SMTP_HOST (required when EMAIL_PROVIDER=smtp)")
		}
		if resolve("SMTP_FROM") == "" && resolve("SMTP_USER") == "" {
			addMissing("SMTP_FROM or SMTP_USER (required when EMAIL_PROVIDER=smtp)")
		}
	case "ses":
		if resolveRaw("SES_REGION") == "" {
			addMissing("SES_REGION (required when EMAIL_PROVIDER=ses)")
		}
		if resolveRaw("SES_ACCESS_KEY_ID") == "" {
			addMissing("SES_ACCESS_KEY_ID (required when EMAIL_PROVIDER=ses)")
		}
		if resolveRaw("SES_SECRET_ACCESS_KEY") == "" {
			addMissing("SES_SECRET_ACCESS_KEY (required when EMAIL_PROVIDER=ses)")
		}
		if resolveRaw("SES_FROM") == "" && resolve("SMTP_FROM") == "" {
			addMissing("SES_FROM or SMTP_FROM (required when EMAIL_PROVIDER=ses)")
		}
	}

	var warnings []string
	if len(exampleKeys) > 0 {
		for _, f := range fields {
			if _, ok := exampleKeys[f.Key]; !ok {
				warnings = append(warnings, fmt.Sprintf(".env.example is missing key %s", f.Key))
			}
		}
		for key := range exampleKeys {
			if _, ok := exists[key]; !ok {
				if strings.HasPrefix(key, "SES_") {
					continue
				}
				warnings = append(warnings, fmt.Sprintf(".env.example contains unknown key %s", key))
			}
		}
		sort.Strings(warnings)
	}

	sort.Strings(missing)
	return missing, warnings
}
