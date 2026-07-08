package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func flowHTTPStatusExpr(raw any, fallback string) string {
	switch v := raw.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%d", int(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return fallback
		}
		switch strings.ToLower(s) {
		case "400", "bad_request":
			return "http.StatusBadRequest"
		case "401", "unauthorized":
			return "http.StatusUnauthorized"
		case "403", "forbidden":
			return "http.StatusForbidden"
		case "404", "not_found":
			return "http.StatusNotFound"
		case "409", "conflict":
			return "http.StatusConflict"
		case "422", "unprocessable_entity":
			return "http.StatusUnprocessableEntity"
		case "429", "too_many_requests":
			return "http.StatusTooManyRequests"
		case "500", "internal", "internal_error":
			return "http.StatusInternalServerError"
		case "503", "service_unavailable":
			return "http.StatusServiceUnavailable"
		default:
			return s
		}
	default:
		return fallback
	}
}

func renderFlowStepDomain(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	if out, ok := renderFlowStepDomainRepoMapping(st, step, indent, sfx, arg, child); ok {
		return out, true
	}

	switch step.Action {
	case "errors.New":
		typed, err := decodeCurrentActionAs[flowir.ErrorNew](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		message := normalizeFlowExpr(typed.Message.Source)
		statusExpr := flowHTTPStatusExpr(typed.Status.Source, "http.StatusInternalServerError")
		codeExpr := normalizeFlowExpr(typed.Code.Source)
		if codeExpr == "" {
			codeExpr = `"ERROR"`
		}
		errExpr := fmt.Sprintf("errors.New(%s, %s, %s)", statusExpr, codeExpr, message)
		output, throwNow := typed.Output, typed.Throw
		if output != "" {
			b := &strings.Builder{}
			if !st.declared[output] {
				b.WriteString(fmt.Sprintf("%svar %s error\n", pad, output))
			}
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "error"
			b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, output, errExpr))
			if throwNow {
				b.WriteString(errReturn(st, pad, output))
			}
			return b.String(), true
		}
		return errReturn(st, pad, errExpr), true

	case "logic.Check":
		typed, err := decodeCurrentActionAs[flowir.LogicCheck](st, step)
		if err != nil {
			return "", true
		}
		cond := normalizeFlowExpr(typed.Condition.Source)
		throw := typed.Throw
		if cond == "" {
			return "", true
		}
		if throw == "" {
			throw = "validation failed"
		}
		httpStatus := "http.StatusBadRequest"
		statusLabel := "Validation Error"
		if s := strings.TrimSpace(arg("status")); s != "" {
			switch s {
			case "403", "forbidden":
				httpStatus = "http.StatusForbidden"
				statusLabel = "Forbidden"
			case "404", "not_found":
				httpStatus = "http.StatusNotFound"
				statusLabel = "Not Found"
			case "409", "conflict":
				httpStatus = "http.StatusConflict"
				statusLabel = "Conflict"
			case "401", "unauthorized":
				httpStatus = "http.StatusUnauthorized"
				statusLabel = "Unauthorized"
			}
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !(%s) {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", httpStatus, statusLabel, throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.ThrowIf":
		typed, err := decodeCurrentActionAs[flowir.ErrorThrowIf](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		cond, throw := normalizeFlowExpr(typed.Condition.Source), typed.Throw
		httpStatus := "http.StatusBadRequest"
		statusLabel := "Error"
		if s := strings.TrimSpace(typed.Status.Source); s != "" {
			switch s {
			case "403", "forbidden":
				httpStatus = "http.StatusForbidden"
				statusLabel = "Forbidden"
			case "404", "not_found":
				httpStatus = "http.StatusNotFound"
				statusLabel = "Not Found"
			case "409", "conflict":
				httpStatus = "http.StatusConflict"
				statusLabel = "Conflict"
			case "401", "unauthorized":
				httpStatus = "http.StatusUnauthorized"
				statusLabel = "Unauthorized"
			default:
				httpStatus = s
			}
		}
		if code := strings.TrimSpace(normalizeFlowExpr(typed.Code.Source)); code != "" {
			statusLabel = code
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", httpStatus, statusLabel, throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.Wrap":
		typed, err := decodeCurrentActionAs[flowir.ErrorWrap](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		errExpr, message, output := normalizeFlowExpr(typed.Error.Source), normalizeFlowExpr(typed.Message.Source), typed.Output
		if output != "" {
			alreadyDeclared := st.declared[output]
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "error"
			var b strings.Builder
			if !alreadyDeclared {
				b.WriteString(fmt.Sprintf("%svar %s error\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
			b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"%%s: %%w\", %s, %s)\n", pad, output, message, errExpr))
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s = nil\n", pad, output))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%%s: %%w\", %s, %s)", message, errExpr)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.Map":
		typed, err := decodeCurrentActionAs[flowir.ErrorMap](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output, mode, rawCases := normalizeFlowExpr(typed.Input.Source), typed.Output, typed.Mode, typed.Cases
		keys := make([]string, 0, len(rawCases))
		for k := range rawCases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		defaultMessage := typed.DefaultMessage
		defaultCode := typed.DefaultCode
		defaultStatus := typed.DefaultStatus
		if defaultCode == "" {
			defaultCode = "INTERNAL_ERROR"
		}
		if defaultStatus == "" {
			defaultStatus = "http.StatusInternalServerError"
		}
		errVar := "_mappedErr" + sfx
		msgVar := "_mappedErrMsg" + sfx
		checkVar := "_mapErrCheck" + sfx
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, input))
		b.WriteString(fmt.Sprintf("%s\t%s := strings.TrimSpace(%s.Error())\n", pad, msgVar, input))
		b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, errVar))
		for i, key := range keys {
			caseCfg := rawCases[key]
			status := strings.TrimSpace(caseCfg.Status)
			if status == "" {
				status = "http.StatusBadGateway"
			}
			code := strings.TrimSpace(caseCfg.Code)
			if code == "" {
				code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), " ", "_"))
			}
			message := strings.TrimSpace(caseCfg.Message)
			if message == "" {
				message = key
			}
			prefix := "if"
			if i > 0 {
				prefix = "} else if"
			}
			if mode == "equals" {
				b.WriteString(fmt.Sprintf("%s\t%s %s == %q {\n", pad, prefix, msgVar, key))
			} else {
				b.WriteString(fmt.Sprintf("%s\t%s strings.Contains(%s, %q) {\n", pad, prefix, msgVar, key))
			}
			b.WriteString(fmt.Sprintf("%s\t\t%s = errors.New(%s, %q, %q)\n", pad, errVar, status, code, message))
		}
		if len(keys) > 0 {
			if defaultMessage != "" {
				b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
				b.WriteString(fmt.Sprintf("%s\t\t%s = errors.New(%s, %q, %q)\n", pad, errVar, defaultStatus, defaultCode, defaultMessage))
				b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			} else {
				b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			}
		}
		if output != "" {
			alreadyDeclared := st.declared[output]
			st.declared[output] = true
			st.pointers[output] = false
			st.types[output] = "error"
			if !alreadyDeclared {
				b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, output))
			}
			b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errVar))
			b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, output, errVar))
			b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, output, input))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, checkVar, errVar))
		b.WriteString(fmt.Sprintf("%s\tif %s == nil {\n", pad, checkVar))
		b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, checkVar, input))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		b.WriteString(errReturn(st, pad+"\t", checkVar))
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
		typed, err := decodeCurrentActionAs[flowir.MappingAssign](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		to := normalizeFlowExpr(typed.Target.Source)
		val := normalizeFlowExpr(typed.Value.Source)
		if to == "" || val == "" {
			return renderInvalidFlowStepConfig(st, pad, "mapping.Assign", "mapping.Assign requires to and value"), true
		}
		if typed.Declare && !st.declared[to] {
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
			return renderInvalidFlowStepConfig(st, pad, "audit.Log", "audit.Log requires actor, company, and event"), true
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

	case "auth.RequireRole":
		typed, err := decodeCurrentActionAs[flowir.AuthRequireRole](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		userID, companyID, roles, output, adminBypass := normalizeFlowExpr(typed.UserID.Source), normalizeFlowExpr(typed.CompanyID.Source), normalizeFlowExpr(typed.Roles.Source), typed.Output, typed.AdminBypass
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
		typed, err := decodeCurrentActionAs[flowir.AuthCheckRole](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		user, roles, companyID := normalizeFlowExpr(typed.User.Source), normalizeFlowExpr(typed.Roles.Source), normalizeFlowExpr(typed.CompanyID.Source)
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
		typed, err := decodeCurrentActionAs[flowir.EntityPatchNonZero](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		target, from := normalizeFlowExpr(typed.Target.Source), normalizeFlowExpr(typed.From.Source)
		quotedFields := make([]string, 0, len(typed.Fields))
		for _, field := range typed.Fields {
			quotedFields = append(quotedFields, fmt.Sprintf("%q", field))
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, target, from, strings.Join(quotedFields, ", ")), true

	case "field.CopyNonEmpty":
		typed, err := decodeCurrentActionAs[flowir.FieldCopyNonEmpty](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		from, to := normalizeFlowExpr(typed.From.Source), normalizeFlowExpr(typed.To.Source)
		if len(typed.Fields) == 0 {
			return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s)\n", pad, to, from), true
		}
		quotedFields := make([]string, 0, len(typed.Fields))
		for _, field := range typed.Fields {
			quotedFields = append(quotedFields, fmt.Sprintf("%q", field))
		}
		return fmt.Sprintf("%shelpers.CopyNonEmptyFields(&%s, %s, %s)\n", pad, to, from, strings.Join(quotedFields, ", ")), true

	case "entity.PatchValidated":
		typed, err := decodeCurrentActionAs[flowir.EntityPatchValidated](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		target, from, source, fieldsMap := normalizeFlowExpr(typed.Target.Source), normalizeFlowExpr(typed.From.Source), typed.Source, typed.Fields
		fieldNames := make([]string, 0, len(fieldsMap))
		for k := range fieldsMap {
			fieldNames = append(fieldNames, k)
		}
		sort.Strings(fieldNames)
		var b strings.Builder
		for _, fieldName := range fieldNames {
			rules := fieldsMap[fieldName]
			normalize, format, unique := rules.Normalize, rules.Format, rules.Unique
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
		typed, err := decodeCurrentActionAs[flowir.EnumValidate](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		value, throw := normalizeFlowExpr(typed.Value.Source), typed.Throw
		quotedAllowed := make([]string, 0, len(typed.Allowed))
		for _, allowed := range typed.Allowed {
			quotedAllowed = append(quotedAllowed, fmt.Sprintf("%q", allowed))
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif !helpers.IsOneOf(%s, []string{%s}) {\n", pad, value, strings.Join(quotedAllowed, ", ")))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusBadRequest, \"INVALID_VALUE\", %q)", throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "list.Len":
		typed, err := decodeCurrentActionAs[flowir.ListLen](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("len(%s)", input), "int"), true

	case "convert.ToFloat":
		typed, err := decodeCurrentActionAs[flowir.ConvertToFloat](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("float64(%s)", input), "float64"), true

	case "convert.ToInt":
		typed, err := decodeCurrentActionAs[flowir.ConvertToInt](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, output := normalizeFlowExpr(typed.Input.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("int64(%s)", input), "int64"), true

	case "list.New":
		typed, err := decodeCurrentActionAs[flowir.ListNew](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, typ, capExpr := typed.Output, typed.GoType, normalizeFlowExpr(typed.Capacity.Source)
		if strings.TrimSpace(capExpr) != "" {
			return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s, 0, %s)", typ, capExpr), typ), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s, 0)", typ), typ), true

	case "map.New":
		typed, err := decodeCurrentActionAs[flowir.MapNew](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		output, typ := typed.Output, typed.GoType
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s)", typ), typ), true

	case "map.Get":
		typed, err := decodeCurrentActionAs[flowir.MapGet](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, key, output, into, defaultExpr, found := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), typed.Output, typed.Into, normalizeFlowExpr(typed.Default.Source), typed.Found
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		if into == "" {
			into = "any"
		}
		foundDeclaredBefore := false
		if found != "" {
			foundDeclaredBefore = st.declared[found]
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = into
		if found != "" {
			st.declared[found] = true
			st.pointers[found] = false
			st.types[found] = "bool"
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s_mapVal%s, _mapFound%s := %s[%s]\n", pad, sfx, sfx, input, key))
		if found != "" {
			assignFound := ":="
			if foundDeclaredBefore {
				assignFound = "="
			}
			b.WriteString(fmt.Sprintf("%s%s %s _mapFound%s\n", pad, found, assignFound, sfx))
		}
		if into == "any" {
			b.WriteString(fmt.Sprintf("%s%s %s _mapVal%s\n", pad, output, assign, sfx))
		} else {
			if assign == ":=" {
				b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, into))
			}
			b.WriteString(fmt.Sprintf("%sif _mapFound%s {\n", pad, sfx))
			b.WriteString(fmt.Sprintf("%s\tif _typedVal%s, _ok%s := _mapVal%s.(%s); _ok%s {\n", pad, sfx, sfx, sfx, into, sfx))
			b.WriteString(fmt.Sprintf("%s\t\t%s = _typedVal%s\n", pad, output, sfx))
			b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
			b.WriteString(errReturn(st, pad+"\t\t", fmt.Sprintf("fmt.Errorf(\"map.Get: value for key %%v is not %s\", %s)", into, key)))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
			if defaultExpr != "" {
				b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, output, defaultExpr))
			}
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		if into == "any" && defaultExpr != "" {
			b.WriteString(fmt.Sprintf("%sif !_mapFound%s {\n", pad, sfx))
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, output, defaultExpr))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "map.Has":
		typed, err := decodeCurrentActionAs[flowir.MapHas](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, key, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("func() bool { _, _ok := %s[%s]; return _ok }()", input, key), "bool"), true

	case "map.Set":
		typed, err := decodeCurrentActionAs[flowir.MapSet](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, key, value, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Value.Source), typed.Output
		var b strings.Builder
		target := input
		if output != "" && output != input {
			b.WriteString(renderFlowAssignTarget(st, pad, output, fmt.Sprintf("maps.Clone(%s)", input), ""))
			target = output
		}
		b.WriteString(fmt.Sprintf("%s%s[%s] = %s\n", pad, target, key, value))
		return b.String(), true

	case "map.Merge":
		typed, err := decodeCurrentActionAs[flowir.MapMerge](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		left, right, output := normalizeFlowExpr(typed.Left.Source), normalizeFlowExpr(typed.Right.Source), typed.Output
		var b strings.Builder
		b.WriteString(renderFlowAssignTarget(st, pad, output, fmt.Sprintf("maps.Clone(%s)", left), ""))
		b.WriteString(fmt.Sprintf("%smaps.Copy(%s, %s)\n", pad, output, right))
		return b.String(), true

	case "list.Enrich":
		typed, err := decodeCurrentActionAs[flowir.ListEnrich](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		items, lookupSource, lookupInput, as := normalizeFlowExpr(typed.Items.Source), typed.LookupSource, normalizeFlowExpr(typed.LookupInput.Source), typed.As
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sfor _i := range %s {\n", pad, items))
		innerPad := pad + "\t"
		b.WriteString(fmt.Sprintf("%s%s := &%s[_i]\n", innerPad, as, items))
		b.WriteString(fmt.Sprintf("%s_enriched, _eErr := s.%sRepo.FindByID(ctx, %s)\n", innerPad, ExportName(lookupSource), lookupInput))
		b.WriteString(fmt.Sprintf("%sif _eErr == nil && _enriched != nil {\n", innerPad))
		innerInnerPad := innerPad + "\t"
		for _, field := range typed.Fields {
			b.WriteString(fmt.Sprintf("%s%s.%s = _enriched.%s\n", innerInnerPad, as, field.Target, field.Source))
		}
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "time.Parse":
		typed, err := decodeCurrentActionAs[flowir.TimeParse](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		value, output, format := normalizeFlowExpr(typed.Value.Source), typed.Output, normalizeFlowExpr(typed.Format)
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

	case "time.Add":
		typed, err := decodeCurrentActionAs[flowir.TimeAdd](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		input, duration, output := normalizeFlowExpr(typed.Input.Source), normalizeFlowExpr(typed.Duration.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("%s.Add(%s)", input, duration), "time.Time"), true

	case "time.Sub":
		typed, err := decodeCurrentActionAs[flowir.TimeSub](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		a, bExpr, output := normalizeFlowExpr(typed.A.Source), normalizeFlowExpr(typed.B.Source), typed.Output
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("%s.Sub(%s)", a, bExpr), "time.Duration"), true

	case "time.Diff":
		typed, err := decodeCurrentActionAs[flowir.TimeDiff](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		from, to, output, unit := normalizeFlowExpr(typed.From.Source), normalizeFlowExpr(typed.To.Source), typed.Output, typed.Unit
		if unit == "" || unit == "duration" {
			return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("%s.Sub(%s)", to, from), "time.Duration"), true
		}
		expr := fmt.Sprintf("%s.Sub(%s).Hours()", to, from)
		switch unit {
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
			return renderInvalidFlowStepConfig(st, pad, "time.Diff", "time.Diff unit must be duration, milliseconds, seconds, minutes, hours, or days"), true
		}
		return renderFlowAssignTarget(st, pad, output, expr, "float64"), true

	case "time.CheckExpiry":
		typed, err := decodeCurrentActionAs[flowir.TimeCheckExpiry](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		value, throw, mustBe := normalizeFlowExpr(typed.Value.Source), typed.Throw, typed.MustBe
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
		typed, err := decodeCurrentActionAs[flowir.MapBuild](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		from, as, key, value, output, valueType := normalizeFlowExpr(typed.From.Source), typed.As, normalizeFlowExpr(typed.Key.Source), normalizeFlowExpr(typed.Value.Source), typed.Output, typed.ValueType
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
		typed, err := decodeCurrentActionAs[flowir.RepositoryCall](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		source := typed.Entity
		method := typed.Method
		input := normalizeFlowExpr(typed.Input.Source)
		output := typed.Output
		errMsg := typed.Error
		// required: true → auto-inject nil-check with generic "not found" message
		if typed.Required && errMsg == "" {
			errMsg = source + " not found"
		}
		if source == "" || method == "" {
			return "", true
		}
		var b strings.Builder
		inputArg := ""
		if input != "" {
			inputArg = ", " + input
		}
		// multi-arg fallback: args: ["req.TenderID", "req.CompanyID"]
		if inputArg == "" && len(typed.Arguments) > 0 {
			parts := make([]string, 0, len(typed.Arguments))
			for _, argument := range typed.Arguments {
				if source := normalizeFlowExpr(argument.Source); source != "" {
					parts = append(parts, source)
				}
			}
			if len(parts) > 0 {
				inputArg = ", " + strings.Join(parts, ", ")
			}
		}
		// list:true → output is a slice, not a pointer
		isList := typed.List
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
		st.pointers[output] = !isList
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(ctx%s)\n", pad, output+", err", assign, ExportName(source), ExportName(method), inputArg))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		if errMsg != "" {
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", errMsg)))
		} else {
			b.WriteString(errReturn(st, pad+"\t", "err"))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if errMsg != "" && !isList {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", errMsg)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "math.Expr":
		typed, err := decodeCurrentActionAs[flowir.MathExpression](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		expr, output, declare := normalizeFlowExpr(typed.Value.Source), typed.Output, typed.Declare
		assign := "="
		if declare && !st.declared[output] {
			assign = ":="
			st.declared[output] = true
			st.pointers[output] = false
		}
		return fmt.Sprintf("%s%s %s %s\n", pad, output, assign, expr), true

	case "repo.Upsert":
		source := arg("source")
		find := arg("find")
		input := arg("input")
		output := arg("output")
		if source == "" || find == "" || input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "repo.Upsert", "repo.Upsert requires source, find, input, and output"), true
		}
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
		if flowChildStepCount(st, child, "_ifNew") > 0 {
			b.WriteString(renderFlowChildSteps(ifNewState, child, "_ifNew", indent+1))
		}
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(source), output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		ifExistsState := cloneFlowState(st)
		if flowChildStepCount(st, child, "_ifExists") > 0 {
			b.WriteString(renderFlowChildSteps(ifExistsState, child, "_ifExists", indent+1))
		}
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(source), output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "fsm.Transition":
		typed, err := decodeCurrentActionAs[flowir.FSMTransition](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		entity, to := normalizeFlowExpr(typed.Entity.Source), typed.To
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := %s.TransitionTo(%q); err != nil {\n", pad, entity, to))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "notification.Dispatch", "notify.Dispatch":
		typed, err := decodeCurrentActionAs[flowir.NotificationDispatch](st, step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		event, userID, entityID, msgType, payload, tmpl := normalizeFlowExpr(typed.Event.Source), normalizeFlowExpr(typed.UserID.Source), normalizeFlowExpr(typed.EntityID.Source), normalizeFlowExpr(typed.Type.Source), normalizeFlowExpr(typed.Payload.Source), normalizeFlowExpr(typed.Template.Source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif s.dispatcher == nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("notify.Dispatch: notification dispatcher wiring is not configured")`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif _dispatchErr := s.dispatcher.Dispatch(ctx, port.NotificationMessage{Event: strings.TrimSpace(fmt.Sprint(%s))", pad, event))
		if msgType != "" {
			b.WriteString(fmt.Sprintf(", Type: strings.TrimSpace(fmt.Sprint(%s))", msgType))
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
			b.WriteString(fmt.Sprintf(", Template: strings.TrimSpace(fmt.Sprint(%s))", tmpl))
		}
		b.WriteString("}); _dispatchErr != nil {\n")
		b.WriteString(errReturn(st, pad+"\t", `fmt.Errorf("notify.Dispatch: %w", _dispatchErr)`))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}

	return "", false
}
