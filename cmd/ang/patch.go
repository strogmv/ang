package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type patchDocument struct {
	Schema string                   `json:"schema"`
	Ops    []map[string]interface{} `json:"ops"`
}

type patchIssue struct {
	Index   int    `json:"index"`
	Op      string `json:"op,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type patchPlanItem struct {
	Index   int    `json:"index"`
	Op      string `json:"op"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

type patchApplyResult struct {
	ChangedFiles []string `json:"changed_files,omitempty"`
	Plan         []patchPlanItem
}

func runPatch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang patch <lint|plan|apply> <patch.json|-> [--json]")
		os.Exit(1)
	}
	cmd := strings.TrimSpace(args[0])
	switch cmd {
	case "lint":
		runPatchLint(args[1:])
	case "plan":
		runPatchPlan(args[1:])
	case "apply":
		runPatchApply(args[1:])
	default:
		fmt.Printf("Patch FAILED: unknown subcommand %q\n", cmd)
		fmt.Println("Usage: ang patch <lint|plan|apply> <patch.json|-> [--json]")
		os.Exit(1)
	}
}

func runPatchLint(args []string) {
	patchFile, jsonOut := parsePatchCommonFlags("patch lint", args)
	doc, err := readPatchDocument(patchFile)
	if err != nil {
		fmt.Printf("Patch lint FAILED: %v\n", err)
		os.Exit(1)
	}
	issues := lintPatchDocument(doc)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema": "ang/patch-lint/v1",
			"ok":     len(issues) == 0,
			"issues": issues,
		})
	} else {
		if len(issues) == 0 {
			fmt.Println("Patch lint OK")
		} else {
			fmt.Println("Patch lint FAILED")
			for _, it := range issues {
				fmt.Printf("  - [%s] op[%d] %s\n", it.Code, it.Index, it.Message)
				if it.Path != "" {
					fmt.Printf("      path: %s\n", it.Path)
				}
				if it.Hint != "" {
					fmt.Printf("      hint: %s\n", it.Hint)
				}
			}
		}
	}
	if len(issues) > 0 {
		os.Exit(1)
	}
}

func runPatchPlan(args []string) {
	patchFile, jsonOut := parsePatchCommonFlags("patch plan", args)
	doc, err := readPatchDocument(patchFile)
	if err != nil {
		fmt.Printf("Patch plan FAILED: %v\n", err)
		os.Exit(1)
	}
	issues := lintPatchDocument(doc)
	if len(issues) > 0 {
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{
				"schema": "ang/patch-plan/v1",
				"ok":     false,
				"issues": issues,
			})
		} else {
			fmt.Println("Patch plan FAILED (lint errors):")
			for _, it := range issues {
				fmt.Printf("  - op[%d] %s: %s\n", it.Index, it.Code, it.Message)
			}
		}
		os.Exit(1)
	}
	plan := buildPatchPlan(doc)
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema": "ang/patch-plan/v1",
			"ok":     true,
			"plan":   plan,
		})
		return
	}
	fmt.Println("Patch plan")
	for _, it := range plan {
		fmt.Printf("  - op[%d] %s -> %s (%s)\n", it.Index, it.Op, it.Path, it.Summary)
	}
}

func runPatchApply(args []string) {
	patchFile, jsonOut := parsePatchCommonFlags("patch apply", args)
	doc, err := readPatchDocument(patchFile)
	if err != nil {
		fmt.Printf("Patch apply FAILED: %v\n", err)
		os.Exit(1)
	}
	issues := lintPatchDocument(doc)
	if len(issues) > 0 {
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{
				"schema": "ang/patch-apply/v1",
				"ok":     false,
				"issues": issues,
			})
		} else {
			fmt.Println("Patch apply FAILED (lint errors):")
			for _, it := range issues {
				fmt.Printf("  - op[%d] %s: %s\n", it.Index, it.Code, it.Message)
			}
		}
		os.Exit(1)
	}

	result, err := applyPatchDocument(doc)
	if err != nil {
		fmt.Printf("Patch apply FAILED: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema": "ang/patch-apply/v1",
			"ok":     true,
			"result": result,
		})
		return
	}
	fmt.Printf("Patch apply OK: changed_files=%d\n", len(result.ChangedFiles))
	for _, f := range result.ChangedFiles {
		fmt.Printf("  - %s\n", f)
	}
}

func parsePatchCommonFlags(name string, args []string) (string, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit machine-readable output")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("%s FAILED: %v\n", strings.Title(name), err)
		os.Exit(1)
	}
	patchFile := "-"
	if fs.NArg() > 0 {
		patchFile = fs.Arg(0)
	}
	return patchFile, *jsonOut
}

func readPatchDocument(path string) (patchDocument, error) {
	var raw []byte
	var err error
	if path == "-" || strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(os.Stdin)
		if err != nil {
			return patchDocument{}, fmt.Errorf("read stdin: %w", err)
		}
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			return patchDocument{}, err
		}
	}
	var doc patchDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return patchDocument{}, fmt.Errorf("parse patch json: %w", err)
	}
	return doc, nil
}

func lintPatchDocument(doc patchDocument) []patchIssue {
	var issues []patchIssue
	if strings.TrimSpace(doc.Schema) == "" {
		issues = append(issues, patchIssue{
			Index:   -1,
			Code:    "MISSING_SCHEMA",
			Message: "patch.schema is required",
			Hint:    "Use schema: \"ang/patch/v1\"",
		})
	} else if strings.TrimSpace(doc.Schema) != "ang/patch/v1" {
		issues = append(issues, patchIssue{
			Index:   -1,
			Code:    "UNSUPPORTED_SCHEMA",
			Message: fmt.Sprintf("unsupported patch schema %q", doc.Schema),
			Hint:    "Use schema: \"ang/patch/v1\"",
		})
	}
	if len(doc.Ops) == 0 {
		issues = append(issues, patchIssue{
			Index:   -1,
			Code:    "MISSING_OPS",
			Message: "patch.ops must not be empty",
		})
		return issues
	}

	for i, op := range doc.Ops {
		opName := opString(op, "op")
		if opName == "" {
			issues = append(issues, patchIssue{Index: i, Code: "MISSING_OP", Message: "op is required"})
			continue
		}
		file := opString(op, "file")
		if file == "" {
			issues = append(issues, patchIssue{Index: i, Op: opName, Code: "MISSING_FILE", Message: "file is required"})
		} else if _, err := cleanPatchPath(file); err != nil {
			issues = append(issues, patchIssue{Index: i, Op: opName, Code: "INVALID_FILE", Message: err.Error(), Path: file})
		}

		switch opName {
		case "addEntity":
			requireOpKeys(&issues, i, opName, op, "entity", "fields")
		case "addField":
			requireOpKeys(&issues, i, opName, op, "entity", "field", "type")
		case "addEndpoint":
			requireOpKeys(&issues, i, opName, op, "name", "method", "path")
		case "appendFlowStep":
			requireOpKeys(&issues, i, opName, op, "operation", "step")
		case "replaceFlowStep":
			requireOpKeys(&issues, i, opName, op, "operation", "index", "step")
			if _, ok := opInt(op, "index"); !ok {
				issues = append(issues, patchIssue{Index: i, Op: opName, Code: "INVALID_INDEX", Message: "index must be integer >= 0"})
			}
		case "setPolicy":
			requireOpKeys(&issues, i, opName, op, "permission", "roles")
		default:
			issues = append(issues, patchIssue{
				Index:   i,
				Op:      opName,
				Code:    "UNKNOWN_OP",
				Message: fmt.Sprintf("unknown patch op %q", opName),
			})
		}
	}
	return issues
}

func requireOpKeys(issues *[]patchIssue, idx int, opName string, op map[string]interface{}, keys ...string) {
	for _, key := range keys {
		if strings.TrimSpace(fmt.Sprint(op[key])) == "" || op[key] == nil {
			*issues = append(*issues, patchIssue{
				Index:   idx,
				Op:      opName,
				Code:    "MISSING_" + strings.ToUpper(key),
				Message: fmt.Sprintf("%s requires %q", opName, key),
			})
		}
	}
}

func buildPatchPlan(doc patchDocument) []patchPlanItem {
	items := make([]patchPlanItem, 0, len(doc.Ops))
	for i, op := range doc.Ops {
		opName := opString(op, "op")
		path := opString(op, "file")
		if path == "" {
			path = "<missing>"
		}
		summary := opName
		switch opName {
		case "addEntity":
			summary = "add entity " + opString(op, "entity")
		case "addField":
			summary = "add field " + opString(op, "entity") + "." + opString(op, "field")
		case "addEndpoint":
			summary = "add endpoint " + opString(op, "name")
		case "appendFlowStep":
			summary = "append flow step to " + opString(op, "operation")
		case "replaceFlowStep":
			summary = fmt.Sprintf("replace flow step %s[%d]", opString(op, "operation"), mustInt(op, "index"))
		case "setPolicy":
			summary = "set policy " + opString(op, "permission")
		}
		items = append(items, patchPlanItem{
			Index:   i,
			Op:      opName,
			Path:    filepath.ToSlash(path),
			Summary: summary,
		})
	}
	return items
}

func applyPatchDocument(doc patchDocument) (patchApplyResult, error) {
	cache := map[string]string{}
	changed := map[string]struct{}{}
	plan := buildPatchPlan(doc)

	load := func(path string) (string, error) {
		if v, ok := cache[path]; ok {
			return v, nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		cache[path] = string(raw)
		return cache[path], nil
	}
	save := func(path, content string) {
		cache[path] = content
		changed[path] = struct{}{}
	}

	for i, op := range doc.Ops {
		opName := opString(op, "op")
		p, err := cleanPatchPath(opString(op, "file"))
		if err != nil {
			return patchApplyResult{}, err
		}
		content, err := load(p)
		if err != nil {
			return patchApplyResult{}, err
		}
		next := content
		switch opName {
		case "addEntity":
			next, err = patchAddEntity(content, op)
		case "addField":
			next, err = patchAddField(content, op)
		case "addEndpoint":
			next, err = patchAddEndpoint(content, op)
		case "appendFlowStep":
			next, err = patchAppendFlowStep(content, op)
		case "replaceFlowStep":
			next, err = patchReplaceFlowStep(content, op)
		case "setPolicy":
			next, err = patchSetPolicy(content, op)
		default:
			err = fmt.Errorf("unknown op %q", opName)
		}
		if err != nil {
			return patchApplyResult{}, fmt.Errorf("op[%d] %s: %w", i, opName, err)
		}
		if next != content {
			save(p, next)
		}
	}

	changedFiles := make([]string, 0, len(changed))
	for p := range changed {
		if err := os.WriteFile(p, []byte(cache[p]), 0o644); err != nil {
			return patchApplyResult{}, err
		}
		changedFiles = append(changedFiles, filepath.ToSlash(p))
	}
	sort.Strings(changedFiles)
	return patchApplyResult{
		ChangedFiles: changedFiles,
		Plan:         plan,
	}, nil
}

func patchAddEntity(content string, op map[string]interface{}) (string, error) {
	entity := strings.TrimSpace(opString(op, "entity"))
	if entity == "" {
		return content, errors.New("entity is required")
	}
	if _, _, _, err := findBlockRange(content, "#"+entity, 0, len(content)); err == nil {
		return content, nil
	}
	fields, err := parseEntityFields(op["fields"])
	if err != nil {
		return content, err
	}
	var b strings.Builder
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("#" + entity + ": {\n")
	b.WriteString("\tname: " + strconv.Quote(entity) + "\n")
	b.WriteString("\tfields: {\n")
	for _, f := range fields {
		opt := ""
		if f.Optional {
			opt = "?"
		}
		b.WriteString("\t\t" + f.Name + opt + ": {type: " + strconv.Quote(f.Type) + "}\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return content + b.String(), nil
}

func patchAddField(content string, op map[string]interface{}) (string, error) {
	entity := strings.TrimSpace(opString(op, "entity"))
	field := strings.TrimSpace(opString(op, "field"))
	typ := strings.TrimSpace(opString(op, "type"))
	optional := opBool(op, "optional")
	if entity == "" || field == "" || typ == "" {
		return content, errors.New("entity/field/type are required")
	}
	open, close, _, err := findBlockRange(content, "#"+entity, 0, len(content))
	if err != nil {
		open, close, _, err = findBlockRange(content, entity, 0, len(content))
		if err != nil {
			return content, fmt.Errorf("entity block not found: %s", entity)
		}
	}
	fOpen, fClose, fIndent, err := findBlockRange(content, "fields", open, close+1)
	if err != nil || fOpen < open || fClose > close {
		return content, fmt.Errorf("fields block not found in entity %s", entity)
	}
	fieldPattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(field) + `\??\s*:`)
	if fieldPattern.FindStringIndex(content[fOpen:fClose+1]) != nil {
		return content, nil
	}
	opt := ""
	if optional {
		opt = "?"
	}
	line := fIndent + "\t" + field + opt + ": {type: " + strconv.Quote(typ) + "}\n"
	return content[:fClose] + line + content[fClose:], nil
}

func patchAddEndpoint(content string, op map[string]interface{}) (string, error) {
	name := strings.TrimSpace(opString(op, "name"))
	method := strings.TrimSpace(opString(op, "method"))
	path := strings.TrimSpace(opString(op, "path"))
	if name == "" || method == "" || path == "" {
		return content, errors.New("name/method/path are required")
	}
	open, close, indent, err := findBlockRange(content, "HTTP", 0, len(content))
	if err != nil {
		return content, errors.New("HTTP block not found")
	}
	if _, _, _, err := findBlockRange(content, name, open, close+1); err == nil {
		return content, nil
	}
	entry := ""
	if !strings.HasSuffix(content[:close], "\n") {
		entry += "\n"
	}
	entry += indent + "\t" + name + ": {\n"
	entry += indent + "\t\tmethod: " + strconv.Quote(strings.ToUpper(method)) + "\n"
	entry += indent + "\t\tpath:   " + strconv.Quote(path) + "\n"
	entry += indent + "\t}\n"
	return content[:close] + entry + content[close:], nil
}

func patchAppendFlowStep(content string, op map[string]interface{}) (string, error) {
	operation := strings.TrimSpace(opString(op, "operation"))
	stepMap, ok := op["step"].(map[string]interface{})
	if operation == "" || !ok {
		return content, errors.New("operation and step object are required")
	}
	opOpen, opClose, opIndent, err := findBlockRange(content, operation, 0, len(content))
	if err != nil {
		return content, fmt.Errorf("operation %s not found", operation)
	}
	stepText := renderCueValue(stepMap, opIndent+"\t\t")
	flowOpen, flowClose, flowIndent, flowErr := findListRangeByKey(content, "flow", opOpen, opClose+1)
	if flowErr != nil {
		insert := ""
		if !strings.HasSuffix(content[:opClose], "\n") {
			insert += "\n"
		}
		insert += opIndent + "\tflow: [\n"
		insert += stepText + ",\n"
		insert += opIndent + "\t]\n"
		return content[:opClose] + insert + content[opClose:], nil
	}
	body := strings.TrimSpace(content[flowOpen+1 : flowClose])
	if body == "" {
		replacement := "\n" + stepText + ",\n" + flowIndent
		return content[:flowOpen+1] + replacement + content[flowClose:], nil
	}
	insert := "\n" + stepText + ","
	return content[:flowClose] + insert + content[flowClose:], nil
}

func patchReplaceFlowStep(content string, op map[string]interface{}) (string, error) {
	operation := strings.TrimSpace(opString(op, "operation"))
	index, ok := opInt(op, "index")
	stepMap, stepOK := op["step"].(map[string]interface{})
	if operation == "" || !ok || index < 0 || !stepOK {
		return content, errors.New("operation/index/step are required")
	}
	opOpen, opClose, _, err := findBlockRange(content, operation, 0, len(content))
	if err != nil {
		return content, fmt.Errorf("operation %s not found", operation)
	}
	flowOpen, flowClose, flowIndent, err := findListRangeByKey(content, "flow", opOpen, opClose+1)
	if err != nil {
		return content, fmt.Errorf("flow list not found in operation %s", operation)
	}
	body := content[flowOpen+1 : flowClose]
	items := findTopLevelListItems(body)
	if index >= len(items) {
		return content, fmt.Errorf("flow index %d out of range (len=%d)", index, len(items))
	}
	stepText := renderCueValue(stepMap, flowIndent+"\t")
	absStart := flowOpen + 1 + items[index][0]
	absEnd := flowOpen + 1 + items[index][1]
	return content[:absStart] + stepText + content[absEnd:], nil
}

func patchSetPolicy(content string, op map[string]interface{}) (string, error) {
	permission := strings.TrimSpace(opString(op, "permission"))
	if permission == "" {
		return content, errors.New("permission is required")
	}
	description := strings.TrimSpace(opString(op, "description"))
	if description == "" {
		description = permission
	}
	roles := opStringSlice(op, "roles")
	if len(roles) == 0 {
		return content, errors.New("roles must not be empty")
	}
	rOpen, rClose, _, err := findBlockRange(content, "#RBAC", 0, len(content))
	if err != nil {
		rOpen, rClose, _, err = findBlockRange(content, "RBAC", 0, len(content))
		if err != nil {
			return content, errors.New("RBAC block not found")
		}
	}
	rolesOpen, rolesClose, rolesIndent, err := findBlockRange(content, "roles", rOpen, rClose+1)
	if err != nil {
		return content, errors.New("roles block not found")
	}
	permsOpen, permsClose, permsIndent, err := findBlockRange(content, "permissions", rOpen, rClose+1)
	if err != nil {
		return content, errors.New("permissions block not found")
	}

	next := content
	permPattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(strconv.Quote(permission)) + `\s*:`)
	if permPattern.FindStringIndex(next[permsOpen:permsClose+1]) == nil {
		line := permsIndent + "\t" + strconv.Quote(permission) + ": " + strconv.Quote(description) + "\n"
		next = next[:permsClose] + line + next[permsClose:]
		shift := len(line)
		if permsClose < rolesOpen {
			rolesOpen += shift
			rolesClose += shift
		}
	}

	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		nextRoles, changed := patchRolePermission(next, rolesOpen, rolesClose, rolesIndent, role, permission)
		if changed {
			next = nextRoles
			// Recalculate roles range after mutation.
			rolesOpen, rolesClose, _, _ = findBlockRange(next, "roles", rOpen, len(next))
		}
	}
	return next, nil
}

func patchRolePermission(content string, rolesOpen, rolesClose int, rolesIndent, role, permission string) (string, bool) {
	sub := content[rolesOpen:rolesClose]
	re := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(role) + `\s*:\s*\[(.*)\]\s*$`)
	loc := re.FindStringSubmatchIndex(sub)
	quoted := strconv.Quote(permission)
	if loc == nil {
		line := rolesIndent + "\t" + role + ": [" + quoted + "]\n"
		out := content[:rolesClose] + line + content[rolesClose:]
		return out, true
	}
	line := sub[loc[0]:loc[1]]
	if strings.Contains(line, quoted) {
		return content, false
	}
	repl := line
	if strings.Contains(line, "[]") {
		repl = strings.Replace(line, "[]", "["+quoted+"]", 1)
	} else {
		last := strings.LastIndex(line, "]")
		if last < 0 {
			return content, false
		}
		repl = line[:last] + ", " + quoted + line[last:]
	}
	absStart := rolesOpen + loc[0]
	absEnd := rolesOpen + loc[1]
	out := content[:absStart] + repl + content[absEnd:]
	return out, true
}

type entityField struct {
	Name     string
	Type     string
	Optional bool
}

func parseEntityFields(raw interface{}) ([]entityField, error) {
	switch x := raw.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fields := make([]entityField, 0, len(keys))
		for _, k := range keys {
			switch v := x[k].(type) {
			case string:
				fields = append(fields, entityField{Name: k, Type: strings.TrimSpace(v)})
			case map[string]interface{}:
				fields = append(fields, entityField{
					Name:     k,
					Type:     strings.TrimSpace(opString(v, "type")),
					Optional: opBool(v, "optional"),
				})
			default:
				return nil, fmt.Errorf("invalid field spec for %s", k)
			}
		}
		return fields, nil
	case []interface{}:
		fields := make([]entityField, 0, len(x))
		for _, item := range x {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, errors.New("fields list must contain objects")
			}
			name := strings.TrimSpace(opString(m, "name"))
			typ := strings.TrimSpace(opString(m, "type"))
			if name == "" || typ == "" {
				return nil, errors.New("field object requires name and type")
			}
			fields = append(fields, entityField{Name: name, Type: typ, Optional: opBool(m, "optional")})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		return fields, nil
	default:
		return nil, errors.New("fields must be object or array")
	}
}

func cleanPatchPath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", errors.New("empty file path")
	}
	if filepath.IsAbs(p) {
		return "", errors.New("absolute file paths are not allowed")
	}
	cp := filepath.Clean(p)
	if cp == "." || strings.HasPrefix(cp, "..") {
		return "", errors.New("path traversal is not allowed")
	}
	return cp, nil
}

func renderCueValue(v interface{}, indent string) string {
	switch x := v.(type) {
	case map[string]interface{}:
		if len(x) == 0 {
			return "{}"
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i] == "action" {
				return true
			}
			if keys[j] == "action" {
				return false
			}
			return keys[i] < keys[j]
		})
		var b strings.Builder
		b.WriteString("{\n")
		for _, k := range keys {
			b.WriteString(indent + "\t" + cueKey(k) + ": " + renderCueValue(x[k], indent+"\t") + "\n")
		}
		b.WriteString(indent + "}")
		return b.String()
	case []interface{}:
		if len(x) == 0 {
			return "[]"
		}
		allSimple := true
		for _, item := range x {
			switch item.(type) {
			case string, float64, bool, int, int64:
			default:
				allSimple = false
			}
		}
		if allSimple {
			parts := make([]string, 0, len(x))
			for _, item := range x {
				parts = append(parts, renderCueValue(item, indent))
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		var b strings.Builder
		b.WriteString("[\n")
		for _, item := range x {
			b.WriteString(indent + "\t" + renderCueValue(item, indent+"\t") + ",\n")
		}
		b.WriteString(indent + "]")
		return b.String()
	case string:
		return strconv.Quote(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return "null"
	default:
		return strconv.Quote(fmt.Sprint(x))
	}
}

func cueKey(key string) string {
	if regexp.MustCompile(`^[A-Za-z_#][A-Za-z0-9_#]*$`).MatchString(key) {
		return key
	}
	return strconv.Quote(key)
}

func findBlockRange(content, label string, from, to int) (int, int, string, error) {
	if from < 0 {
		from = 0
	}
	if to > len(content) || to <= from {
		to = len(content)
	}
	sub := content[from:to]
	re := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(label) + `\s*:\s*{`)
	loc := re.FindStringSubmatchIndex(sub)
	if loc == nil {
		return 0, 0, "", fmt.Errorf("block %s not found", label)
	}
	start := from + loc[0]
	end := from + loc[1]
	indent := sub[loc[2]:loc[3]]
	openRel := strings.Index(content[start:end], "{")
	if openRel < 0 {
		return 0, 0, "", fmt.Errorf("invalid block %s", label)
	}
	open := start + openRel
	close, err := findMatchingDelimiter(content, open, '{', '}')
	if err != nil {
		return 0, 0, "", err
	}
	return open, close, indent, nil
}

func findListRangeByKey(content, key string, from, to int) (int, int, string, error) {
	if from < 0 {
		from = 0
	}
	if to > len(content) || to <= from {
		to = len(content)
	}
	sub := content[from:to]
	re := regexp.MustCompile(`(?m)^([ \t]*)` + regexp.QuoteMeta(key) + `\s*:\s*\[`)
	loc := re.FindStringSubmatchIndex(sub)
	if loc == nil {
		return 0, 0, "", fmt.Errorf("list %s not found", key)
	}
	start := from + loc[0]
	end := from + loc[1]
	indent := sub[loc[2]:loc[3]]
	openRel := strings.Index(content[start:end], "[")
	if openRel < 0 {
		return 0, 0, "", fmt.Errorf("invalid list %s", key)
	}
	open := start + openRel
	close, err := findMatchingDelimiter(content, open, '[', ']')
	if err != nil {
		return 0, 0, "", err
	}
	return open, close, indent, nil
}

func findMatchingDelimiter(s string, openIdx int, openChar, closeChar byte) (int, error) {
	depth := 0
	var quote byte
	escaped := false
	for i := openIdx; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == openChar {
			depth++
			continue
		}
		if ch == closeChar {
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return -1, errors.New("unbalanced delimiters")
}

func findTopLevelListItems(body string) [][2]int {
	var items [][2]int
	i := 0
	n := len(body)
	for i < n {
		for i < n {
			ch := body[i]
			if ch == ',' || ch == ' ' || ch == '\n' || ch == '\t' || ch == '\r' {
				i++
				continue
			}
			break
		}
		if i >= n {
			break
		}
		start := i
		depthBrace := 0
		depthBracket := 0
		depthParen := 0
		var quote byte
		escaped := false
		itemDone := false
		for i < n {
			ch := body[i]
			if quote != 0 {
				if escaped {
					escaped = false
					i++
					continue
				}
				if ch == '\\' {
					escaped = true
					i++
					continue
				}
				if ch == quote {
					quote = 0
				}
				i++
				continue
			}
			if ch == '"' || ch == '\'' {
				quote = ch
				i++
				continue
			}
			switch ch {
			case '{':
				depthBrace++
			case '}':
				depthBrace--
			case '[':
				depthBracket++
			case ']':
				depthBracket--
			case '(':
				depthParen++
			case ')':
				depthParen--
			case ',':
				if depthBrace == 0 && depthBracket == 0 && depthParen == 0 {
					end := i
					for end > start && (body[end-1] == ' ' || body[end-1] == '\n' || body[end-1] == '\t' || body[end-1] == '\r') {
						end--
					}
					items = append(items, [2]int{start, end})
					i++
					itemDone = true
					break
				}
			}
			i++
		}
		if itemDone {
			continue
		}
		end := i
		for end > start && (body[end-1] == ' ' || body[end-1] == '\n' || body[end-1] == '\t' || body[end-1] == '\r') {
			end--
		}
		if end > start {
			items = append(items, [2]int{start, end})
		}
	}
	return items
}

func opString(op map[string]interface{}, key string) string {
	v, ok := op[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func opBool(op map[string]interface{}, key string) bool {
	v, ok := op[key]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "true")
	default:
		return false
	}
}

func opInt(op map[string]interface{}, key string) (int, bool) {
	v, ok := op[key]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func mustInt(op map[string]interface{}, key string) int {
	n, _ := opInt(op, key)
	return n
}

func opStringSlice(op map[string]interface{}, key string) []string {
	v, ok := op[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out
	case []string:
		out := append([]string(nil), x...)
		return out
	case string:
		if strings.TrimSpace(x) == "" {
			return nil
		}
		return []string{strings.TrimSpace(x)}
	default:
		return nil
	}
}
