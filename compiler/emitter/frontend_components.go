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

// FormFieldData describes a field for form generation.
type FormFieldData struct {
	Name        string
	Label       string
	UIType      string
	Placeholder string
	HelperText  string
	Rows        int
	Min         *float64
	Max         *float64
	Currency    string
	Source      string
	Options     []string
	Multiple    bool
	Accept      string
	Disabled    bool
	Required    bool
}

// FormData describes a form component.
type FormData struct {
	Name             string
	InputType        string
	ResponseType     string
	Fields           []FormFieldData
	SubmitLabel      string
	SubmitLoadingLabel string
	CancelLabel      string
	IsUpdate         bool
	Imports          []ImportData
	CustomImports    []ImportData
	HasTextField     bool
	HasSelect        bool
	HasCheckbox      bool
	HasSwitch        bool
	HasCurrency      bool
	HasDatePicker    bool
	HasDateTimePicker bool
	HasTimePicker    bool
	HasAutocomplete  bool
	HasFile          bool
}

// ImportData describes an import statement.
type ImportData struct {
	Component string
	Path      string
}

// TableColumnData describes a table column.
type TableColumnData struct {
	Field      string
	Header     string
	Width      int
	Sortable   bool
	Render     string
	RenderCode string
}

// TableData describes a table component.
type TableData struct {
	Name          string
	QueryName     string
	Columns       []TableColumnData
	HasActions    bool
	PageSize      int
	QueryParams   []QueryParamData
	ExtraProps    []PropData
	CustomImports []ImportData
}

// QueryParamData describes a query parameter.
type QueryParamData struct {
	Name  string
	Value string
}

// PropData describes a component prop.
type PropData struct {
	Name     string
	Type     string
	Optional bool
}

func (e *Emitter) EmitFrontendComponents(services []ir.Service, endpoints []ir.Endpoint, entities []ir.Entity) error {
	servicesNorm := IRServicesToNormalizer(services)
	entitiesNorm := IREntitiesToNormalizer(entities)
	_ = endpoints

	targetDir := filepath.Join(e.FrontendDir, "components")
	formsDir := filepath.Join(targetDir, "forms")
	tablesDir := filepath.Join(targetDir, "tables")

	if err := os.MkdirAll(formsDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"ToLower":    strings.ToLower,
		"JSONName":   JSONName,
		"PascalCase": toPascalCase,
		"default": func(def, val interface{}) interface{} {
			if val == nil || val == "" || val == 0 {
				return def
			}
			return val
		},
	}

	// Generate forms for Create/Update operations
	for _, svc := range servicesNorm {
		for _, m := range svc.Methods {
			if !isFormOperation(m.Name) {
				continue
			}

			formData := buildFormData(m)
			if len(formData.Fields) == 0 {
				continue
			}

			outPath := filepath.Join("components", "forms", formData.Name+"Form.tsx")
			if err := e.emitMUIForm(formData, funcMap, outPath); err != nil {
				return err
			}
		}
	}

	// Generate tables for List operations
	for _, svc := range servicesNorm {
		for _, m := range svc.Methods {
			if !isListOperation(m.Name) {
				continue
			}

			tableData := buildTableData(m, entitiesNorm)
			if len(tableData.Columns) == 0 {
				continue
			}

			outPath := filepath.Join("components", "tables", tableData.Name+"Table.tsx")
			if err := e.emitMUITable(tableData, funcMap, outPath); err != nil {
				return err
			}
		}
	}

	// Generate index files
	if err := e.emitComponentIndex(formsDir, "forms"); err != nil {
		return err
	}
	if err := e.emitComponentIndex(tablesDir, "tables"); err != nil {
		return err
	}

	return e.EmitCRUDPages(entitiesNorm)
}

// EmitCRUDPages generates full pages (List, Create, Edit) for CRUD-enabled entities.
func (e *Emitter) EmitCRUDPages(entities []normalizer.Entity) error {
	pagesDir := filepath.Join(e.FrontendDir, "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"ToLower":    strings.ToLower,
		"PascalCase": toPascalCase,
	}

	for _, ent := range entities {
		if ent.UI == nil {
			continue
		}
		if ent.UI.CRUD == nil || !ent.UI.CRUD.Enabled {
			continue
		}

		// Skip if custom is true
		if ent.UI.CRUD.Custom {
			fmt.Printf("Skipping CRUD page generation for %s (custom=true)\n", ent.Name)
			continue
		}

		entDir := filepath.Join(pagesDir, ent.Name)
		if err := os.MkdirAll(entDir, 0755); err != nil {
			return err
		}

		// Generate ListPage
		if ent.UI.CRUD.Views["list"] {
			if err := e.emitFrontendTemplate("templates/frontend/providers/mui/list_page.tsx.tmpl", ent, funcMap, filepath.Join("pages", ent.Name, "ListPage.tsx")); err != nil {
				return err
			}
		}

		// Generate CreatePage
		if ent.UI.CRUD.Views["create"] {
			if err := e.emitFrontendTemplate("templates/frontend/providers/mui/create_page.tsx.tmpl", ent, funcMap, filepath.Join("pages", ent.Name, "CreatePage.tsx")); err != nil {
				return err
			}
		}

		// Generate EditPage
		if ent.UI.CRUD.Views["edit"] {
			if err := e.emitFrontendTemplate("templates/frontend/providers/mui/edit_page.tsx.tmpl", ent, funcMap, filepath.Join("pages", ent.Name, "EditPage.tsx")); err != nil {
				return err
			}
		}
	}

	return nil
}

func isFormOperation(name string) bool {
	cleanName := strings.TrimPrefix(name, "Admin")
	prefixes := []string{"Create", "Update", "Edit", "Add", "Register", "Submit", "Place", "Post", "Set", "Upload"}
	for _, p := range prefixes {
		if strings.HasPrefix(cleanName, p) {
			return true
		}
	}
	return false
}

func isListOperation(name string) bool {
	cleanName := strings.TrimPrefix(name, "Admin")
	return strings.HasPrefix(cleanName, "List") || strings.HasPrefix(cleanName, "Get") && strings.Contains(cleanName, "s")
}

func buildFormData(m normalizer.Method) FormData {
	name := strings.TrimPrefix(m.Name, "Admin")
	data := FormData{
		Name:               name,
		InputType:          m.Input.Name,
		ResponseType:       m.Output.Name,
		SubmitLabel:        "Сохранить",
		SubmitLoadingLabel: "Сохранение...",
		CancelLabel:        "Отмена",
		IsUpdate:           strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Edit"),
	}

	// Collect fields sorted by UI order
	type fieldWithOrder struct {
		field normalizer.Field
		order int
	}
	var fieldsWithOrder []fieldWithOrder

	for _, f := range m.Input.Fields {
		// Skip internal fields
		if isInternalField(f.Name) {
			continue
		}

		order := 999
		if f.UI != nil && f.UI.Order > 0 {
			order = f.UI.Order
		}
		fieldsWithOrder = append(fieldsWithOrder, fieldWithOrder{f, order})
	}

	sort.Slice(fieldsWithOrder, func(i, j int) bool {
		return fieldsWithOrder[i].order < fieldsWithOrder[j].order
	})

	for _, fw := range fieldsWithOrder {
		f := fw.field
		if f.UI != nil && f.UI.Hidden {
			continue
		}

		fd := FormFieldData{
			Name:     JSONName(f.Name),
			Label:    inferLabel(f),
			UIType:   inferUIType(f),
			Required: !f.IsOptional,
		}

		if f.UI != nil {
			if f.UI.Label != "" {
				fd.Label = f.UI.Label
			}
			if f.UI.Type != "" {
				fd.UIType = f.UI.Type
			}
			fd.Placeholder = f.UI.Placeholder
			fd.HelperText = f.UI.HelperText
			fd.Rows = f.UI.Rows
			fd.Min = f.UI.Min
			fd.Max = f.UI.Max
			fd.Currency = f.UI.Currency
			fd.Source = f.UI.Source
			fd.Options = f.UI.Options
			fd.Multiple = f.UI.Multiple
			fd.Accept = f.UI.Accept
			fd.Disabled = f.UI.Disabled
		}

		// Track which components are needed
		switch fd.UIType {
		case "text", "textarea", "number", "email", "password", "phone", "url":
			data.HasTextField = true
		case "currency":
			data.HasTextField = true
			data.HasCurrency = true
		case "select":
			data.HasSelect = true
		case "autocomplete":
			data.HasAutocomplete = true
			if fd.Source != "" {
				data.CustomImports = append(data.CustomImports, ImportData{
					Component: toPascalCase(fd.Source) + "Autocomplete",
					Path:      "../autocomplete/" + toPascalCase(fd.Source) + "Autocomplete",
				})
			}
		case "checkbox":
			data.HasCheckbox = true
		case "switch":
			data.HasSwitch = true
		case "date":
			data.HasDatePicker = true
		case "datetime":
			data.HasDateTimePicker = true
		case "time":
			data.HasTimePicker = true
		case "file", "image":
			data.HasFile = true
			data.CustomImports = append(data.CustomImports, ImportData{
				Component: "FileUpload",
				Path:      "../upload/FileUpload",
			})
		}

		data.Fields = append(data.Fields, fd)
	}

	return data
}

func buildTableData(m normalizer.Method, entities []normalizer.Entity) TableData {
	name := strings.TrimPrefix(m.Name, "Admin")
	name = strings.TrimPrefix(name, "List")
	name = singularize(name)

	data := TableData{
		Name:       name,
		QueryName:  m.Name,
		HasActions: true,
		PageSize:   25,
	}

	for _, f := range m.Input.Fields {
		if f.UI != nil && f.UI.Hidden {
			continue
		}

		lower := strings.ToLower(f.Name)
		switch lower {
		case "limit":
			data.QueryParams = append(data.QueryParams, QueryParamData{
				Name:  JSONName(f.Name),
				Value: "paginationModel.pageSize",
			})
			continue
		case "offset":
			data.QueryParams = append(data.QueryParams, QueryParamData{
				Name:  JSONName(f.Name),
				Value: "paginationModel.page * paginationModel.pageSize",
			})
			continue
		}

		jsonName := JSONName(f.Name)
		data.QueryParams = append(data.QueryParams, QueryParamData{
			Name:  jsonName,
			Value: jsonName,
		})
		data.ExtraProps = append(data.ExtraProps, PropData{
			Name:     jsonName,
			Type:     tsTypeForField(f),
			Optional: true,
		})
	}

	// Find the Data field in output
	for _, f := range m.Output.Fields {
		if strings.ToLower(f.Name) == "data" {
			itemFields := f.ItemFields
			
			// If fields are empty but we have a type name, look it up in global entities
			if len(itemFields) == 0 && f.ItemTypeName != "" {
				cleanTypeName := strings.TrimPrefix(f.ItemTypeName, "domain.")
				for _, ent := range entities {
					if strings.EqualFold(ent.Name, cleanTypeName) {
						itemFields = ent.Fields
						break
					}
				}
			}

			if len(itemFields) > 0 {
				for _, itemField := range itemFields {
					if isInternalField(itemField.Name) && itemField.Name != "id" {
						continue
					}

					col := TableColumnData{
						Field:    JSONName(itemField.Name),
						Header:   inferLabel(itemField),
						Sortable: true,
					}

					// Determine render type
					col.Render, col.RenderCode = inferColumnRender(itemField)
					data.Columns = append(data.Columns, col)
				}
			}
			break
		}
	}

	return data
}
func isInternalField(name string) bool {
	internal := []string{
		"companyid", "userid", "ownerid", "createdby", "updatedby",
		"createdat", "updatedat", "deletedat", // timestamps
		"leadercompanyid", "lastbidat", // tender internal
		"autobetwaitseconds", "extendwindowseconds", "extendonbidseconds", // tender config
		"companyrating", "companylogolink", // denormalized fields
		"metadata", "passwordhash", // sensitive
	}
	lower := strings.ToLower(name)
	for _, i := range internal {
		if lower == i {
			return true
		}
	}
	return false
}

func inferLabel(f normalizer.Field) string {
	if f.UI != nil && f.UI.Label != "" {
		return f.UI.Label
	}

	return HumanizeName(f.Name)
}

func tsTypeForField(f normalizer.Field) string {
	switch f.Type {
	case "int", "int64", "float64", "float":
		return "number"
	case "bool":
		return "boolean"
	case "string":
		return "string"
	case "[]string":
		return "string[]"
	case "[]any", "[]interface{}":
		return "any[]"
	case "map[string]any":
		return "Record<string, any>"
	case "time.Time":
		return "string"
	default:
		return "any"
	}
}

func inferUIType(f normalizer.Field) string {
	if f.UI != nil && f.UI.Type != "" {
		return f.UI.Type
	}

	lower := strings.ToLower(f.Name)

	// By name
	switch {
	case lower == "email":
		return "email"
	case lower == "password":
		return "password"
	case lower == "phone" || strings.Contains(lower, "phone"):
		return "phone"
	case strings.Contains(lower, "url") || strings.Contains(lower, "link"):
		return "url"
	case strings.Contains(lower, "description") || strings.Contains(lower, "content") || strings.Contains(lower, "body") || strings.Contains(lower, "text"):
		return "textarea"
	case strings.HasSuffix(lower, "at") && (strings.Contains(lower, "date") || strings.Contains(lower, "time") || lower == "createdat" || lower == "updatedat" || lower == "endsat" || lower == "startsat"):
		return "datetime"
	case strings.Contains(lower, "date"):
		return "date"
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount") || strings.Contains(lower, "cost") || strings.Contains(lower, "budget") || strings.Contains(lower, "sum"):
		return "currency"
	case strings.HasSuffix(lower, "id") && lower != "id":
		return "autocomplete"
	}

	// By type
	switch f.Type {
	case "bool":
		return "checkbox"
	case "int", "int64", "float64":
		return "number"
	}

	return "text"
}

func inferColumnRender(f normalizer.Field) (string, string) {
	lower := strings.ToLower(f.Name)

	switch {
	case strings.HasSuffix(lower, "at"):
		return "date", "dayjs(params.value).format('DD.MM.YYYY HH:mm')"
	case lower == "status":
		return "status", "<Chip label={params.value} size=\"small\" />"
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount") || strings.Contains(lower, "budget"):
		return "currency", "new Intl.NumberFormat('ru-BY', { style: 'currency', currency: 'BYN' }).format(params.value)"
	case strings.Contains(lower, "avatar") || strings.Contains(lower, "image") || strings.Contains(lower, "logo"):
		return "avatar", "<Avatar src={params.value} />"
	}

	return "", ""
}

func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	// Simple implementation - capitalize first letter
	return strings.ToUpper(s[:1]) + s[1:]
}

func singularize(name string) string {
	if strings.HasSuffix(name, "ies") && len(name) > 3 {
		return name[:len(name)-3] + "y"
	}
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		return name[:len(name)-1]
	}
	return name
}

func (e *Emitter) emitMUIForm(data FormData, funcMap template.FuncMap, outPath string) error {
	tmplPath := "templates/frontend/providers/mui/form.tmpl"
	fieldsTmplPath := "templates/frontend/providers/mui/fields.tmpl"

	// Read templates
	formTmpl, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read form template: %w", err)
	}
	fieldsTmpl, err := ReadTemplateByPath(fieldsTmplPath)
	if err != nil {
		return fmt.Errorf("read fields template: %w", err)
	}

	// Combine templates
	combined := string(formTmpl) + "\n" + string(fieldsTmpl)

	t, err := template.New("form").Funcs(funcMap).Parse(combined)
	if err != nil {
		return fmt.Errorf("parse form template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute form template: %w", err)
	}

	path := filepath.Join(e.FrontendDir, outPath)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated Component: %s\n", path)
	return nil
}

func (e *Emitter) emitMUITable(data TableData, funcMap template.FuncMap, outPath string) error {
	tmplPath := "templates/frontend/providers/mui/table.tmpl"

	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read table template: %w", err)
	}

	t, err := template.New("table").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse table template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute table template: %w", err)
	}

	path := filepath.Join(e.FrontendDir, outPath)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated Component: %s\n", path)
	return nil
}

func (e *Emitter) emitComponentIndex(dir, componentType string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Directory might not exist yet
	}

	var exports []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tsx") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".tsx")
		exports = append(exports, fmt.Sprintf("export { %s } from './%s';", name, name))
	}

	if len(exports) == 0 {
		return nil
	}

	content := "// Generated by ANG. Do not edit.\n" + strings.Join(exports, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, "index.ts"), []byte(content), 0644)
}
