package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
		message := arg("message")
		if message == "" {
			return renderInvalidFlowStepConfig(st, pad, "errors.New", "errors.New requires message"), true
		}
		statusExpr := flowHTTPStatusExpr(step.Args["status"], "http.StatusInternalServerError")
		codeExpr := arg("code")
		if codeExpr == "" {
			codeExpr = `"ERROR"`
		}
		errExpr := fmt.Sprintf("errors.New(%s, %s, %s)", statusExpr, codeExpr, message)
		output := arg("output")
		throwNow := false
		if raw, ok := step.Args["throw"].(bool); ok {
			throwNow = raw
		}
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
		cond := arg("condition")
		throw := arg("throw")
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
		cond := arg("condition")
		throw := arg("throw")
		if cond == "" || throw == "" {
			return renderInvalidFlowStepConfig(st, pad, "errors.ThrowIf", "errors.ThrowIf requires condition and throw"), true
		}
		httpStatus := "http.StatusBadRequest"
		statusLabel := "Error"
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
			default:
				httpStatus = s
			}
		}
		if code := strings.TrimSpace(arg("code")); code != "" {
			statusLabel = code
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, cond))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", httpStatus, statusLabel, throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.Wrap":
		errExpr := arg("err")
		message := arg("message")
		output := arg("output")
		if errExpr == "" || message == "" {
			return renderInvalidFlowStepConfig(st, pad, "errors.Wrap", "errors.Wrap requires err and message"), true
		}
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
		input := arg("input")
		output := arg("output")
		mode := strings.TrimSpace(arg("mode"))
		if mode == "" {
			mode = "contains"
		}
		rawCases := map[string]map[string]string{}
		switch v := step.Args["cases"].(type) {
		case map[string]map[string]string:
			rawCases = v
		case map[string]any:
			for key, rawCase := range v {
				switch cfg := rawCase.(type) {
				case map[string]string:
					rawCases[key] = cfg
				case map[string]any:
					flat := map[string]string{}
					for ck, cv := range cfg {
						flat[ck] = fmt.Sprint(cv)
					}
					rawCases[key] = flat
				}
			}
		}
		if input == "" || len(rawCases) == 0 {
			return renderInvalidFlowStepConfig(st, pad, "errors.Map", "errors.Map requires input and cases"), true
		}
		keys := make([]string, 0, len(rawCases))
		for k := range rawCases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		defaultMessage := strings.TrimSpace(arg("defaultMessage"))
		defaultCode := strings.TrimSpace(arg("defaultCode"))
		defaultStatus := strings.TrimSpace(arg("defaultStatus"))
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
			status := strings.TrimSpace(caseCfg["status"])
			if status == "" {
				status = "http.StatusBadGateway"
			}
			code := strings.TrimSpace(caseCfg["code"])
			if code == "" {
				code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), " ", "_"))
			}
			message := strings.TrimSpace(caseCfg["message"])
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
		to := arg("to")
		val := arg("value")
		if to == "" || val == "" {
			return renderInvalidFlowStepConfig(st, pad, "mapping.Assign", "mapping.Assign requires to and value"), true
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
		userID := arg("userID")
		companyID := arg("companyID")
		roles := arg("roles")
		output := arg("output")
		if userID == "" || companyID == "" || roles == "" {
			return renderInvalidFlowStepConfig(st, pad, "auth.RequireRole", "auth.RequireRole requires userID, companyID, and roles"), true
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
			return renderInvalidFlowStepConfig(st, pad, "auth.CheckRole", "auth.CheckRole requires user and roles"), true
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
			return renderInvalidFlowStepConfig(st, pad, "entity.PatchNonZero", "entity.PatchNonZero requires target, from, and fields"), true
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
			return renderInvalidFlowStepConfig(st, pad, "field.CopyNonEmpty", "field.CopyNonEmpty requires from and to"), true
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
			return renderInvalidFlowStepConfig(st, pad, "entity.PatchValidated", "entity.PatchValidated requires target and from"), true
		}
		fieldsMap, ok := step.Args["fields"].(map[string]map[string]string)
		if !ok || len(fieldsMap) == 0 {
			return renderInvalidFlowStepConfig(st, pad, "entity.PatchValidated", "entity.PatchValidated requires fields map"), true
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
			return renderInvalidFlowStepConfig(st, pad, "enum.Validate", "enum.Validate requires value, allowed, and throw"), true
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

	case "list.Len":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Len", "list.Len requires input and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("len(%s)", input), "int"), true

	case "convert.ToFloat":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "convert.ToFloat", "convert.ToFloat requires input and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("float64(%s)", input), "float64"), true

	case "convert.ToInt":
		input := arg("input")
		output := arg("output")
		if input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "convert.ToInt", "convert.ToInt requires input and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("int64(%s)", input), "int64"), true

	case "list.New":
		output := arg("output")
		typ := arg("type")
		if output == "" || typ == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.New", "list.New requires output and type"), true
		}
		capExpr := arg("cap")
		if strings.TrimSpace(capExpr) != "" {
			return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s, 0, %s)", typ, capExpr), typ), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s, 0)", typ), typ), true

	case "map.New":
		output := arg("output")
		typ := arg("type")
		if output == "" || typ == "" {
			return renderInvalidFlowStepConfig(st, pad, "map.New", "map.New requires output and type"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("make(%s)", typ), typ), true

	case "map.Get":
		input := arg("input")
		key := arg("key")
		output := arg("output")
		into := arg("into")
		defaultExpr := arg("default")
		found := arg("found")
		if input == "" || key == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "map.Get", "map.Get requires input, key, and output"), true
		}
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
		input := arg("input")
		key := arg("key")
		output := arg("output")
		if input == "" || key == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "map.Has", "map.Has requires input, key, and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("func() bool { _, _ok := %s[%s]; return _ok }()", input, key), "bool"), true

	case "map.Set":
		input := arg("input")
		key := arg("key")
		value := arg("value")
		output := arg("output")
		if input == "" || key == "" || value == "" {
			return renderInvalidFlowStepConfig(st, pad, "map.Set", "map.Set requires input, key, and value"), true
		}
		var b strings.Builder
		target := input
		if output != "" && output != input {
			b.WriteString(renderFlowAssignTarget(st, pad, output, fmt.Sprintf("maps.Clone(%s)", input), ""))
			target = output
		}
		b.WriteString(fmt.Sprintf("%s%s[%s] = %s\n", pad, target, key, value))
		return b.String(), true

	case "map.Merge":
		left := arg("left")
		right := arg("right")
		output := arg("output")
		if left == "" || right == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "map.Merge", "map.Merge requires left, right, and output"), true
		}
		var b strings.Builder
		b.WriteString(renderFlowAssignTarget(st, pad, output, fmt.Sprintf("maps.Clone(%s)", left), ""))
		b.WriteString(fmt.Sprintf("%smaps.Copy(%s, %s)\n", pad, output, right))
		return b.String(), true

	case "list.Enrich":
		items := arg("items")
		lookupSource := arg("lookupSource")
		lookupInput := arg("lookupInput")
		set := arg("set")
		as := arg("as")
		if items == "" || lookupSource == "" || lookupInput == "" || set == "" {
			return renderInvalidFlowStepConfig(st, pad, "list.Enrich", "list.Enrich requires items, lookupSource, lookupInput, and set"), true
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
			return renderInvalidFlowStepConfig(st, pad, "time.Parse", "time.Parse requires value and output"), true
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

	case "time.Add":
		input := arg("input")
		duration := arg("duration")
		output := arg("output")
		if input == "" || duration == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "time.Add", "time.Add requires input, duration, and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("%s.Add(%s)", input, duration), "time.Time"), true

	case "time.Sub":
		a := arg("a")
		bExpr := arg("b")
		output := arg("output")
		if a == "" || bExpr == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "time.Sub", "time.Sub requires a, b, and output"), true
		}
		return renderFlowAssignTarget(st, pad, output, fmt.Sprintf("%s.Sub(%s)", a, bExpr), "time.Duration"), true

	case "time.Diff":
		from := arg("from")
		to := arg("to")
		output := arg("output")
		unit := strings.TrimSpace(arg("unit"))
		if from == "" || to == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "time.Diff", "time.Diff requires from, to, and output"), true
		}
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
		value := arg("value")
		throw := arg("throw")
		mustBe := arg("mustBe")
		if value == "" || throw == "" {
			return renderInvalidFlowStepConfig(st, pad, "time.CheckExpiry", "time.CheckExpiry requires value and throw"), true
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
			return renderInvalidFlowStepConfig(st, pad, "map.Build", "map.Build requires from, key, value, and output"), true
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
		// multi-arg fallback: args: ["req.TenderID", "req.CompanyID"]
		if inputArg == "" {
			if v, ok := step.Args["args"]; ok {
				switch x := v.(type) {
				case []string:
					if len(x) > 0 {
						inputArg = ", " + strings.Join(x, ", ")
					}
				case []interface{}:
					var parts []string
					for _, item := range x {
						if s, ok := item.(string); ok && s != "" {
							parts = append(parts, normalizeFlowExpr(strings.TrimSpace(s)))
						}
					}
					if len(parts) > 0 {
						inputArg = ", " + strings.Join(parts, ", ")
					}
				case string:
					if x != "" {
						inputArg = ", " + x
					}
				}
			}
		}
		// list:true → output is a slice, not a pointer
		isList := false
		if b2, ok := step.Args["list"].(bool); ok && b2 {
			isList = true
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
		expr := arg("expr")
		output := arg("output")
		if expr == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "math.Expr", "math.Expr requires expr and output"), true
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
			return renderInvalidFlowStepConfig(st, pad, "notify.Dispatch", "notify.Dispatch requires event or message"), true
		}
		userID := arg("userID")
		entityID := arg("entityID")
		msgType := arg("type")
		payload := arg("payload")
		tmpl := arg("template")
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
