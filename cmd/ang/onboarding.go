package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func isFunEnabled(flagValue bool) bool {
	if flagValue {
		return true
	}
	v := strings.TrimSpace(strings.ToLower(os.Getenv("ANG_FUN")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func printFunRocket() {
	fmt.Println("      /^\\")
	fmt.Println("      |- |")
	fmt.Println("      |ANG|")
	fmt.Println("     /|___|\\")
	fmt.Println("      /___\\")
	fmt.Println("       / \\")
}

func printInitOnboarding(projectName, targetDir string, templated, fun bool) {
	if fun {
		printFunRocket()
	}
	fmt.Printf("Welcome to %s. Scaffold is ready.\n", projectName)

	created := listCreatedArtifacts(targetDir)
	if len(created) > 0 {
		fmt.Println("Created:")
		for _, item := range created {
			fmt.Printf("  - %s\n", item)
		}
	}

	fmt.Println("Quick start:")
	fmt.Printf("  1. cd %s\n", targetDir)
	if templated {
		fmt.Println("  2. make doctor   # or: ang doctor start")
		fmt.Println("  3. ang up")
		fmt.Println("  4. ang smoke")
	} else {
		fmt.Println("  2. ang doctor start")
		fmt.Println("  3. ang build")
		fmt.Println("  4. ang tips")
	}
}

func listCreatedArtifacts(root string) []string {
	type candidate struct {
		path  string
		label string
	}
	candidates := []candidate{
		{path: "cue", label: "cue/"},
		{path: "db", label: "db/"},
		{path: "scripts", label: "scripts/"},
		{path: "tests", label: "tests/"},
		{path: "docker-compose.yml", label: "docker-compose.yml"},
		{path: ".env.example", label: ".env.example"},
		{path: "Taskfile.yml", label: "Taskfile.yml"},
		{path: "Makefile", label: "Makefile"},
	}
	var out []string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(root, c.path)); err == nil {
			out = append(out, c.label)
		}
	}
	sort.Strings(out)
	return out
}

func printCommandFailure(command, reason, fix string) {
	fmt.Printf("%s FAILED: %s\n", command, reason)
	if strings.TrimSpace(fix) != "" {
		fmt.Printf("Fix: %s\n", fix)
	}
}
