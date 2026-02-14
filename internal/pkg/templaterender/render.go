package templaterender

import (
	"bytes"
	"text/template"
)

// RenderString renders a Go template string with missing keys defaulting to zero values.
func RenderString(src string, data any) (string, error) {
	if src == "" {
		return "", nil
	}
	t, err := template.New("tpl").Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
