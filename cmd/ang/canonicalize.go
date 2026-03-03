package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type canonicalizeResult struct {
	FilesScanned int      `json:"files_scanned"`
	FilesChanged int      `json:"files_changed"`
	Replacements int      `json:"replacements"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

type aliasRule struct {
	From string
	To   string
}

var flowAliasRules = []aliasRule{
	{From: "notify.Dispatch", To: "notification.Dispatch"},
	{From: "idem.DeriveKey", To: "idempotency.DeriveKey"},
	{From: "idem.Check", To: "idempotency.Check"},
	{From: "idem.SaveResult", To: "idempotency.SaveResult"},
}

var actionLineRe = regexp.MustCompile(`(?m)(action\s*:\s*")([^"]+)(")`)

func canonicalizeCueAliases(root string, checkOnly bool) (canonicalizeResult, error) {
	files, err := collectCueFiles(root)
	if err != nil {
		return canonicalizeResult{}, err
	}
	res := canonicalizeResult{
		FilesScanned: len(files),
	}

	ruleMap := make(map[string]string, len(flowAliasRules))
	for _, rule := range flowAliasRules {
		ruleMap[rule.From] = rule.To
	}

	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			return canonicalizeResult{}, readErr
		}
		out, replacements := rewriteCueActionAliases(string(raw), ruleMap)
		if replacements == 0 {
			continue
		}
		res.Replacements += replacements
		res.FilesChanged++
		rel := filepath.ToSlash(file)
		res.ChangedFiles = append(res.ChangedFiles, rel)
		if !checkOnly {
			if writeErr := os.WriteFile(file, []byte(out), 0o644); writeErr != nil {
				return canonicalizeResult{}, writeErr
			}
		}
	}
	sort.Strings(res.ChangedFiles)
	return res, nil
}

func rewriteCueActionAliases(src string, rules map[string]string) (string, int) {
	replacements := 0
	out := actionLineRe.ReplaceAllStringFunc(src, func(line string) string {
		m := actionLineRe.FindStringSubmatch(line)
		if len(m) != 4 {
			return line
		}
		current := strings.TrimSpace(m[2])
		next, ok := rules[current]
		if !ok || next == current {
			return line
		}
		replacements++
		return m[1] + next + m[3]
	})
	return out, replacements
}

func aliasRuleMap() map[string]string {
	out := make(map[string]string, len(flowAliasRules))
	for _, rule := range flowAliasRules {
		out[rule.From] = rule.To
	}
	return out
}

func collectCueFiles(root string) ([]string, error) {
	base := filepath.Clean(root)
	var files []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".cue") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
