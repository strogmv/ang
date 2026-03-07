package python

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Runtime struct {
	Funcs        template.FuncMap
	ReadTemplate func(path string) ([]byte, error)
}

func (r Runtime) RenderTemplate(root, tmplPath string, data any, outName string, mode os.FileMode) error {
	tmplContent, err := r.ReadTemplate(tmplPath)
	if err != nil {
		return fmt.Errorf("read python template %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Funcs(r.Funcs).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse python template %s: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render python template %s: %w", tmplPath, err)
	}

	path := filepath.Join(root, outName)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", outName, err)
	}
	content := buf.Bytes()
	if ShouldPreserveCustomBlocks(outName) {
		if prev, err := os.ReadFile(path); err == nil {
			content = []byte(MergeCustomBlocks(string(content), string(prev)))
		}
	}
	if err := writeFileAtomic(path, content, mode); err != nil {
		return fmt.Errorf("write python file %s: %w", outName, err)
	}
	fmt.Printf("Generated Python SDK: %s\n", path)
	return nil
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ang-py-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}
	ok = true
	return nil
}

func ShouldPreserveCustomBlocks(outName string) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(outName))
	return strings.HasPrefix(normalized, "services/") || strings.HasPrefix(normalized, "repositories/postgres/")
}

func MergeCustomBlocks(generated, existing string) string {
	existingBlocks := extractCustomBlocks(existing)
	if len(existingBlocks) == 0 {
		return generated
	}
	lines := strings.SplitAfter(generated, "\n")
	var out strings.Builder

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		beginKey, isBegin := parseCustomMarkerLine(line, "BEGIN")
		if !isBegin {
			out.WriteString(line)
			continue
		}

		out.WriteString(line)
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseCustomMarkerLine(lines[j], "END")
			if isEnd && endKey == beginKey {
				break
			}
		}
		if j >= len(lines) {
			for k := i + 1; k < len(lines); k++ {
				out.WriteString(lines[k])
			}
			break
		}

		if prevBody, ok := existingBlocks[beginKey]; ok {
			out.WriteString(prevBody)
		} else {
			for k := i + 1; k < j; k++ {
				out.WriteString(lines[k])
			}
		}
		out.WriteString(lines[j])
		i = j
	}
	return out.String()
}

func extractCustomBlocks(content string) map[string]string {
	lines := strings.SplitAfter(content, "\n")
	out := map[string]string{}

	for i := 0; i < len(lines); i++ {
		key, isBegin := parseCustomMarkerLine(lines[i], "BEGIN")
		if !isBegin {
			continue
		}
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseCustomMarkerLine(lines[j], "END")
			if isEnd && endKey == key {
				break
			}
		}
		if j >= len(lines) {
			continue
		}
		var body strings.Builder
		for k := i + 1; k < j; k++ {
			body.WriteString(lines[k])
		}
		out[key] = body.String()
		i = j
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseCustomMarkerLine(line, kind string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := "# ANG:" + kind + "_CUSTOM "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if key == "" {
		return "", false
	}
	return key, true
}
