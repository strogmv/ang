package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepDomainPrimitives emits simple collection/map primitives from
// typed actions only; complex map.Get remains in its dedicated legacy helper.
func renderTypedStepDomainPrimitives(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "list.Len":
		action, err := typedActionAs[flowir.ListLen](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("len(%s)", normalizeFlowExpr(action.Input.Source)), "int"), true
	case "convert.ToFloat":
		action, err := typedActionAs[flowir.ConvertToFloat](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("float64(%s)", normalizeFlowExpr(action.Input.Source)), "float64"), true
	case "convert.ToInt":
		action, err := typedActionAs[flowir.ConvertToInt](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("int64(%s)", normalizeFlowExpr(action.Input.Source)), "int64"), true
	case "list.New":
		action, err := typedActionAs[flowir.ListNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		capacity := normalizeFlowExpr(action.Capacity.Source)
		if capacity != "" {
			return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("make(%s, 0, %s)", action.GoType, capacity), action.GoType), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("make(%s, 0)", action.GoType), action.GoType), true
	case "map.New":
		action, err := typedActionAs[flowir.MapNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("make(%s)", action.GoType), action.GoType), true
	case "map.Get":
		action, err := typedActionAs[flowir.MapGet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(action.Input.Source)
		key := normalizeFlowExpr(action.Key.Source)
		defaultExpr := normalizeFlowExpr(action.Default.Source)
		into := action.Into
		if into == "" {
			into = "any"
		}
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		foundDeclared := action.Found != "" && st.declared[action.Found]
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		st.types[action.Output] = into
		if action.Found != "" {
			st.declared[action.Found] = true
			st.pointers[action.Found] = false
			st.types[action.Found] = "bool"
		}
		valueVar := "_mapVal" + sfx
		foundVar := "_mapFound" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := %s[%s]\n", pad, valueVar, foundVar, input, key))
		if action.Found != "" {
			foundAssign := ":="
			if foundDeclared {
				foundAssign = "="
			}
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, action.Found, foundAssign, foundVar))
		}
		if into == "any" {
			b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, action.Output, assign, valueVar))
			if defaultExpr != "" {
				b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, foundVar))
				b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, action.Output, defaultExpr))
				b.WriteString(fmt.Sprintf("%s}\n", pad))
			}
			return b.String(), true
		}
		if assign == ":=" {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, action.Output, into))
		}
		typedVar := "_typedVal" + sfx
		okVar := "_ok" + sfx
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, foundVar))
		b.WriteString(fmt.Sprintf("%s\tif %s, %s := %s.(%s); %s {\n", pad, typedVar, okVar, valueVar, into, okVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, action.Output, typedVar))
		b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"map.Get: value for key %%v is not %s\", %s)", into, key)))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		if defaultExpr != "" {
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, action.Output, defaultExpr))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "map.Has":
		action, err := typedActionAs[flowir.MapHas](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(action.Input.Source)
		key := normalizeFlowExpr(action.Key.Source)
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("func() bool { _, _ok := %s[%s]; return _ok }()", input, key), "bool"), true
	case "map.Set":
		action, err := typedActionAs[flowir.MapSet](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(action.Input.Source)
		key := normalizeFlowExpr(action.Key.Source)
		value := normalizeFlowExpr(action.Value.Source)
		target := input
		var b strings.Builder
		if action.Output != "" && action.Output != input {
			b.WriteString(renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("maps.Clone(%s)", input), ""))
			target = action.Output
		}
		b.WriteString(fmt.Sprintf("%s%s[%s] = %s\n", pad, target, key, value))
		return b.String(), true
	case "map.Merge":
		action, err := typedActionAs[flowir.MapMerge](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		left := normalizeFlowExpr(action.Left.Source)
		right := normalizeFlowExpr(action.Right.Source)
		var b strings.Builder
		b.WriteString(renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("maps.Clone(%s)", left), ""))
		b.WriteString(fmt.Sprintf("%smaps.Copy(%s, %s)\n", pad, action.Output, right))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepDomainTime(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "time.Parse":
		action, err := typedActionAs[flowir.TimeParse](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		format := normalizeFlowExpr(action.Format)
		if format == "" {
			format = "time.RFC3339"
		}
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s time.Parse(%s, %s)\n", pad, action.Output+", err", assign, format, normalizeFlowExpr(action.Value.Source)))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_DATE\", \"invalid date format\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "time.Add":
		action, err := typedActionAs[flowir.TimeAdd](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("%s.Add(%s)", normalizeFlowExpr(action.Input.Source), normalizeFlowExpr(action.Duration.Source)), "time.Time"), true
	case "time.Sub":
		action, err := typedActionAs[flowir.TimeSub](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("%s.Sub(%s)", normalizeFlowExpr(action.A.Source), normalizeFlowExpr(action.B.Source)), "time.Duration"), true
	case "time.Diff":
		action, err := typedActionAs[flowir.TimeDiff](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		from, to := normalizeFlowExpr(action.From.Source), normalizeFlowExpr(action.To.Source)
		if action.Unit == "" || action.Unit == "duration" {
			return renderFlowAssignTarget(st, pad, action.Output, fmt.Sprintf("%s.Sub(%s)", to, from), "time.Duration"), true
		}
		expr := ""
		switch action.Unit {
		case "milliseconds":
			expr = fmt.Sprintf("float64(%s.Sub(%s).Milliseconds())", to, from)
		case "seconds":
			expr = fmt.Sprintf("%s.Sub(%s).Seconds()", to, from)
		case "minutes":
			expr = fmt.Sprintf("%s.Sub(%s).Minutes()", to, from)
		case "hours":
			expr = fmt.Sprintf("%s.Sub(%s).Hours()", to, from)
		case "days":
			expr = fmt.Sprintf("%s.Sub(%s).Hours() / 24", to, from)
		default:
			return renderInvalidFlowStepConfig(st, pad, step.Name, "time.Diff unit must be duration, milliseconds, seconds, minutes, hours, or days"), true
		}
		return renderFlowAssignTarget(st, pad, action.Output, expr, "float64"), true
	case "time.CheckExpiry":
		action, err := typedActionAs[flowir.TimeCheckExpiry](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		timeVar, errVar := "_t"+sfx, "_tErr"+sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s, %s := time.Parse(time.RFC3339, %s)\n", pad, timeVar, errVar, normalizeFlowExpr(action.Value.Source)))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_DATE\", \"invalid date format\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if action.MustBe == "past" {
			b.WriteString(fmt.Sprintf("%sif !time.Now().After(%s) {\n", pad, timeVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"NOT_EXPIRED\", %q)", action.Throw)))
		} else {
			b.WriteString(fmt.Sprintf("%sif time.Now().After(%s) {\n", pad, timeVar))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"EXPIRED\", %q)", action.Throw)))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepDomainValidation(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	quotedFields := func(fields []string) string {
		quoted := make([]string, 0, len(fields))
		for _, field := range fields {
			quoted = append(quoted, fmt.Sprintf("%q", field))
		}
		return strings.Join(quoted, ", ")
	}
	switch step.Name {
	case "entity.PatchNonZero":
		action, err := typedActionAs[flowir.EntityPatchNonZero](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, normalizeFlowExpr(action.Target.Source), normalizeFlowExpr(action.From.Source), quotedFields(action.Fields)), true
	case "field.CopyNonEmpty":
		action, err := typedActionAs[flowir.FieldCopyNonEmpty](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		from, to := normalizeFlowExpr(action.From.Source), normalizeFlowExpr(action.To.Source)
		if len(action.Fields) == 0 {
			return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s)\n", pad, to, from), true
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, to, from, quotedFields(action.Fields)), true
	case "entity.PatchValidated":
		action, err := typedActionAs[flowir.EntityPatchValidated](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		target, from := normalizeFlowExpr(action.Target.Source), normalizeFlowExpr(action.From.Source)
		fieldNames := make([]string, 0, len(action.Fields))
		for field := range action.Fields {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)
		var b strings.Builder
		for _, field := range fieldNames {
			rule := action.Fields[field]
			b.WriteString(fmt.Sprintf("%sif %s.%s != \"\" {\n", pad, from, field))
			innerPad := pad + "\t"
			switch rule.Normalize {
			case "lower":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.ToLower(strings.TrimSpace(%s.%s))\n", innerPad, target, field, from, field))
			case "upper":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.ToUpper(strings.TrimSpace(%s.%s))\n", innerPad, target, field, from, field))
			case "trim":
				b.WriteString(fmt.Sprintf("%s%s.%s = strings.TrimSpace(%s.%s)\n", innerPad, target, field, from, field))
			default:
				b.WriteString(fmt.Sprintf("%s%s.%s = %s.%s\n", innerPad, target, field, from, field))
			}
			switch rule.Format {
			case "email":
				b.WriteString(fmt.Sprintf("%sif !helpers.IsEmail(%s.%s) {\n", innerPad, target, field))
				b.WriteString(errReturn(st, innerPad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_EMAIL\", \"Invalid email format\")"))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			case "phone":
				b.WriteString(fmt.Sprintf("%sif !helpers.IsPhone(%s.%s) {\n", innerPad, target, field))
				b.WriteString(errReturn(st, innerPad+"\t", "errors.New(http.StatusBadRequest, \"INVALID_PHONE\", \"Invalid phone format\")"))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			}
			if rule.Unique != "" && action.Source != "" {
				b.WriteString(fmt.Sprintf("%sif _uExisting, _ := s.%sRepo.%s(ctx, %s.%s); _uExisting != nil && _uExisting.ID != %s.ID {\n", innerPad, ExportName(action.Source), rule.Unique, target, field, target))
				b.WriteString(errReturn(st, innerPad+"\t", fmt.Sprintf("errors.New(http.StatusConflict, \"CONFLICT\", \"%s already in use\")", field)))
				b.WriteString(fmt.Sprintf("%s}\n", innerPad))
			}
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true
	case "enum.Validate":
		action, err := typedActionAs[flowir.EnumValidate](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		allowed := quotedFields(action.Allowed)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !helpers.IsOneOf(%s, []string{%s}) {\n", pad, normalizeFlowExpr(action.Value.Source), allowed))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"INVALID_VALUE\", %q)", action.Throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepListEnrich(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	action, err := typedActionAs[flowir.ListEnrich](step)
	pad := strings.Repeat("\t", indent)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	items := normalizeFlowExpr(action.Items.Source)
	lookupInput := normalizeFlowExpr(action.LookupInput.Source)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sfor _i := range %s {\n", pad, items))
	innerPad := pad + "\t"
	b.WriteString(fmt.Sprintf("%s%s := &%s[_i]\n", innerPad, action.As, items))
	b.WriteString(fmt.Sprintf("%s_enriched, _eErr := s.%sRepo.FindByID(ctx, %s)\n", innerPad, ExportName(action.LookupSource), lookupInput))
	b.WriteString(fmt.Sprintf("%sif _eErr == nil && _enriched != nil {\n", innerPad))
	for _, field := range action.Fields {
		b.WriteString(fmt.Sprintf("%s\t%s.%s = _enriched.%s\n", innerPad, action.As, field.Target, field.Source))
	}
	b.WriteString(fmt.Sprintf("%s}\n", innerPad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String(), true
}

func renderTypedStepDomainSpecial(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "audit.Log":
		action, err := typedActionAs[flowir.AuditLog](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "audit.Log requires actor, company, and event"), true
		}
		actor, company, event := normalizeFlowExpr(action.Actor.Source), normalizeFlowExpr(action.Company.Source), strings.TrimSpace(action.Event.Source)
		if actor == "" || company == "" || event == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "audit.Log requires actor, company, and event"), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s{\n", pad))
		b.WriteString(fmt.Sprintf("%s\t_auditRec := &domain.AuditLog{ID: uuid.NewString(), ActorID: %s, CompanyID: %s, Action: %q, CreatedAt: time.Now().UTC()}\n", pad, actor, company, event))
		b.WriteString(fmt.Sprintf("%s\tif s.AuditLogRepo == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `fmt.Errorf("audit.Log: audit repository wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s\tif _auditErr := s.AuditLogRepo.Save(ctx, _auditRec); _auditErr != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t\t", `fmt.Errorf("audit.Log: %w", _auditErr)`))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "fsm.Transition":
		action, err := typedActionAs[flowir.FSMTransition](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := %s.TransitionTo(%q); err != nil {\n", pad, normalizeFlowExpr(action.Entity.Source), action.To))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "notification.Dispatch", "notify.Dispatch":
		action, err := typedActionAs[flowir.NotificationDispatch](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		event := normalizeFlowExpr(action.Event.Source)
		messageType := normalizeFlowExpr(action.Type.Source)
		userID := normalizeFlowExpr(action.UserID.Source)
		entityID := normalizeFlowExpr(action.EntityID.Source)
		payload := normalizeFlowExpr(action.Payload.Source)
		template := normalizeFlowExpr(action.Template.Source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif s.dispatcher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("notify.Dispatch: notification dispatcher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _dispatchErr := s.dispatcher.Dispatch(ctx, port.NotificationMessage{Event: strings.TrimSpace(fmt.Sprint(%s))", pad, event))
		if messageType != "" {
			b.WriteString(fmt.Sprintf(", Type: strings.TrimSpace(fmt.Sprint(%s))", messageType))
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
		if template != "" {
			b.WriteString(fmt.Sprintf(", Template: strings.TrimSpace(fmt.Sprint(%s))", template))
		}
		b.WriteString("}); _dispatchErr != nil {\n")
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("notify.Dispatch: %w", _dispatchErr)`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepRBAC(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	if step.Name != "rbac.CheckPermission" {
		return "", false
	}
	pad := strings.Repeat("\t", indent)
	action, err := typedActionAs[flowir.RBACCheckPermission](step)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	user := normalizeFlowExpr(action.User.Source)
	permission := normalizeFlowExpr(action.Permission.Source)
	if user == "" || permission == "" {
		return "", true
	}
	shouldThrow := true
	throwMsg := action.Throw
	if throwMsg == "" {
		if action.Output != "" {
			shouldThrow = false
		} else {
			throwMsg = `"Insufficient permission"`
		}
	}
	code := action.Code
	if code == "" {
		code = `"FORBIDDEN"`
	}
	status := normalizeFlowExpr(action.Status.Source)
	if status == "" {
		status = "http.StatusForbidden"
	}
	permOK := "_permOK" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%s := rbac.CheckPermission(%s.Role, %s)\n", pad, permOK, user, permission))
	if action.Output != "" {
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		st.types[action.Output] = "bool"
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", pad, action.Output, assign, permOK))
	}
	if shouldThrow {
		b.WriteString(fmt.Sprintf("%sif !%s {\n", pad, permOK))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %s, %s)", status, code, throwMsg)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
	return b.String(), true
}

func renderTypedStepLogicCheck(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	action, err := typedActionAs[flowir.LogicCheck](step)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	status, label := "http.StatusBadRequest", "Validation Error"
	switch action.Status {
	case "403", "forbidden":
		status, label = "http.StatusForbidden", "Forbidden"
	case "404", "not_found":
		status, label = "http.StatusNotFound", "Not Found"
	case "409", "conflict":
		status, label = "http.StatusConflict", "Conflict"
	case "401", "unauthorized":
		status, label = "http.StatusUnauthorized", "Unauthorized"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, normalizeFlowExpr(action.Condition.Source)))
	b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", status, label, action.Throw)))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String(), true
}

func renderTypedStepDomainComputed(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "map.Build":
		action, err := typedActionAs[flowir.MapBuild](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		from := normalizeFlowExpr(action.From.Source)
		key := normalizeFlowExpr(action.Key.Source)
		value := normalizeFlowExpr(action.Value.Source)
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		st.types[action.Output] = fmt.Sprintf("map[string]%s", action.ValueType)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s make(map[string]%s, len(%s))\n", pad, action.Output, assign, action.ValueType, from))
		b.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n", pad, action.As, from))
		b.WriteString(fmt.Sprintf("%s\t%s[%s] = %s\n", pad, action.Output, key, value))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	case "math.Expr":
		action, err := typedActionAs[flowir.MathExpression](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		assign := "="
		if action.Declare && !st.declared[action.Output] {
			assign = ":="
			st.declared[action.Output] = true
			st.pointers[action.Output] = false
		}
		return fmt.Sprintf("%s%s %s %s\n", pad, action.Output, assign, normalizeFlowExpr(action.Value.Source)), true
	}
	return "", false
}
