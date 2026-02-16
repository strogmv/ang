package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type AdminPageData struct {
	Key                 string // URL key (e.g. "tender")
	ComponentName       string
	ResourceName        string
	Title               string
	RoutePath           string
	TableComponent      string
	ServiceName         string
	ListRPC             string
	HasCreate           bool
	HasUpdate           bool
	HasDelete           bool
	CreateFormComponent string
	UpdateFormComponent string
	UpdateIDField       string // Field name in form that maps from row.id (e.g. "tenderid")
	DeleteRPC           string
	DeleteIDField       string
}

type AdminRoutesData struct {
	Pages         []AdminPageData
	DefaultPath   string
	DefaultEntity string
}

type AdminConfigData struct {
	Entities []AdminPageData
}

type AdminNavItem struct {
	ID    string
	Title string
	URL   string
}

type AdminNavData struct {
	Items []AdminNavItem
}

// EmitFrontendAdmin generates universal admin page and config.
func (e *Emitter) EmitFrontendAdmin(entities []ir.Entity, services []ir.Service) error {
	entitiesNorm := IREntitiesToNormalizer(entities)
	servicesNorm := IRServicesToNormalizer(services)

	adminDir := strings.TrimSpace(e.FrontendAdminDir)
	if adminDir == "" {
		return nil
	}

	pages := collectAdminPages(entitiesNorm, servicesNorm)
	if len(pages) == 0 {
		fmt.Printf("Warning: No admin pages collected.\n")
		return nil
	}

	if err := os.MkdirAll(adminDir, 0755); err != nil {
		return err
	}

	// Generate adminConfig.ts
	if err := e.emitAdminConfig(AdminConfigData{Entities: pages}); err != nil {
		return err
	}

	// Generate AdminPage.tsx (universal component)
	if err := e.emitUniversalAdminPage(); err != nil {
		return err
	}

	// Generate routes.tsx
	defaultEntity := ""
	if len(pages) > 0 {
		defaultEntity = pages[0].Key
	}
	if err := e.emitAdminRoutes(AdminRoutesData{
		Pages:         pages,
		DefaultEntity: defaultEntity,
	}); err != nil {
		return err
	}

	// Generate navigation.ts
	navItems := make([]AdminNavItem, 0, len(pages))
	for _, page := range pages {
		navItems = append(navItems, AdminNavItem{
			ID:    "admin-" + page.Key,
			Title: page.Title,
			URL:   "admin/" + page.Key,
		})
	}
	if err := e.emitAdminNavigation(AdminNavData{Items: navItems}); err != nil {
		return err
	}

	return nil
}

func collectAdminPages(entities []normalizer.Entity, services []normalizer.Service) []AdminPageData {
	var pages []AdminPageData

	type entityOps struct {
		list   *normalizer.Method
		create *normalizer.Method
		update *normalizer.Method
		delete *normalizer.Method
		svc    string
	}
	opsByEntity := make(map[string]*entityOps)

	// We only care about domain entities
	for _, ent := range entities {
		name := ent.Name
		lower := strings.ToLower(name)
		// Skip DTOs and internal types
		if strings.HasSuffix(lower, "request") || strings.HasSuffix(lower, "response") || strings.HasSuffix(lower, "data") || strings.HasSuffix(lower, "event") {
			continue
		}
		opsByEntity[name] = &entityOps{}
	}

	for _, svc := range services {
		for i := range svc.Methods {
			m := &svc.Methods[i]

			// 1. Identify primary List (e.g. ListTenders)
			if strings.HasPrefix(m.Name, "List") {
				res := strings.TrimPrefix(m.Name, "List")
				for entName := range opsByEntity {
					if res == entName || res == entName+"s" || res == entName+"es" {
						if opsByEntity[entName].list == nil {
							opsByEntity[entName].list = m
							opsByEntity[entName].svc = svc.Name
						}
					}
				}
			}

			// 2. Identify Create
			if hasPrefix(m.Name, []string{"Create", "Add", "Register"}) {
				res := stripPrefix(m.Name, []string{"Create", "Add", "Register"})
				if op, ok := opsByEntity[res]; ok && op.create == nil {
					op.create = m
				}
			}

			// 3. Identify Update
			if hasPrefix(m.Name, []string{"Update", "Edit", "Set"}) {
				res := stripPrefix(m.Name, []string{"Update", "Edit", "Set"})
				if op, ok := opsByEntity[res]; ok && op.update == nil {
					op.update = m
				}
			}

			// 4. Identify Delete
			if hasPrefix(m.Name, []string{"Delete", "Remove"}) {
				res := stripPrefix(m.Name, []string{"Delete", "Remove"})
				if op, ok := opsByEntity[res]; ok && op.delete == nil {
					op.delete = m
				}
			}
		}
	}

	var keys []string
	for k := range opsByEntity {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, entName := range keys {
		ops := opsByEntity[entName]
		if ops.list == nil {
			continue
		}

		routePath := toKebabCase(entName)
		page := AdminPageData{
			Key:            routePath,
			ComponentName:  entName + "AdminPage",
			ResourceName:   entName,
			Title:          HumanizeName(entName),
			RoutePath:      routePath,
			TableComponent: entName + "Table",
			ServiceName:    ops.svc,
			ListRPC:        ops.list.Name,
		}

		if ops.create != nil {
			page.HasCreate = true
			page.CreateFormComponent = ops.create.Name + "Form"
		}
		if ops.update != nil {
			page.HasUpdate = true
			page.UpdateFormComponent = ops.update.Name + "Form"
			// Find the ID field in the update form's input
			idField := findIDField(ops.update.Input.Fields)
			if idField != "" && strings.ToLower(idField) != "id" {
				page.UpdateIDField = strings.ToLower(idField)
			}
		}
		if ops.delete != nil {
			idField := findIDField(ops.delete.Input.Fields)
			if idField != "" && !isDeletionBlockedResource(entName) {
				page.HasDelete = true
				page.DeleteRPC = ops.delete.Name
				page.DeleteIDField = idField
			}
		}

		pages = append(pages, page)
	}

	return pages
}

func (e *Emitter) emitAdminConfig(data AdminConfigData) error {
	tmplPath := "templates/frontend/admin/admin_config.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read admin config template: %w", err)
	}

	t, err := template.New("admin_config").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse admin config template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute admin config template: %w", err)
	}

	path := filepath.Join(e.FrontendAdminDir, "adminConfig.ts")
	fmt.Printf("Generated Admin Config: %s\n", path)
	return WriteFileIfChanged(path, buf.Bytes(), 0644)
}

func (e *Emitter) emitUniversalAdminPage() error {
	tmplPath := "templates/frontend/admin/AdminPage.tsx.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read AdminPage template: %w", err)
	}

	path := filepath.Join(e.FrontendAdminDir, "AdminPage.tsx")
	fmt.Printf("Generated Admin Page: %s\n", path)
	return WriteFileIfChanged(path, tmplContent, 0644)
}

func (e *Emitter) emitAdminRoutes(data AdminRoutesData) error {
	tmplPath := "templates/frontend/admin/admin_routes.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	t, err := template.New("admin_routes").Parse(string(tmplContent))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	path := filepath.Join(e.FrontendAdminDir, "routes.tsx")
	fmt.Printf("Generated Admin Routes: %s\n", path)
	return WriteFileIfChanged(path, buf.Bytes(), 0644)
}

func (e *Emitter) emitAdminNavigation(data AdminNavData) error {
	tmplPath := "templates/frontend/admin/admin_navigation.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	t, err := template.New("admin_navigation").Parse(string(tmplContent))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	path := filepath.Join(e.FrontendAdminDir, "navigation.ts")
	fmt.Printf("Generated Admin Navigation: %s\n", path)
	return WriteFileIfChanged(path, buf.Bytes(), 0644)
}

func toKebabCase(name string) string {
	if name == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteRune('-')
			}
			result.WriteRune(r + 32)
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func findIDField(fields []normalizer.Field) string {
	for _, f := range fields {
		lower := strings.ToLower(f.Name)
		if lower == "id" || strings.HasSuffix(lower, "id") {
			return f.Name
		}
	}
	return ""
}

func isDeletionBlockedResource(name string) bool {
	switch strings.ToLower(name) {
	case "company", "user":
		return true
	default:
		return false
	}
}

func hasPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func stripPrefix(name string, prefixes []string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return strings.TrimPrefix(name, p)
		}
	}
	return ""
}
