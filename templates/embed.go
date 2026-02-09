package templates

import "embed"

// FS contains all template files embedded in the binary.
// This allows `go install` to work without external template files.
//
//go:embed *.tmpl
//go:embed frontend
//go:embed k8s
//go:embed python
var FS embed.FS
