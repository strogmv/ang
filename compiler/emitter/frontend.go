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
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueload "cuelang.org/go/cue/load"
	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/policy"
)

type FrontendContext struct {
	Entities               []normalizer.Entity
	NamedEnums             []NamedEnum
	Services               []normalizer.Service
	Endpoints              []normalizer.Endpoint
	Events                 []normalizer.EventDef
	Errors                 []normalizer.ErrorDef
	RBAC                   *normalizer.RBACDef
	QueryResources         []QueryResource
	QueryKeysNeedsTypes    bool
	QueryOptionsNeedsTypes bool
	WSInvalidateRules      []WSInvalidateRule
}

type NamedEnum struct {
	Name   string
	Values []string
}

type StoreItemContext struct {
	Entity normalizer.Entity
}

// WSInvalidateRule drives WebSocket → TanStack invalidation (endpoint query key prefixes).
type WSInvalidateRule struct {
	EventName string
	Omit      bool
	Noop      bool
	Prefixes  [][]string
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
	ListCacheTTL    string
	DetailCacheTTL  string
	EntityName      string // derived from DetailRPC — used for Zustand store import
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func namedEnumNameSet(namedEnums []NamedEnum) map[string]struct{} {
	out := make(map[string]struct{}, len(namedEnums))
	for _, enum := range namedEnums {
		if enum.Name == "" {
			continue
		}
		out[enum.Name] = struct{}{}
	}
	return out
}

func matchFieldNamedEnum(f normalizer.Field, namedEnums []NamedEnum, namedEnumSet map[string]struct{}) string {
	baseType := strings.TrimSpace(f.Type)
	isList := f.IsList || strings.HasPrefix(baseType, "[]")
	if isList {
		baseType = strings.TrimPrefix(baseType, "[]")
	}
	baseType = strings.TrimPrefix(baseType, "domain.")
	if _, ok := namedEnumSet[baseType]; ok {
		return baseType
	}
	if f.Constraints == nil || len(f.Constraints.Enum) == 0 {
		return ""
	}
	for _, enum := range namedEnums {
		if equalStringSlices(enum.Values, f.Constraints.Enum) {
			return enum.Name
		}
	}
	return ""
}

func tsEnumKey(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
	})
	if len(parts) == 0 {
		return "Value"
	}
	replacements := map[string]string{
		"ai":     "AI",
		"api":    "API",
		"id":     "ID",
		"paypal": "PayPal",
		"sepa":   "SEPA",
		"ui":     "UI",
		"url":    "URL",
	}
	var b strings.Builder
	for _, part := range parts {
		lower := strings.ToLower(strings.TrimSpace(part))
		if lower == "" {
			continue
		}
		if repl, ok := replacements[lower]; ok {
			b.WriteString(repl)
			continue
		}
		b.WriteString(strings.ToUpper(lower[:1]))
		if len(lower) > 1 {
			b.WriteString(lower[1:])
		}
	}
	if b.Len() == 0 {
		return "Value"
	}
	return b.String()
}

func collectFrontendEnumStrings(v cue.Value) []string {
	if v.IncompleteKind() == cue.StringKind {
		s, err := v.String()
		if err == nil {
			s = strings.TrimSpace(s)
			if s != "" {
				return []string{s}
			}
		}
	}
	if v.IncompleteKind() == cue.ListKind {
		var vals []string
		it, _ := v.List()
		for it.Next() {
			s, err := it.Value().String()
			if err != nil {
				return nil
			}
			s = strings.TrimSpace(s)
			if s != "" {
				vals = append(vals, s)
			}
		}
		if len(vals) > 0 {
			return vals
		}
	}
	if op, args := v.Expr(); op == cue.OrOp {
		vals := make([]string, 0, len(args))
		for _, arg := range args {
			s, err := arg.String()
			if err != nil {
				return nil
			}
			s = strings.TrimSpace(s)
			if s != "" {
				vals = append(vals, s)
			}
		}
		if len(vals) > 0 {
			return vals
		}
	}
	return nil
}

func extractFrontendNamedEnums(projectRoot string) []NamedEnum {
	domainDir := filepath.Join(projectRoot, "cue", "domain")
	stat, err := os.Stat(domainDir)
	if err != nil || !stat.IsDir() {
		return nil
	}
	insts := cueload.Instances([]string{"./cue/domain"}, &cueload.Config{Dir: projectRoot})
	if len(insts) == 0 {
		return nil
	}
	ctx := cuecontext.New()
	built := ctx.BuildInstance(insts[0])
	if err := built.Err(); err != nil {
		return nil
	}
	it, err := built.Fields(cue.Definitions(true), cue.Hidden(true), cue.Optional(true))
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []NamedEnum
	for it.Next() {
		name := strings.TrimPrefix(strings.TrimSpace(it.Selector().String()), "#")
		if name == "" || strings.HasPrefix(name, "_") {
			continue
		}
		if strings.HasSuffix(name, "Service") || strings.HasSuffix(name, "API") || name == "AppConfig" || name == "RBAC" || name == "Scopes" {
			continue
		}
		values := collectFrontendEnumStrings(it.Value())
		if len(values) == 0 {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, NamedEnum{Name: name, Values: values})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func findProjectRootWithCueDomain(start string) string {
	base := strings.TrimSpace(start)
	if base == "" {
		return ""
	}
	abs, err := filepath.Abs(base)
	if err == nil {
		base = abs
	}
	for {
		domainDir := filepath.Join(base, "cue", "domain")
		if stat, err := os.Stat(domainDir); err == nil && stat.IsDir() {
			return base
		}
		parent := filepath.Dir(base)
		if parent == base {
			return ""
		}
		base = parent
	}
}

func resolveFrontendProjectRoot(outputDir, frontendDir string) string {
	for _, candidate := range []string{frontendDir, outputDir} {
		if root := findProjectRootWithCueDomain(candidate); root != "" {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if root := findProjectRootWithCueDomain(cwd); root != "" {
			return root
		}
	}
	return strings.TrimSpace(outputDir)
}

func deriveEntityName(rpc string) string {
	for _, pfx := range []string{"AdminGet", "AdminFetch", "Get", "Fetch", "Find", "Load", "Read"} {
		if strings.HasPrefix(rpc, pfx) {
			return rpc[len(pfx):]
		}
	}
	return ""
}

func metadataString(meta map[string]any, path ...string) string {
	if len(path) == 0 {
		return ""
	}
	var current any = meta
	for _, part := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	switch v := current.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func metadataValue(meta map[string]any, path ...string) any {
	var cur any = meta
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[p]
		if !ok {
			return nil
		}
	}
	return cur
}

func metadataBool(meta map[string]any, path ...string) bool {
	v := metadataValue(meta, path...)
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

func metadataStringSlice(meta map[string]any, path ...string) []string {
	v := metadataValue(meta, path...)
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	case []any:
		var out []string
		for _, el := range x {
			switch t := el.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					out = append(out, s)
				}
			default:
				if s := strings.TrimSpace(fmt.Sprint(t)); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func metadataHasFrontendWS(meta map[string]any) bool {
	f, ok := meta["frontend"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = f["ws"].(map[string]any)
	return ok
}

func queryKeyPrefixForGETRPC(rpc string, endpoints []normalizer.Endpoint) []string {
	rpc = strings.TrimSpace(rpc)
	for _, ep := range endpoints {
		if strings.TrimSpace(ep.RPC) != rpc {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
			continue
		}
		svc := strings.TrimSpace(ep.ServiceName)
		if svc == "" {
			continue
		}
		return []string{svc, ep.RPC}
	}
	return nil
}

func dedupePrefixSlices(in [][]string) [][]string {
	seen := make(map[string]struct{})
	var out [][]string
	for _, p := range in {
		key := strings.Join(p, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

func buildWSInvalidateRules(events []normalizer.EventDef, endpoints []normalizer.Endpoint) []WSInvalidateRule {
	svcGET := serviceNamesWithGET(endpoints)
	svcSet := make(map[string]struct{}, len(svcGET))
	for _, s := range svcGET {
		svcSet[s] = struct{}{}
	}
	out := make([]WSInvalidateRule, 0, len(events))
	for _, ev := range events {
		name := ev.Name
		hasWS := metadataHasFrontendWS(ev.Metadata)
		owner := strings.TrimSpace(ev.Owner)
		hasCons := len(ev.Consumers) > 0

		if !hasWS && owner == "" && !hasCons {
			out = append(out, WSInvalidateRule{EventName: name, Omit: true})
			continue
		}

		if metadataBool(ev.Metadata, "frontend", "ws", "noop") {
			out = append(out, WSInvalidateRule{EventName: name, Noop: true})
			continue
		}

		if metadataBool(ev.Metadata, "frontend", "ws", "invalidateAll") {
			var prefixes [][]string
			for _, s := range svcGET {
				prefixes = append(prefixes, []string{s})
			}
			out = append(out, WSInvalidateRule{EventName: name, Prefixes: dedupePrefixSlices(prefixes)})
			continue
		}

		var prefixes [][]string
		if rpcs := metadataStringSlice(ev.Metadata, "frontend", "ws", "invalidateRPCs"); len(rpcs) > 0 {
			for _, rpc := range rpcs {
				if p := queryKeyPrefixForGETRPC(rpc, endpoints); len(p) == 2 {
					prefixes = append(prefixes, p)
				}
			}
			prefixes = dedupePrefixSlices(prefixes)
			if len(prefixes) == 0 {
				out = append(out, WSInvalidateRule{EventName: name, Noop: true})
			} else {
				out = append(out, WSInvalidateRule{EventName: name, Prefixes: prefixes})
			}
			continue
		}
		if svcs := metadataStringSlice(ev.Metadata, "frontend", "ws", "invalidateServices"); len(svcs) > 0 {
			for _, s := range svcs {
				s = strings.TrimSpace(s)
				if s != "" {
					prefixes = append(prefixes, []string{s})
				}
			}
			prefixes = dedupePrefixSlices(prefixes)
			if len(prefixes) == 0 {
				out = append(out, WSInvalidateRule{EventName: name, Noop: true})
			} else {
				out = append(out, WSInvalidateRule{EventName: name, Prefixes: prefixes})
			}
			continue
		}
		if owner != "" {
			out = append(out, WSInvalidateRule{EventName: name, Prefixes: [][]string{{owner}}})
			continue
		}
		for _, c := range ev.Consumers {
			c = strings.TrimSpace(c)
			if _, ok := svcSet[c]; ok {
				prefixes = append(prefixes, []string{c})
			}
		}
		prefixes = dedupePrefixSlices(prefixes)
		if len(prefixes) > 0 {
			out = append(out, WSInvalidateRule{EventName: name, Prefixes: prefixes})
			continue
		}
		out = append(out, WSInvalidateRule{EventName: name, Noop: true})
	}
	return out
}

func endpointQueryProfile(ep normalizer.Endpoint) string {
	if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
		return ""
	}
	// Aliases for cache/query behavior (documented for frontend intent).
	if v := metadataString(ep.Metadata, "client", "query", "profile"); v != "" {
		return v
	}
	if v := metadataString(ep.Metadata, "frontend", "queryProfile"); v != "" {
		return v
	}
	if v := metadataString(ep.Metadata, "frontend", "cacheProfile"); v != "" {
		return v
	}
	if v := metadataString(ep.Metadata, "cacheProfile"); v != "" {
		return v
	}
	return metadataString(ep.Metadata, "queryProfile")
}

func serviceNamesWithGET(endpoints []normalizer.Endpoint) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, ep := range endpoints {
		if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
			continue
		}
		svc := strings.TrimSpace(ep.ServiceName)
		if svc == "" {
			continue
		}
		if _, ok := seen[svc]; ok {
			continue
		}
		seen[svc] = struct{}{}
		out = append(out, svc)
	}
	sort.Strings(out)
	return out
}

func endpointCachePolicy(ep normalizer.Endpoint) string {
	if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
		return ""
	}
	if v := metadataString(ep.Metadata, "frontend", "cachePolicy"); v != "" {
		return v
	}
	return metadataString(ep.Metadata, "cachePolicy")
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
				entry.r.ListCacheTTL = ep.CacheTTL
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
				entry.r.DetailCacheTTL = ep.CacheTTL
				entry.r.EntityName = deriveEntityName(ep.RPC)
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

func wsStaticPathPrefix(path string) string {
	if idx := strings.Index(path, "{"); idx >= 0 {
		path = path[:idx]
	}
	return strings.TrimRight(path, "/")
}

func hasDynamicRoomSiblings(ep normalizer.Endpoint, endpoints []normalizer.Endpoint) bool {
	if strings.ToUpper(strings.TrimSpace(ep.Method)) != "WS" {
		return false
	}
	if len(pathParams(ep.Path)) != 0 {
		return false
	}
	base := wsStaticPathPrefix(ep.Path)
	if base == "" {
		return false
	}
	for _, other := range endpoints {
		if other.ServiceName != ep.ServiceName || other.RPC == ep.RPC || strings.ToUpper(strings.TrimSpace(other.Method)) != "WS" {
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

func isPrimaryAppWSEndpoint(ep normalizer.Endpoint) bool {
	if strings.ToUpper(strings.TrimSpace(ep.Method)) != "WS" {
		return false
	}
	path := strings.TrimSpace(ep.Path)
	return path == "/ws/app" || path == "ws/app"
}

func primaryAppWSEndpoint(endpoints []normalizer.Endpoint) (normalizer.Endpoint, bool) {
	for _, ep := range endpoints {
		if isPrimaryAppWSEndpoint(ep) {
			return ep, true
		}
	}
	return normalizer.Endpoint{}, false
}

func isPathParamSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

type frontendInvalidateTarget struct {
	Store      string
	Service    string
	RPC        string
	ScopeParam string
	Mode       string
}

func inferStoreFromName(name string, entities []normalizer.Entity) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}
	for _, prefix := range []string{"Create", "Delete", "Update", "Patch", "Remove", "List", "Get"} {
		if strings.HasPrefix(n, prefix) {
			n = strings.TrimSpace(strings.TrimPrefix(n, prefix))
			break
		}
	}
	for _, ent := range entities {
		if strings.HasPrefix(strings.ToLower(n), strings.ToLower(ent.Name)) {
			return strings.ToLower(ent.Name)
		}
	}
	return ""
}

func inferStoreFromPath(path string) string {
	segs := pathSegments(path)
	for _, seg := range segs {
		if isPathParamSegment(seg) {
			continue
		}
		if seg == "" {
			continue
		}
		key := strings.TrimSpace(seg)
		if strings.HasSuffix(key, "s") && len(key) > 1 {
			key = key[:len(key)-1]
		}
		return strings.ToLower(key)
	}
	return ""
}

func firstPathParamName(path string) string {
	for _, seg := range pathSegments(path) {
		if isPathParamSegment(seg) {
			return strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
		}
	}
	return ""
}

func endpointMode(ep normalizer.Endpoint) string {
	if strings.ToUpper(ep.Method) != "GET" {
		return "list"
	}
	if firstPathParamName(ep.Path) != "" {
		return "detail"
	}
	return "list"
}

func mutationInvalidateTargetsForEndpoint(ep normalizer.Endpoint, all []normalizer.Endpoint, entities []normalizer.Entity) []frontendInvalidateTarget {
	method := strings.ToUpper(strings.TrimSpace(ep.Method))
	if method == "" || method == "GET" || method == "WS" {
		return nil
	}

	seen := make(map[string]struct{})
	var out []frontendInvalidateTarget
	add := func(t frontendInvalidateTarget) {
		if t.Store == "" {
			return
		}
		key := t.Store + "|" + t.Service + "|" + t.RPC + "|" + t.ScopeParam + "|" + t.Mode
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}

	for _, rpc := range ep.Invalidate {
		rpc = strings.TrimSpace(rpc)
		if rpc == "" {
			continue
		}
		found := false
		for _, target := range all {
			if target.RPC != rpc {
				continue
			}
			store := inferStoreFromName(target.RPC, entities)
			if store == "" {
				store = inferStoreFromPath(target.Path)
			}
			add(frontendInvalidateTarget{
				Store:      store,
				Service:    target.ServiceName,
				RPC:        target.RPC,
				ScopeParam: firstPathParamName(target.Path),
				Mode:       endpointMode(target),
			})
			found = true
		}
		if !found {
			add(frontendInvalidateTarget{
				Store: inferStoreFromName(rpc, entities),
				Mode:  "list",
			})
		}
	}

	store := inferStoreFromName(ep.RPC, entities)
	if store == "" {
		store = inferStoreFromPath(ep.Path)
	}
	add(frontendInvalidateTarget{Store: store, Mode: "list"})

	sort.Slice(out, func(i, j int) bool {
		if out[i].Store != out[j].Store {
			return out[i].Store < out[j].Store
		}
		if out[i].ScopeParam != out[j].ScopeParam {
			return out[i].ScopeParam < out[j].ScopeParam
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// EmitFrontendSDK generates the React SDK (TS, Zod, React Query).
func (e *Emitter) EmitFrontendSDK(entities []ir.Entity, services []ir.Service, endpoints []ir.Endpoint, events []ir.Event, errors []ir.Error, rbac *normalizer.RBACDef) error {
	entitiesNorm := IREntitiesToNormalizer(entities)
	servicesNorm := IRServicesToNormalizer(services)
	endpointsNorm := IREndpointsToNormalizer(endpoints)
	eventsNorm := IREventsToNormalizer(events)
	errorsNorm := IRErrorsToNormalizer(errors)

	servicesNorm, entitiesNorm = applyFrontendAuthInjectFilters(servicesNorm, entitiesNorm, endpointsNorm)

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

	namedEnums := extractFrontendNamedEnums(resolveFrontendProjectRoot(e.OutputDir, e.FrontendDir))
	namedEnumSet := namedEnumNameSet(namedEnums)

	ctx := FrontendContext{
		Entities:               entitiesNorm,
		NamedEnums:             namedEnums,
		Services:               servicesNorm,
		Endpoints:              endpointsNorm,
		Events:                 eventsNorm,
		Errors:                 uniqueErrors,
		RBAC:                   rbac,
		QueryResources:         queryResources,
		QueryKeysNeedsTypes:    queryKeysNeedsTypes,
		QueryOptionsNeedsTypes: queryOptionsNeedsTypes,
		WSInvalidateRules:      buildWSInvalidateRules(eventsNorm, endpointsNorm),
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
	if err := os.MkdirAll(filepath.Join(targetDir, "adapters"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "app-hooks"), 0755); err != nil {
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
		"TrimSpace":  strings.TrimSpace,
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
			if enumName := matchFieldNamedEnum(f, namedEnums, namedEnumSet); enumName != "" {
				base = enumName
				if f.IsList || strings.HasPrefix(strings.TrimSpace(f.Type), "[]") {
					base += "[]"
				}
			}
			if f.Metadata != nil && f.Metadata["client_side_encryption"] == true {
				return "Encrypted<" + base + ">"
			}
			return base
		},
		"ZodFieldType": func(f normalizer.Field) string {
			if enumName := matchFieldNamedEnum(f, namedEnums, namedEnumSet); enumName != "" {
				if f.IsList || strings.HasPrefix(strings.TrimSpace(f.Type), "[]") {
					return fmt.Sprintf("z.array(%sSchema)", enumName)
				}
				return fmt.Sprintf("%sSchema", enumName)
			}
			goType := f.Type
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
		"TSEnumKey": tsEnumKey,
		"IsNamedEnumEntity": func(name string) bool {
			_, ok := namedEnumSet[strings.TrimSpace(name)]
			return ok
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
		"HasEvent": func(name string) bool {
			for _, ev := range eventsNorm {
				if strings.EqualFold(strings.TrimSpace(ev.Name), strings.TrimSpace(name)) {
					return true
				}
			}
			return false
		},
		"HasGETRPC": func(rpc string) bool {
			rpc = strings.TrimSpace(rpc)
			for _, ep := range endpointsNorm {
				if strings.TrimSpace(ep.RPC) != rpc {
					continue
				}
				if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
					continue
				}
				return true
			}
			return false
		},
		"EntityHasID": func(entity normalizer.Entity) bool {
			return entityIDField(entity) != nil
		},
		"EntityIDFieldIsSecret": func(entity normalizer.Entity) bool {
			field := entityIDField(entity)
			return field != nil && field.IsSecret
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
		"HasDynamicRoomSiblings": func(ep normalizer.Endpoint) bool {
			return hasDynamicRoomSiblings(ep, endpointsNorm)
		},
		"HasPrimaryAppWSEndpoint": func() bool {
			_, ok := primaryAppWSEndpoint(endpointsNorm)
			return ok
		},
		"PrimaryAppWSRPC": func() string {
			if ep, ok := primaryAppWSEndpoint(endpointsNorm); ok {
				return ep.RPC
			}
			return ""
		},
		"IsPrimaryAppWSEndpoint": func(ep normalizer.Endpoint) bool {
			return isPrimaryAppWSEndpoint(ep)
		},
		"HasEndpointQueryProfiles": func() bool {
			for _, ep := range endpointsNorm {
				if endpointQueryProfile(ep) != "" {
					return true
				}
			}
			return false
		},
		"ServiceNameForGETRPC": func(rpc string) string {
			rpc = strings.TrimSpace(rpc)
			for _, ep := range endpointsNorm {
				if ep.RPC != rpc {
					continue
				}
				if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
					continue
				}
				return ep.ServiceName
			}
			return ""
		},
		"InvalidateGETEndpointTuple": func(rpc string) string {
			rpc = strings.TrimSpace(rpc)
			for _, ep := range endpointsNorm {
				if ep.RPC != rpc {
					continue
				}
				if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
					continue
				}
				return fmt.Sprintf("['%s', '%s'] as const", ep.ServiceName, ep.RPC)
			}
			return ""
		},
		"DetailPlaceholderListPrefixTuple": func(detailRPC string) string {
			detailRPC = strings.TrimSpace(detailRPC)
			res, ok := queryOptionsByRPC[detailRPC]
			if !ok || !res.HasList || strings.TrimSpace(res.ListRPC) == "" {
				return ""
			}
			listRPC := strings.TrimSpace(res.ListRPC)
			for _, ep := range endpointsNorm {
				if ep.RPC != listRPC {
					continue
				}
				if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
					continue
				}
				return fmt.Sprintf("['%s', '%s'] as const", ep.ServiceName, ep.RPC)
			}
			return ""
		},
		"HasGETEndpoints": func() bool {
			for _, ep := range endpointsNorm {
				if strings.ToUpper(strings.TrimSpace(ep.Method)) == "GET" {
					return true
				}
			}
			return false
		},
		"GETServiceNames": func() []string {
			return serviceNamesWithGET(endpointsNorm)
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
		"MutationInvalidateTargets": func(ep normalizer.Endpoint) []frontendInvalidateTarget {
			return mutationInvalidateTargetsForEndpoint(ep, endpointsNorm, entitiesNorm)
		},
		"EndpointPolicy": func(ep normalizer.Endpoint) policy.EndpointPolicy {
			return policy.FromEndpoint(ep)
		},
		"EndpointQueryProfile": func(ep normalizer.Endpoint) string {
			return endpointQueryProfile(ep)
		},
		"EndpointCachePolicy": func(ep normalizer.Endpoint) string {
			return endpointCachePolicy(ep)
		},
		"HasStreamingEndpoints": func() bool {
			for _, ep := range endpointsNorm {
				if strings.ToUpper(strings.TrimSpace(ep.Method)) == "WS" {
					continue
				}
				if ep.IsStreaming {
					return true
				}
			}
			return false
		},
		"QueryResourceKeyForRPC": func(rpc string) string {
			if res, ok := queryOptionsByRPC[rpc]; ok {
				return res.Key
			}
			return ""
		},
		"QueryResourceKindForRPC": func(rpc string) string {
			return queryOptionsKindByRPC[rpc]
		},
		"IsAuthLoginRPC": func(rpc string) bool {
			return rpc == "LoginUser" || rpc == "RegisterUser"
		},
		"Title": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"CacheTTLToMs": func(ttl string) int64 {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				return 0
			}
			return d.Milliseconds()
		},
		"UniqueQueryResourceStoreImports": func(resources []QueryResource) []string {
			seen := map[string]bool{}
			var stores []string
			for _, r := range resources {
				if !r.HasList || !r.HasDetail || r.EntityName == "" {
					continue
				}
				storeName := "use" + r.EntityName + "Store"
				if !seen[storeName] {
					seen[storeName] = true
					stores = append(stores, storeName)
				}
			}
			sort.Strings(stores)
			return stores
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
		{"query-keys-reexport", "queries/keys.ts"},
		{"websocket-hooks", "hooks/websocket-hooks.ts"},
		{"adapters-index", "adapters/index.ts"},
		{"app-hooks-index", "app-hooks/index.ts"},
		{"routes", "routes.ts"},
		{"handlers", "mocks/handlers.ts"},
		{"msw-server", "mocks/server.ts"},
		{"forms", "forms/index.ts"},
		{"format", "format.ts"},
		{"gdpr-policy", "gdpr-policy.ts"},
		{"optimistic-hooks", "hooks/optimistic-hooks.ts"},
		{"app-router", "app-router.ts"},
		{"error-boundaries", "error-boundaries.tsx"},
		{"suspense-boundaries", "suspense-boundaries.tsx"},
		{"a11y", "a11y.ts"},
		{"cookie-banner", "cookie-banner.tsx"},
	}

	for _, f := range files {
		if err := e.emitFrontendFile(f.tmpl, ctx, funcMap, f.out); err != nil {
			return err
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

	// Per-entity Zustand store files (stores/apikey.ts, stores/tender.ts, …)
	for _, ent := range entitiesNorm {
		if entityIDField(ent) == nil {
			continue
		}
		outName := "stores/" + strings.ToLower(ent.Name) + ".ts"
		if err := e.emitFrontendFile("store-item", StoreItemContext{Entity: ent}, funcMap, outName); err != nil {
			return err
		}
	}

	if err := e.EmitSDKManifest(endpointsNorm); err != nil {
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
	if _, err := ReadTemplateByPath(tmplPath); err != nil {
		if strings.HasSuffix(outName, ".tsx") {
			tmplPath = fmt.Sprintf("templates/frontend/%s.tsx.tmpl", tmplName)
		} else {
			return err
		}
	}
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
