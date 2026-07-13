package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepDomainErrors owns the direct typed dispatch for the simple
// error actions. No normalizer metadata or ScalarArgs are part of this path.
func renderTypedStepDomainErrors(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "errors.New":
		action, err := typedActionAs[flowir.ErrorNew](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		message := normalizeFlowExpr(action.Message.Source)
		statusExpr := flowHTTPStatusExpr(action.Status.Source, "http.StatusInternalServerError")
		codeExpr := normalizeFlowExpr(action.Code.Source)
		if codeExpr == "" {
			codeExpr = `"ERROR"`
		}
		errExpr := fmt.Sprintf("errors.New(%s, %s, %s)", statusExpr, codeExpr, message)
		if action.Output == "" {
			return errReturn(st, pad, errExpr), true
		}
		var b strings.Builder
		if !st.declared[action.Output] {
			b.WriteString(fmt.Sprintf("%svar %s error\n", pad, action.Output))
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		st.types[action.Output] = "error"
		b.WriteString(fmt.Sprintf("%s%s = %s\n", pad, action.Output, errExpr))
		if action.Throw {
			b.WriteString(errReturn(st, pad, action.Output))
		}
		return b.String(), true

	case "errors.ThrowIf":
		action, err := typedActionAs[flowir.ErrorThrowIf](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		condition := normalizeFlowExpr(action.Condition.Source)
		status := "http.StatusBadRequest"
		label := "Error"
		if raw := strings.TrimSpace(action.Status.Source); raw != "" {
			switch raw {
			case "403", "forbidden":
				status, label = "http.StatusForbidden", "Forbidden"
			case "404", "not_found":
				status, label = "http.StatusNotFound", "Not Found"
			case "409", "conflict":
				status, label = "http.StatusConflict", "Conflict"
			case "401", "unauthorized":
				status, label = "http.StatusUnauthorized", "Unauthorized"
			default:
				status = raw
			}
		}
		if code := strings.TrimSpace(normalizeFlowExpr(action.Code.Source)); code != "" {
			label = code
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif %s {\n", pad, condition))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(%s, %q, %q)", status, label, action.Throw)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.Wrap":
		action, err := typedActionAs[flowir.ErrorWrap](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		errExpr := normalizeFlowExpr(action.Error.Source)
		message := normalizeFlowExpr(action.Message.Source)
		if action.Output == "" {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"%%s: %%w\", %s, %s)", message, errExpr)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		var b strings.Builder
		if !st.declared[action.Output] {
			b.WriteString(fmt.Sprintf("%svar %s error\n", pad, action.Output))
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = false
		st.types[action.Output] = "error"
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errExpr))
		b.WriteString(fmt.Sprintf("%s\t%s = fmt.Errorf(\"%%s: %%w\", %s, %s)\n", pad, action.Output, message, errExpr))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		b.WriteString(fmt.Sprintf("%s\t%s = nil\n", pad, action.Output))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "errors.Map":
		action, err := typedActionAs[flowir.ErrorMap](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(action.Input.Source)
		keys := make([]string, 0, len(action.Cases))
		for key := range action.Cases {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		defaultCode := action.DefaultCode
		if defaultCode == "" {
			defaultCode = "INTERNAL_ERROR"
		}
		defaultStatus := action.DefaultStatus
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
			cfg := action.Cases[key]
			status := strings.TrimSpace(cfg.Status)
			if status == "" {
				status = "http.StatusBadGateway"
			}
			code := strings.TrimSpace(cfg.Code)
			if code == "" {
				code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(key), " ", "_"))
			}
			message := strings.TrimSpace(cfg.Message)
			if message == "" {
				message = key
			}
			prefix := "if"
			if i > 0 {
				prefix = "} else if"
			}
			if action.Mode == "equals" {
				b.WriteString(fmt.Sprintf("%s\t%s %s == %q {\n", pad, prefix, msgVar, key))
			} else {
				b.WriteString(fmt.Sprintf("%s\t%s strings.Contains(%s, %q) {\n", pad, prefix, msgVar, key))
			}
			b.WriteString(fmt.Sprintf("%s\t\t%s = errors.New(%s, %q, %q)\n", pad, errVar, status, code, message))
		}
		if len(keys) > 0 {
			if action.DefaultMessage != "" {
				b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
				b.WriteString(fmt.Sprintf("%s\t\t%s = errors.New(%s, %q, %q)\n", pad, errVar, defaultStatus, defaultCode, action.DefaultMessage))
				b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			} else {
				b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			}
		}
		if action.Output != "" {
			if !st.declared[action.Output] {
				b.WriteString(fmt.Sprintf("%s\tvar %s error\n", pad, action.Output))
			}
			st.declared[action.Output] = true
			st.pointers[action.Output] = false
			st.types[action.Output] = "error"
			b.WriteString(fmt.Sprintf("%s\tif %s != nil {\n", pad, errVar))
			b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, action.Output, errVar))
			b.WriteString(fmt.Sprintf("%s\t} else {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t\t%s = %s\n", pad, action.Output, input))
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
	}
	return "", false
}

func renderTypedStepDomainAuth(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "auth.RequireRole":
		action, err := typedActionAs[flowir.AuthRequireRole](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		userID := normalizeFlowExpr(action.UserID.Source)
		companyID := normalizeFlowExpr(action.CompanyID.Source)
		roles := normalizeFlowExpr(action.Roles.Source)
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = true
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.UserRepo.FindByID(ctx, %s)\n", pad, action.Output+", err", assign, userID))
		b.WriteString(fmt.Sprintf("%sif err != nil || %s == nil {\n", pad, action.Output))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"User not found\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if action.AdminBypass {
			b.WriteString(fmt.Sprintf("%sif %s.CompanyID != %s {\n", pad, action.Output, companyID))
			b.WriteString(fmt.Sprintf("%s\tif %s.Role != \"admin\" {\n", pad, action.Output))
			b.WriteString(errReturn(st, pad+"\t\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Access denied\")"))
			b.WriteString(fmt.Sprintf("%s\t}\n", pad))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		} else {
			b.WriteString(fmt.Sprintf("%sif %s.CompanyID != %s {\n", pad, action.Output, companyID))
			b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Access denied\")"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		b.WriteString(fmt.Sprintf("%sif !helpers.HasRole(%s.Role, %s) {\n", pad, action.Output, roles))
		b.WriteString(errReturn(st, pad+"\t", "errors.New(http.StatusForbidden, \"FORBIDDEN\", \"Insufficient role\")"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "auth.CheckRole":
		action, err := typedActionAs[flowir.AuthCheckRole](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		user := normalizeFlowExpr(action.User.Source)
		roles := normalizeFlowExpr(action.Roles.Source)
		companyID := normalizeFlowExpr(action.CompanyID.Source)
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
	}
	return "", false
}
