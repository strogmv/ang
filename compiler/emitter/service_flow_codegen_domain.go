package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderFlowStepDomain(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	if out, ok := renderFlowStepDomainRepoMapping(st, step, indent, sfx, arg, child); ok {
		return out, true
	}

	switch step.Action {
	case "logic.Check":
		cond := arg("condition")
		throw := arg("throw")
		if cond == "" {
			return "", true
		}
		if throw == "" {
			throw = "validation failed"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"Validation Error\", %q)", throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "repo.Get", "repo.Find", "repo.GetForUpdate", "repo.List":
		source := arg("source")
		if source == "" {
			return "", true
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
			st.pointers[output] = step.Action != "repo.List"
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
			if errMsg := arg("error"); errMsg != "" && step.Action != "repo.List" {
				b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
				b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", errMsg)))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
			return b.String(), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "repo.Save", "repo.Delete":
		source := arg("source")
		if source == "" {
			return "", true
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
		return b.String(), true

	case "mapping.Assign":
		to := arg("to")
		val := arg("value")
		if to == "" || val == "" {
			return "", true
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
			st.pointers[to] = false
			return fmt.Sprintf("%s%s := %s\n", pad, to, val), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, to, val))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "audit.Log":
		actor := arg("actor")
		company := arg("company")
		event := arg("event")
		if actor == "" || company == "" || event == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_auditRec := &domain.AuditLog{ID: uuid.NewString(), ActorID: %s, CompanyID: %s, Action: %q, CreatedAt: time.Now().UTC()}\n", pad, actor, company, event))
		b.WriteString(fmt.Sprintf("%s\t_ = s.AuditLogRepo.Save(ctx, _auditRec)\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "auth.RequireRole":
		userID := arg("userID")
		companyID := arg("companyID")
		roles := arg("roles")
		output := arg("output")
		if userID == "" || companyID == "" || roles == "" {
			return "", true
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
		return b.String(), true

	case "auth.CheckRole":
		user := arg("user")
		roles := arg("roles")
		companyID := arg("companyID")
		if user == "" || roles == "" {
			return "", true
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
		return b.String(), true

	case "rbac.CheckPermission":
		user := arg("user")
		permission := arg("permission")
		if user == "" || permission == "" {
			return "", true
		}
		output := arg("output")
		throwMsg := arg("throw")
		shouldThrow := true
		if throwMsg == "" {
			if output != "" {
				shouldThrow = false
			} else {
				throwMsg = `"Insufficient permission"`
			}
		}
		code := arg("code")
		if code == "" {
			code = `"FORBIDDEN"`
		}
		status := arg("status")
		if status == "" {
			status = "http.StatusForbidden"
		}
		permOK := "_permOK" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s := rbac.CheckPermission(%s.Role, %s)\n", pad, permOK, user, permission))
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "bool"
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, output, assign, permOK))
		}
		if shouldThrow {
			b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, permOK))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %s, %s)", status, code, throwMsg)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "entity.PatchNonZero":
		target := arg("target")
		from := arg("from")
		fields := arg("fields")
		if target == "" || from == "" || fields == "" {
			return "", true
		}
		parts := strings.Split(fields, ",")
		var quotedFields []string
		for _, f := range parts {
			f = strings.TrimSpace(f)
			if f != "" {
				quotedFields = append(quotedFields, fmt.Sprintf("%q", f))
			}
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, target, from, strings.Join(quotedFields, ", ")), true

	case "field.CopyNonEmpty":
		from := arg("from")
		to := arg("to")
		fields := arg("fields")
		if from == "" || to == "" {
			return "", true
		}
		if fields == "" {
			return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s)\n", pad, to, from), true
		}
		parts := strings.Split(fields, ",")
		var quotedFields []string
		for _, f := range parts {
			f = strings.TrimSpace(f)
			if f != "" {
				quotedFields = append(quotedFields, fmt.Sprintf("%q", f))
			}
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, to, from, strings.Join(quotedFields, ", ")), true

	case "entity.PatchValidated":
		target := arg("target")
		from := arg("from")
		source := arg("source")
		if target == "" || from == "" {
			return "", true
		}
		fieldsMap, ok := step.Args["fields"].(map[string]map[string]string)
		if !ok || len(fieldsMap) == 0 {
			return "", true
		}
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
		return b.String(), true

	case "enum.Validate":
		value := arg("value")
		allowed := arg("allowed")
		throw := arg("throw")
		if value == "" || allowed == "" || throw == "" {
			return "", true
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
		return b.String(), true

	case "list.Enrich":
		items := arg("items")
		lookupSource := arg("lookupSource")
		lookupInput := arg("lookupInput")
		set := arg("set")
		as := arg("as")
		if items == "" || lookupSource == "" || lookupInput == "" || set == "" {
			return "", true
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
		return b.String(), true

	case "time.Parse":
		value := arg("value")
		output := arg("output")
		format := arg("format")
		if value == "" || output == "" {
			return "", true
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
		return b.String(), true

	case "time.CheckExpiry":
		value := arg("value")
		throw := arg("throw")
		mustBe := arg("mustBe")
		if value == "" || throw == "" {
			return "", true
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
		return b.String(), true

	case "map.Build":
		from := arg("from")
		as := arg("as")
		key := arg("key")
		value := arg("value")
		output := arg("output")
		valueType := arg("valueType")
		if from == "" || key == "" || value == "" || output == "" {
			return "", true
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
		return b.String(), true

	case "repo.Query":
		source := arg("source")
		method := arg("method")
		input := arg("input")
		output := arg("output")
		errMsg := arg("error")
		if source == "" || method == "" {
			return "", true
		}
		var b strings.Builder
		inputArg := ""
		if input != "" {
			inputArg = ", " + input
		}
		if output == "" {
			b.WriteString(fmt.Sprintf("%sif _, _qrErr := s.%sRepo.%s(ctx%s); _qrErr != nil {\n", pad, ExportName(source), ExportName(method), inputArg))
			b.WriteString(errReturn(st, pad+"\t", "_qrErr"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
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
		return b.String(), true

	case "repo.Upsert":
		source := arg("source")
		find := arg("find")
		input := arg("input")
		output := arg("output")
		if source == "" || find == "" || input == "" || output == "" {
			return "", true
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
		return b.String(), true

	case "fsm.Transition":
		entity := arg("entity")
		to := arg("to")
		if entity == "" || to == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := %s.TransitionTo(%q); err != nil {\n", pad, entity, to))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "notification.Dispatch", "notify.Dispatch":
		event := arg("event")
		if event == "" {
			event = arg("message")
		}
		if event == "" {
			return "", true
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
		return b.String(), true
	}

	return "", false
}
