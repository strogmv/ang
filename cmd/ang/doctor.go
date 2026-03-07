package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/doctor"
	"github.com/strogmv/ang/compiler/flowsem"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
)

func runDoctor(args []string) {
	if len(args) > 0 && strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		runDoctorStart(args[1:])
		return
	}

	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logFile := fs.String("log-file", "ang-build.log", "path to build log file")
	inlineLog := fs.String("log", "", "inline log text to analyze")
	fromStdin := fs.Bool("stdin", false, "read log from stdin")
	fix := fs.Bool("fix", false, "apply safe fixes for current project diagnostics (including ARCHITECTURE_VIOLATION)")
	projectPath := fs.String("project-path", ".", "path to project root for --fix")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Doctor FAILED: %v\n", err)
		os.Exit(1)
	}

	logText := strings.TrimSpace(*inlineLog)
	if logText == "" {
		useStdin := *fromStdin || isPipedStdin()
		if useStdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Printf("Doctor FAILED: read stdin: %v\n", err)
				os.Exit(1)
			}
			logText = string(b)
		}
	}
	if strings.TrimSpace(logText) == "" {
		b, err := os.ReadFile(*logFile)
		if err != nil {
			fmt.Printf("Doctor FAILED: cannot read %s (%v)\n", *logFile, err)
			fmt.Println("Provide --log, --stdin, or run with an existing ang-build.log")
			os.Exit(1)
		}
		logText = string(b)
	}

	resp := doctor.NewAnalyzer(".").Analyze(logText)
	if msg, risky := detectReleaseRootModuleMismatch("."); risky {
		resp.Summary = append(resp.Summary, "Output guard: "+msg)
	}
	if *fix {
		fixReport, fixErr := applyArchitectureDoctorFix(*projectPath)
		if fixErr != nil {
			fmt.Printf("Doctor FAILED: apply fix: %v\n", fixErr)
			os.Exit(1)
		}
		resp.Summary = append(resp.Summary, fixReport...)
		flowFixReport, flowFixErr := applyTopDoctorFixes(*projectPath, logText, resp.Suggestions)
		if flowFixErr != nil {
			fmt.Printf("Doctor FAILED: apply flow fix: %v\n", flowFixErr)
			os.Exit(1)
		}
		resp.Summary = append(resp.Summary, flowFixReport...)
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Printf("Doctor FAILED: marshal response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

func applyArchitectureDoctorFix(projectPath string) ([]string, error) {
	projectRoot := strings.TrimSpace(projectPath)
	if projectRoot == "" {
		projectRoot = "."
	}

	var diagnostics []normalizer.Warning
	_, err := compiler.RunSemanticPhasesWithOptions(projectRoot, compiler.PipelineOptions{
		ArchitectureMode: "strict",
		WarningSink: func(w normalizer.Warning) {
			diagnostics = append(diagnostics, w)
		},
	})
	if err != nil {
		return nil, err
	}

	reViolation := regexp.MustCompile(`Service '([^']+)' is not allowed to directly access entity '([^']+)'`)
	requested := make(map[string]normalizer.CrossServiceRule)
	for _, d := range diagnostics {
		if d.Code != "ARCHITECTURE_VIOLATION" {
			continue
		}
		m := reViolation.FindStringSubmatch(d.Message)
		if len(m) != 3 {
			continue
		}
		service := strings.TrimSpace(m[1])
		entity := strings.TrimSpace(m[2])
		if service == "" || entity == "" {
			continue
		}
		requested[strings.ToLower(service)+"|"+strings.ToLower(entity)] = normalizer.CrossServiceRule{Service: service, Entity: entity}
	}
	if len(requested) == 0 {
		return []string{"Doctor --fix: ARCHITECTURE_VIOLATION not detected, nothing to patch."}, nil
	}

	p := parser.New()
	n := normalizer.New()
	var projectDef *normalizer.ProjectDef
	if val, ok, loadErr := compiler.LoadOptionalDomain(p, filepath.Join(projectRoot, "cue/project")); loadErr == nil && ok {
		projectDef, _ = n.ExtractProject(val)
	}

	existing := make(map[string]struct{})
	if projectDef != nil {
		for _, r := range projectDef.AllowCrossService {
			k := strings.ToLower(strings.TrimSpace(r.Service)) + "|" + strings.ToLower(strings.TrimSpace(r.Entity))
			if k != "|" {
				existing[k] = struct{}{}
			}
		}
	}

	var newRules []normalizer.CrossServiceRule
	for key, rule := range requested {
		if _, ok := existing[key]; ok {
			continue
		}
		newRules = append(newRules, rule)
	}
	if len(newRules) == 0 {
		return []string{"Doctor --fix: архитектурные исключения уже добавлены, новых правил нет."}, nil
	}
	sort.Slice(newRules, func(i, j int) bool {
		li := strings.ToLower(newRules[i].Service + "|" + newRules[i].Entity)
		lj := strings.ToLower(newRules[j].Service + "|" + newRules[j].Entity)
		return li < lj
	})

	overridesPath := filepath.Join(projectRoot, "cue", "project", "architecture_overrides.cue")
	if err := os.MkdirAll(filepath.Dir(overridesPath), 0o755); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("package project\n\n")
	b.WriteString("#Project: {\n")
	b.WriteString("\tarchitecture: {\n")
	b.WriteString("\t\tallow_cross_service: [\n")
	for _, rule := range newRules {
		b.WriteString(fmt.Sprintf("\t\t\t{service: %q, entity: %q},\n", rule.Service, rule.Entity))
	}
	b.WriteString("\t\t]\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	if err := os.WriteFile(overridesPath, []byte(b.String()), 0o644); err != nil {
		return nil, err
	}

	return []string{
		fmt.Sprintf("Doctor --fix: added %d architecture override rule(s) in %s", len(newRules), filepath.ToSlash(overridesPath)),
		"Next: replace overrides with service.Call/read-model where possible and switch back to strict mode.",
	}, nil
}

func isPipedStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}

type doctorLogHint struct {
	File          string
	Line          int
	UnknownAction string
}

var (
	reDoctorLoc        = regexp.MustCompile(`\bat\s+([^\s:]+\.cue):(\d+):\d+`)
	reUnknownActionMsg = regexp.MustCompile(`unknown action '([^']+)'`)
	reAssignTarget     = regexp.MustCompile(`to:\s*"([^"]+)"`)
	reRepoSaveInput    = regexp.MustCompile(`input:\s*"([^"]+)"`)
)

func applyTopDoctorFixes(projectPath, logText string, suggestions []doctor.Suggestion) ([]string, error) {
	root := strings.TrimSpace(projectPath)
	if root == "" {
		root = "."
	}
	autoCodes := map[string]struct{}{}
	for _, s := range suggestions {
		if s.CanAutoApply {
			autoCodes[s.Code] = struct{}{}
		}
	}
	if len(autoCodes) == 0 {
		return []string{"Doctor --fix: no auto-fixable diagnostics detected in log."}, nil
	}
	hints := extractDoctorLogHints(logText, autoCodes)
	var report []string

	changed, total, err := applyMissingAssignFixes(root, hints["MISSING_ID"], "ID", "uuid.NewString()")
	if err != nil {
		return nil, err
	}
	if total > 0 {
		report = append(report, fmt.Sprintf("Doctor --fix: MISSING_ID patched %d/%d location(s).", changed, total))
	}

	changed, total, err = applyMissingAssignFixes(root, hints["MISSING_CREATED_AT"], "CreatedAt", "time.Now().UTC().Format(time.RFC3339)")
	if err != nil {
		return nil, err
	}
	if total > 0 {
		report = append(report, fmt.Sprintf("Doctor --fix: MISSING_CREATED_AT patched %d/%d location(s).", changed, total))
	}

	changed, total, err = applyUnknownActionFixes(root, hints["UNKNOWN_ACTION"])
	if err != nil {
		return nil, err
	}
	if total > 0 {
		report = append(report, fmt.Sprintf("Doctor --fix: UNKNOWN_ACTION patched %d/%d location(s).", changed, total))
	}

	outboxHints := hints["W_FLOW_OUTBOX_PREFERRED"]
	if len(outboxHints) == 0 {
		outboxHints = hints["E_FLOW_OUTBOX_PREFERRED"]
	}
	changed, total, err = applyOutboxPreferredFixes(root, outboxHints)
	if err != nil {
		return nil, err
	}
	if total > 0 {
		report = append(report, fmt.Sprintf("Doctor --fix: W_FLOW_OUTBOX_PREFERRED patched %d/%d location(s).", changed, total))
	}

	if _, hasLarge := autoCodes["E_FLOW_TOO_LARGE"]; hasLarge {
		report = append(report, "Doctor --fix: E_FLOW_TOO_LARGE is not auto-applied; use flow split scaffold from `ang explain E_FLOW_TOO_LARGE`.")
	}
	if _, hasGoUndefined := autoCodes["GO_UNDEFINED_SELECTOR"]; hasGoUndefined || strings.Contains(logText, "undefined: req.") {
		report = append(report, "Doctor --fix: GO_UNDEFINED_SELECTOR is advisory only. Align CUE field names to canonical *ID (e.g. TenderID) and rebuild.")
	}

	if len(report) == 0 {
		report = append(report, "Doctor --fix: no applicable top-5 fixes were applied.")
	}
	return report, nil
}

func extractDoctorLogHints(logText string, codes map[string]struct{}) map[string][]doctorLogHint {
	out := make(map[string][]doctorLogHint)
	lines := strings.Split(logText, "\n")

	pendingCode := ""
	pendingUnknown := ""
	for _, line := range lines {
		lineTrim := strings.TrimSpace(line)
		for code := range codes {
			if strings.Contains(lineTrim, code) {
				pendingCode = code
				if m := reUnknownActionMsg.FindStringSubmatch(lineTrim); len(m) == 2 {
					pendingUnknown = strings.TrimSpace(m[1])
				}
			}
		}
		if m := reUnknownActionMsg.FindStringSubmatch(lineTrim); len(m) == 2 {
			pendingUnknown = strings.TrimSpace(m[1])
		}
		loc := reDoctorLoc.FindStringSubmatch(lineTrim)
		if len(loc) != 3 || pendingCode == "" {
			continue
		}
		lineNo, _ := strconv.Atoi(loc[2])
		out[pendingCode] = append(out[pendingCode], doctorLogHint{
			File:          loc[1],
			Line:          lineNo,
			UnknownAction: pendingUnknown,
		})
		pendingCode = ""
		pendingUnknown = ""
	}
	return out
}

func resolveDoctorPath(root, p string) string {
	path := strings.TrimSpace(p)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

func applyMissingAssignFixes(root string, hints []doctorLogHint, field, value string) (changed, total int, err error) {
	for _, h := range hints {
		total++
		path := resolveDoctorPath(root, h.File)
		ok, fixErr := insertAssignBeforeRepoSave(path, h.Line, field, value)
		if fixErr != nil {
			return changed, total, fixErr
		}
		if ok {
			changed++
		}
	}
	return changed, total, nil
}

func applyOutboxPreferredFixes(root string, hints []doctorLogHint) (changed, total int, err error) {
	for _, h := range hints {
		total++
		path := resolveDoctorPath(root, h.File)
		ok, fixErr := replaceActionNearLine(path, h.Line, "event.Publish", "event.Outbox")
		if fixErr != nil {
			return changed, total, fixErr
		}
		if ok {
			changed++
		}
	}
	return changed, total, nil
}

func applyUnknownActionFixes(root string, hints []doctorLogHint) (changed, total int, err error) {
	entries := flowsem.ActionCatalog()
	actions := make([]string, 0, len(entries))
	for _, e := range entries {
		actions = append(actions, e.Name)
	}
	for _, h := range hints {
		total++
		unknown := strings.TrimSpace(h.UnknownAction)
		if unknown == "" {
			continue
		}
		replacement, ok := closestActionName(unknown, actions)
		if !ok {
			continue
		}
		path := resolveDoctorPath(root, h.File)
		okPatched, fixErr := replaceActionNearLine(path, h.Line, unknown, replacement)
		if fixErr != nil {
			return changed, total, fixErr
		}
		if okPatched {
			changed++
		}
	}
	return changed, total, nil
}

func insertAssignBeforeRepoSave(path string, aroundLine int, field, value string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	saveIdx := nearestActionLine(lines, aroundLine, "repo.Save")
	if saveIdx < 0 {
		return false, nil
	}
	entityVar := inferRepoSaveInput(lines, saveIdx)
	if entityVar == "" {
		entityVar = "newEntity"
	}
	target := entityVar + "." + field
	if hasAssignTargetNearby(lines, saveIdx, target) {
		return false, nil
	}

	indent := leadingWhitespace(lines[saveIdx])
	injected := fmt.Sprintf("%s{action: \"mapping.Assign\", to: %q, value: %q},", indent, target, value)
	lines = append(lines[:saveIdx], append([]string{injected}, lines[saveIdx:]...)...)
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func replaceActionNearLine(path string, aroundLine int, fromAction, toAction string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	idx := nearestActionLine(lines, aroundLine, fromAction)
	if idx < 0 {
		return false, nil
	}
	needle := `"` + fromAction + `"`
	if !strings.Contains(lines[idx], needle) {
		return false, nil
	}
	lines[idx] = strings.Replace(lines[idx], needle, `"`+toAction+`"`, 1)
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func nearestActionLine(lines []string, aroundLine int, action string) int {
	best := -1
	bestDist := int(^uint(0) >> 1)
	for i, line := range lines {
		if !strings.Contains(line, `action:`) || !strings.Contains(line, `"`+action+`"`) {
			continue
		}
		dist := 0
		if aroundLine > 0 {
			dist = i - (aroundLine - 1)
			if dist < 0 {
				dist = -dist
			}
		}
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

func inferRepoSaveInput(lines []string, saveIdx int) string {
	max := saveIdx + 6
	if max > len(lines)-1 {
		max = len(lines) - 1
	}
	for i := saveIdx; i <= max; i++ {
		if m := reRepoSaveInput.FindStringSubmatch(lines[i]); len(m) == 2 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func hasAssignTargetNearby(lines []string, idx int, target string) bool {
	start := idx - 24
	if start < 0 {
		start = 0
	}
	end := idx + 2
	if end > len(lines)-1 {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		m := reAssignTarget.FindStringSubmatch(lines[i])
		if len(m) != 2 {
			continue
		}
		if strings.TrimSpace(m[1]) == target {
			return true
		}
	}
	return false
}

func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

func closestActionName(input string, actions []string) (string, bool) {
	best := ""
	bestDist := int(^uint(0) >> 1)
	ambiguous := false
	for _, action := range actions {
		d := levenshtein(strings.ToLower(input), strings.ToLower(action))
		if d < bestDist {
			bestDist = d
			best = action
			ambiguous = false
			continue
		}
		if d == bestDist {
			ambiguous = true
		}
	}
	if best == "" || ambiguous || bestDist > 2 {
		return "", false
	}
	return best, true
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}
