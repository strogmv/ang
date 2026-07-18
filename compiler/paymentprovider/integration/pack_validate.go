package integration

import (
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const expertManifestSchema = `
#Manifest: close({
	id: string
	version: string
	goals: [string, ...string]
	dependencies?: [...string]
	priority?: int
	reads?: [...string]
	writes?: [...string]
	report_format?: "v1" | "v2"
})
`

// ValidateExpertPack checks that path is loadable CUE with a valid pack manifest.
// Does not require Expert runtime; uses the same closed manifest shape as Expert packcue.
func ValidateExpertPack(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("pack path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read pack %q: %w", path, err)
	}
	ctx := cuecontext.New()
	schemaRoot := ctx.CompileString(expertManifestSchema)
	if err := schemaRoot.Err(); err != nil {
		return fmt.Errorf("compile manifest schema: %w", err)
	}
	doc := ctx.CompileBytes(data)
	if err := doc.Err(); err != nil {
		return fmt.Errorf("compile pack %q: %w", path, err)
	}
	value := doc.LookupPath(cue.ParsePath("manifest"))
	if !value.Exists() {
		return fmt.Errorf("pack %q: manifest field is required", path)
	}
	value = value.Unify(schemaRoot.LookupPath(cue.ParsePath("#Manifest")))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("pack %q: invalid manifest: %w", path, err)
	}
	id, _ := value.LookupPath(cue.ParsePath("id")).String()
	version, _ := value.LookupPath(cue.ParsePath("version")).String()
	if strings.TrimSpace(id) == "" || strings.TrimSpace(version) == "" {
		return fmt.Errorf("pack %q: manifest id and version are required", path)
	}
	return nil
}
