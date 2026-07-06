package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var versionLineRe = regexp.MustCompile(`(?m)(version:\s*")(\d+\.\d+\.\d+)(")`)

func runSDKBump(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: ang sdk <version|bump|patch|minor|major> [--path dir]")
		os.Exit(1)
	}

	sub := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("sdk "+sub, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("path", ".", "project root")
	if err := fs.Parse(rest); err != nil {
		fmt.Fprintf(os.Stderr, "sdk: %v\n", err)
		os.Exit(1)
	}

	switch sub {
	case "bump":
		part := "patch"
		if fs.NArg() >= 1 {
			part = fs.Arg(0)
		}
		runSDKBumpVersion(*projectPath, part)
	case "version":
		runSDKShowVersion(*projectPath)
	case "patch", "minor", "major":
		runSDKBumpVersion(*projectPath, sub)
	default:
		fmt.Fprintf(os.Stderr, "sdk: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "Usage: ang sdk <version|patch|minor|major|bump [patch|minor|major]>")
		os.Exit(1)
	}
}

func findProjectCUE(projectPath string) string {
	cr := loadProjectConfig(projectPath).CueRoot
	candidates := []string{
		filepath.Join(projectPath, cr, "project", "project.cue"),
		filepath.Join(projectPath, cr, "meta", "meta.cue"),
		filepath.Join(projectPath, cr, "project.cue"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func readVersionFromCUE(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	m := versionLineRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[2])
}

func bumpVersion(current, part string) (string, error) {
	parts := strings.Split(current, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver %q", current)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major in %q", current)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor in %q", current)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch in %q", current)
	}
	switch strings.ToLower(part) {
	case "patch":
		patch++
	case "minor":
		minor++
		patch = 0
	case "major":
		major++
		minor = 0
		patch = 0
	default:
		return "", fmt.Errorf("unknown part %q, use patch|minor|major", part)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func runSDKShowVersion(projectPath string) {
	cuePath := findProjectCUE(projectPath)
	if cuePath == "" {
		fmt.Fprintln(os.Stderr, "sdk: no project.cue found (checked cue/project/project.cue, cue/meta/meta.cue, cue/project.cue)")
		os.Exit(1)
	}
	v := readVersionFromCUE(cuePath)
	if v == "" {
		fmt.Fprintln(os.Stderr, "sdk: no version: field found in", cuePath)
		os.Exit(1)
	}
	fmt.Println(v)
}

func runSDKBumpVersion(projectPath, part string) {
	cuePath := findProjectCUE(projectPath)
	if cuePath == "" {
		fmt.Fprintln(os.Stderr, "sdk: no project.cue found (checked cue/project/project.cue, cue/meta/meta.cue, cue/project.cue)")
		os.Exit(1)
	}

	data, err := os.ReadFile(cuePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdk: read %s: %v\n", cuePath, err)
		os.Exit(1)
	}

	current := readVersionFromCUE(cuePath)
	if current == "" {
		fmt.Fprintln(os.Stderr, "sdk: no version: \"x.y.z\" field found in", cuePath)
		fmt.Fprintln(os.Stderr, `hint: add  version: "0.1.0"  to your #Project block`)
		os.Exit(1)
	}

	next, err := bumpVersion(current, part)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdk: %v\n", err)
		os.Exit(1)
	}

	updated := versionLineRe.ReplaceAll(data, []byte(`${1}`+next+`${3}`))
	if err := os.WriteFile(cuePath, updated, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "sdk: write %s: %v\n", cuePath, err)
		os.Exit(1)
	}

	fmt.Printf("%s → %s  (%s)\n", current, next, cuePath)
}
