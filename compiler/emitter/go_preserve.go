package emitter

import "strings"

func shouldPreserveGoCustomBlocks(outName string) bool {
	normalized := strings.TrimSpace(outName)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	switch normalized {
	case "internal/bootstrap/runtime_container.go", "cmd/server/main.go":
		return true
	default:
		return false
	}
}

func mergeGoCustomBlocks(generated, existing string) string {
	existingBlocks := extractGoCustomBlocks(existing)
	if len(existingBlocks) == 0 {
		return generated
	}
	lines := strings.SplitAfter(generated, "\n")
	var out strings.Builder
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		beginKey, isBegin := parseGoCustomMarkerLine(line, "BEGIN")
		if !isBegin {
			out.WriteString(line)
			continue
		}
		out.WriteString(line)
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseGoCustomMarkerLine(lines[j], "END")
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

// MergeGoCustomBlocksCompat preserves existing custom blocks for selected Go outputs.
func MergeGoCustomBlocksCompat(generated, existing, outName string) (string, string, bool) {
	if !shouldPreserveGoCustomBlocks(outName) || !strings.Contains(existing, "ANG:BEGIN_CUSTOM") {
		return "", "", false
	}
	return mergeGoCustomBlocks(generated, existing), "go_custom_blocks", true
}

func extractGoCustomBlocks(content string) map[string]string {
	lines := strings.SplitAfter(content, "\n")
	out := map[string]string{}
	for i := 0; i < len(lines); i++ {
		key, isBegin := parseGoCustomMarkerLine(lines[i], "BEGIN")
		if !isBegin {
			continue
		}
		j := i + 1
		for ; j < len(lines); j++ {
			endKey, isEnd := parseGoCustomMarkerLine(lines[j], "END")
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

func parseGoCustomMarkerLine(line, kind string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := "// ANG:" + kind + "_CUSTOM "
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	if key == "" {
		return "", false
	}
	return key, true
}
