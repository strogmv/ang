package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// MethodMetaOutputFile is the method metadata key of a file output:
//
//	output: {kind: "file", contentType: "text/csv", filename: "orders.csv"}
//
// The handler writes the bytes with Content-Type and Content-Disposition
// instead of a JSON object.
const MethodMetaOutputFile = "output_file"

// FileOutputSpec is what a file output declares. ContentType and Filename are
// defaults: the operation may set others on the response.
type FileOutputSpec struct {
	ContentType string
	Filename    string
	// Disposition is "attachment" (the default: the browser saves the file)
	// or "inline" (it shows it).
	Disposition string
}

// fileOutputFields are the response fields of a file output, in Go:
// Body io.ReadCloser, Content []byte, ContentType, Filename string, Size int64.
// They are marked secret so JSON-facing emitters (TypeScript types, OpenAPI
// schemas) leave them out: they never travel as JSON.
func fileOutputFields() []Field {
	return []Field{
		{Name: "body", Type: "any", IsOptional: true, IsSecret: true},
		{Name: "content", Type: "any", IsOptional: true, IsSecret: true},
		{Name: "contentType", Type: "string", IsOptional: true, IsSecret: true},
		{Name: "filename", Type: "string", IsOptional: true, IsSecret: true},
		{Name: "size", Type: "int64", IsOptional: true, IsSecret: true},
	}
}

// parseFileOutput reads output: {kind: "file", ...}. ok is false for any
// other output (no kind, or kind is a field of the response).
func parseFileOutput(name string, outVal cue.Value) (Entity, *FileOutputSpec, bool, error) {
	kindVal := outVal.LookupPath(cue.ParsePath("kind"))
	if !kindVal.Exists() || !kindVal.IsConcrete() {
		return Entity{}, nil, false, nil
	}
	kind, err := kindVal.String()
	if err != nil || kind != "file" {
		return Entity{}, nil, false, nil
	}
	spec := &FileOutputSpec{Disposition: "attachment"}
	iter, err := outVal.Fields(cue.All())
	if err != nil {
		return Entity{}, nil, false, err
	}
	for iter.Next() {
		key := cleanName(iter.Selector().String())
		v := iter.Value()
		str := func() (string, error) {
			s, err := v.String()
			if err != nil {
				return "", fmt.Errorf("output %s must be a string: %w", key, err)
			}
			return strings.TrimSpace(s), nil
		}
		switch key {
		case "kind":
		case "contentType":
			if spec.ContentType, err = str(); err != nil {
				return Entity{}, nil, false, err
			}
			if strings.ContainsAny(spec.ContentType, "\r\n") {
				return Entity{}, nil, false, fmt.Errorf("output contentType must be one line")
			}
		case "filename":
			if spec.Filename, err = str(); err != nil {
				return Entity{}, nil, false, err
			}
			if strings.ContainsAny(spec.Filename, "\r\n\"/\\") {
				return Entity{}, nil, false, fmt.Errorf("output filename %q: no quotes, slashes or line breaks", spec.Filename)
			}
		case "disposition":
			if spec.Disposition, err = str(); err != nil {
				return Entity{}, nil, false, err
			}
			if spec.Disposition != "attachment" && spec.Disposition != "inline" {
				return Entity{}, nil, false, fmt.Errorf("output disposition must be \"attachment\" or \"inline\", got %q", spec.Disposition)
			}
		default:
			return Entity{}, nil, false, fmt.Errorf("a file output has kind, contentType, filename and disposition; %q is not one of them (the operation sets resp.Content or resp.Body)", key)
		}
	}
	ent := Entity{Name: name, Fields: fileOutputFields(), Source: formatPos(outVal)}
	return ent, spec, true, nil
}

// MethodFileOutput returns the file output a method declares, nil for a
// JSON output. It reads the metadata after an IR round trip as well.
func MethodFileOutput(meta map[string]any) *FileOutputSpec {
	raw, ok := meta[MethodMetaOutputFile]
	if !ok {
		return nil
	}
	get := func(key string) string {
		switch m := raw.(type) {
		case map[string]any:
			s, _ := m[key].(string)
			return s
		case map[string]string:
			return m[key]
		}
		return ""
	}
	spec := &FileOutputSpec{ContentType: get("content_type"), Filename: get("filename"), Disposition: get("disposition")}
	if spec.Disposition == "" {
		spec.Disposition = "attachment"
	}
	return spec
}

func (s *FileOutputSpec) metadata() map[string]any {
	return map[string]any{"content_type": s.ContentType, "filename": s.Filename, "disposition": s.Disposition}
}
