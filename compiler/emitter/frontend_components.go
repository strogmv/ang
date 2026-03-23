package emitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
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
	Component   string
	Section     string
	UI          FormUIHintsData
}

// FormUIHintsData is a serializable UI hints view passed into proxy components.
type FormUIHintsData struct {
	Type        string   `json:"type,omitempty"`
	Importance  string   `json:"importance,omitempty"`
	InputKind   string   `json:"inputKind,omitempty"`
	Intent      string   `json:"intent,omitempty"`
	Density     string   `json:"density,omitempty"`
	LabelMode   string   `json:"labelMode,omitempty"`
	Surface     string   `json:"surface,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	HelperText  string   `json:"helperText,omitempty"`
	Rows        int      `json:"rows,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	Source      string   `json:"source,omitempty"`
	Multiple    bool     `json:"multiple,omitempty"`
	Accept      string   `json:"accept,omitempty"`
	Disabled    bool     `json:"disabled,omitempty"`
	Required    bool     `json:"required,omitempty"`
	FullWidth   bool     `json:"fullWidth,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
	Columns     int      `json:"columns,omitempty"`
	Component   string   `json:"component,omitempty"`
	Section     string   `json:"section,omitempty"`
}

// FormData describes a form component.
type FormData struct {
	Name               string
	SchemaVar          string
	InputType          string
	ResponseType       string
	IsAdmin            bool
	Fields             []FormFieldData
	SubmitLabel        string
	SubmitLoadingLabel string
	CancelLabel        string
	IsUpdate           bool
	Imports            []ImportData
	CustomImports      []ImportData
	HasTextField       bool
	HasSelect          bool
	HasCheckbox        bool
	HasSwitch          bool
	HasCurrency        bool
	HasDatePicker      bool
	HasDateTimePicker  bool
	HasTimePicker      bool
	HasAutocomplete    bool
	HasFile            bool
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
	endpointsNorm := IREndpointsToNormalizer(endpoints)

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
		"JSArray": func(items []string) string {
			if len(items) == 0 {
				return "[]"
			}
			out := make([]string, 0, len(items))
			for _, it := range items {
				out = append(out, strconv.Quote(it))
			}
			return "[" + strings.Join(out, ", ") + "]"
		},
		"UIHintsJSON": func(ui FormUIHintsData) string {
			b, err := json.Marshal(ui)
			if err != nil || len(b) == 0 {
				return "{}"
			}
			return string(b)
		},
		"default": func(def, val interface{}) interface{} {
			if val == nil || val == "" || val == 0 {
				return def
			}
			return val
		},
	}

	if err := e.emitBaseUIFormsProxyLayer(); err != nil {
		return err
	}
	if err := e.emitBaseUIAutoFormLayer(); err != nil {
		return err
	}
	if err := e.emitFrontendTSConfig(); err != nil {
		return err
	}

	// Generate forms for Create/Update operations
	formDefs := make([]FormData, 0)
	for _, svc := range servicesNorm {
		for _, m := range svc.Methods {
			if !isFormOperation(m.Name) {
				continue
			}

			formData, err := buildFormData(m)
			if err != nil {
				return err
			}
			if len(formData.Fields) == 0 {
				continue
			}

			if err := e.emitMUIFormSchema(formData, funcMap, filepath.Join("components", "forms", formData.Name+"Form.schema.ts")); err != nil {
				return err
			}
			formDefs = append(formDefs, formData)
		}
	}
	if err := e.emitGeneratedFormsRuntime(formDefs); err != nil {
		return err
	}

	// Generate tables for List operations
	for _, svc := range servicesNorm {
		for _, m := range svc.Methods {
			if !isListOperation(m.Name) {
				continue
			}

			tableData := buildTableData(svc.Name, m, entitiesNorm, endpointsNorm)
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

func buildFormData(m normalizer.Method) (FormData, error) {
	isAdmin := strings.HasPrefix(m.Name, "Admin")
	name := strings.TrimPrefix(m.Name, "Admin")
	data := FormData{
		Name:               name,
		SchemaVar:          lowerFirst(name) + "Schema",
		InputType:          m.Input.Name,
		ResponseType:       m.Output.Name,
		IsAdmin:            isAdmin,
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
		importance := ""
		inputKind := ""
		intent := ""
		density := ""
		labelMode := ""
		surface := ""
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
			importance = f.UI.Importance
			inputKind = f.UI.InputKind
			intent = f.UI.Intent
			density = f.UI.Density
			labelMode = f.UI.LabelMode
			surface = f.UI.Surface
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
			fd.Component = f.UI.Component
			fd.Section = f.UI.Section
		}
		if !isSupportedUIType(fd.UIType) {
			return FormData{}, fmt.Errorf("unknown ui type %q for field %q in method %s", fd.UIType, f.Name, m.Name)
		}
		if fd.UIType == "custom" && strings.TrimSpace(fd.Component) == "" {
			return FormData{}, fmt.Errorf("ui type custom requires component for field %q in method %s", f.Name, m.Name)
		}
		if strings.TrimSpace(fd.Component) != "" && !isValidTSIdentifier(fd.Component) {
			return FormData{}, fmt.Errorf("invalid custom component %q for field %q in method %s: must be a valid TS identifier", fd.Component, f.Name, m.Name)
		}
		if strings.EqualFold(fd.UIType, "custom") && strings.TrimSpace(fd.Source) != "" && !isLikelyImportPath(fd.Source) {
			return FormData{}, fmt.Errorf("invalid ui source %q for field %q in method %s: expected import-like path", fd.Source, f.Name, m.Name)
		}
		if strings.EqualFold(fd.UIType, "select") && len(fd.Options) == 0 && strings.TrimSpace(fd.Source) == "" {
			return FormData{}, fmt.Errorf("ui type select requires options or source for field %q in method %s", f.Name, m.Name)
		}
		if f.UI != nil && f.UI.Hidden && fd.Required {
			return FormData{}, fmt.Errorf("field %q in method %s cannot be both required and hidden", f.Name, m.Name)
		}

		fd.UI = FormUIHintsData{
			Type:        fd.UIType,
			Importance:  importance,
			InputKind:   inputKind,
			Intent:      intent,
			Density:     density,
			LabelMode:   labelMode,
			Surface:     surface,
			Placeholder: fd.Placeholder,
			HelperText:  fd.HelperText,
			Rows:        fd.Rows,
			Min:         fd.Min,
			Max:         fd.Max,
			Currency:    fd.Currency,
			Source:      fd.Source,
			Multiple:    fd.Multiple,
			Accept:      fd.Accept,
			Disabled:    fd.Disabled,
			Required:    fd.Required,
			FullWidth:   true,
			Hidden:      false,
			Columns:     0,
			Component:   fd.Component,
			Section:     fd.Section,
		}
		if f.UI != nil {
			fd.UI.FullWidth = f.UI.FullWidth
			fd.UI.Hidden = f.UI.Hidden
			fd.UI.Columns = f.UI.Columns
		}
		if fd.UI.Hidden {
			continue
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
		case "custom":
			if fd.Component != "" {
				componentPath := fd.Source
				if componentPath == "" {
					componentPath = DefaultUIProviderPath + "/custom"
				}
				data.CustomImports = append(data.CustomImports, ImportData{
					Component: fd.Component,
					Path:      componentPath,
				})
			}
		}

		data.Fields = append(data.Fields, fd)
	}

	return data, nil
}

func buildTableData(serviceName string, m normalizer.Method, entities []normalizer.Entity, endpoints []normalizer.Endpoint) TableData {
	name := strings.TrimPrefix(m.Name, "Admin")
	name = strings.TrimPrefix(name, "List")
	name = singularize(name)

	data := TableData{
		Name:       name,
		QueryName:  m.Name,
		HasActions: true,
		PageSize:   25,
	}

	injected := make(map[string]struct{})
	for _, ep := range endpoints {
		if !strings.EqualFold(ep.ServiceName, serviceName) || !strings.EqualFold(ep.RPC, m.Name) {
			continue
		}
		for _, field := range ep.AuthInject {
			injected[strings.ToLower(field)] = struct{}{}
		}
		break
	}

	for _, f := range m.Input.Fields {
		if f.UI != nil && f.UI.Hidden {
			continue
		}

		lower := strings.ToLower(f.Name)
		if _, skip := injected[lower]; skip {
			continue
		}
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

func isSupportedUIType(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "text", "textarea", "number", "currency", "email", "password", "phone", "url",
		"date", "datetime", "time", "select", "autocomplete", "checkbox", "switch",
		"file", "image", "custom":
		return true
	default:
		return false
	}
}

func isValidTSIdentifier(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	ok, _ := regexp.MatchString(`^[A-Za-z_$][A-Za-z0-9_$]*$`, v)
	return ok
}

func isLikelyImportPath(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	ok, _ := regexp.MatchString(`^(@?[A-Za-z0-9._-]+)(/[A-Za-z0-9._@-]+)*$`, v)
	return ok
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
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated Component: %s\n", path)
	return nil
}

func (e *Emitter) emitMUIFormSchema(data FormData, funcMap template.FuncMap, outPath string) error {
	const tmpl = `// AUTO-GENERATED by ANG. DO NOT EDIT.
import type { FormSchema } from '@/components/ui/auto-form';
import type { {{ .InputType }} } from '../../types';

export const {{ .Name }}FormSchema: FormSchema<{{ .InputType }}> = {
  schemaVersion: 1,
  layout: { type: 'grid', columns: 12 },
  fields: [
{{- range .Fields }}
  {
    name: "{{ .Name }}",
    label: "{{ .Label }}",
    type: "{{ .UIType }}",
    required: {{ .Required }},
    options: {{ JSArray .Options }},
    ui: {{ UIHintsJSON .UI }},
  },
{{- end }}
  ],
};
`
	t, err := template.New("form_schema").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parse form schema template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute form schema template: %w", err)
	}
	path := filepath.Join(e.FrontendDir, outPath)
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated Component Schema: %s\n", path)
	return nil
}

func (e *Emitter) emitGeneratedFormsRuntime(formDefs []FormData) error {
	formsDir := filepath.Join(e.FrontendDir, "components", "forms")
	if err := os.MkdirAll(formsDir, 0755); err != nil {
		return err
	}
	if matches, err := filepath.Glob(filepath.Join(formsDir, "*Form.tsx")); err == nil {
		for _, match := range matches {
			if strings.HasSuffix(match, "GeneratedForms.tsx") {
				continue
			}
			if removeErr := os.Remove(match); removeErr != nil && !os.IsNotExist(removeErr) {
				return removeErr
			}
		}
	}

	if len(formDefs) == 0 {
		indexPath := filepath.Join(formsDir, "index.ts")
		return WriteFileIfChanged(indexPath, []byte("// Generated by ANG. Do not edit.\n"), 0644)
	}

	sort.Slice(formDefs, func(i, j int) bool { return formDefs[i].Name < formDefs[j].Name })

	_ = os.Remove(filepath.Join(formsDir, "GeneratedForms.tsx"))

	var publicDefs []FormData
	var adminDefs []FormData
	for _, fd := range formDefs {
		if fd.IsAdmin {
			adminDefs = append(adminDefs, fd)
		} else {
			publicDefs = append(publicDefs, fd)
		}
	}

	var manifest strings.Builder
	manifest.WriteString("// AUTO-GENERATED by ANG. DO NOT EDIT.\n")
	manifest.WriteString("import type { FormSchema } from '@/components/ui/auto-form';\n\n")
	manifest.WriteString("export type FormSchemaLoader = () => Promise<FormSchema<any>>;\n")
	manifest.WriteString("\n")
	manifest.WriteString("export type GeneratedFormName =\n")
	for i, fd := range formDefs {
		sep := " |"
		if i == 0 {
			sep = "  "
		}
		fmt.Fprintf(&manifest, "%s %q\n", sep, fd.Name)
	}
	manifest.WriteString(";\n\n")
	manifest.WriteString("export type FormManifestEntry = {\n")
	manifest.WriteString("  name: GeneratedFormName;\n")
	manifest.WriteString("  loadSchema: FormSchemaLoader;\n")
	manifest.WriteString("  useFormHook: string;\n")
	manifest.WriteString("  useMutationHook: string;\n")
	manifest.WriteString("  scope: 'public' | 'admin';\n")
	manifest.WriteString("};\n\n")
	writeManifestMap := func(b *strings.Builder, varName string, defs []FormData, scope string) {
		fmt.Fprintf(b, "export const %s: Record<string, FormManifestEntry> = {\n", varName)
		for _, fd := range defs {
			fmt.Fprintf(b, "  %q: {\n", fd.Name)
			fmt.Fprintf(b, "    name: %q,\n", fd.Name)
			fmt.Fprintf(b, "    loadSchema: () => import('./%sForm.schema').then((m) => m.%sFormSchema),\n", fd.Name, fd.Name)
			fmt.Fprintf(b, "    useFormHook: %q,\n", "use"+fd.Name+"Form")
			fmt.Fprintf(b, "    useMutationHook: %q,\n", "use"+fd.Name)
			fmt.Fprintf(b, "    scope: %q,\n", scope)
			b.WriteString("  },\n")
		}
		b.WriteString("};\n\n")
	}
	writeManifestMap(&manifest, "publicFormsManifest", publicDefs, "public")
	writeManifestMap(&manifest, "adminFormsManifest", adminDefs, "admin")
	manifest.WriteString("export const formsManifest: Record<string, FormManifestEntry> = {\n")
	manifest.WriteString("  ...publicFormsManifest,\n")
	manifest.WriteString("  ...adminFormsManifest,\n")
	manifest.WriteString("};\n\n")
	manifest.WriteString("export function resolveFormEntry(name: GeneratedFormName): FormManifestEntry {\n")
	manifest.WriteString("  const entry = formsManifest[name];\n")
	manifest.WriteString("  if (!entry) throw new Error(`ANG forms manifest: unknown form ${name}`);\n")
	manifest.WriteString("  return entry;\n")
	manifest.WriteString("}\n")

	if err := WriteFileIfChanged(filepath.Join(formsDir, "forms.manifest.ts"), []byte(manifest.String()), 0644); err != nil {
		return err
	}

	var renderer strings.Builder
	renderer.WriteString("// AUTO-GENERATED by ANG. DO NOT EDIT.\n")
	renderer.WriteString("import { useEffect, useMemo, useState } from 'react';\n")
	renderer.WriteString("import { AutoForm } from '@/components/ui/auto-form';\n")
	renderer.WriteString("import type { FormSchema } from '@/components/ui/auto-form';\n")
	renderer.WriteString("import * as Hooks from '../../hooks';\n")
	renderer.WriteString("import * as FormHooks from '../../forms';\n")
	renderer.WriteString("import { resolveFormEntry, type GeneratedFormName } from './forms.manifest';\n\n")
	renderer.WriteString("export type FormRendererProps = {\n")
	renderer.WriteString("  formName: GeneratedFormName;\n")
	renderer.WriteString("  onSuccess?: (data: any) => void;\n")
	renderer.WriteString("  onError?: (error: Error) => void;\n")
	renderer.WriteString("  onCancel?: () => void;\n")
	renderer.WriteString("  defaultValues?: Record<string, unknown>;\n")
	renderer.WriteString("  fallback?: JSX.Element | null;\n")
	renderer.WriteString("};\n\n")
	renderer.WriteString("export function FormRenderer({ formName, onSuccess, onError, onCancel, defaultValues, fallback = null }: FormRendererProps) {\n")
	renderer.WriteString("  const entry = useMemo(() => resolveFormEntry(formName), [formName]);\n")
	renderer.WriteString("  const [schema, setSchema] = useState<FormSchema<any> | null>(null);\n")
	renderer.WriteString("  const [loadErr, setLoadErr] = useState<Error | null>(null);\n")
	renderer.WriteString("  useEffect(() => {\n")
	renderer.WriteString("    let active = true;\n")
	renderer.WriteString("    setSchema(null);\n")
	renderer.WriteString("    setLoadErr(null);\n")
	renderer.WriteString("    entry.loadSchema().then((next) => {\n")
	renderer.WriteString("      if (!active) return;\n")
	renderer.WriteString("      setSchema(next);\n")
	renderer.WriteString("    }).catch((e) => {\n")
	renderer.WriteString("      if (!active) return;\n")
	renderer.WriteString("      setLoadErr(e as Error);\n")
	renderer.WriteString("    });\n")
	renderer.WriteString("    return () => { active = false; };\n")
	renderer.WriteString("  }, [entry]);\n")
	renderer.WriteString("  const useForm = (FormHooks as any)[entry.useFormHook];\n")
	renderer.WriteString("  const useMutation = (Hooks as any)[entry.useMutationHook];\n")
	renderer.WriteString("  if (typeof useForm !== 'function' || typeof useMutation !== 'function') {\n")
	renderer.WriteString("    throw new Error(`ANG FormRenderer: hooks are not resolved for form ${formName}`);\n")
	renderer.WriteString("  }\n")
	renderer.WriteString("  if (loadErr) throw loadErr;\n")
	renderer.WriteString("  if (!schema) return fallback;\n")
	renderer.WriteString("  const form = useForm(defaultValues ? { defaultValues } : undefined);\n")
	renderer.WriteString("  const { mutate, isPending } = useMutation({\n")
	renderer.WriteString("    onSuccess: (data: any) => onSuccess?.(data),\n")
	renderer.WriteString("    onError: (error: any) => onError?.(error as Error),\n")
	renderer.WriteString("  });\n")
	renderer.WriteString("  return (\n")
	renderer.WriteString("    <AutoForm\n")
	renderer.WriteString("      form={form}\n")
	renderer.WriteString("      schema={schema}\n")
	renderer.WriteString("      onSubmit={(data) => mutate(data)}\n")
	renderer.WriteString("      isPending={isPending}\n")
	renderer.WriteString("      onCancel={onCancel}\n")
	renderer.WriteString("    />\n")
	renderer.WriteString("  );\n")
	renderer.WriteString("}\n")

	if err := WriteFileIfChanged(filepath.Join(formsDir, "FormRenderer.tsx"), []byte(renderer.String()), 0644); err != nil {
		return err
	}

	var idx strings.Builder
	idx.WriteString("// Generated by ANG. Do not edit.\n")
	idx.WriteString("export { FormRenderer } from './FormRenderer';\n")
	idx.WriteString("export { formsManifest, publicFormsManifest, adminFormsManifest } from './forms.manifest';\n")
	idx.WriteString("export type { GeneratedFormName, FormManifestEntry, FormSchemaLoader } from './forms.manifest';\n")
	idx.WriteString("export type { FormRendererProps } from './FormRenderer';\n")
	for _, fd := range formDefs {
		fmt.Fprintf(&idx, "export { %sFormSchema } from './%sForm.schema';\n", fd.Name, fd.Name)
	}
	if err := WriteFileIfChanged(filepath.Join(formsDir, "index.ts"), []byte(idx.String()), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated Forms Runtime: %s\n", filepath.Join(formsDir, "FormRenderer.tsx"))
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
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
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
	return WriteFileIfChanged(filepath.Join(dir, "index.ts"), []byte(content), 0644)
}
