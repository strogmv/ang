package python

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

type rbacData struct {
	Roles map[string][]string
}

type authData struct {
	StoreMode string
}

func EmitRBAC(outputDir string, rt Runtime, rbac *normalizer.RBACDef) error {
	root := filepath.Join(outputDir, "app")
	if err := rt.RenderTemplate(root, "templates/python/fastapi/security_init.py.tmpl", map[string]any{}, filepath.Join("security", "__init__.py"), 0644); err != nil {
		return err
	}
	roles := map[string][]string{}
	if rbac != nil {
		for role, perms := range rbac.Roles {
			pp := append([]string(nil), perms...)
			sort.Strings(pp)
			roles[role] = pp
		}
	}
	return rt.RenderTemplate(root, "templates/python/fastapi/rbac.py.tmpl", rbacData{Roles: roles}, filepath.Join("security", "rbac.py"), 0644)
}

func EmitAuthStores(outputDir string, rt Runtime, auth *normalizer.AuthDef) error {
	root := filepath.Join(outputDir, "app")
	if err := rt.RenderTemplate(root, "templates/python/fastapi/security_init.py.tmpl", map[string]any{}, filepath.Join("security", "__init__.py"), 0644); err != nil {
		return err
	}
	mode := "memory"
	if auth != nil {
		switch strings.ToLower(strings.TrimSpace(auth.RefreshStore)) {
		case "postgres", "hybrid":
			mode = "postgres"
		case "memory":
			mode = "memory"
		}
	}
	data := authData{StoreMode: mode}
	files := []string{
		"templates/python/fastapi/auth.py.tmpl",
		"templates/python/fastapi/refresh_store.py.tmpl",
		"templates/python/fastapi/refresh_store_memory.py.tmpl",
		"templates/python/fastapi/refresh_store_postgres.py.tmpl",
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".tmpl")
		if err := rt.RenderTemplate(root, f, data, filepath.Join("security", name), 0644); err != nil {
			return err
		}
	}
	return nil
}
