package emitter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

var reFlowSelectorID = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9]*)Id\b`)
var reFlowTimeIsZero = regexp.MustCompile(`(\w+At)\s*==\s*""`)
var rePayloadTimeAt = regexp.MustCompile(`(\w+At):\s*([\w.]+At)\b`)

func normalizeFlowExpr(s string) string {
	if s == "" {
		return s
	}
	s = reFlowSelectorID.ReplaceAllString(s, ".$1ID")
	// time.Time fields can't be compared with ""; convert to .IsZero()
	s = reFlowTimeIsZero.ReplaceAllString(s, "$1.IsZero()")
	return s
}

// normalizePayloadExpr transforms event payload struct literals: XxxAt fields that
// reference time.Time domain variables are converted to .Format(time.RFC3339) so they
// satisfy the string type of the domain event struct fields.
func normalizePayloadExpr(s string) string {
	return rePayloadTimeAt.ReplaceAllString(s, "$1: $2.Format(time.RFC3339)")
}

// flowRenderable reports whether all actions inside steps are supported by RenderFlow.
func flowRenderable(steps []normalizer.FlowStep) bool {
	for _, step := range steps {
		if !flowActionSupported(step.Action) {
			return false
		}
		for _, child := range flowChildSteps(step) {
			if !flowRenderable(child) {
				return false
			}
		}
	}
	return true
}

func flowActionSupported(action string) bool {
	switch action {
	case "logic.Check",
		"repo.Find", "repo.Get", "repo.GetForUpdate", "repo.List", "repo.Save", "repo.Delete",
		"repo.Query", "repo.Upsert",
		"mapping.Assign", "mapping.Map",
		"flow.If", "flow.For", "flow.Block", "flow.Switch", "flow.While", "tx.Block",
		"flow.Checkpoint", "flow.Resume", "flow.Validate", "flow.Try", "flow.Catch",
		"flow.Retry", "flow.Fallback", "flow.Timeout", "flow.SuggestNext", "flow.ExplainError",
		"list.Filter", "list.Paginate", "list.Append", "list.Sort",
		"list.Enrich",
		"str.Normalize",
		"event.Publish", "logic.Call",
		"exec.Run",
		"fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove",
		"audit.Log",
		"auth.RequireRole", "auth.CheckRole",
		"entity.PatchNonZero", "entity.PatchValidated", "field.CopyNonEmpty",
		"enum.Validate",
		"time.Parse", "time.CheckExpiry",
		"map.Build",
		"fsm.Transition",
		"notification.Dispatch", "notify.Dispatch",
		"cache.Get", "cache.Set", "cache.Del",
		"mail.Send",
		"storage.Upload", "storage.Download", "storage.GetURL",
		"http.Call",
		"rand.Code", "rand.Token",
		"str.Format",
		"json.Parse", "json.Marshal",
		"parallel.Run",
		"pdf.Render",
		"webhook.Send",
		"queue.Enqueue":
		return true
	default:
		return false
	}
}

func flowChildSteps(step normalizer.FlowStep) [][]normalizer.FlowStep {
	var out [][]normalizer.FlowStep
	if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_catch"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_fallback"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_onTimeout"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if v, ok := step.Args["_onMissing"].([]normalizer.FlowStep); ok && len(v) > 0 {
		out = append(out, v)
	}
	if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(cases))
		for k := range cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(cases[k]) > 0 {
				out = append(out, cases[k])
			}
		}
	}
	if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
		keys := make([]string, 0, len(branches))
		for k := range branches {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(branches[k]) > 0 {
				out = append(out, branches[k])
			}
		}
	}
	return out
}

type flowRenderState struct {
	declared map[string]bool
	// pointers tracks whether a declared variable is a pointer type (from repo.Find/Get)
	// vs a value type (from mapping.Assign declare). Used to decide whether to pass &var to repo.Save.
	pointers      map[string]bool
	types         map[string]string // varName → Go type (e.g. "*domain.User", "[]domain.Order")
	goroutineMode bool              // if true, errors set _pErr/_mu instead of returning
	stepN         *int              // shared monotonic counter; unique suffix for internal temp vars
	returnErrOnly bool              // if true, errReturn emits `return err` for inner error closures
}

func cloneFlowState(st *flowRenderState) *flowRenderState {
	cp := &flowRenderState{
		declared:      make(map[string]bool, len(st.declared)),
		pointers:      make(map[string]bool, len(st.pointers)),
		types:         make(map[string]string, len(st.types)),
		goroutineMode: st.goroutineMode,
		stepN:         st.stepN, // share counter across all clones
		returnErrOnly: st.returnErrOnly,
	}
	for k, v := range st.declared {
		cp.declared[k] = v
	}
	for k, v := range st.pointers {
		cp.pointers[k] = v
	}
	for k, v := range st.types {
		cp.types[k] = v
	}
	return cp
}

func renderFlow(steps []normalizer.FlowStep) string {
	n := 0
	st := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	var b strings.Builder
	if flowHasAction(steps, "flow.Checkpoint", "flow.Resume") {
		st.declared["_flowCheckpoints"] = true
		st.pointers["_flowCheckpoints"] = false
		st.types["_flowCheckpoints"] = "map[string]any"
		b.WriteString("var _flowCheckpoints map[string]any\n")
	}
	if flowHasAction(steps, "flow.Try", "flow.Catch", "flow.Retry", "flow.Fallback", "flow.Timeout", "flow.ExplainError") {
		st.declared["_flowLastError"] = true
		st.pointers["_flowLastError"] = false
		st.types["_flowLastError"] = "error"
		b.WriteString("var _flowLastError error\n")
	}
	b.WriteString(renderFlowSteps(st, steps, 0))
	return b.String()
}

func flowHasAction(steps []normalizer.FlowStep, actions ...string) bool {
	need := make(map[string]struct{}, len(actions))
	for _, a := range actions {
		need[a] = struct{}{}
	}
	var walk func([]normalizer.FlowStep) bool
	walk = func(items []normalizer.FlowStep) bool {
		for _, s := range items {
			if _, ok := need[s.Action]; ok {
				return true
			}
			for _, child := range flowChildSteps(s) {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(steps)
}

func renderFlowSteps(st *flowRenderState, steps []normalizer.FlowStep, indent int) string {
	var b strings.Builder
	for _, step := range steps {
		b.WriteString(renderOneFlowStep(st, step, indent))
	}
	return b.String()
}

// errReturn generates the appropriate error-return code depending on context.
// In goroutine mode, errors are captured via mutex rather than returning from the outer func.
func errReturn(st *flowRenderState, pad, errExpr string) string {
	if st.goroutineMode {
		return fmt.Sprintf("%s_mu.Lock()\nif _pErr == nil { _pErr = %s; _pCancel() }\n%s_mu.Unlock()\n%sreturn\n", pad, errExpr, pad, pad)
	}
	if st.returnErrOnly {
		return fmt.Sprintf("%sreturn %s\n", pad, errExpr)
	}
	return fmt.Sprintf("%sreturn resp, %s\n", pad, errExpr)
}

func renderOneFlowStep(st *flowRenderState, step normalizer.FlowStep, indent int) string {
	pad := strings.Repeat("\t", indent)
	sfx := fmt.Sprintf("_%d", *st.stepN)
	*st.stepN++
	_ = sfx // consumed by actions with internal temp vars
	arg := func(name string) string {
		if v, ok := step.Args[name]; ok {
			if s, ok := v.(string); ok {
				return normalizeFlowExpr(strings.TrimSpace(s))
			}
		}
		return ""
	}
	child := func(name string) []normalizer.FlowStep {
		if v, ok := step.Args[name].([]normalizer.FlowStep); ok {
			return v
		}
		return nil
	}

	switch step.Action {
	case "logic.Check":
		cond := arg("condition")
		throw := arg("throw")
		if cond == "" {
			return ""
		}
		if throw == "" {
			throw = "validation failed"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"Validation Error\", %q)", throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "repo.Get", "repo.Find", "repo.GetForUpdate", "repo.List":
		source := arg("source")
		if source == "" {
			return ""
		}
		method := arg("method")
		input := arg("input")
		output := arg("output")
		if method == "" {
			switch step.Action {
			case "repo.List":
				method = "ListAll"
			case "repo.GetForUpdate":
				method = "GetByIDForUpdate"
			default:
				method = "FindByID"
			}
		}
		call := "ctx"
		if input != "" {
			call += ", " + input
		}
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			// repo.List returns a slice (value type); repo.Find/Get return a pointer type
			st.pointers[output] = step.Action != "repo.List"
			// Track the Go type for use in parallel.Run pre-declaration
			if source != "" {
				if step.Action == "repo.List" {
					st.types[output] = "[]domain." + ExportName(source)
				} else {
					st.types[output] = "*domain." + ExportName(source)
				}
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n", pad, output+", err", assign, ExportName(source), method, call))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			// If error: is specified, generate nil check for not-found case
			if errMsg := arg("error"); errMsg != "" && step.Action != "repo.List" {
				b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
				b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", errMsg)))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
			return b.String()
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "repo.Save", "repo.Delete":
		source := arg("source")
		if source == "" {
			return ""
		}
		method := arg("method")
		if method == "" {
			if step.Action == "repo.Save" {
				method = "Save"
			} else {
				method = "Delete"
			}
		}
		input := arg("input")
		call := "ctx"
		if input != "" {
			inputArg := input
			if step.Action == "repo.Save" {
				// repo.Save expects a pointer to the entity. If the variable is a value type
				// (not tracked as pointer), take its address.
				if !strings.HasPrefix(input, "&") && !st.pointers[input] {
					inputArg = "&" + input
				}
			}
			call += ", " + inputArg
		}
		var b strings.Builder
		if step.Action == "repo.Delete" && strings.HasPrefix(method, "DeleteBy") {
			b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		} else {
			b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		}
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "mapping.Assign":
		to := arg("to")
		val := arg("value")
		if to == "" || val == "" {
			return ""
		}
		declare := false
		if v, ok := step.Args["declare"]; ok {
			switch x := v.(type) {
			case bool:
				declare = x
			case string:
				declare = strings.EqualFold(strings.TrimSpace(x), "true")
			}
		}
		if declare && !st.declared[to] {
			st.declared[to] = true
			// Variable declared via mapping.Assign is a value type (not a pointer)
			st.pointers[to] = false
			return fmt.Sprintf("%s%s := %s\n", pad, to, val)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, to, val))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "flow.If", "flow.For", "flow.Block", "tx.Block", "list.Filter", "list.Paginate", "list.Append", "list.Sort", "str.Normalize", "mapping.Map", "event.Publish", "logic.Call", "exec.Run", "fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove", "flow.Switch", "flow.While", "flow.Checkpoint", "flow.Resume", "flow.Validate", "flow.Try", "flow.Catch", "flow.Retry", "flow.Fallback", "flow.Timeout", "flow.SuggestNext", "flow.ExplainError":
		return renderFlowStepControl(st, step, indent, sfx, arg, child)

	case "audit.Log":
		actor := arg("actor")
		company := arg("company")
		event := arg("event")
		if actor == "" || company == "" || event == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_auditRec := &domain.AuditLog{ID: uuid.NewString(), ActorID: %s, CompanyID: %s, Action: %q, CreatedAt: time.Now().UTC()}\n", pad, actor, company, event))
		b.WriteString(fmt.Sprintf("%s\t_ = s.AuditLogRepo.Save(ctx, _auditRec)\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "auth.RequireRole":
		userID := arg("userID")
		companyID := arg("companyID")
		roles := arg("roles")
		output := arg("output")
		if userID == "" || companyID == "" || roles == "" {
			return ""
		}
		if output == "" {
			output = "currentUser"
		}
		adminBypass := true
		if v, ok := step.Args["adminBypass"].(bool); ok {
			adminBypass = v
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = true
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.UserRepo.FindByID(ctx, %s)\n", pad, output+", err", assign, userID))
		b.WriteString(fmt.Sprintf("%sif err != nil || %s == nil {\n", pad, output))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"User not found\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if adminBypass {
			b.WriteString(fmt.Sprintf("%sif %s.CompanyID != %s {\n", pad, output, companyID))
			b.WriteString(fmt.Sprintf("%s\tif %s.Role != \"admin\" {\n", pad, output))
			b.WriteString(errReturn(st, pad+"\t\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Access denied\")"))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%sif %s.CompanyID != %s {\n", pad, output, companyID))
			b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Access denied\")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%sif !helpers.HasRole(%s.Role, %s) {\n", pad, output, roles))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Insufficient role\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "auth.CheckRole":
		user := arg("user")
		roles := arg("roles")
		companyID := arg("companyID")
		if user == "" || roles == "" {
			return ""
		}
		var b strings.Builder
		if companyID != "" {
			b.WriteString(fmt.Sprintf("%sif %s.CompanyID != %s && %s.Role != \"admin\" {\n", pad, user, companyID, user))
			b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Access denied\")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%sif !helpers.HasRole(%s.Role, %s) {\n", pad, user, roles))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Insufficient role\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "entity.PatchNonZero":
		target := arg("target")
		from := arg("from")
		fields := arg("fields")
		if target == "" || from == "" || fields == "" {
			return ""
		}
		parts := strings.Split(fields, ",")
		var quotedFields []string
		for _, f := range parts {
			f = strings.TrimSpace(f)
			if f != "" {
				quotedFields = append(quotedFields, fmt.Sprintf("%q", f))
			}
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, target, from, strings.Join(quotedFields, ", "))

	case "field.CopyNonEmpty":
		from := arg("from")
		to := arg("to")
		fields := arg("fields")
		if from == "" || to == "" {
			return ""
		}
		if fields == "" {
			return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s)\n", pad, to, from)
		}
		parts := strings.Split(fields, ",")
		var quotedFields []string
		for _, f := range parts {
			f = strings.TrimSpace(f)
			if f != "" {
				quotedFields = append(quotedFields, fmt.Sprintf("%q", f))
			}
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, to, from, strings.Join(quotedFields, ", "))

	case "entity.PatchValidated":
		target := arg("target")
		from := arg("from")
		source := arg("source")
		if target == "" || from == "" {
			return ""
		}
		fieldsMap, ok := step.Args["fields"].(map[string]map[string]string)
		if !ok || len(fieldsMap) == 0 {
			return ""
		}
		// Sort field names for deterministic output
		fieldNames := make([]string, 0, len(fieldsMap))
		for k := range fieldsMap {
			fieldNames = append(fieldNames, k)
		}
		sort.Strings(fieldNames)
		var b strings.Builder
		for _, fieldName := range fieldNames {
			rules := fieldsMap[fieldName]
			normalize := rules["normalize"]
			format := rules["format"]
			unique := rules["unique"]
			b.WriteString(fmt.Sprintf("%sif %s.%s != \"\" {\n", pad, from, fieldName))
			innerPad := pad + "\t"
			switch normalize {
			case "lower":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.ToLower(strings.TrimSpace(%s.%s))\n", innerPad, target, fieldName, from, fieldName))
			case "upper":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.ToUpper(strings.TrimSpace(%s.%s))\n", innerPad, target, fieldName, from, fieldName))
			case "trim":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.TrimSpace(%s.%s)\n", innerPad, target, fieldName, from, fieldName))
			default:
				b.WriteString(fmt.Sprintf("%s%s.%s = %s.%s\n", innerPad, target, fieldName, from, fieldName))
			}
			switch format {
			case "email":
				b.WriteString(fmt.Sprintf("%sif !helpers.IsEmail(%s.%s) {\n", innerPad, target, fieldName))
				b.WriteString(errReturn(st, innerPad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_EMAIL\", \"Invalid email format\")"))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			case "phone":
				b.WriteString(fmt.Sprintf("%sif !helpers.IsPhone(%s.%s) {\n", innerPad, target, fieldName))
				b.WriteString(errReturn(st, innerPad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_PHONE\", \"Invalid phone format\")"))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			}
			if unique != "" && source != "" {
				b.WriteString(fmt.Sprintf("%sif _uExisting, _ := s.%sRepo.%s(ctx, %s.%s); _uExisting != nil && _uExisting.ID != %s.ID {\n", innerPad, ExportName(source), unique, target, fieldName, target))
				b.WriteString(errReturn(st, innerPad+"\t", fmt.Sprintf("errors.New(http.StatusConflict, \"CONFLICT\", \"%s already in use\")", fieldName)))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			}
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String()

	case "enum.Validate":
		value := arg("value")
		allowed := arg("allowed")
		throw := arg("throw")
		if value == "" || allowed == "" || throw == "" {
			return ""
		}
		parts := strings.Split(allowed, ",")
		var quotedAllowed []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				quotedAllowed = append(quotedAllowed, fmt.Sprintf("%q", p))
			}
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !helpers.IsOneOf(%s, []string{%s}) {\n", pad, value, strings.Join(quotedAllowed, ", ")))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"INVALID_VALUE\", %q)", throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "list.Enrich":
		items := arg("items")
		lookupSource := arg("lookupSource")
		lookupInput := arg("lookupInput")
		set := arg("set")
		as := arg("as")
		if items == "" || lookupSource == "" || lookupInput == "" || set == "" {
			return ""
		}
		if as == "" {
			as = "_item"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor _i := range %s {\n", pad, items))
		innerPad := pad + "\t"
		b.WriteString(fmt.Sprintf("%s%s := &%s[_i]\n", innerPad, as, items))
		b.WriteString(fmt.Sprintf("%s_enriched, _eErr := s.%sRepo.FindByID(ctx, %s)\n", innerPad, ExportName(lookupSource), lookupInput))
		b.WriteString(fmt.Sprintf("%sif _eErr == nil && _enriched != nil {\n", innerPad))
		innerInnerPad := innerPad + "\t"
		pairs := strings.Split(set, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(kv) == 2 {
				targetField := strings.TrimSpace(kv[0])
				lookupField := strings.TrimSpace(kv[1])
				b.WriteString(fmt.Sprintf("%s%s.%s = _enriched.%s\n", innerInnerPad, as, targetField, lookupField))
			}
		}
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "time.Parse":
		value := arg("value")
		output := arg("output")
		format := arg("format")
		if value == "" || output == "" {
			return ""
		}
		if format == "" {
			format = "time.RFC3339"
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s time.Parse(%s, %s)\n", pad, output+", err", assign, format, value))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_DATE\", \"invalid date format\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "time.CheckExpiry":
		value := arg("value")
		throw := arg("throw")
		mustBe := arg("mustBe")
		if value == "" || throw == "" {
			return ""
		}
		if mustBe == "" {
			mustBe = "future"
		}
		tv, terrv := "_t"+sfx, "_tErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := time.Parse(time.RFC3339, %s)\n", pad, tv, terrv, value))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, terrv))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_DATE\", \"invalid date format\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if mustBe == "past" {
			b.WriteString(fmt.Sprintf("%sif !time.Now().After(%s) {\n", pad, tv))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"NOT_EXPIRED\", %q)", throw)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%sif time.Now().After(%s) {\n", pad, tv))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"EXPIRED\", %q)", throw)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String()

	case "map.Build":
		from := arg("from")
		as := arg("as")
		key := arg("key")
		value := arg("value")
		output := arg("output")
		valueType := arg("valueType")
		if from == "" || key == "" || value == "" || output == "" {
			return ""
		}
		if as == "" {
			as = "_item"
		}
		if valueType == "" {
			valueType = "string"
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = fmt.Sprintf("map[string]%s", valueType)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s make(map[string]%s, len(%s))\n", pad, output, assign, valueType, from))
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, as, from))
		b.WriteString(fmt.Sprintf("%s\t%s[%s] = %s\n", pad, output, key, value))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "repo.Query":
		// repo.Query calls a custom repository finder method.
		// Args: source (entity), method (finder name), input (optional arg), output (result var), error (optional msg for nil check)
		source := arg("source")
		method := arg("method")
		input := arg("input")
		output := arg("output")
		errMsg := arg("error")
		if source == "" || method == "" {
			return ""
		}
		var b strings.Builder
		inputArg := ""
		if input != "" {
			inputArg = ", " + input
		}
		if output == "" {
			// fire-and-forget call
			b.WriteString(fmt.Sprintf("%sif _, _qrErr := s.%sRepo.%s(ctx%s); _qrErr != nil {\n", pad, ExportName(source), ExportName(method), inputArg))
			b.WriteString(errReturn(st, pad+"\t", "_qrErr"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String()
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = true
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(ctx%s)\n", pad, output+", err", assign, ExportName(source), ExportName(method), inputArg))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		if errMsg != "" {
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", errMsg)))
		} else {
			b.WriteString(errReturn(st, pad+"\t", "err"))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if errMsg != "" {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", errMsg)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String()

	case "repo.Upsert":
		source := arg("source")
		find := arg("find")
		input := arg("input")
		output := arg("output")
		if source == "" || find == "" || input == "" || output == "" {
			return ""
		}
		ifNewSteps := child("_ifNew")
		ifExistsSteps := child("_ifExists")
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = true
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.FindByID(ctx, %s)\n", pad, output+", err", assign, ExportName(source), find))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
		innerPad := pad + "\t"
		b.WriteString(fmt.Sprintf("%s_uNew := %s\n", innerPad, input))
		b.WriteString(fmt.Sprintf("%s%s = &_uNew\n", innerPad, output))
		ifNewState := cloneFlowState(st)
		if len(ifNewSteps) > 0 {
			b.WriteString(renderFlowSteps(ifNewState, ifNewSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(source), output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		ifExistsState := cloneFlowState(st)
		if len(ifExistsSteps) > 0 {
			b.WriteString(renderFlowSteps(ifExistsState, ifExistsSteps, indent+1))
		}
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(source), output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "fsm.Transition":
		entity := arg("entity")
		to := arg("to")
		if entity == "" || to == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := %s.TransitionTo(%q); err != nil {\n", pad, entity, to))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String()

	case "notification.Dispatch", "notify.Dispatch":
		// primary arg: "event" (falls back to "message" for compat)
		event := arg("event")
		if event == "" {
			event = arg("message")
		}
		if event == "" {
			return ""
		}
		userID := arg("userID")
		entityID := arg("entityID")
		msgType := arg("type")
		payload := arg("payload")
		tmpl := arg("template")
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s_ = s.dispatcher.Dispatch(ctx, port.NotificationMessage{Event: %q", pad, event))
		if msgType != "" {
			b.WriteString(fmt.Sprintf(", Type: %q", msgType))
		}
		if userID != "" {
			b.WriteString(fmt.Sprintf(", UserID: %s", userID))
		}
		if entityID != "" {
			b.WriteString(fmt.Sprintf(", EntityID: %s", entityID))
		}
		if payload != "" {
			b.WriteString(fmt.Sprintf(", Payload: %s", payload))
		}
		if tmpl != "" {
			b.WriteString(fmt.Sprintf(", Template: %q", tmpl))
		}
		b.WriteString("})\n")
		return b.String()

	// -------------------------------------------------------------------------
	// STAGE 2: Infrastructure actions
	// -------------------------------------------------------------------------

	case "cache.Get", "cache.Set", "cache.Del", "mail.Send", "storage.Upload", "storage.Download", "storage.GetURL", "http.Call", "rand.Code", "rand.Token", "json.Parse", "json.Marshal", "parallel.Run", "pdf.Render", "webhook.Send", "queue.Enqueue":
		return renderFlowStepInfra(st, step, indent, sfx, arg, child)

	default:
		return ""
	}
}
