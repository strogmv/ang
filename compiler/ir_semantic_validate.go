package compiler

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/strogmv/ang/compiler/ir"
)

var templateVarPathRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
var uiComponentNameRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
var uiImportPathRE = regexp.MustCompile(`^(@?[A-Za-z0-9._-]+)(/[A-Za-z0-9._@-]+)*$`)

// ValidateIRSemantics performs fail-fast semantic validation on IR before emitters run.
func ValidateIRSemantics(schema *ir.Schema) error {
	if schema == nil {
		return fmt.Errorf("ir schema is nil")
	}

	var errs []string
	entityByName := make(map[string]ir.Entity, len(schema.Entities))
	for _, ent := range schema.Entities {
		entityByName[ent.Name] = ent
	}
	serviceByName := make(map[string]ir.Service, len(schema.Services))
	for _, svc := range schema.Services {
		serviceByName[svc.Name] = svc
	}

	// 1) Entity type references and type map integrity.
	for _, ent := range schema.Entities {
		for _, f := range ent.Fields {
			validateTypeRef(&errs, entityByName, fmt.Sprintf("entity %s field %s", ent.Name, f.Name), f.Type)
			validateFieldUISemantics(&errs, fmt.Sprintf("entity %s field %s", ent.Name, f.Name), f)
		}
	}

	// 2) Service/method semantic references.
	for _, svc := range schema.Services {
		for _, m := range svc.Methods {
			if m.Input != nil {
				for _, f := range m.Input.Fields {
					validateTypeRef(&errs, entityByName, fmt.Sprintf("service %s method %s input field %s", svc.Name, m.Name, f.Name), f.Type)
					validateFieldUISemantics(&errs, fmt.Sprintf("service %s method %s input field %s", svc.Name, m.Name, f.Name), f)
				}
			}
			if m.Output != nil {
				for _, f := range m.Output.Fields {
					validateTypeRef(&errs, entityByName, fmt.Sprintf("service %s method %s output field %s", svc.Name, m.Name, f.Name), f.Type)
					validateFieldUISemantics(&errs, fmt.Sprintf("service %s method %s output field %s", svc.Name, m.Name, f.Name), f)
				}
			}
			for _, src := range m.Sources {
				if strings.TrimSpace(src.Entity) == "" {
					continue
				}
				if _, ok := entityByName[src.Entity]; !ok {
					errs = append(errs, fmt.Sprintf("service %s method %s source references unknown entity %q", svc.Name, m.Name, src.Entity))
				}
			}
		}
	}

	// 3) Repository/finder semantic references.
	for _, repo := range schema.Repos {
		ent, ok := entityByName[repo.Entity]
		if !ok {
			errs = append(errs, fmt.Sprintf("repository %s references unknown entity %q", repo.Name, repo.Entity))
			continue
		}
		fieldSet := make(map[string]bool, len(ent.Fields))
		for _, f := range ent.Fields {
			fieldSet[normalizeFinderFieldKey(f.Name)] = true
		}
		for _, finder := range repo.Finders {
			for _, w := range finder.Where {
				if strings.TrimSpace(w.Field) == "" {
					continue
				}
				if !fieldSet[normalizeFinderFieldKey(w.Field)] {
					errs = append(errs, fmt.Sprintf("repository %s finder %s where field %q does not exist on entity %s", repo.Name, finder.Name, w.Field, repo.Entity))
				}
			}
			for _, col := range finder.Select {
				if strings.TrimSpace(col) == "" {
					continue
				}
				if !fieldSet[normalizeFinderFieldKey(col)] {
					errs = append(errs, fmt.Sprintf("repository %s finder %s select field %q does not exist on entity %s", repo.Name, finder.Name, col, repo.Entity))
				}
			}
		}
	}

	// 4) Endpoint RPC/service references.
	for _, ep := range schema.Endpoints {
		svc, ok := serviceByName[ep.Service]
		if !ok {
			errs = append(errs, fmt.Sprintf("endpoint %s %s references unknown service %q", ep.Method, ep.Path, ep.Service))
			continue
		}
		foundMethod := false
		for _, m := range svc.Methods {
			if m.Name == ep.RPC {
				foundMethod = true
				break
			}
		}
		if !foundMethod {
			errs = append(errs, fmt.Sprintf("endpoint %s %s references unknown RPC %q on service %s", ep.Method, ep.Path, ep.RPC, ep.Service))
		}
	}

	// 5) Cycle detection in service dependencies.
	if cycleErr := validateServiceDependencyCycles(schema.Services); cycleErr != nil {
		errs = append(errs, cycleErr.Error())
	}
	// 6) Template catalog semantic checks.
	validateTemplateCatalog(schema, &errs)
	// 7) Notification template reference integrity against schema template catalog.
	validateNotificationTemplateRefs(schema, &errs)

	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("ir semantic validation failed:\n - %s", strings.Join(errs, "\n - "))
	}
	return nil
}

func validateTemplateCatalog(schema *ir.Schema, errs *[]string) {
	if schema == nil || len(schema.Templates) == 0 {
		return
	}
	seen := map[string]bool{}
	for i, t := range schema.Templates {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			*errs = append(*errs, fmt.Sprintf("template[%d] has empty id", i))
			continue
		}
		if seen[id] {
			*errs = append(*errs, fmt.Sprintf("template %q is duplicated", id))
		}
		seen[id] = true

		engine := strings.TrimSpace(strings.ToLower(t.Engine))
		if engine == "" {
			engine = "go_template"
		}
		if !isSupportedTemplateEngine(engine) {
			*errs = append(*errs, fmt.Sprintf("template %q uses unsupported engine %q", id, t.Engine))
		}

		channel := strings.TrimSpace(strings.ToLower(t.Channel))
		if !templateEngineCompatibleWithChannel(engine, channel) {
			*errs = append(*errs, fmt.Sprintf("template %q uses engine %q incompatible with channel %q", id, engine, channel))
		}

		subject := strings.TrimSpace(t.Subject)
		text := strings.TrimSpace(t.Text)
		html := strings.TrimSpace(t.HTML)
		body := strings.TrimSpace(t.Body)
		if strings.EqualFold(channel, "email") || strings.EqualFold(strings.TrimSpace(t.Kind), "email") {
			if subject == "" {
				*errs = append(*errs, fmt.Sprintf("email template %q requires non-empty subject", id))
			}
			if text == "" && html == "" && body == "" {
				*errs = append(*errs, fmt.Sprintf("email template %q requires text/html/body content", id))
			}
		} else if text == "" && html == "" && body == "" {
			*errs = append(*errs, fmt.Sprintf("template %q requires at least one content field: text/html/body", id))
		}

		requiredVars := normalizeTemplateVars(t.RequiredVars)
		optionalVars := normalizeTemplateVars(t.OptionalVars)
		validateTemplateVarLists(id, requiredVars, optionalVars, errs)

		if strings.EqualFold(engine, "go_template") {
			used, err := collectTemplateVarPaths(subject, text, html, body)
			if err != nil {
				*errs = append(*errs, fmt.Sprintf("template %q parse error: %v", id, err))
				continue
			}
			for _, rv := range requiredVars {
				if !isTemplateVarUsed(rv, used) {
					*errs = append(*errs, fmt.Sprintf("template %q requiredVars contains %q but template content does not reference it", id, rv))
				}
			}
		}
	}
}

func validateTemplateVarLists(id string, required, optional []string, errs *[]string) {
	seenReq := map[string]bool{}
	for _, v := range required {
		if !templateVarPathRE.MatchString(v) {
			*errs = append(*errs, fmt.Sprintf("template %q has invalid requiredVars name %q", id, v))
			continue
		}
		if seenReq[v] {
			*errs = append(*errs, fmt.Sprintf("template %q has duplicate requiredVars %q", id, v))
		}
		seenReq[v] = true
	}
	seenOpt := map[string]bool{}
	for _, v := range optional {
		if !templateVarPathRE.MatchString(v) {
			*errs = append(*errs, fmt.Sprintf("template %q has invalid optionalVars name %q", id, v))
			continue
		}
		if seenOpt[v] {
			*errs = append(*errs, fmt.Sprintf("template %q has duplicate optionalVars %q", id, v))
		}
		if seenReq[v] {
			*errs = append(*errs, fmt.Sprintf("template %q declares var %q in both requiredVars and optionalVars", id, v))
		}
		seenOpt[v] = true
	}
}

func normalizeTemplateVars(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectTemplateVarPaths(content ...string) (map[string]bool, error) {
	used := map[string]bool{}
	for i, c := range content {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		t, err := template.New(fmt.Sprintf("template_%d", i)).Option("missingkey=zero").Parse(s)
		if err != nil {
			return nil, err
		}
		for _, tmpl := range t.Templates() {
			collectTemplateVarsNode(tmpl.Tree.Root, used)
		}
	}
	return used, nil
}

func collectTemplateVarsNode(n parse.Node, used map[string]bool) {
	if n == nil {
		return
	}
	switch x := n.(type) {
	case *parse.ListNode:
		for _, node := range x.Nodes {
			collectTemplateVarsNode(node, used)
		}
	case *parse.ActionNode:
		collectTemplateVarsPipe(x.Pipe, used)
	case *parse.RangeNode:
		collectTemplateVarsPipe(x.Pipe, used)
		collectTemplateVarsNode(x.List, used)
		collectTemplateVarsNode(x.ElseList, used)
	case *parse.IfNode:
		collectTemplateVarsPipe(x.Pipe, used)
		collectTemplateVarsNode(x.List, used)
		collectTemplateVarsNode(x.ElseList, used)
	case *parse.WithNode:
		collectTemplateVarsPipe(x.Pipe, used)
		collectTemplateVarsNode(x.List, used)
		collectTemplateVarsNode(x.ElseList, used)
	case *parse.TemplateNode:
		collectTemplateVarsPipe(x.Pipe, used)
	}
}

func collectTemplateVarsPipe(p *parse.PipeNode, used map[string]bool) {
	if p == nil {
		return
	}
	for _, cmd := range p.Cmds {
		for _, arg := range cmd.Args {
			field, ok := arg.(*parse.FieldNode)
			if !ok || len(field.Ident) == 0 {
				continue
			}
			used[strings.Join(field.Ident, ".")] = true
		}
	}
}

func isTemplateVarUsed(required string, used map[string]bool) bool {
	if used[required] {
		return true
	}
	prefix := required + "."
	for k := range used {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func isSupportedTemplateEngine(engine string) bool {
	switch strings.TrimSpace(strings.ToLower(engine)) {
	case "go_template", "plain", "json":
		return true
	default:
		return false
	}
}

func templateEngineCompatibleWithChannel(engine, channel string) bool {
	e := strings.TrimSpace(strings.ToLower(engine))
	c := strings.TrimSpace(strings.ToLower(channel))
	if e == "" || c == "" {
		return true
	}
	switch c {
	case "email", "in_app":
		return e == "go_template" || e == "plain"
	case "nats", "kafka", "webhook":
		return e == "go_template" || e == "plain" || e == "json"
	default:
		return true
	}
}

func validateNotificationTemplateRefs(schema *ir.Schema, errs *[]string) {
	if schema == nil || schema.Notifications == nil || len(schema.Templates) == 0 {
		return
	}
	templateSet := make(map[string]bool, len(schema.Templates))
	templateByID := make(map[string]ir.Template, len(schema.Templates))
	for _, t := range schema.Templates {
		id := strings.TrimSpace(t.ID)
		if id != "" {
			templateSet[id] = true
			templateByID[id] = t
		}
	}
	if len(templateSet) == 0 {
		return
	}

	if schema.Notifications.Channels != nil {
		for chName, spec := range schema.Notifications.Channels.Channels {
			tpl := strings.TrimSpace(spec.Template)
			if tpl == "" {
				continue
			}
			if !templateSet[tpl] {
				*errs = append(*errs, fmt.Sprintf("notifications channel %q references unknown template %q", chName, tpl))
				continue
			}
			tmpl := templateByID[tpl]
			if !templateChannelCompatible(tmpl.Channel, chName) {
				*errs = append(*errs, fmt.Sprintf("notifications channel %q uses template %q with incompatible channel %q", chName, tpl, tmpl.Channel))
			}
		}
	}
	if schema.Notifications.Policies != nil {
		for i, rule := range schema.Notifications.Policies.Rules {
			tpl := strings.TrimSpace(rule.Template)
			if tpl == "" {
				continue
			}
			if !templateSet[tpl] {
				*errs = append(*errs, fmt.Sprintf("notifications policy rule[%d] references unknown template %q", i, tpl))
				continue
			}
			tmpl := templateByID[tpl]
			channelsToCheck := make([]string, 0, len(rule.Channels))
			for _, ch := range rule.Channels {
				if c := strings.TrimSpace(ch); c != "" {
					channelsToCheck = append(channelsToCheck, c)
				}
			}
			if len(channelsToCheck) == 0 && schema.Notifications.Channels != nil {
				for _, ch := range schema.Notifications.Channels.DefaultChannels {
					if c := strings.TrimSpace(ch); c != "" {
						channelsToCheck = append(channelsToCheck, c)
					}
				}
			}
			for _, ch := range channelsToCheck {
				if !templateChannelCompatible(tmpl.Channel, ch) {
					*errs = append(*errs, fmt.Sprintf("notifications policy rule[%d] channel %q uses template %q with incompatible channel %q", i, ch, tpl, tmpl.Channel))
				}
			}
		}
	}
}

func templateChannelCompatible(templateChannel, usageChannel string) bool {
	tplCh := strings.TrimSpace(strings.ToLower(templateChannel))
	useCh := strings.TrimSpace(strings.ToLower(usageChannel))
	if tplCh == "" || useCh == "" {
		return true
	}
	return tplCh == useCh
}

func normalizeFinderFieldKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func validateFieldUISemantics(errs *[]string, where string, field ir.Field) {
	uiType := strings.TrimSpace(strings.ToLower(field.UI.Type))
	if uiType != "" && !isSupportedUIType(uiType) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_UNKNOWN_TYPE] %s uses unknown ui.type %q", where, field.UI.Type))
	}

	component := strings.TrimSpace(field.UI.Component)
	source := strings.TrimSpace(field.UI.Source)
	if uiType == "custom" && component == "" {
		*errs = append(*errs, fmt.Sprintf("[E_UI_CUSTOM_COMPONENT_REQUIRED] %s uses ui.type=custom but ui.component is empty", where))
	}
	if component != "" && !uiComponentNameRE.MatchString(component) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_INVALID_COMPONENT] %s has invalid ui.component %q", where, component))
	}
	// Source path validation is strict only for custom fields.
	if uiType == "custom" && source != "" && !uiImportPathRE.MatchString(source) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_INVALID_SOURCE] %s has invalid ui.source %q", where, source))
	}
	if uiType == "select" && len(field.UI.Options) == 0 && source == "" {
		*errs = append(*errs, fmt.Sprintf("[E_UI_SELECT_SOURCE_OR_OPTIONS_REQUIRED] %s uses ui.type=select but neither ui.options nor ui.source is set", where))
	}
	if field.UI.Hidden && fieldIsRequired(field) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_HIDDEN_REQUIRED_CONFLICT] %s is hidden but required", where))
	}
	if field.UI.Columns < 0 {
		*errs = append(*errs, fmt.Sprintf("[E_UI_COLUMNS_INVALID] %s has negative ui.columns=%d", where, field.UI.Columns))
	}
	importance := strings.TrimSpace(strings.ToLower(field.UI.Importance))
	if importance != "" && !isSupportedUIImportance(importance) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_IMPORTANCE_INVALID] %s has unsupported ui.importance %q", where, field.UI.Importance))
	}
	inputKind := strings.TrimSpace(strings.ToLower(field.UI.InputKind))
	if inputKind != "" && !isSupportedUIInputKind(inputKind) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_INPUT_KIND_INVALID] %s has unsupported ui.inputKind %q", where, field.UI.InputKind))
	}
	intent := strings.TrimSpace(strings.ToLower(field.UI.Intent))
	if intent != "" && !isSupportedUIIntent(intent) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_INTENT_INVALID] %s has unsupported ui.intent %q", where, field.UI.Intent))
	}
	density := strings.TrimSpace(strings.ToLower(field.UI.Density))
	if density != "" && !isSupportedUIDensity(density) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_DENSITY_INVALID] %s has unsupported ui.density %q", where, field.UI.Density))
	}
	labelMode := strings.TrimSpace(strings.ToLower(field.UI.LabelMode))
	if labelMode != "" && !isSupportedUILabelMode(labelMode) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_LABEL_MODE_INVALID] %s has unsupported ui.labelMode %q", where, field.UI.LabelMode))
	}
	surface := strings.TrimSpace(strings.ToLower(field.UI.Surface))
	if surface != "" && !isSupportedUISurface(surface) {
		*errs = append(*errs, fmt.Sprintf("[E_UI_SURFACE_INVALID] %s has unsupported ui.surface %q", where, field.UI.Surface))
	}
}

func isSupportedUIType(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "text", "textarea", "number", "currency", "email", "password", "phone", "url",
		"date", "datetime", "time", "select", "autocomplete", "checkbox", "switch",
		"file", "image", "custom":
		return true
	default:
		return false
	}
}

func isSupportedUIImportance(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "high", "normal", "low":
		return true
	default:
		return false
	}
}

func isSupportedUIInputKind(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "sensitive", "email", "phone", "money", "search", "none":
		return true
	default:
		return false
	}
}

func isSupportedUIIntent(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "danger", "warning", "success", "info", "neutral":
		return true
	default:
		return false
	}
}

func isSupportedUIDensity(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "compact", "normal", "spacious":
		return true
	default:
		return false
	}
}

func isSupportedUILabelMode(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "static", "floating", "hidden":
		return true
	default:
		return false
	}
}

func isSupportedUISurface(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "paper", "flat", "raised":
		return true
	default:
		return false
	}
}

func fieldIsRequired(field ir.Field) bool {
	if !field.Optional {
		return true
	}
	tag := strings.TrimSpace(field.ValidateTag)
	if tag == "" {
		return false
	}
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), "required") {
			return true
		}
	}
	return false
}

func validateTypeRef(errs *[]string, entities map[string]ir.Entity, where string, ref ir.TypeRef) {
	switch ref.Kind {
	case ir.KindString, ir.KindInt, ir.KindInt64, ir.KindFloat, ir.KindBool, ir.KindTime, ir.KindUUID, ir.KindJSON, ir.KindEnum, ir.KindFile, ir.KindAny:
		return
	case ir.KindEntity:
		if strings.TrimSpace(ref.Name) == "" {
			*errs = append(*errs, fmt.Sprintf("%s has entity type without name", where))
			return
		}
		if _, ok := entities[ref.Name]; !ok {
			*errs = append(*errs, fmt.Sprintf("%s references unknown entity type %q", where, ref.Name))
		}
	case ir.KindList:
		if ref.ItemType == nil {
			*errs = append(*errs, fmt.Sprintf("%s list type has nil item type", where))
			return
		}
		validateTypeRef(errs, entities, where+"[]", *ref.ItemType)
	case ir.KindMap:
		if ref.KeyType == nil || ref.ItemType == nil {
			*errs = append(*errs, fmt.Sprintf("%s map type has nil key/item type", where))
			return
		}
		validateTypeRef(errs, entities, where+"<key>", *ref.KeyType)
		validateTypeRef(errs, entities, where+"<value>", *ref.ItemType)
	default:
		*errs = append(*errs, fmt.Sprintf("%s has unsupported type kind %q", where, ref.Kind))
	}
}

func validateServiceDependencyCycles(services []ir.Service) error {
	byName := make(map[string]ir.Service, len(services))
	for _, s := range services {
		byName[s.Name] = s
	}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var path []string

	var dfs func(string) error
	dfs = func(name string) error {
		if visiting[name] {
			idx := 0
			for i, p := range path {
				if p == name {
					idx = i
					break
				}
			}
			cycle := append(path[idx:], name)
			return fmt.Errorf("service dependency cycle detected: %s", strings.Join(cycle, " -> "))
		}
		if visited[name] {
			return nil
		}
		visited[name] = true
		visiting[name] = true
		path = append(path, name)

		svc, ok := byName[name]
		if ok {
			for _, dep := range svc.Uses {
				if _, exists := byName[dep]; !exists {
					continue
				}
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		path = path[:len(path)-1]
		visiting[name] = false
		return nil
	}

	for _, s := range services {
		if err := dfs(s.Name); err != nil {
			return err
		}
	}
	return nil
}
