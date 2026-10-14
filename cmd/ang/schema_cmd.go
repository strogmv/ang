package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/schemavendor"
)

// Diagnostic codes of the vendored schema (docs/schema.md).
const (
	codeSchemaStale       = "SCHEMA_STALE"
	codeSchemaProjectCopy = "SCHEMA_PROJECT_COPY"
)

// runSchema implements `ang schema sync|check [project]`.
func runSchema(args []string) {
	sub := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("schema "+sub, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	allowMismatch := fs.Bool("allow-lock-mismatch", false, "use this binary's schema although ang.lock pins another revision")
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ang schema sync [project]   write ANG's CUE schema (the revision ang.lock pins) to cue.mod/pkg/"+schemavendor.ImportPath)
		fmt.Fprintln(os.Stderr, "  ang schema check [project]  exit 1 when that directory differs from it")
		fmt.Fprintln(os.Stderr, "Projects import it as "+schemavendor.ImportPath+"; ang build syncs it too (docs/schema.md).")
	}
	if sub != "sync" && sub != "check" {
		fs.Usage()
		os.Exit(2)
	}
	project := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		project, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		project = fs.Arg(0)
	}

	if pin, pinned, err := readANGLock(project); err != nil {
		fmt.Fprintf(os.Stderr, "Schema FAILED: %v\n", err)
		os.Exit(1)
	} else if pinned {
		warnings, lockErr := checkANGLock(pin, runningCompilerRevision(), *allowMismatch)
		if lockErr != nil {
			fmt.Fprintf(os.Stderr, "Schema FAILED: %v\n", lockErr)
			os.Exit(1)
		}
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
	}

	cueRoot := loadProjectConfig(project).CueRoot
	opts := schemavendor.Options{CueRoot: cueRoot, Always: sub == "sync"}
	var status schemavendor.Status
	var err error
	if sub == "sync" {
		status, err = schemavendor.Sync(project, opts)
	} else {
		status, err = schemavendor.Plan(project, opts)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Schema FAILED: %v\n", err)
		os.Exit(1)
	}
	legacy, hasLegacy := schemavendor.LegacyCopy(project, cueRoot)
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		out := map[string]any{"schema": "ang/schema/v1", "command": sub, "status": status, "in_sync": status.InSync()}
		if hasLegacy {
			out["project_copy"] = legacy
		}
		_ = enc.Encode(out)
	} else {
		printSchemaStatus(os.Stdout, sub, status)
		if hasLegacy {
			fmt.Fprintf(os.Stderr, "Warning [%s]: %s\n", codeSchemaProjectCopy, legacySchemaCopyMessage(legacy))
		}
	}
	if sub == "check" && !status.InSync() {
		os.Exit(1)
	}
}

func printSchemaStatus(w io.Writer, sub string, status schemavendor.Status) {
	if !status.Applies {
		fmt.Fprintf(w, "Schema: nothing to vendor (%s).\n", status.Reason)
		return
	}
	switch {
	case status.InSync():
		fmt.Fprintf(w, "Schema: %s matches this ANG.\n", status.Dir)
	case sub == "sync":
		fmt.Fprintf(w, "Schema: %s synced with this ANG (%d files):\n", status.Dir, len(status.Changes))
		printSchemaChanges(w, status.Changes)
	default:
		fmt.Fprintf(w, "Schema check FAILED: %s differs from this ANG's schema (%d files):\n", status.Dir, len(status.Changes))
		printSchemaChanges(w, status.Changes)
		fmt.Fprintln(w, "Run ang schema sync (ang build does it too) and commit the result.")
	}
}

func printSchemaChanges(w io.Writer, changes []schemavendor.Change) {
	for _, change := range changes {
		marker := "~"
		switch change.Action {
		case "create":
			marker = "+"
		case "delete":
			marker = "-"
		}
		fmt.Fprintf(w, "  %s %s\n", marker, change.Path)
	}
}

func legacySchemaCopyMessage(dir string) string {
	return fmt.Sprintf("%s is a copy of ANG's schema; it drifts from the ANG ang.lock pins. Import %s instead (vendored by ang schema sync), move the project's own definitions to its own package and delete the copy (docs/schema.md).", dir, schemavendor.ImportPath)
}

// syncSchemaForBuild runs before generation: a build vendors the schema of
// this binary, a dry run (and --check) only compares. Nothing happens for a
// project that does not import the schema.
func syncSchemaForBuild(projectPath, cueRoot string, dryRun bool) (schemavendor.Status, error) {
	opts := schemavendor.Options{CueRoot: cueRoot}
	if dryRun {
		return schemavendor.Plan(projectPath, opts)
	}
	return schemavendor.Sync(projectPath, opts)
}

// schemaDiagnostics is what ang validate reports about the vendored schema:
// SCHEMA_STALE when it differs from this binary's (an error when the binary is
// the revision ang.lock pins, else a warning, since then the binary's schema
// is not the one the project should have), SCHEMA_PROJECT_COPY for a project
// copy of the schema.
func schemaDiagnostics(projectPath, cueRoot string) []normalizer.Warning {
	var diags []normalizer.Warning
	status, err := schemavendor.Plan(projectPath, schemavendor.Options{CueRoot: cueRoot})
	if err != nil {
		diags = append(diags, normalizer.Warning{Kind: "schema", Code: codeSchemaStale, Severity: "error", Message: "compare the vendored schema: " + err.Error()})
	} else if status.Applies && !status.InSync() {
		paths := make([]string, 0, len(status.Changes))
		for _, change := range status.Changes {
			paths = append(paths, change.Action+" "+change.Path)
		}
		severity := "error"
		message := fmt.Sprintf("%s differs from the schema of this ANG (%s)", status.Dir, strings.Join(paths, ", "))
		if pin, pinned, _ := readANGLock(projectPath); pinned {
			if running := runningCompilerRevision(); running.Revision != "" && !sameRevision(pin.Commit, running.Revision) {
				severity = "warn"
				message += fmt.Sprintf("; this binary is %s, ang.lock pins %s, so the difference may be the revisions'", shortRevision(running.Revision), shortRevision(pin.Commit))
			}
		}
		diags = append(diags, normalizer.Warning{
			Kind:     "schema",
			Code:     codeSchemaStale,
			Severity: severity,
			Message:  message,
			File:     status.Dir,
			Hint:     "Run ang schema sync (ang build does it too) and commit the result; never edit the vendored files.",
		})
	}
	if dir, ok := schemavendor.LegacyCopy(projectPath, cueRoot); ok {
		diags = append(diags, normalizer.Warning{
			Kind:     "schema",
			Code:     codeSchemaProjectCopy,
			Severity: "warn",
			Message:  legacySchemaCopyMessage(dir),
			File:     dir,
			Hint:     "Replace the import of <module>/cue/schema with " + schemavendor.ImportPath + ", run ang schema sync, delete " + dir + ".",
		})
	}
	return diags
}
