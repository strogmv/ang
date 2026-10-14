package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/strogmv/ang/compiler/sqlmigrate"
)

func runMigrate(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang migrate <diff|apply> [name]")
		os.Exit(1)
	}
	switch args[0] {
	case "diff":
		fs := flag.NewFlagSet("migrate diff", flag.ExitOnError)
		opts := migrateDiffOptions{}
		fs.StringVar(&opts.DevURL, "dev-url", "", "Atlas dev database (default: atlas.hcl env local, then ANG_ATLAS_DEV_URL, then "+sqlmigrate.DevURL("")+")")
		fs.StringVar(&opts.Dir, "dir", "db/migrations", "migration directory")
		fs.StringVar(&opts.To, "to", "db/schema/schema.sql", "desired schema")
		fs.BoolVar(&opts.DryRun, "dry-run", false, "print the migration instead of writing it")
		name := ""
		rest := args[1:]
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			name, rest = rest[0], rest[1:]
		}
		_ = fs.Parse(rest)
		if name == "" && fs.NArg() > 0 {
			name = fs.Arg(0)
		}
		if name == "" {
			fmt.Println("Usage: ang migrate diff <name> [--dev-url URL] [--dir db/migrations] [--to db/schema/schema.sql] [--dry-run]")
			os.Exit(1)
		}
		opts.Name = name
		if err := runSafeMigrateDiff(opts, os.Stdout, os.Stderr); err != nil {
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

type migrateDiffOptions struct {
	Name, DevURL, Dir, To string
	DryRun                bool
	// Now names the file; zero means time.Now.
	Now time.Time
}

// atlasSchemaDiff is how the plan is computed; a variable for tests.
var atlasSchemaDiff = func(from, to, devURL string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("atlas", "schema", "diff", "--from", from, "--to", to, "--dev-url", devURL)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("atlas schema diff: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}

// runSafeMigrateDiff writes a migration with the additive part of what the
// migration directory lacks compared with schema.sql. Atlas only plans; the
// file is written here, so a DROP never reaches the migration directory.
func runSafeMigrateDiff(opts migrateDiffOptions, stdout, stderr io.Writer) error {
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(opts.Name) {
		return fmt.Errorf("migration name %q: use letters, digits and _", opts.Name)
	}
	schemaSQL, err := os.ReadFile(opts.To)
	if err != nil {
		return err
	}
	desired, cleanup, err := desiredSchemaForAtlas(string(schemaSQL), stderr)
	if err != nil {
		return err
	}
	defer cleanup()
	diff, err := atlasSchemaDiff("file://"+opts.Dir, "file://"+desired, atlasDevURL(opts.DevURL))
	if err != nil {
		return err
	}
	plan := sqlmigrate.Safe(diff, sqlmigrate.ConcurrentIndexNames(string(schemaSQL)))
	for _, w := range plan.Warnings {
		fmt.Fprintln(stderr, "WARNING: "+w)
	}
	if len(plan.Statements) == 0 {
		fmt.Fprintln(stdout, "No additive changes: the migrations already create everything schema.sql declares.")
		return nil
	}
	content := sqlmigrate.Render(plan)
	if opts.DryRun {
		fmt.Fprint(stdout, content)
		return nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	version, err := nextMigrationVersion(opts.Dir, now)
	if err != nil {
		return err
	}
	path := filepath.Join(opts.Dir, version+"_"+opts.Name+".sql")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s: %d statement(s), %d left out (see the warnings).\n", path, len(plan.Statements), len(plan.Warnings))
	if plan.NoTransaction {
		fmt.Fprintln(stdout, "It builds indexes CONCURRENTLY: apply it outside a transaction.")
	}
	if _, err := os.Stat(filepath.Join(opts.Dir, "atlas.sum")); err == nil {
		cmd := exec.Command("atlas", "migrate", "hash", "--dir", "file://"+opts.Dir)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("atlas migrate hash: %w", err)
		}
	}
	return nil
}

// desiredSchemaForAtlas writes schema.sql without its raw ddl blocks to a
// temporary file and says which entities' raw SQL was left out.
func desiredSchemaForAtlas(schemaSQL string, stderr io.Writer) (string, func(), error) {
	stripped, entities := sqlmigrate.StripRawDDL(schemaSQL)
	if len(entities) > 0 {
		fmt.Fprintf(stderr, "NOTE: raw ddl of %s is not compared (Atlas does not plan functions and triggers); a migration for it is written by hand.\n", strings.Join(entities, ", "))
	}
	f, err := os.CreateTemp("", "ang-schema-*.sql")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(stripped); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return f.Name(), cleanup, nil
}

// nextMigrationVersion is the UTC timestamp of now, or one second after the
// newest migration when that one is dated later (a squashed baseline may carry
// a future date), so the new file always sorts last.
func nextMigrationVersion(dir string, now time.Time) (string, error) {
	const layout = "20060102150405"
	version := now.UTC().Truncate(time.Second)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") || len(name) < len(layout) {
			continue
		}
		t, err := time.Parse(layout, name[:len(layout)])
		if err != nil {
			continue
		}
		if !t.Before(version) {
			version = t.Add(time.Second)
		}
	}
	return version.Format(layout), nil
}

var atlasHCLDevRe = regexp.MustCompile(`(?m)^\s*dev\s*=\s*"([^"]+)"`)

// atlasDevURL picks the Atlas dev database: an explicit value, then
// ANG_ATLAS_DEV_URL, then the dev of atlas.hcl (generated from
// target.postgres_version), then the default Postgres version.
func atlasDevURL(explicit string) string {
	if s := strings.TrimSpace(explicit); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("ANG_ATLAS_DEV_URL")); s != "" {
		return s
	}
	if data, err := os.ReadFile("atlas.hcl"); err == nil {
		if m := atlasHCLDevRe.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	return sqlmigrate.DevURL("")
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

// applyStatements runs each statement on its own (autocommit), so CREATE INDEX
// CONCURRENTLY works.
func applyStatements(dbURL string, stmts []string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	for _, stmt := range stmts {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}
