package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ang vet logic only parsed the Go written in CUE. A wrong field name, a
// finder returning (int64, error) where the code expects a value, a missing
// argument — all of that surfaced only after a full build and go build.
//
// ang vet logic --types generates the project into a dry-run directory, hands
// the generated .go files to the compiler with go build -overlay (the project
// on disk is not touched, and unchanged packages come from the build cache),
// and reports each compiler error at the CUE line the failing Go came from.
//
// Generated service files do not record which CUE line produced each Go line,
// but embedded Go is copied through nearly verbatim. A compiler error is
// therefore placed by finding the same line of text — scored by how many
// neighbouring lines match too — first in the CUE files the generated file's
// header names, then anywhere under the CUE root. When no line matches (the
// error is in scaffolding, or CUE escapes changed the text) the generated
// location is reported with the CUE files it came from.

type typeCheckError struct {
	Message         string   `json:"message"`
	GeneratedFile   string   `json:"generated_file"`
	GeneratedLine   int      `json:"generated_line"`
	GeneratedColumn int      `json:"generated_column"`
	CUEFile         string   `json:"cue_file,omitempty"`
	CUELine         int      `json:"cue_line,omitempty"`
	CUEColumn       int      `json:"cue_column,omitempty"`
	Match           string   `json:"match"`
	OtherMatches    int      `json:"other_matches,omitempty"`
	From            []string `json:"from,omitempty"`

	contentPath string
}

const (
	typeCheckMatchExact     = "exact"
	typeCheckMatchAmbiguous = "ambiguous"
	typeCheckMatchNone      = "none"
)

type typeCheckReport struct {
	Schema       string           `json:"schema"`
	OK           bool             `json:"ok"`
	Packages     []string         `json:"packages"`
	GenerateSecs float64          `json:"generate_seconds"`
	CompileSecs  float64          `json:"compile_seconds"`
	Errors       []typeCheckError `json:"errors"`
	Notes        []string         `json:"notes,omitempty"`
}

func runLogicVetTypes(args []string) {
	fset := flag.NewFlagSet("vet logic --types", flag.ContinueOnError)
	fset.SetOutput(os.Stderr)
	_ = fset.Bool("types", true, "type-check embedded Go against the generated project")
	packages := fset.String("packages", "./internal/service/...", "comma-separated go package patterns to compile")
	jsonOut := fset.Bool("json", false, "emit machine-readable report")
	target := fset.String("target", "", "build only selected target(s), as in ang build --target")
	allowLockMismatch := fset.Bool("allow-lock-mismatch", false, "as in ang build")
	projectPath := "."
	rest := args
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		projectPath, rest = rest[0], rest[1:]
	}
	if err := fset.Parse(rest); err != nil {
		os.Exit(2)
	}
	if fset.NArg() > 0 {
		projectPath = fset.Arg(0)
	}
	var patterns []string
	for _, p := range strings.Split(*packages, ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, p)
		}
	}

	report, err := typeCheckEmbeddedGo(projectPath, patterns, *target, *allowLockMismatch)
	if *jsonOut {
		if err != nil {
			report.Notes = append(report.Notes, err.Error())
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		if err != nil {
			fmt.Printf("Type check FAILED: %v\n", err)
			os.Exit(1)
		}
		printTypeCheckReport(report)
	}
	if err != nil || !report.OK {
		os.Exit(1)
	}
}

func printTypeCheckReport(report typeCheckReport) {
	for _, e := range report.Errors {
		switch e.Match {
		case typeCheckMatchNone:
			fmt.Printf("%s:%d:%d: %s\n", e.GeneratedFile, e.GeneratedLine, e.GeneratedColumn, e.Message)
			if len(e.From) > 0 {
				fmt.Printf("    from: %s (no matching CUE line)\n", strings.Join(e.From, ", "))
			}
		default:
			fmt.Printf("%s:%d:%d: %s\n", e.CUEFile, e.CUELine, e.CUEColumn, e.Message)
			note := ""
			if e.Match == typeCheckMatchAmbiguous {
				note = fmt.Sprintf("; the same text is on %d other CUE line(s)", e.OtherMatches)
			}
			fmt.Printf("    generated: %s:%d:%d%s\n", e.GeneratedFile, e.GeneratedLine, e.GeneratedColumn, note)
		}
	}
	for _, note := range report.Notes {
		fmt.Println(note)
	}
	timing := fmt.Sprintf("generate %.1fs, compile %.1fs", report.GenerateSecs, report.CompileSecs)
	if report.OK {
		fmt.Printf("✅ Embedded Go type-checks: go build %s (%s)\n", strings.Join(report.Packages, " "), timing)
		return
	}
	fmt.Printf("Type check FAILED: %d error(s) (%s)\n", len(report.Errors), timing)
}

func typeCheckEmbeddedGo(projectPath string, patterns []string, target string, allowLockMismatch bool) (typeCheckReport, error) {
	report := typeCheckReport{Schema: "ang/vet-logic-types/v1", Packages: patterns, Errors: []typeCheckError{}}
	scratch, err := os.MkdirTemp("", "ang-vet-types-*")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(scratch)
	if resolved, err := filepath.EvalSymlinks(scratch); err == nil {
		scratch = resolved
	}

	dryRoot := filepath.Join(scratch, "dry-run")
	reportPath := filepath.Join(scratch, "dry-run-report.json")
	buildArgs := []string{projectPath, "--dry-run", "--dry-run-root", dryRoot, "--dry-run-report", reportPath, "--skip-go-verify"}
	if strings.TrimSpace(target) != "" {
		buildArgs = append(buildArgs, "--target", target)
	}
	if allowLockMismatch {
		buildArgs = append(buildArgs, "--allow-lock-mismatch")
	}
	started := time.Now()
	// Generation warnings (large CUE files, architecture hints) are not what
	// was asked for; they are shown only if generation itself fails.
	generationLog, err := os.Create(filepath.Join(scratch, "generate.log"))
	if err != nil {
		return report, err
	}
	originalStderr := os.Stderr
	os.Stderr = generationLog
	buildErr := withSuppressedStdout(func() error { return runBuild(buildArgs) })
	os.Stderr = originalStderr
	_ = generationLog.Close()
	if buildErr != nil {
		logged, _ := os.ReadFile(generationLog.Name())
		tail := string(logged)
		if len(tail) > 4000 {
			tail = "…" + tail[len(tail)-4000:]
		}
		return report, fmt.Errorf("generate: %w\n%s", buildErr, strings.TrimSpace(tail))
	}
	report.GenerateSecs = time.Since(started).Seconds()

	data, err := os.ReadFile(reportPath)
	if err != nil {
		return report, fmt.Errorf("read dry-run report: %w", err)
	}
	var manifest dryRunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return report, fmt.Errorf("parse dry-run report: %w", err)
	}
	cueRoot := filepath.Join(projectPath, loadProjectConfig(projectPath).CueRoot)

	started = time.Now()
	checked := 0
	for _, tm := range manifest.Targets {
		if !strings.EqualFold(tm.Lang, "go") {
			continue
		}
		backend, err := filepath.Abs(filepath.FromSlash(tm.Backend))
		if err != nil {
			return report, err
		}
		if _, err := os.Stat(filepath.Join(backend, "go.mod")); err != nil {
			report.Notes = append(report.Notes, fmt.Sprintf("target %s: no go.mod in %s, skipped", tm.Target, backend))
			continue
		}
		generated := filepath.Join(dryRoot, "backend", safeTargetDirName(tm.Target))
		replace, err := generatedGoOverlay(generated, backend)
		if err != nil {
			return report, err
		}
		output, buildErr := goBuildWithOverlay(backend, replace, patterns, filepath.Join(scratch, "overlay-"+safeTargetDirName(tm.Target)))
		checked++
		errs, notes := parseTypeCheckOutput(output, backend, replace)
		if buildErr != nil && len(errs) == 0 {
			return report, fmt.Errorf("go build failed in %s:\n%s", backend, strings.TrimSpace(output))
		}
		locator := newCUELineLocator(projectPath, cueRoot)
		for i := range errs {
			locator.locate(&errs[i])
		}
		report.Errors = append(report.Errors, errs...)
		report.Notes = append(report.Notes, notes...)
	}
	report.CompileSecs = time.Since(started).Seconds()
	if checked == 0 {
		return report, errors.New("no Go target with a go.mod to type-check")
	}
	report.OK = len(report.Errors) == 0
	return report, nil
}

// generatedGoOverlay maps every generated .go file's place in the project to
// the file the dry run wrote.
func generatedGoOverlay(generatedRoot, backend string) (map[string]string, error) {
	// go matches overlay entries against the paths it reads, which start from
	// the real working directory: under a symlink (macOS /var → /private/var)
	// an unresolved path is silently ignored.
	backend = realPath(backend)
	generatedRoot = realPath(generatedRoot)
	replace := map[string]string{}
	err := filepath.WalkDir(generatedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == generatedRoot {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(generatedRoot, path)
		if err != nil {
			return err
		}
		replace[filepath.Join(backend, rel)] = path
		return nil
	})
	return replace, err
}

func goBuildWithOverlay(dir string, replace map[string]string, patterns []string, scratch string) (string, error) {
	dir = realPath(dir)
	if err := os.MkdirAll(filepath.Join(scratch, "bin"), 0o755); err != nil {
		return "", err
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(scratch, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		return "", err
	}
	// A main package must not drop a binary into the project, but go build
	// rejects -o <dir>/ when no package is main and -o /dev/null for several
	// packages; ask go list which case this is.
	output, err := goBuildOutputFlag(dir, overlayPath, patterns, filepath.Join(scratch, "bin"))
	if err != nil {
		return err.Error(), err
	}
	args := []string{"build", "-overlay", overlayPath}
	if output != "" {
		args = append(args, "-o", output)
	}
	cmd := exec.Command("go", append(args, patterns...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	configureBuildSubprocess(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func goBuildOutputFlag(dir, overlayPath string, patterns []string, binDir string) (string, error) {
	cmd := exec.Command("go", append([]string{"list", "-e", "-overlay", overlayPath, "-f", "{{.Name}}"}, patterns...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	configureBuildSubprocess(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list %s: %w", strings.Join(patterns, " "), err)
	}
	var names []string
	for _, name := range strings.Fields(string(out)) {
		names = append(names, name)
	}
	switch {
	case len(names) == 1:
		return os.DevNull, nil
	case len(names) > 1:
		for _, name := range names {
			if name == "main" {
				return binDir + string(filepath.Separator), nil
			}
		}
	}
	return "", nil
}

var reGoCompilerError = regexp.MustCompile(`^(.+?\.go):([0-9]+):([0-9]+):\s*(.+)$`)

// parseTypeCheckOutput turns compiler output into errors named by their place
// in the project; the dry-run copy is kept to read the generated line from.
func parseTypeCheckOutput(output, dir string, replace map[string]string) ([]typeCheckError, []string) {
	// go prints paths relative to the real working directory.
	dir = realPath(dir)
	reverse := make(map[string]string, len(replace))
	for projectFile, generatedFile := range replace {
		reverse[filepath.Clean(generatedFile)] = projectFile
	}
	var errs []typeCheckError
	var notes []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		m := reGoCompilerError.FindStringSubmatch(line)
		if m == nil {
			notes = append(notes, line)
			continue
		}
		path := m[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		path = filepath.Clean(path)
		projectFile, contentPath := path, path
		if original, ok := reverse[path]; ok {
			projectFile = original
		}
		display := projectFile
		if rel, err := filepath.Rel(dir, projectFile); err == nil && !strings.HasPrefix(rel, "..") {
			display = rel
		}
		lineNo, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		errs = append(errs, typeCheckError{
			Message:         strings.TrimSpace(m[4]),
			GeneratedFile:   filepath.ToSlash(display),
			GeneratedLine:   lineNo,
			GeneratedColumn: col,
			Match:           typeCheckMatchNone,
			contentPath:     contentPath,
		})
	}
	return errs, notes
}

type cueLineLocator struct {
	projectPath string
	cueRoot     string
	files       map[string][]string
	allFiles    []string
}

func newCUELineLocator(projectPath, cueRoot string) *cueLineLocator {
	return &cueLineLocator{projectPath: projectPath, cueRoot: cueRoot, files: map[string][]string{}}
}

var reProvenanceCUE = regexp.MustCompile(`^//\s+(\S+\.cue)\s+\(sha:`)

func (l *cueLineLocator) locate(e *typeCheckError) {
	content, err := os.ReadFile(e.contentPath)
	if err != nil {
		return
	}
	generated := strings.Split(string(content), "\n")
	if e.GeneratedLine < 1 || e.GeneratedLine > len(generated) {
		return
	}
	for i, line := range generated {
		if i > 40 {
			break
		}
		if m := reProvenanceCUE.FindStringSubmatch(line); m != nil {
			e.From = append(e.From, m[1])
		}
	}

	var headerFiles []string
	for _, rel := range e.From {
		headerFiles = append(headerFiles, filepath.Join(l.projectPath, filepath.FromSlash(rel)))
	}
	best := l.bestMatches(headerFiles, generated, e.GeneratedLine-1)
	if len(best) == 0 {
		best = l.bestMatches(l.cueFiles(), generated, e.GeneratedLine-1)
	}
	if len(best) == 0 {
		return
	}
	match := best[0]
	cueLines := l.lines(match.file)
	e.CUEFile = filepath.ToSlash(l.display(match.file))
	e.CUELine = match.line + 1
	e.CUEColumn = translateColumn(generated[e.GeneratedLine-1], cueLines[match.line], e.GeneratedColumn)
	e.Match = typeCheckMatchExact
	if len(best) > 1 {
		e.Match = typeCheckMatchAmbiguous
		e.OtherMatches = len(best) - 1
	}
}

type cueLineMatch struct {
	file  string
	line  int
	score int
}

// bestMatches returns the CUE lines whose text equals the generated line,
// keeping only those with the most matching neighbours (three either side).
func (l *cueLineLocator) bestMatches(files []string, generated []string, index int) []cueLineMatch {
	want := strings.TrimSpace(generated[index])
	if want == "" {
		return nil
	}
	var best []cueLineMatch
	for _, file := range files {
		lines := l.lines(file)
		for i, line := range lines {
			if strings.TrimSpace(line) != want {
				continue
			}
			score := 1
			for k := 1; k <= 3; k++ {
				if index-k >= 0 && i-k >= 0 && strings.TrimSpace(generated[index-k]) != "" && strings.TrimSpace(generated[index-k]) == strings.TrimSpace(lines[i-k]) {
					score++
				}
				if index+k < len(generated) && i+k < len(lines) && strings.TrimSpace(generated[index+k]) != "" && strings.TrimSpace(generated[index+k]) == strings.TrimSpace(lines[i+k]) {
					score++
				}
			}
			switch {
			case len(best) == 0 || score > best[0].score:
				best = []cueLineMatch{{file: file, line: i, score: score}}
			case score == best[0].score:
				best = append(best, cueLineMatch{file: file, line: i, score: score})
			}
		}
	}
	return best
}

// translateColumn moves a column from the generated line to the CUE line: the
// text after the indentation is the same, the indentation is not.
func translateColumn(generatedLine, cueLine string, column int) int {
	generatedIndent := len(generatedLine) - len(strings.TrimLeft(generatedLine, " \t"))
	cueIndent := len(cueLine) - len(strings.TrimLeft(cueLine, " \t"))
	if column <= generatedIndent {
		return cueIndent + 1
	}
	return cueIndent + column - generatedIndent
}

func (l *cueLineLocator) lines(file string) []string {
	if lines, ok := l.files[file]; ok {
		return lines
	}
	data, err := os.ReadFile(file)
	var lines []string
	if err == nil {
		lines = strings.Split(string(data), "\n")
	}
	l.files[file] = lines
	return lines
}

func (l *cueLineLocator) cueFiles() []string {
	if l.allFiles != nil {
		return l.allFiles
	}
	l.allFiles = []string{}
	_ = filepath.WalkDir(l.cueRoot, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".cue") {
			l.allFiles = append(l.allFiles, path)
		}
		return nil
	})
	return l.allFiles
}

func (l *cueLineLocator) display(file string) string {
	if rel, err := filepath.Rel(l.projectPath, file); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return file
}
