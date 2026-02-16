package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type HttpEndpointView struct {
	normalizer.Endpoint
	Input                 normalizer.Entity
	Output                normalizer.Entity
	Broadcasts            []normalizer.Entity
	RoomField             string
	AuthCheckHasCompanyID bool
	HasBodyField          bool
}

type HttpServiceGroup struct {
	Name           string
	Endpoints      []HttpEndpointView
	HasViews       bool
	HasQueryParse  bool
	HasETag        bool
	HasBroadcast   bool
	HasDomainUsage bool
}

type WsEndpointView struct {
	normalizer.Endpoint
	Broadcasts            []normalizer.Entity
	Input                 normalizer.Entity
	RoomParam             string
	RoomField             string
	AuthCheckHasCompanyID bool
}

type WsServiceGroup struct {
	Name         string
	Endpoints    []WsEndpointView
	HasBroadcast bool
	HasRooms     bool
}

func buildRequireRoles(roles []string) string {
	quoted := make([]string, 0, len(roles))
	for _, role := range roles {
		quoted = append(quoted, fmt.Sprintf("%q", role))
	}
	return fmt.Sprintf("RequireRoles([]string{%s})", strings.Join(quoted, ", "))
}

func buildMiddlewareList(ep normalizer.Endpoint, includeCache, includeIdempotency bool) string {
	var parts []string
	if ep.MaxBodySize > 0 {
		parts = append(parts, fmt.Sprintf("MaxBodySizeMiddleware(%d)", ep.MaxBodySize))
	}
	if ep.AuthType != "" {
		parts = append(parts, "AuthMiddleware")
		if len(ep.AuthRoles) > 0 {
			parts = append(parts, buildRequireRoles(ep.AuthRoles))
		}
		if ep.Permission != "" {
			parts = append(parts, fmt.Sprintf("RequirePermission(%q)", ep.Permission))
		}
	}
	if includeCache && ep.CacheTTL != "" {
		parts = append(parts, fmt.Sprintf("CacheMiddleware(%q)", ep.CacheTTL))
	}
	if ep.RateLimit != nil {
		parts = append(parts, fmt.Sprintf("RateLimitMiddleware(%d, %d)", ep.RateLimit.RPS, ep.RateLimit.Burst))
	}
	if ep.CircuitBreaker != nil {
		parts = append(parts, fmt.Sprintf("CircuitBreakerMiddleware(%d, %q, %d)", ep.CircuitBreaker.Threshold, ep.CircuitBreaker.Timeout, ep.CircuitBreaker.HalfOpenMax))
	}
	if ep.Timeout != "" {
		parts = append(parts, fmt.Sprintf("TimeoutMiddleware(%q)", ep.Timeout))
	}
	if includeIdempotency && ep.Idempotency {
		parts = append(parts, "IdempotencyMiddleware()")
	}
	return strings.Join(parts, ", ")
}

// EmitHTTP generates HTTP routers.
func (e *Emitter) EmitHTTP(irEndpoints []ir.Endpoint, irServices []ir.Service, irEvents []ir.Event, auth *normalizer.AuthDef) error {
	endpoints := IREndpointsToNormalizer(irEndpoints)
	services := IRServicesToNormalizer(irServices)
	events := IREventsToNormalizer(irEvents)

	tmplPath := "templates/http.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["ParamForField"] = func(path, field string) string {
		normalizedField := strings.ToLower(strings.ReplaceAll(field, "_", ""))
		for _, p := range pathParams(path) {
			normalizedParam := strings.ToLower(strings.ReplaceAll(p, "_", ""))
			if normalizedParam == normalizedField {
				return p
			}
		}
		return ""
	}
	funcMap["JoinQuoted"] = func(items []string) string {
		if len(items) == 0 {
			return ""
		}
		quoted := make([]string, 0, len(items))
		for _, item := range items {
			quoted = append(quoted, fmt.Sprintf("%q", item))
		}
		return strings.Join(quoted, ", ")
	}
	funcMap["MiddlewareList"] = func(ep normalizer.Endpoint) string {
		return buildMiddlewareList(ep, true, true)
	}

	t, err := template.New("http").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	hasField := func(ent normalizer.Entity, name string) bool {
		for _, f := range ent.Fields {
			if strings.EqualFold(f.Name, name) {
				return true
			}
		}
		return false
	}

	methodsByService := make(map[string]map[string]normalizer.Method)
	for _, svc := range services {
		methods := make(map[string]normalizer.Method)
		for _, m := range svc.Methods {
			methods[m.Name] = m
		}
		methodsByService[svc.Name] = methods
	}

	eventMap := make(map[string]normalizer.Entity)
	for _, ev := range events {
		eventMap[ev.Name] = normalizer.Entity{Name: ev.Name, Fields: ev.Fields}
	}

	wsEventsAll := make(map[string]bool)
	wsRoomFieldByService := make(map[string]string)
	wsRoomFieldDefault := ""
	for _, ep := range endpoints {
		if strings.ToUpper(ep.Method) != "WS" {
			continue
		}
		for _, msg := range ep.Messages {
			if msg != "" {
				wsEventsAll[msg] = true
			}
		}
		if wsRoomFieldByService[ep.ServiceName] == "" {
			param := ep.RoomParam
			if param == "" {
				param = firstPathParam(ep.Path)
			}
			if param != "" {
				wsRoomFieldByService[ep.ServiceName] = ToTitle(param)
				if wsRoomFieldDefault == "" {
					wsRoomFieldDefault = wsRoomFieldByService[ep.ServiceName]
				}
			}
		}
	}

	groups := make(map[string]*HttpServiceGroup)
	for _, ep := range endpoints {
		if _, ok := groups[ep.ServiceName]; !ok {
			groups[ep.ServiceName] = &HttpServiceGroup{
				Name: ep.ServiceName,
			}
		}
		methods := methodsByService[ep.ServiceName]
		method, ok := methods[ep.RPC]
		if !ok {
			return fmt.Errorf("missing method %s for service %s", ep.RPC, ep.ServiceName)
		}
		authCheckHasCompanyID := false
		if ep.AuthCheck != "" {
			if authMethod, ok := methods[ep.AuthCheck]; ok {
				authCheckHasCompanyID = hasField(authMethod.Input, "companyId")
			}
		}
		ep.Errors = method.Throws
		ep.Pagination = method.Pagination
		var broadcasts []normalizer.Entity
		if len(wsEventsAll) > 0 {
			for _, evt := range method.Broadcasts {
				if wsEventsAll[evt] {
					if ent, ok := eventMap[evt]; ok {
						broadcasts = append(broadcasts, ent)
					}
				}
			}
		}
		groups[ep.ServiceName].Endpoints = append(groups[ep.ServiceName].Endpoints, HttpEndpointView{
			Endpoint:              ep,
			Input:                 method.Input,
			Output:                method.Output,
			Broadcasts:            broadcasts,
			AuthCheckHasCompanyID: authCheckHasCompanyID,
			HasBodyField:          hasField(method.Input, "body") && len(method.Input.Fields) == 1,
			RoomField: func() string {
				roomField := wsRoomFieldByService[ep.ServiceName]
				if roomField == "" {
					roomField = wsRoomFieldDefault
				}
				if roomField == "" {
					return ""
				}
				for _, f := range method.Input.Fields {
					if strings.EqualFold(f.Name, roomField) {
						return roomField
					}
				}
				return ""
			}(),
		})
		if ep.View != "" {
			groups[ep.ServiceName].HasViews = true
		}
		pathParamsByField := func(path string, fields []normalizer.Field) bool {
			for _, f := range fields {
				normalizedField := strings.ToLower(strings.ReplaceAll(f.Name, "_", ""))
				for _, p := range pathParams(path) {
					normalizedParam := strings.ToLower(strings.ReplaceAll(p, "_", ""))
					if normalizedParam != normalizedField {
						continue
					}
					if f.Type == "int" || f.Type == "float64" || f.Type == "bool" {
						return true
					}
				}
			}
			return false
		}
		if strings.ToUpper(ep.Method) == "GET" || pathParamsByField(ep.Path, method.Input.Fields) {
			for _, f := range method.Input.Fields {
				if f.Type == "int" || f.Type == "float64" || f.Type == "bool" {
					groups[ep.ServiceName].HasQueryParse = true
					break
				}
			}
		}
		if strings.ToUpper(ep.Method) == "GET" && method.Output.Name != "" {
			groups[ep.ServiceName].HasETag = true
		}
		if len(broadcasts) > 0 {
			groups[ep.ServiceName].HasBroadcast = true
		}
	}

	for _, group := range groups {
		var buf bytes.Buffer

		hasBroadcastInGroup := false
		hasDomainUsageInGroup := false
		for _, ep := range group.Endpoints {
			if strings.ToUpper(ep.Method) != "WS" {
				if len(ep.Broadcasts) > 0 {
					hasBroadcastInGroup = true
					hasDomainUsageInGroup = true
				}
			}
		}

		httpOnly := HttpServiceGroup{
			Name:           group.Name,
			HasViews:       group.HasViews,
			HasQueryParse:  group.HasQueryParse,
			HasETag:        group.HasETag,
			HasBroadcast:   hasBroadcastInGroup,
			HasDomainUsage: hasDomainUsageInGroup,
		}
		for _, ep := range group.Endpoints {
			if strings.ToUpper(ep.Method) != "WS" {
				httpOnly.Endpoints = append(httpOnly.Endpoints, ep)
			}
		}
		if len(httpOnly.Endpoints) == 0 {
			continue
		}
		if err := t.Execute(&buf, httpOnly); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for HTTP %s. Writing raw.\n", group.Name)
			formatted = buf.Bytes()
		}

		filename := fmt.Sprintf("%s.go", strings.ToLower(group.Name))
		path := filepath.Join(targetDir, filename)
		if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated HTTP: %s\n", path)
	}

	if err := e.EmitHTTPCommon(auth); err != nil {
		return err
	}
	return e.EmitWebSocket(irEndpoints, irServices, irEvents)
}

// EmitWebSocket generates WebSocket routers.
func (e *Emitter) EmitWebSocket(irEndpoints []ir.Endpoint, irServices []ir.Service, irEvents []ir.Event) error {
	endpoints := IREndpointsToNormalizer(irEndpoints)
	services := IRServicesToNormalizer(irServices)
	events := IREventsToNormalizer(irEvents)

	// First, emit the common WS infrastructure
	if err := e.emitWSCommon(); err != nil {
		return err
	}

	tmplPath := "templates/websocket.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"InputHash":    func() string { return e.InputHash },
		"CompilerHash": func() string { return e.CompilerHash },
		"GoModule":     func() string { return e.GoModule },
		"Title":        ToTitle,
		"ExportName":   ExportName,
		"ToLower":      strings.ToLower,
		"makeMap": func() map[string]bool {
			return make(map[string]bool)
		},
		"mapHas": func(m map[string]bool, key string) bool {
			return m[key]
		},
		"mapSet": func(m map[string]bool, key string, val bool) string {
			m[key] = val
			return ""
		},
		"JoinQuoted": func(items []string) string {
			if len(items) == 0 {
				return ""
			}
			quoted := make([]string, 0, len(items))
			for _, item := range items {
				quoted = append(quoted, fmt.Sprintf("%q", item))
			}
			return strings.Join(quoted, ", ")
		},
		"MiddlewareList": func(ep normalizer.Endpoint) string {
			return buildMiddlewareList(ep, false, false)
		},
		"WSMiddlewareList": func(ep WsEndpointView) string {
			return buildMiddlewareList(ep.Endpoint, false, false)
		},
		"ParamForField": func(path, field string) string {
			normalizedField := strings.ToLower(strings.ReplaceAll(field, "_", ""))
			for _, p := range pathParams(path) {
				normalizedParam := strings.ToLower(strings.ReplaceAll(p, "_", ""))
				if normalizedParam == normalizedField {
					return p
				}
			}
			return ""
		},
		"stringsEqualFold": strings.EqualFold,
		"HasInputField": func(input normalizer.Entity, name string) bool {
			for _, f := range input.Fields {
				if strings.EqualFold(f.Name, name) {
					return true
				}
			}
			return false
		},
	}

	t, err := template.New("websocket").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	hasField := func(ent normalizer.Entity, name string) bool {
		for _, f := range ent.Fields {
			if strings.EqualFold(f.Name, name) {
				return true
			}
		}
		return false
	}

	eventMap := make(map[string]normalizer.Entity)
	for _, ev := range events {
		eventMap[ev.Name] = normalizer.Entity{Name: ev.Name, Fields: ev.Fields}
	}

	methodsByService := make(map[string]map[string]normalizer.Method)
	for _, svc := range services {
		methods := make(map[string]normalizer.Method)
		for _, m := range svc.Methods {
			methods[m.Name] = m
		}
		methodsByService[svc.Name] = methods
	}

	groups := make(map[string]*WsServiceGroup)
	for _, ep := range endpoints {
		if strings.ToUpper(ep.Method) != "WS" {
			continue
		}
		if _, ok := groups[ep.ServiceName]; !ok {
			groups[ep.ServiceName] = &WsServiceGroup{
				Name: ep.ServiceName,
			}
		}
		roomParam := ep.RoomParam
		if roomParam == "" {
			roomParam = firstPathParam(ep.Path)
		}
		roomField := ""
		if roomParam != "" {
			roomField = ToTitle(roomParam)
			groups[ep.ServiceName].HasRooms = true
		}
		methods := methodsByService[ep.ServiceName]
		method, ok := methods[ep.RPC]
		if !ok {
			return fmt.Errorf("missing method %s for service %s", ep.RPC, ep.ServiceName)
		}
		authCheckHasCompanyID := false
		if ep.AuthCheck != "" {
			if authMethod, ok := methods[ep.AuthCheck]; ok {
				authCheckHasCompanyID = hasField(authMethod.Input, "companyId")
			}
		}
		var broadcasts []normalizer.Entity
		for _, evt := range ep.Messages {
			if ent, ok := eventMap[evt]; ok {
				broadcasts = append(broadcasts, ent)
			}
		}
		if len(broadcasts) > 0 {
			groups[ep.ServiceName].HasBroadcast = true
		}
		groups[ep.ServiceName].Endpoints = append(groups[ep.ServiceName].Endpoints, WsEndpointView{
			Endpoint:              ep,
			Broadcasts:            broadcasts,
			Input:                 method.Input,
			RoomParam:             roomParam,
			RoomField:             roomField,
			AuthCheckHasCompanyID: authCheckHasCompanyID,
		})
	}

	for _, group := range groups {
		var buf bytes.Buffer
		if err := t.Execute(&buf, group); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Printf("Formatting failed for WS %s. Writing raw.\n", group.Name)
			formatted = buf.Bytes()
		}

		filename := fmt.Sprintf("ws_%s.go", strings.ToLower(group.Name))
		path := filepath.Join(targetDir, filename)
		if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Generated WebSocket: %s\n", path)
	}

	return nil
}

func firstPathParam(path string) string {
	params := pathParams(path)
	if len(params) == 0 {
		return ""
	}
	return params[0]
}

func pathParams(path string) []string {
	var params []string
	start := strings.Index(path, "{")
	for start != -1 {
		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}
		param := path[start+1 : start+end]
		if param != "" {
			params = append(params, param)
		}
		next := start + end + 1
		start = strings.Index(path[next:], "{")
		if start != -1 {
			start += next
		}
	}
	return params
}

// emitWSCommon generates the shared WebSocket infrastructure (types, hub, etc.)
func (e *Emitter) emitWSCommon() error {
	tmplPath := "templates/websocket_common.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read ws common template: %w", err)
	}

	funcMap := template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"InputHash":    func() string { return e.InputHash },
		"CompilerHash": func() string { return e.CompilerHash },
	}

	t, err := template.New("ws_common").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse ws common template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute ws common template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Formatting failed for ws_common.go. Writing raw.\n")
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "ws_common.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated WebSocket Common: %s\n", path)
	return nil
}

// EmitHTTPCommon generates shared HTTP middleware.
func (e *Emitter) EmitHTTPCommon(auth *normalizer.AuthDef) error {
	tmplPath := "templates/http_common.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("http_common").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	var buf bytes.Buffer
	if auth == nil {
		auth = &normalizer.AuthDef{
			Alg:              "RS256",
			Issuer:           "",
			Audience:         "",
			UserIDClaim:      "sub",
			CompanyIDClaim:   "cid",
			RolesClaim:       "roles",
			PermissionsClaim: "perms",
		}
	}
	if err := t.Execute(&buf, auth); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "common.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated HTTP Common: %s\n", path)
	return nil
}

// EmitMetrics generates Prometheus middleware.
func (e *Emitter) EmitMetrics() error {
	tmplPath := "templates/metrics.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("metrics").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "metrics.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Metrics Middleware: %s\n", path)
	return nil
}

// EmitLoggingMiddleware generates middleware for structured logging.
func (e *Emitter) EmitLoggingMiddleware() error {
	tmplPath := "templates/logging_middleware.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("logging_middleware").Funcs(e.getSharedFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	// dir exists

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	path := filepath.Join(targetDir, "logging.go")
	if err := WriteFileIfChanged(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated Logging Middleware: %s\n", path)
	return nil
}

// EmitOpenAPI generates the Swagger specification.
type OpenAPIEndpoint struct {
	Endpoint  normalizer.Endpoint
	ErrorDefs []normalizer.ErrorDef
	Input     normalizer.Entity
	Output    normalizer.Entity
}

type OpenAPIContext struct {
	Endpoints    []OpenAPIEndpoint
	Schemas      []normalizer.Entity
	Title        string
	Version      string
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

func (e *Emitter) EmitOpenAPI(irEndpoints []ir.Endpoint, irServices []ir.Service, irErrors []ir.Error, project *normalizer.ProjectDef) error {
	endpoints := IREndpointsToNormalizer(irEndpoints)
	services := IRServicesToNormalizer(irServices)
	errors := IRErrorsToNormalizer(irErrors)

	tmplPath := "templates/openapi.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
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
	requiredField := func(f normalizer.Field) bool {
		rules := parseValidateTag(f.ValidateTag)
		return !f.IsOptional || rules.Required
	}
	openAPIRules := func(f normalizer.Field) []string {
		rules := parseValidateTag(f.ValidateTag)
		isString := f.Type == "string"
		isNumber := f.Type == "int" || f.Type == "int64" || f.Type == "float64" || f.Type == "float32" || f.Type == "float"
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
		var out []string
		if isString {
			if rules.Len != nil {
				out = append(out, fmt.Sprintf("minLength: %d", int(*rules.Len)))
				out = append(out, fmt.Sprintf("maxLength: %d", int(*rules.Len)))
			} else {
				if minVal != nil {
					out = append(out, fmt.Sprintf("minLength: %d", int(*minVal)))
				}
				if maxVal != nil {
					out = append(out, fmt.Sprintf("maxLength: %d", int(*maxVal)))
				}
			}
		}
		if isNumber {
			if minVal != nil {
				out = append(out, fmt.Sprintf("minimum: %s", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", *minVal), "0"), ".")))
			}
			if maxVal != nil {
				out = append(out, fmt.Sprintf("maximum: %s", strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", *maxVal), "0"), ".")))
			}
		}
		return out
	}
	openAPIFieldFormat := func(f normalizer.Field) string {
		rules := parseValidateTag(f.ValidateTag)
		if f.Type == "string" {
			if rules.Email {
				return "email"
			}
			if rules.URL {
				return "uri"
			}
		}
		return ""
	}
	openAPIRequiredFields := func(entity normalizer.Entity) []string {
		var fields []string
		for _, f := range entity.Fields {
			if requiredField(f) {
				fields = append(fields, strings.ToLower(f.Name))
			}
		}
		return fields
	}

	funcMap := e.getSharedFuncMap()
	funcMap["Lower"] = strings.ToLower
	funcMap["IsArray"] = func(goType string) bool {
		return strings.HasPrefix(goType, "[]")
	}
	funcMap["OpenAPIType"] = func(goType string) string {
		if strings.HasPrefix(goType, "[]") {
			return "array"
		}
		switch goType {
		case "int", "int64":
			return "integer"
		case "float64", "float32":
			return "number"
		case "bool":
			return "boolean"
		case "time.Time":
			return "string"
		default:
			return "string"
		}
	}
	funcMap["OpenAPIFormat"] = func(goType string) string {
		switch goType {
		case "int", "int64":
			return "int64"
		case "float64", "float32":
			return "double"
		case "time.Time":
			return "date-time"
		default:
			return ""
		}
	}
	funcMap["OpenAPIFieldFormat"] = func(f normalizer.Field) string {
		if format := openAPIFieldFormat(f); format != "" {
			return format
		}
		return ""
	}
	funcMap["OpenAPIRules"] = func(f normalizer.Field) []string {
		return openAPIRules(f)
	}
	funcMap["OpenAPIRequiredFields"] = func(entity normalizer.Entity) []string {
		return openAPIRequiredFields(entity)
	}
	funcMap["IsRequiredField"] = func(f normalizer.Field) bool {
		return requiredField(f)
	}
	funcMap["OpenAPIItemsType"] = func(goType string) string {
		if strings.HasPrefix(goType, "[]") {
			return strings.TrimPrefix(goType, "[]")
		}
		return ""
	}

	t, err := template.New("openapi").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "api")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	errMap := make(map[string]normalizer.ErrorDef)
	for _, e := range errors {
		errMap[e.Name] = e
	}

	methodsByService := make(map[string]map[string]normalizer.Method)
	for _, svc := range services {
		methods := make(map[string]normalizer.Method)
		for _, m := range svc.Methods {
			methods[m.Name] = m
		}
		methodsByService[svc.Name] = methods
	}

	var apiEndpoints []OpenAPIEndpoint
	schemaMap := make(map[string]normalizer.Entity)
	nestedMap := make(map[string]normalizer.Entity)
	for _, ep := range endpoints {
		methods := methodsByService[ep.ServiceName]
		method, ok := methods[ep.RPC]
		if ok {
			ep.Errors = method.Throws
			ep.Pagination = method.Pagination
			if method.Input.Name != "" {
				schemaMap[method.Input.Name] = method.Input
				for _, nested := range nestedEntitiesFromEntity(method.Input) {
					nestedMap[nested.Name] = nested
				}
			}
			if method.Output.Name != "" {
				schemaMap[method.Output.Name] = method.Output
				for _, nested := range nestedEntitiesFromEntity(method.Output) {
					nestedMap[nested.Name] = nested
				}
			}
		}

		var defs []normalizer.ErrorDef
		for _, name := range ep.Errors {
			if def, ok := errMap[name]; ok {
				defs = append(defs, def)
			}
		}
		oe := OpenAPIEndpoint{
			Endpoint:  ep,
			ErrorDefs: defs,
		}
		if ok {
			oe.Input = method.Input
			oe.Output = method.Output
		}
		apiEndpoints = append(apiEndpoints, oe)
	}

	var schemas []normalizer.Entity
	schemaNames := make([]string, 0, len(schemaMap))
	for name := range schemaMap {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)
	for _, name := range schemaNames {
		schemas = append(schemas, schemaMap[name])
	}
	nestedNames := make([]string, 0, len(nestedMap))
	for name := range nestedMap {
		nestedNames = append(nestedNames, name)
	}
	sort.Strings(nestedNames)
	for _, name := range nestedNames {
		schemas = append(schemas, nestedMap[name])
	}

	var buf bytes.Buffer
	title := "ANG API"
	version := "0.1.0"
	if project != nil {
		if strings.TrimSpace(project.Name) != "" {
			title = project.Name + " API"
		}
		if strings.TrimSpace(project.Version) != "" {
			version = project.Version
		}
	}
	ctx := OpenAPIContext{
		Endpoints:    apiEndpoints,
		Schemas:      schemas,
		Title:        title,
		Version:      version,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}
	if err := t.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	path := filepath.Join(targetDir, "openapi.yaml")
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated OpenAPI Spec: %s\n", path)
	return nil
}

// EmitAsyncAPI generates the AsyncAPI specification for events.
func (e *Emitter) EmitAsyncAPI(irEvents []ir.Event, project *normalizer.ProjectDef) error {
	events := IREventsToNormalizer(irEvents)

	tmplPath := "templates/asyncapi.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	funcMap := e.getSharedFuncMap()
	funcMap["AsyncApiType"] = func(goType string) string {
		switch goType {
		case "int", "int64":
			return "integer"
		case "float64", "float":
			return "number"
		case "bool":
			return "boolean"
		default:
			return "string"
		}
	}

	t, err := template.New("asyncapi").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "api")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	title := "ANG Events"
	version := "0.1.0"
	if project != nil {
		if strings.TrimSpace(project.Name) != "" {
			title = project.Name + " Events"
		}
		if strings.TrimSpace(project.Version) != "" {
			version = project.Version
		}
	}
	data := struct {
		Events       []normalizer.EventDef
		Title        string
		Version      string
		ANGVersion   string
		InputHash    string
		CompilerHash string
	}{
		Events:       events,
		Title:        title,
		Version:      version,
		ANGVersion:   e.Version,
		InputHash:    e.InputHash,
		CompilerHash: e.CompilerHash,
	}
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	path := filepath.Join(targetDir, "asyncapi.yaml")
	if err := WriteFileIfChanged(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	fmt.Printf("Generated AsyncAPI Spec: %s\n", path)
	return nil
}
