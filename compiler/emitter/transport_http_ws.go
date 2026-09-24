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

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/rawbody"
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
	for name, fn := range authCookieFuncMap(auth) {
		funcMap[name] = fn
	}
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

	targetDir := e.outDir("internal", "transport", "http")
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
	for _, svc := range services {
		if _, ok := groups[svc.Name]; !ok {
			groups[svc.Name] = &HttpServiceGroup{
				Name: svc.Name,
			}
		}
	}
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
			HasBodyField:          rawbody.Passthrough(method, ep.Path),
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
			stub := fmt.Sprintf(`// Code generated by ANG. DO NOT EDIT.
package http

import (
	"github.com/go-chi/chi/v5"
	"%s/internal/port"
)

func Register%sRoutes(r chi.Router, svc port.%s) {
	_ = r
	_ = svc
}
`, e.GoModule, group.Name, group.Name)
			filename := fmt.Sprintf("%s.go", strings.ToLower(group.Name))
			path := filepath.Join(targetDir, filename)
			if err := WriteFileIfChanged(path, []byte(stub), 0o644); err != nil {
				return fmt.Errorf("write http stub: %w", err)
			}
			logGenerated("Generated HTTP stub: %s\n", path)
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
		logGenerated("Generated HTTP: %s\n", path)
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

	targetDir := e.outDir("internal", "transport", "http")
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

	wsStaticPathPrefix := func(path string) string {
		if idx := strings.Index(path, "{"); idx >= 0 {
			path = path[:idx]
		}
		return strings.TrimRight(path, "/")
	}

	hasDynamicRoomSiblings := func(ep normalizer.Endpoint, all []normalizer.Endpoint) bool {
		if strings.ToUpper(ep.Method) != "WS" {
			return false
		}
		if len(pathParams(ep.Path)) != 0 {
			return false
		}
		base := wsStaticPathPrefix(ep.Path)
		if base == "" {
			return false
		}
		for _, other := range all {
			if other.ServiceName != ep.ServiceName || other.RPC == ep.RPC || strings.ToUpper(other.Method) != "WS" {
				continue
			}
			if len(pathParams(other.Path)) == 0 {
				continue
			}
			if wsStaticPathPrefix(other.Path) == base {
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
	hasLive := false
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
		var authCheckInput normalizer.Entity
		authCheckCompanyField, authCheckUserField := "", ""
		if ep.AuthCheck != "" {
			authMethod, ok := methods[ep.AuthCheck]
			if !ok {
				return fmt.Errorf("websocket %s %s: auth.check %q is not a method of service %s", ep.Method, ep.Path, ep.AuthCheck, ep.ServiceName)
			}
			authCheckInput = authMethod.Input
			authCheckHasCompanyID = hasField(authMethod.Input, "companyId")
			authCheckCompanyField = exportedFieldName(authMethod.Input, "companyId")
			authCheckUserField = exportedFieldName(authMethod.Input, "userId")
			groups[ep.ServiceName].HasAuthCheck = true
		}
		// A per-room check guards the room in the path. Dynamic subscribe
		// frames would let the socket join other rooms without it.
		allowDynamicRooms := ep.AuthCheck == "" && hasDynamicRoomSiblings(ep, endpoints)
		var broadcasts []normalizer.Entity
		for _, evt := range ep.Messages {
			if ent, ok := eventMap[evt]; ok {
				broadcasts = append(broadcasts, ent)
			}
		}
		if len(broadcasts) > 0 {
			groups[ep.ServiceName].HasBroadcast = true
		}
		view := WsEndpointView{
			Endpoint:              ep,
			Broadcasts:            broadcasts,
			Input:                 method.Input,
			RoomParam:             roomParam,
			RoomField:             roomField,
			AllowDynamicRooms:     allowDynamicRooms,
			AuthCheckHasCompanyID: authCheckHasCompanyID,
			AuthCheckInput:        authCheckInput,
			AuthCheckCompanyField: authCheckCompanyField,
			AuthCheckUserField:    authCheckUserField,
		}
		if ep.Live != nil {
			if err := resolveWSLiveRoom(&view, methods, eventMap); err != nil {
				return err
			}
			groups[ep.ServiceName].HasLive = true
			hasLive = true
		}
		groups[ep.ServiceName].Endpoints = append(groups[ep.ServiceName].Endpoints, view)
	}

	wsGroupNames := make([]string, 0, len(groups))
	desiredWSFiles := map[string]struct{}{"ws_common.go": {}}
	if hasLive {
		desiredWSFiles["ws_live.go"] = struct{}{}
		if err := e.emitWSLive(targetDir); err != nil {
			return err
		}
	}
	for name := range groups {
		wsGroupNames = append(wsGroupNames, name)
		desiredWSFiles[fmt.Sprintf("ws_%s.go", strings.ToLower(name))] = struct{}{}
	}
	sort.Strings(wsGroupNames)
	existingWSFiles, err := filepath.Glob(filepath.Join(targetDir, "ws_*.go"))
	if err != nil {
		return fmt.Errorf("glob ws files: %w", err)
	}
	for _, existing := range existingWSFiles {
		name := filepath.Base(existing)
		if _, ok := desiredWSFiles[name]; ok {
			continue
		}
		if err := os.Remove(existing); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale ws file %s: %w", existing, err)
		}
		fmt.Printf("Removed stale generated WebSocket: %s\n", existing)
	}
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
		logGenerated("Generated WebSocket: %s\n", path)
	}

	return nil
}

// exportedFieldName returns the Go name of ent's field matching name
// case-insensitively (companyId matches companyID), or "" when there is none.
func exportedFieldName(ent normalizer.Entity, name string) string {
	for _, f := range ent.Fields {
		if strings.EqualFold(f.Name, name) {
			return ExportName(f.Name)
		}
	}
	return ""
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

// resolveWSLiveRoom checks a live room against its operations and records the
// Go field names the generated code uses. The contract:
//
//	state op: request has the room field (e.g. tenderId); response has `state` (JSON string)
//	view op:  request has `state`, `companyIds` ([]string), `now` (RFC3339Nano);
//	          response has `snapshots` ([]string, JSON per company), `denied` ([]string,
//	          the companies that lost access)
//	          and `refreshAt` (RFC3339Nano, "" = never)
func resolveWSLiveRoom(view *WsEndpointView, methods map[string]normalizer.Method, events map[string]normalizer.Entity) error {
	ep := view.Endpoint
	where := fmt.Sprintf("websocket %s %s: live", ep.Method, ep.Path)
	if ep.AuthCheck == "" || view.AuthCheckCompanyField == "" {
		return fmt.Errorf("%s needs auth.check with a companyId input (the viewer comes from the handshake)", where)
	}
	if view.RoomParam == "" {
		return fmt.Errorf("%s needs a room", where)
	}
	stateOp, ok := methods[ep.Live.State]
	if !ok {
		return fmt.Errorf("%s: state %q is not a method of service %s", where, ep.Live.State, ep.ServiceName)
	}
	viewOp, ok := methods[ep.Live.View]
	if !ok {
		return fmt.Errorf("%s: view %q is not a method of service %s", where, ep.Live.View, ep.ServiceName)
	}
	need := func(ent normalizer.Entity, name, op string) (string, error) {
		f := exportedFieldName(ent, name)
		if f == "" {
			return "", fmt.Errorf("%s: %s has no field %q", where, op, name)
		}
		return f, nil
	}
	var err error
	if view.LiveStateRoomField, err = need(stateOp.Input, view.RoomParam, ep.Live.State+" request"); err != nil {
		return err
	}
	if _, err = need(stateOp.Output, "state", ep.Live.State+" response"); err != nil {
		return err
	}
	if view.LiveViewStateField, err = need(viewOp.Input, "state", ep.Live.View+" request"); err != nil {
		return err
	}
	if view.LiveViewCompaniesField, err = need(viewOp.Input, "companyIds", ep.Live.View+" request"); err != nil {
		return err
	}
	if view.LiveViewNowField, err = need(viewOp.Input, "now", ep.Live.View+" request"); err != nil {
		return err
	}
	if view.LiveViewSnapshotsField, err = need(viewOp.Output, "snapshots", ep.Live.View+" response"); err != nil {
		return err
	}
	if view.LiveViewDeniedField, err = need(viewOp.Output, "denied", ep.Live.View+" response"); err != nil {
		return err
	}
	if view.LiveViewRefreshField, err = need(viewOp.Output, "refreshAt", ep.Live.View+" response"); err != nil {
		return err
	}
	if len(ep.Live.Triggers) == 0 {
		return fmt.Errorf("%s needs at least one trigger event", where)
	}
	seen := map[string]bool{}
	for _, name := range ep.Live.Triggers {
		if seen[name] {
			continue
		}
		seen[name] = true
		evt, ok := events[name]
		if !ok {
			return fmt.Errorf("%s: trigger %q is not an event", where, name)
		}
		roomField := exportedFieldName(evt, view.RoomParam)
		if roomField == "" {
			return fmt.Errorf("%s: trigger %q has no field %q naming the room", where, name, view.RoomParam)
		}
		view.LiveTriggers = append(view.LiveTriggers, WsLiveTrigger{Event: name, GoType: ExportName(name), RoomField: roomField})
	}
	view.IsLive = true
	view.LiveState = ep.Live.State
	view.LiveView = ep.Live.View
	view.LiveName = strings.ToLower(ep.RPC)
	return nil
}

// emitWSLive writes ws_live.go: the live-room runtime shared by every live
// WebSocket endpoint.
func (e *Emitter) emitWSLive(targetDir string) error {
	tmplContent, err := ReadTemplateByPath("templates/websocket_live.tmpl")
	if err != nil {
		return fmt.Errorf("read ws live template: %w", err)
	}
	t, err := template.New("ws_live").Funcs(template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"CompilerHash": func() string { return e.CompilerHash },
		"GoModule":     func() string { return e.GoModule },
	}).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse ws live template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute ws live template: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format ws_live.go: %w", err)
	}
	path := filepath.Join(targetDir, "ws_live.go")
	if err := WriteFileIfChanged(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	logGenerated("Generated WebSocket Live Rooms: %s\n", path)
	return nil
}
