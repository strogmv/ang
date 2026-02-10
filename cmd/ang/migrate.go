package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func runMigrate(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang migrate <diff|apply> [name]")
		os.Exit(1)
	}
	switch args[0] {
	case "diff":
		if len(args) < 2 {
			fmt.Println("Usage: ang migrate diff <name>")
			os.Exit(1)
		}
		name := args[1]
		if err := runAtlasDiff(name); err != nil {
			fmt.Printf("Migrate diff FAILED: %v\n", err)
			os.Exit(1)
		}
	case "apply":
		if err := runAtlasApply(); err != nil {
			fmt.Printf("Migrate apply FAILED: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown migrate command: %s\n", args[0])
		os.Exit(1)
	}
}

func runAtlasDiff(name string) error {
	cmd := exec.Command("atlas", "migrate", "diff", name, "--env", "local", "--to", "file://db/schema/schema.sql", "--dir", "file://db/migrations")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	latest, err := latestMigrationFile("db/migrations")
	if err != nil {
		return err
	}
	if latest == "" {
		return nil
	}
	data, err := os.ReadFile(latest)
	if err != nil {
		return err
	}
	upper := strings.ToUpper(string(data))
	if strings.Contains(upper, "DROP TABLE") || strings.Contains(upper, "DROP COLUMN") {
		if os.Getenv("ALLOW_DROP") != "1" {
			return fmt.Errorf("destructive statements detected in %s (set ALLOW_DROP=1 to accept)", latest)
		}
	}
	return nil
}

func runAtlasApply() error {
	dbURL := os.Getenv("DB_URL")
	if strings.TrimSpace(dbURL) == "" {
		return fmt.Errorf("DB_URL is required")
	}
	cmd := exec.Command("atlas", "migrate", "apply", "--dir", "file://db/migrations", "--url", dbURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func latestMigrationFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	type info struct {
		path string
		mod  int64
	}
	var files []info
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		stat, err := entry.Info()
		if err != nil {
			return "", err
		}
		files = append(files, info{
			path: filepath.Join(dir, entry.Name()),
			mod:  stat.ModTime().UnixNano(),
		})
	}
	if len(files) == 0 {
		return "", nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	return files[0].path, nil
}
