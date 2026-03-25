package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

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
	funcMap["HasAuthInject"] = func(ep normalizer.Endpoint, field string) bool {
		for _, injected := range ep.AuthInject {
			if strings.EqualFold(injected, field) {
				return true
			}
		}
		return false
	}
	funcMap["RequestBodyEncoding"] = func(ep HttpEndpointView) string {
		if ep.Metadata == nil {
			return ""
		}
		v, _ := ep.Metadata["request_body"].(string)
		return strings.TrimSpace(v)
	}

	t, err := template.New("http").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "transport", "http")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
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
		if ep.IsStreaming {
			groups[ep.ServiceName].HasStreaming = true
		}
		if len(broadcasts) > 0 {
			groups[ep.ServiceName].HasBroadcast = true
		}
	}

	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, groupName := range groupNames {
		group := groups[groupName]
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
			HasStreaming:   group.HasStreaming,
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
		if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
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

	// First, emit the common WS infrastructure.
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
			// Auth is excluded: WS auth happens post-upgrade via first message frame.
			return buildMiddlewareListFull(ep.Endpoint, false, false, true)
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
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
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

	wsGroupNames := make([]string, 0, len(groups))
	for name := range groups {
		wsGroupNames = append(wsGroupNames, name)
	}
	sort.Strings(wsGroupNames)
	for _, groupName := range wsGroupNames {
		group := groups[groupName]
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
		if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
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
