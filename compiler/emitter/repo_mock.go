package emitter

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) EmitRepoMocks(repos []ir.Repository) error {
	tmplPath := "templates/mock_repo.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	reposNorm := IRReposToNormalizer(repos)

	funcMap := e.getSharedFuncMap()
	funcMap["ZeroValue"] = func(t string) string {
		if strings.HasPrefix(t, "*") || strings.HasPrefix(t, "[]") || strings.HasPrefix(t, "map") {
			return "nil"
		}
		switch t {
		case "int", "int64", "float64":
			return "0"
		case "string":
			return "\"\""
		case "bool":
			return "false"
		default:
			return "nil"
		}
	}

	t, err := template.New("repo_mock").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "port")

	for _, repo := range reposNorm {
		var buf bytes.Buffer
		data := struct {
			Entity   string
			Finders  []normalizer.RepositoryFinder
			GoModule string
		}{
			Entity:   repo.Entity,
			Finders:  repo.Finders,
			GoModule: e.GoModule,
		}

		if err := t.Execute(&buf, data); err != nil {
			return err
		}

		formatted, err := formatGoStrict(buf.Bytes(), "internal/port/mock_"+strings.ToLower(repo.Entity)+"_test.go")
		if err != nil {
			return err
		}

		filename := "mock_" + strings.ToLower(repo.Entity) + "_test.go"
		path := filepath.Join(targetDir, filename)
		if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
			return err
		}
		fmt.Printf("Generated Repo Mock: %s\n", path)
	}

	return nil
}
