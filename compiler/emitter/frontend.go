package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/policy"
)

type FrontendContext struct {
	Entities               []normalizer.Entity
	Services               []normalizer.Service
	Endpoints              []normalizer.Endpoint
	Events                 []normalizer.EventDef
	Errors                 []normalizer.ErrorDef
	RBAC                   *normalizer.RBACDef
	QueryResources         []QueryResource
	QueryKeysNeedsTypes    bool
	QueryOptionsNeedsTypes bool
}

type QueryResource struct {
	Key             string
	Segment         string
	HasList         bool
	HasDetail       bool
	HasMe           bool
	ListRPC         string
	DetailRPC       string
	MeRPC           string
	ListFiltersType string
	DetailParamName string
}

func buildQueryResources(endpoints []normalizer.Endpoint) ([]QueryResource, bool, bool) {
	type resourceEntry struct {
		r QueryResource
	}
	resources := map[string]*resourceEntry{}

	for _, ep := range endpoints {
		if strings.ToUpper(ep.Method) != "GET" {
			continue
		}
		segs := pathSegments(ep.Path)
		if len(segs) == 0 {
			continue
		}
		resourceSegment := segs[0]
		resourceKeySource := segs[0]
		detailIndex := 1
		if segs[0] == "admin" && len(segs) >= 2 {
			resourceSegment = "admin/" + segs[1]
			resourceKeySource = "admin-" + segs[1]
			detailIndex = 2
		}
		if resourceKeySource == "" {
			continue
		}
		entry, ok := resources[resourceSegment]
		if !ok {
			entry = &resourceEntry{
				r: QueryResource{
					Key:     JSONName(resourceKeySource),
					Segment: resourceSegment,
				},
			}
			resources[resourceSegment] = entry
		}

		switch {
		case len(segs) == detailIndex:
			if !entry.r.HasList {
				entry.r.HasList = true
				entry.r.ListRPC = ep.RPC
				entry.r.ListFiltersType = "Types." + ep.RPC + "Request"
			}
		case len(segs) == detailIndex+1 && segs[detailIndex] == "me":
			if !entry.r.HasMe {
				entry.r.HasMe = true
				entry.r.MeRPC = ep.RPC
			}
		case len(segs) == detailIndex+1 && isPathParamSegment(segs[detailIndex]):
			if !entry.r.HasDetail {
				entry.r.HasDetail = true
				entry.r.DetailRPC = ep.RPC
				param := strings.TrimSuffix(strings.TrimPrefix(segs[detailIndex], "{"), "}")
				entry.r.DetailParamName = JSONName(param)
			}
		}
	}

	var out []QueryResource
	for _, entry := range resources {
		out = append(out, entry.r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})

	keysNeedsTypes := false
	optionsNeedsTypes := false
	for _, r := range out {
		if r.HasList {
			keysNeedsTypes = true
			optionsNeedsTypes = true
		}
		if r.HasDetail {
			optionsNeedsTypes = true
		}
	}

	return out, keysNeedsTypes, optionsNeedsTypes
}

func pathSegments(path string) []string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	segs := strings.Split(path, "/")
	if len(segs) > 0 && segs[0] == "api" {
		segs = segs[1:]
	}
	return segs
}

func isPathParamSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

func mutationInvalidateStoresForEndpoint(ep normalizer.Endpoint, entities []normalizer.Entity) []string {
	method := strings.ToUpper(strings.TrimSpace(ep.Method))
	if method == "" || method == "GET" || method == "WS" {
		return nil
	}

	verbs := []string{"Create", "Delete", "Update", "Patch", "Remove"}
	rpc := strings.TrimSpace(ep.RPC)
	if rpc == "" {
		return nil
	}

	stem := ""
	for _, verb := range verbs {
		if idx := strings.Index(rpc, verb); idx >= 0 {
			stem = strings.TrimSpace(rpc[idx+len(verb):])
			break
		}
	}
	if stem == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var keys []string
	for _, ent := range entities {
		name := strings.TrimSpace(ent.Name)
		if name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(stem), strings.ToLower(name)) {
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// EmitFrontendSDK generates the React SDK (TS, Zod, React Query).
func (e *Emitter) EmitFrontendSDK(entities []ir.Entity, services []ir.Service, endpoints []ir.Endpoint, events []ir.Event, errors []ir.Error, rbac *normalizer.RBACDef) error {
	entitiesNorm := IREntitiesToNormalizer(entities)
	servicesNorm := IRServicesToNormalizer(services)
	endpointsNorm := IREndpointsToNormalizer(endpoints)
	eventsNorm := IREventsToNormalizer(events)
	errorsNorm := IRErrorsToNormalizer(errors)

	// 1. Collect implicit DTOs from services
	for _, svc := range servicesNorm {
		for _, m := range svc.Methods {
			// Add Input Entity if not exists
			exists := false
			for _, ent := range entitiesNorm {
				if ent.Name == m.Input.Name {
					exists = true
					break
				}
			}
			if !exists && m.Input.Name != "" {
				entitiesNorm = append(entitiesNorm, m.Input)
			}

			// Add Output Entity if not exists
			exists = false
			for _, ent := range entitiesNorm {
				if ent.Name == m.Output.Name {
					exists = true
					break
				}
			}
			if !exists && m.Output.Name != "" {
				entitiesNorm = append(entitiesNorm, m.Output)
			}
			for _, ent := range nestedEntitiesFromEntity(m.Input) {
				if !entityExists(entitiesNorm, ent.Name) {
					entitiesNorm = append(entitiesNorm, ent)
				}
			}
			for _, ent := range nestedEntitiesFromEntity(m.Output) {
				if !entityExists(entitiesNorm, ent.Name) {
					entitiesNorm = append(entitiesNorm, ent)
				}
			}
		}
	}

	// Deduplicate errors for Enum generation
	uniqueErrors := make([]normalizer.ErrorDef, 0)
	seenErrorNames := make(map[string]bool)

	// Pre-populate with standard system errors if they are not in the list
	systemErrors := []normalizer.ErrorDef{
		{Name: "VALIDATION_FAILED", Code: 40010},
		{Name: "UNAUTHORIZED", Code: 40100},
		{Name: "FORBIDDEN", Code: 40300},
		{Name: "NOT_FOUND", Code: 40400},
		{Name: "CONFLICT", Code: 40900},
		{Name: "RATE_LIMIT_EXCEEDED", Code: 42900},
		{Name: "INTERNAL_ERROR", Code: 50000},
	}

	for _, se := range systemErrors {
		uniqueErrors = append(uniqueErrors, se)
		seenErrorNames[se.Name] = true
	}

	for _, e := range errorsNorm {
		if !seenErrorNames[e.Name] {
			uniqueErrors = append(uniqueErrors, e)
			seenErrorNames[e.Name] = true
		}
	}

	queryResources, queryKeysNeedsTypes, queryOptionsNeedsTypes := buildQueryResources(endpointsNorm)
	queryOptionsByRPC := make(map[string]QueryResource)
	queryOptionsKindByRPC := make(map[string]string)
	for _, r := range queryResources {
		if r.HasList {
			queryOptionsByRPC[r.ListRPC] = r
			queryOptionsKindByRPC[r.ListRPC] = "list"
		}
		if r.HasDetail {
			queryOptionsByRPC[r.DetailRPC] = r
			queryOptionsKindByRPC[r.DetailRPC] = "detail"
		}
		if r.HasMe {
			queryOptionsByRPC[r.MeRPC] = r
			queryOptionsKindByRPC[r.MeRPC] = "me"
		}
	}

	ctx := FrontendContext{
		Entities:               entitiesNorm,
		Services:               servicesNorm,
		Endpoints:              endpointsNorm,
		Events:                 eventsNorm,
		Errors:                 uniqueErrors,
		RBAC:                   rbac,
		QueryResources:         queryResources,
		QueryKeysNeedsTypes:    queryKeysNeedsTypes,
		QueryOptionsNeedsTypes: queryOptionsNeedsTypes,
	}

	targetDir := e.FrontendDir
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(targetDir, "schemas"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "hooks"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "queries"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "types"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "mocks"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "forms"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "stores"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "prefetch"), 0755); err != nil {
		return err
	}

	tsType := func(goType string) string {
		if strings.HasPrefix(goType, "[]domain.") {
			return strings.TrimPrefix(goType, "[]domain.") + "[]"
		}
		if strings.HasPrefix(goType, "domain.") {
			return strings.TrimPrefix(goType, "domain.")
		}
		switch goType {
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
			if strings.HasPrefix(goType, "[]") {
				return strings.TrimPrefix(goType, "[]") + "[]"
			}
			return "any"
		}
	}
	type validationRules struct {
		Required bool
		Email    bool
		URL      bool
		Min      *float64
		Max      *float64
		Len      *float64
		Gte      *float64
		Lte      *float64
	}
	parseValidateTag := func(tag string) validationRules {
		var rules validationRules
		parts := strings.Split(tag, ",")
		for _, raw := range parts {
			part := strings.TrimSpace(raw)
			if part == "" {
				continue
			}
			switch part {
			case "required":
				rules.Required = true
				continue
			case "email":
				rules.Email = true
				continue
			case "url":
				rules.URL = true
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if val == "" {
				continue
			}
			num, err := strconv.ParseFloat(val, 64)
			if err != nil {
				continue
			}
			switch key {
			case "min":
				rules.Min = &num
			case "max":
				rules.Max = &num
			case "len":
				rules.Len = &num
			case "gte":
				rules.Gte = &num
			case "lte":
				rules.Lte = &num
			}
		}
		return rules
	}
	formatNumber := func(val float64) string {
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	entityIDField := func(entity normalizer.Entity) *normalizer.Field {
		// Prioritize explicit primary key
		for _, f := range entity.Fields {
			if f.DB.PrimaryKey {
				return &f
			}
		}
		// Fallback to "id" field only for domain entities (not Requests/Responses)
		name := strings.ToLower(entity.Name)
		if strings.HasSuffix(name, "request") || strings.HasSuffix(name, "response") || strings.HasSuffix(name, "data") {
			return nil
		}
		for _, f := range entity.Fields {
			if strings.ToLower(f.Name) == "id" {
				return &f
			}
		}
		return nil
	}

	funcMap := template.FuncMap{
		"ToLower":    strings.ToLower,
		"JSONName":   JSONName,
		"ExportName": ExportName,
		"LowerFirst": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
		"HasPrefix": strings.HasPrefix,
		"HasSuffix": strings.HasSuffix,
		"MockValue": func(f normalizer.Field) string {
			if f.IsList {
				return "[]"
			}
			switch f.Type {
			case "string":
				if strings.Contains(strings.ToLower(f.Name), "id") {
					return "\"gen-id-123\""
				}
				if strings.Contains(strings.ToLower(f.Name), "email") {
					return "\"user@example.com\""
				}
				return "\"sample text\""
			case "int", "int64", "float64":
				return "42"
			case "bool":
				return "true"
			case "time.Time":
				return "\"2026-01-01T12:00:00Z\""
			case "map[string]any":
				return "{}"
			case "[]any":
				return "[]"
			default:
				return "null"
			}
		},
		"TSType": tsType,
		"TSFieldType": func(f normalizer.Field) string {
			base := tsType(f.Type)
			if f.Metadata != nil && f.Metadata["client_side_encryption"] == true {
				return "Encrypted<" + base + ">"
			}
			return base
		},
		"ZodType": func(goType string) string {
			if strings.HasPrefix(goType, "[]") {
				elem := strings.TrimPrefix(goType, "[]")
				if strings.HasPrefix(elem, "domain.") {
					elem = strings.TrimPrefix(elem, "domain.")
				}
				switch elem {
				case "string":
					return "z.array(z.string())"
				case "int", "int64", "float64", "float":
					return "z.array(z.number())"
				case "bool":
					return "z.array(z.boolean())"
				case "time.Time":
					return "z.array(z.coerce.date())"
				default:
					for _, ent := range entitiesNorm {
						if ent.Name == elem {
							return fmt.Sprintf("z.array(z.lazy(() => %sSchema))", elem)
						}
					}
					return "z.array(z.any())"
				}
			}
			base := goType
			if strings.HasPrefix(base, "domain.") {
				base = strings.TrimPrefix(base, "domain.")
			}
			switch base {
			case "int", "int64", "float64", "float":
				return "z.number()"
			case "bool":
				return "z.boolean()"
			case "string":
				return "z.string()"
			case "time.Time":
				return "z.coerce.date()"
			default:
				for _, ent := range entitiesNorm {
					if ent.Name == base {
						return fmt.Sprintf("z.lazy(() => %sSchema)", base)
					}
				}
				return "z.any()"
			}
		},
		"IsRequired": func(f normalizer.Field) bool {
			rules := parseValidateTag(f.ValidateTag)
			return !f.IsOptional || rules.Required
		},
		"ZodRules": func(f normalizer.Field) string {
			rules := parseValidateTag(f.ValidateTag)
			var parts []string
			if rules.Email {
				parts = append(parts, ".email()")
			}
			if rules.URL {
				parts = append(parts, ".url()")
			}
			isString := f.Type == "string"
			isNumber := f.Type == "int" || f.Type == "int64" || f.Type == "float64" || f.Type == "float"
			var minVal *float64
			var maxVal *float64
			if rules.Min != nil {
				minVal = rules.Min
			}
			if rules.Gte != nil && (minVal == nil || *rules.Gte > *minVal) {
				minVal = rules.Gte
			}
			if rules.Max != nil {
				maxVal = rules.Max
			}
			if rules.Lte != nil && (maxVal == nil || *rules.Lte < *maxVal) {
				maxVal = rules.Lte
			}
			if isString {
				if rules.Len != nil {
					parts = append(parts, ".length("+formatNumber(*rules.Len)+")")
				} else {
					if minVal != nil {
						parts = append(parts, ".min("+formatNumber(*minVal)+")")
					}
					if maxVal != nil {
						parts = append(parts, ".max("+formatNumber(*maxVal)+")")
					}
				}
			} else if isNumber {
				if minVal != nil {
					parts = append(parts, ".min("+formatNumber(*minVal)+")")
				}
				if maxVal != nil {
					parts = append(parts, ".max("+formatNumber(*maxVal)+")")
				}
			}
			if f.IsOptional && !rules.Required {
				parts = append(parts, ".optional()")
			}
			return strings.Join(parts, "")
		},
		"PathParams": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			matches := re.FindAllStringSubmatch(path, -1)
			var params []string
			for _, m := range matches {
				params = append(params, fmt.Sprintf("%s: string", m[1]))
			}
			return strings.Join(params, ", ")
		},
		"Replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"GetEntity": func(name string) *normalizer.Entity {
			for _, e := range entitiesNorm {
				if strings.EqualFold(e.Name, name) {
					return &e
				}
			}
			return nil
		},
		"HasEntity": func(name string) bool {
			for _, e := range entitiesNorm {
				if strings.EqualFold(e.Name, name) {
					return true
				}
			}
			return false
		},
		"EntityHasID": func(entity normalizer.Entity) bool {
			return entityIDField(entity) != nil
		},
		"EntityIDFieldName": func(entity normalizer.Entity) string {
			field := entityIDField(entity)
			if field == nil {
				return "id"
			}
			return field.Name
		},
		"EntityIDType": func(entity normalizer.Entity) string {
			field := entityIDField(entity)
			if field == nil {
				return "string"
			}
			return tsType(field.Type)
		},
		"PathKeys": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			matches := re.FindAllStringSubmatch(path, -1)
			var keys []string
			for _, m := range matches {
				keys = append(keys, m[1])
			}
			if len(keys) == 0 {
				return ""
			}
			return ", " + strings.Join(keys, ", ")
		},
		"PathArgs": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			matches := re.FindAllStringSubmatch(path, -1)
			var keys []string
			for _, m := range matches {
				keys = append(keys, m[1])
			}
			return strings.Join(keys, ", ")
		},
		"PathArgNames": func(path string) []string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			matches := re.FindAllStringSubmatch(path, -1)
			var keys []string
			for _, m := range matches {
				keys = append(keys, m[1])
			}
			return keys
		},
		"ParamsFromRouter": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			matches := re.FindAllStringSubmatch(path, -1)
			var args []string
			for _, m := range matches {
				args = append(args, fmt.Sprintf("params.%s", m[1]))
			}
			return strings.Join(args, ", ")
		},
		"IsOptimisticCandidate": func(rpc string) bool {
			return strings.HasPrefix(rpc, "Update") || strings.HasPrefix(rpc, "Edit")
		},
		"GetRelatedReadRPC": func(ep normalizer.Endpoint) string {
			if ep.OptimisticUpdate != "" {
				return ep.OptimisticUpdate
			}
			rpc := ep.RPC
			serviceName := ep.ServiceName
			entity := strings.TrimPrefix(strings.TrimPrefix(rpc, "Update"), "Edit")
			target := "Get" + entity
			for _, otherEp := range endpointsNorm {
				if otherEp.ServiceName == serviceName && otherEp.RPC == target {
					return target
				}
			}
			return ""
		},
		"GetEntityIDParam": func(rpc string) string {
			entity := strings.TrimPrefix(strings.TrimPrefix(rpc, "Update"), "Edit")
			return strings.ToLower(entity) + "Id"
		},
		"QueryOptionsResource": func(rpc string) string {
			if r, ok := queryOptionsByRPC[rpc]; ok {
				return r.Key
			}
			return ""
		},
		"QueryOptionsKind": func(rpc string) string {
			return queryOptionsKindByRPC[rpc]
		},
		"QueryOptionsDetailParam": func(rpc string) string {
			if r, ok := queryOptionsByRPC[rpc]; ok {
				return r.DetailParamName
			}
			return ""
		},
		"PathTemplate": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			return re.ReplaceAllString(path, `${$1}`)
		},
		"RouterPath": func(path string) string {
			re := regexp.MustCompile(`{([a-zA-Z0-9]+)}`)
			return re.ReplaceAllString(path, ":$1")
		},
		"ServiceEndpoints": func(serviceName string) []normalizer.Endpoint {
			var out []normalizer.Endpoint
			for _, ep := range endpointsNorm {
				if ep.ServiceName == serviceName {
					out = append(out, ep)
				}
			}
			return out
		},
		"MutationInvalidateStores": func(ep normalizer.Endpoint) []string {
			return mutationInvalidateStoresForEndpoint(ep, entitiesNorm)
		},
		"EndpointPolicy": func(ep normalizer.Endpoint) policy.EndpointPolicy {
			return policy.FromEndpoint(ep)
		},
	}

	files := []struct {
		tmpl string
		out  string
	}{
		{"index", "index.ts"},
		{"api-client", "api-client.ts"},
		{"error-normalizer", "error-normalizer.ts"},
		{"endpoints", "endpoints.ts"},
		{"websocket", "websocket.ts"},
		{"auth-store", "auth-store.ts"},
		{"stores", "stores/index.ts"},
		{"store-invalidation", "stores/invalidation.ts"},
		{"providers", "providers.tsx"},
		{"rbac", "rbac.ts"},
		{"types", "types/index.ts"},
		{"schemas", "schemas/index.ts"},
		{"query-keys-root", "query-keys.ts"},
		{"query-options", "query-options.ts"},
		{"prefetch", "prefetch/index.ts"},
		{"hooks", "hooks/index.ts"},
		{"queries", "queries/index.ts"},
		{"query-keys", "queries/keys.ts"},
		{"websocket-hooks", "hooks/websocket-hooks.ts"},
		{"routes", "routes.ts"},
		{"handlers", "mocks/handlers.ts"},
		{"forms", "forms/index.ts"},
	}

	for _, f := range files {
		if err := e.emitFrontendFile(f.tmpl, ctx, funcMap, f.out); err != nil {
			return err
		}
	}

	// Generate individual entity stores
	for _, ent := range entitiesNorm {
		if entityIDField(ent) != nil {
			storeData := struct {
				Entity normalizer.Entity
			}{Entity: ent}
			outPath := filepath.Join("stores", strings.ToLower(ent.Name)+".ts")
			if err := e.emitFrontendTemplate("templates/frontend/store-item.ts.tmpl", storeData, funcMap, outPath); err != nil {
				return err
			}
		}
	}

	extraFiles := []struct {
		tmplPath string
		out      string
	}{
		{"templates/frontend/package.json.tmpl", "package.json"},
		{"templates/frontend/README.md.tmpl", "README.md"},
		{"templates/frontend/install-sdk.sh.tmpl", "install-sdk.sh"},
	}

	for _, f := range extraFiles {
		if err := e.emitFrontendTemplate(f.tmplPath, ctx, funcMap, f.out); err != nil {
			return err
		}
	}

	if err := e.EmitSDKManifest(endpointsNorm, queryResources); err != nil {
		return err
	}

	return nil
}

func entityExists(entities []normalizer.Entity, name string) bool {
	for _, ent := range entities {
		if ent.Name == name {
			return true
		}
	}
	return false
}

func nestedEntitiesFromEntity(ent normalizer.Entity) []normalizer.Entity {
	seen := make(map[string]struct{})
	var out []normalizer.Entity
	for _, f := range ent.Fields {
		if f.ItemTypeName == "" || len(f.ItemFields) == 0 {
			continue
		}
		if _, ok := seen[f.ItemTypeName]; ok {
			continue
		}
		seen[f.ItemTypeName] = struct{}{}
		out = append(out, normalizer.Entity{
			Name:   f.ItemTypeName,
			Fields: f.ItemFields,
		})
	}
	return out
}

func (e *Emitter) emitFrontendFile(tmplName string, data interface{}, funcs template.FuncMap, outName string) error {
	tmplPath := fmt.Sprintf("templates/frontend/%s.ts.tmpl", tmplName)
	return e.emitFrontendTemplate(tmplPath, data, funcs, outName)
}

func (e *Emitter) emitFrontendTemplate(tmplPath string, data interface{}, funcs template.FuncMap, outName string) error {
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return err
	}

	t, err := template.New(filepath.Base(tmplPath)).Funcs(funcs).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	path := filepath.Join(e.FrontendDir, outName)
	mode := os.FileMode(0644)
	if strings.HasSuffix(outName, ".sh") {
		mode = 0755
	}
	if err := WriteFileIfChanged(path, buf.Bytes(), mode); err != nil {
		return err
	}

	fmt.Printf("Generated SDK: %s\n", path)
	return nil
}
