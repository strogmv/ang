package emitter

import (
	"go/format"
	"testing"
)

func TestHelpersTemplateFormatsAsGo(t *testing.T) {
	t.Parallel()

	content, err := ReadTemplateByPath("templates/helpers.tmpl")
	if err != nil {
		t.Fatalf("ReadTemplateByPath: %v", err)
	}
	if _, err := format.Source(content); err != nil {
		t.Fatalf("helpers.tmpl must format as valid Go: %v", err)
	}
}
