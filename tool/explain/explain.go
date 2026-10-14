package explain

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type knowledge struct {
	Title       string
	Description string
	// Fix is a concrete, actionable instruction the AI can execute immediately.
	Fix string
	// ActionRef names the primary flow action involved (e.g. "http.Request").
	// AI can look it up in 'ang actions --json'.
	ActionRef string
	// SchemaRef is the ops-schema rule code violated (e.g. "R_OP_MODE_XOR").
	// AI can look it up in 'ang ops schema --json'.
	SchemaRef string
	Hint      string
	DocAnchor string
	Expected  []string
}

type Input struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
	Action  string `json:"action,omitempty"`
	Hint    string `json:"hint,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Stage   string `json:"stage,omitempty"`
}

type Item struct {
	Code        string   `json:"code"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Path        string   `json:"path,omitempty"`
	Expected    []string `json:"expected,omitempty"`
	Found       []string `json:"found,omitempty"`
	Hint        string   `json:"hint,omitempty"`
	// Fix is a concrete, AI-executable instruction (e.g. "Add timeout: \"5s\" to step args").
	Fix string `json:"fix,omitempty"`
	// ActionRef is the primary flow action involved — look up in 'ang actions --json'.
	ActionRef string `json:"action_ref,omitempty"`
	// SchemaRef is the ops-schema rule code violated — look up in 'ang ops schema --json'.
	SchemaRef string `json:"schema_ref,omitempty"`
	DocAnchor string `json:"doc_anchor,omitempty"`
}

type Envelope struct {
	Schema string `json:"schema"`
	Items  []Item `json:"items"`
}

var contractErrRe = regexp.MustCompile(`\[(?P<stage>[A-Z_]+):(?P<code>[A-Z0-9_]+)\]\s*(?P<rest>.*)`)
var quotedWordRe = regexp.MustCompile(`["']([^"']+)["']`)

func ExplainAnyInput(input string) ([]Item, error) {
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}
	if raw, ok, err := readMaybeJSON(input); err != nil {
		return nil, err
	} else if ok {
		diags, derr := DecodeDiagnostics(raw)
		if derr != nil {
			return nil, derr
		}
		items := make([]Item, 0, len(diags))
		for _, d := range diags {
			items = append(items, ExplainFromInput(d))
		}
		return items, nil
	}
	return []Item{ExplainFromInput(Input{Code: strings.ToUpper(strings.TrimSpace(input))})}, nil
}

func readMaybeJSON(input string) ([]byte, bool, error) {
	if strings.HasPrefix(input, "{") || strings.HasPrefix(input, "[") {
		return []byte(input), true, nil
	}
	if input == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, false, fmt.Errorf("read stdin: %w", err)
		}
		return raw, true, nil
	}
	clean := filepath.Clean(input)
	raw, err := os.ReadFile(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", clean, err)
	}
	trim := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") {
		return []byte(trim), true, nil
	}
	return nil, false, nil
}

func DecodeDiagnostics(raw []byte) ([]Input, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse json diagnostics: %w", err)
	}
	items := flattenDiagnostics(payload)
	uniq := make(map[string]Input, len(items))
	for _, it := range items {
		key := strings.Join([]string{
			strings.TrimSpace(it.Code),
			strings.TrimSpace(it.Path),
			strings.TrimSpace(it.Message),
			strconvI(it.Line),
			strconvI(it.Column),
		}, "|")
		uniq[key] = it
	}
	keys := make([]string, 0, len(uniq))
	for key := range uniq {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Input, 0, len(keys))
	for _, key := range keys {
		out = append(out, uniq[key])
	}
	return out, nil
}

func flattenDiagnostics(v any) []Input {
	switch x := v.(type) {
	case []any:
		out := make([]Input, 0, len(x))
		for _, item := range x {
			out = append(out, flattenDiagnostics(item)...)
		}
		return out
	case map[string]any:
		// ang/diags/v1 format: {"schema":"ang/diags/v1","valid":...,"diagnostics":[...]}
		if diagnostics, ok := x["diagnostics"].([]any); ok {
			var out []Input
			for _, d := range diagnostics {
				if dm, ok := d.(map[string]any); ok {
					out = append(out, diagFromMap(dm))
				}
			}
			return out
		}
		if warnings, ok := x["warnings"].([]any); ok {
			var out []Input
			for _, w := range warnings {
				if wm, okMap := w.(map[string]any); okMap {
					out = append(out, diagFromMap(wm))
				}
			}
			if errorsList, okErr := x["errors"].([]any); okErr {
				for _, e := range errorsList {
					if em, okMap := e.(map[string]any); okMap {
						msg := asString(em["message"])
						if parsed, okParsed := parseContractMessage(msg); okParsed {
							out = append(out, parsed)
						}
					}
				}
			}
			return out
		}
		if code := asString(x["code"]); code != "" || asString(x["message"]) != "" {
			return []Input{diagFromMap(x)}
		}
		if msg := asString(x["message"]); msg != "" {
			if parsed, ok := parseContractMessage(msg); ok {
				return []Input{parsed}
			}
		}
	}
	return nil
}

func diagFromMap(m map[string]any) Input {
	it := Input{
		Code:    strings.ToUpper(strings.TrimSpace(asString(m["code"]))),
		Message: asString(m["message"]),
		Path:    asString(m["cue_path"]),
		Action:  asString(m["action"]),
		Hint:    asString(m["hint"]),
		File:    asString(m["file"]),
		Line:    asInt(m["line"]),
		Column:  asInt(m["column"]),
	}
	if it.Path == "" {
		step := asInt(m["step"])
		if step > 0 {
			if it.Action != "" {
				it.Path = fmt.Sprintf("flow[%d] (%s)", step, it.Action)
			} else {
				it.Path = fmt.Sprintf("flow[%d]", step)
			}
		}
	}
	if it.Code == "" && it.Message != "" {
		if parsed, ok := parseContractMessage(it.Message); ok {
			if parsed.Code != "" {
				it.Code = parsed.Code
			}
			if parsed.Stage != "" {
				it.Stage = parsed.Stage
			}
		}
	}
	return it
}

func parseContractMessage(msg string) (Input, bool) {
	m := contractErrRe.FindStringSubmatch(strings.TrimSpace(msg))
	if len(m) < 4 {
		return Input{}, false
	}
	return Input{
		Stage:   strings.TrimSpace(m[1]),
		Code:    strings.TrimSpace(m[2]),
		Message: strings.TrimSpace(m[3]),
	}, true
}

func ExplainFromInput(in Input) Item {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	knowledge, ok := explainDB[code]
	if !ok {
		knowledge = inferKnowledge(code, in)
	}

	item := Item{
		Code:        code,
		Title:       knowledge.Title,
		Description: knowledge.Description,
		Path:        in.Path,
		Expected:    append([]string(nil), knowledge.Expected...),
		Fix:         knowledge.Fix,
		ActionRef:   knowledge.ActionRef,
		SchemaRef:   knowledge.SchemaRef,
		DocAnchor:   knowledge.DocAnchor,
	}
	found := buildFound(in)
	if len(found) > 0 {
		item.Found = found
	}
	if strings.TrimSpace(in.Hint) != "" {
		item.Hint = in.Hint
	} else {
		item.Hint = knowledge.Hint
	}
	if item.Code == "" {
		item.Code = "UNKNOWN_ERROR"
	}
	if item.Title == "" {
		item.Title = "Unknown diagnostic code"
	}
	if item.Description == "" {
		item.Description = "No built-in explanation yet. Check message details and source location."
	}
	return item
}

func buildFound(in Input) []string {
	var out []string
	if in.Action != "" {
		out = append(out, "action="+in.Action)
	}
	if strings.TrimSpace(in.Message) != "" {
		out = append(out, in.Message)
	}
	if in.File != "" && in.Line > 0 {
		out = append(out, fmt.Sprintf("%s:%d:%d", in.File, in.Line, in.Column))
	}
	return out
}
