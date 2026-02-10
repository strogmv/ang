package python

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler/planner"
)

type fastAPIData struct {
	ProjectName   string
	Version       string
	Models        []planner.ModelPlan
	Routers       []planner.ServicePlan
	ServiceStubs  []planner.ServicePlan
	RepoStubs     []planner.RepoPlan
	RouterModules []string
}

func EmitFastAPIBackend(outputDir string, rt Runtime, plan planner.FastAPIPlan) error {
	data := fastAPIData{
		ProjectName:   plan.ProjectName,
		Version:       plan.Version,
		Models:        plan.Models,
		Routers:       plan.Routers,
		ServiceStubs:  plan.ServiceStubs,
		RepoStubs:     plan.RepoStubs,
		RouterModules: plan.RouterModules,
	}

	root := filepath.Join(outputDir, "app")
	for _, dir := range []string{
		root,
		filepath.Join(root, "routers"),
		filepath.Join(root, "services"),
		filepath.Join(root, "repositories"),
		filepath.Join(root, "repositories", "postgres"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	baseFiles := []struct {
		template string
		out      string
	}{
		{"templates/python/fastapi/main.py.tmpl", "main.py"},
		{"templates/python/fastapi/models.py.tmpl", "models.py"},
		{"templates/python/fastapi/pyproject.toml.tmpl", "pyproject.toml"},
		{"templates/python/fastapi/README.md.tmpl", "README.md"},
		{"templates/python/fastapi/routers_init.py.tmpl", filepath.Join("routers", "__init__.py")},
		{"templates/python/fastapi/services_init.py.tmpl", filepath.Join("services", "__init__.py")},
		{"templates/python/fastapi/repositories_init.py.tmpl", filepath.Join("repositories", "__init__.py")},
		{"templates/python/fastapi/repositories_postgres_init.py.tmpl", filepath.Join("repositories", "postgres", "__init__.py")},
	}
	for _, f := range baseFiles {
		if err := rt.RenderTemplate(root, f.template, data, f.out, 0644); err != nil {
			return err
		}
	}

	for _, r := range data.Routers {
		if err := rt.RenderTemplate(root, "templates/python/fastapi/router.py.tmpl", r, filepath.Join("routers", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}
	for _, s := range data.ServiceStubs {
		if err := rt.RenderTemplate(root, "templates/python/fastapi/service.py.tmpl", s, filepath.Join("services", s.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}
	for _, r := range data.RepoStubs {
		if err := rt.RenderTemplate(root, "templates/python/fastapi/repository_port.py.tmpl", r, filepath.Join("repositories", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
		if err := rt.RenderTemplate(root, "templates/python/fastapi/repository_postgres.py.tmpl", r, filepath.Join("repositories", "postgres", r.ModuleName+".py"), 0644); err != nil {
			return err
		}
	}

	return nil
}
