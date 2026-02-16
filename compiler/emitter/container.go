package emitter

import (
	"fmt"
	"path/filepath"
)

func (e *Emitter) EmitContainer() error {
	if e.IRSchema == nil {
		return nil
	}

	tmplPath := "templates/container.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Entities": e.IRSchema.Entities,
		"Services": e.IRSchema.Services,
	}

	rendered, err := e.RenderTemplate(tmplPath, string(tmplContent), data)
	if err != nil {
		return err
	}

	formatted, err := e.FormatGo([]byte(rendered))
	if err != nil {
		formatted = []byte(rendered)
	}

	path := filepath.Join(e.OutputDir, "internal/app/container.go")
	return WriteFileIfChanged(path, formatted, 0644)
}
