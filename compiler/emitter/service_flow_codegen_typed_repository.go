package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/flowir"
)

// renderTypedStepRepositoryBasic emits the common repository operations
// directly from RepositoryCall. It deliberately has no normalizer adapter.
func renderTypedStepRepositoryBasic(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	action, err := typedActionAs[flowir.RepositoryCall](step)
	pad := strings.Repeat("\t", indent)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	source := action.Entity
	input := normalizeFlowExpr(action.Input.Source)
	output := action.Output
	method := action.Method

	switch step.Name {
	case "repo.Exists":
		if source == "" || input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "repo.Exists requires source, input, and output"), true
		}
		if method == "" {
			method = "FindByID"
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output], st.pointers[output], st.types[output] = true, false, "bool"
		var b strings.Builder
		if strings.HasPrefix(method, "Exists") || strings.HasPrefix(method, "Has") {
			b.WriteString(fmt.Sprintf("%s%s, err %s s.%sRepo.%s(ctx, %s)\n", pad, output, assign, ExportName(source), method, input))
		} else {
			b.WriteString(fmt.Sprintf("%s_repoExists%s, err := s.%sRepo.%s(ctx, %s)\n", pad, sfx, ExportName(source), method, input))
		}
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if !strings.HasPrefix(method, "Exists") && !strings.HasPrefix(method, "Has") {
			b.WriteString(fmt.Sprintf("%s%s %s _repoExists%s != nil\n", pad, output, assign, sfx))
		}
		return b.String(), true

	case "repo.Count":
		if source == "" || method == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Name, "repo.Count requires source, method, and output"), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output], st.pointers[output], st.types[output] = true, false, "int"
		call := "ctx"
		if input != "" {
			call += ", " + input
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n", pad, output+", err", assign, ExportName(source), method, call))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "repo.Get", "repo.Find", "repo.GetForUpdate", "repo.List":
		if source == "" {
			return "", true
		}
		if method == "" {
			switch step.Name {
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
		var b strings.Builder
		if output == "" {
			b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = step.Name != "repo.List"
		if step.Name == "repo.List" {
			st.types[output] = "[]domain." + ExportName(source)
		} else {
			st.types[output] = "*domain." + ExportName(source)
		}
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n", pad, output+", err", assign, ExportName(source), method, call))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if action.Error != "" && step.Name != "repo.List" {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", action.Error)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "repo.Save", "repo.Delete":
		if source == "" {
			return "", true
		}
		if method == "" {
			if step.Name == "repo.Save" {
				method = "Save"
			} else {
				method = "Delete"
			}
		}
		call := "ctx"
		if input != "" {
			inputArg := input
			if step.Name == "repo.Save" && !strings.HasPrefix(input, "&") && !st.pointers[input] {
				inputArg = "&" + input
			}
			call += ", " + inputArg
		}
		var b strings.Builder
		if step.Name == "repo.Delete" && strings.HasPrefix(method, "DeleteBy") {
			b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		} else {
			b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(source), method, call))
		}
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func renderTypedStepRepositoryAdvanced(st *flowRenderState, step flowir.TypedStep, indent int, sfx string) (string, bool) {
	action, err := typedActionAs[flowir.RepositoryCall](step)
	pad := strings.Repeat("\t", indent)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}

	switch step.Name {
	case "repo.Query":
		if action.Entity == "" || action.Method == "" {
			return "", true
		}
		inputArg := ""
		if input := normalizeFlowExpr(action.Input.Source); input != "" {
			inputArg = ", " + input
		} else if len(action.Arguments) > 0 {
			args := make([]string, 0, len(action.Arguments))
			for _, expression := range action.Arguments {
				args = append(args, normalizeFlowExpr(expression.Source))
			}
			inputArg = ", " + strings.Join(args, ", ")
		}
		var b strings.Builder
		if action.Output == "" {
			b.WriteString(fmt.Sprintf("%sif _, _qrErr := s.%sRepo.%s(ctx%s); _qrErr != nil {\n", pad, ExportName(action.Entity), ExportName(action.Method), inputArg))
			b.WriteString(errReturn(st, pad+"\t", "_qrErr"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = !action.List
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(ctx%s)\n", pad, action.Output+", err", assign, ExportName(action.Entity), ExportName(action.Method), inputArg))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		if action.Error != "" {
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", action.Error)))
		} else {
			b.WriteString(errReturn(st, pad+"\t", "err"))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if action.Error != "" && !action.List {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, action.Output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", action.Error)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "repo.Upsert":
		find := normalizeFlowExpr(action.Find.Source)
		input := normalizeFlowExpr(action.Input.Source)
		if action.Entity == "" || find == "" || input == "" || action.Output == "" {
			return "", true
		}
		assign := ":="
		if st.declared[action.Output] {
			assign = "="
		}
		st.declared[action.Output] = true
		st.pointers[action.Output] = true
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.FindByID(ctx, %s)\n", pad, action.Output+", err", assign, ExportName(action.Entity), find))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, action.Output))
		innerPad := pad + "\t"
		b.WriteString(fmt.Sprintf("%s_uNew := %s\n", innerPad, input))
		b.WriteString(fmt.Sprintf("%s%s = &_uNew\n", innerPad, action.Output))
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), step.Children["_ifNew"], indent+1))
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(action.Entity), action.Output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s} else {\n", pad))
		b.WriteString(renderTypedFlowSteps(cloneFlowState(st), step.Children["_ifExists"], indent+1))
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", innerPad, ExportName(action.Entity), action.Output))
		b.WriteString(errReturn(st, innerPad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", innerPad))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

func typedDBFields(step flowir.TypedStep) (flowir.DBFields, error) {
	switch step.Name {
	case "db.Get":
		action, err := typedActionAs[flowir.DBGet](step)
		return action.DBFields, err
	case "db.List":
		action, err := typedActionAs[flowir.DBList](step)
		return action.DBFields, err
	case "db.Query":
		action, err := typedActionAs[flowir.DBQuery](step)
		return action.DBFields, err
	case "db.Insert":
		action, err := typedActionAs[flowir.DBInsert](step)
		return action.DBFields, err
	case "db.Update":
		action, err := typedActionAs[flowir.DBUpdate](step)
		return action.DBFields, err
	case "db.Upsert":
		action, err := typedActionAs[flowir.DBUpsert](step)
		return action.DBFields, err
	case "db.Delete":
		action, err := typedActionAs[flowir.DBDelete](step)
		return action.DBFields, err
	case "db.Lock":
		action, err := typedActionAs[flowir.DBLock](step)
		return action.DBFields, err
	case "db.SelectForUpdate":
		action, err := typedActionAs[flowir.DBSelectForUpdate](step)
		return action.DBFields, err
	}
	return flowir.DBFields{}, fmt.Errorf("unsupported DB action %q", step.Name)
}

// renderTypedStepDB emits explicit database primitives from their typed DBFields.
func renderTypedStepDB(st *flowRenderState, step flowir.TypedStep, indent int, _ string) (string, bool) {
	pad := strings.Repeat("\t", indent)
	fields, err := typedDBFields(step)
	if err != nil {
		return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
	}
	input := normalizeFlowExpr(fields.Input.Source)

	switch step.Name {
	case "db.Get", "db.List":
		isList := step.Name == "db.List"
		method := fields.Method
		if method == "" {
			if isList {
				method = "ListAll"
			} else {
				method = "FindByID"
			}
		}
		call := "ctx"
		if input != "" {
			call += ", " + input
		}
		var b strings.Builder
		if fields.Output == "" {
			b.WriteString(fmt.Sprintf("%sif _, err := s.%sRepo.%s(%s); err != nil {\n", pad, ExportName(fields.Source), method, call))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		assign := ":="
		if st.declared[fields.Output] {
			assign = "="
		}
		st.declared[fields.Output] = true
		st.pointers[fields.Output] = !isList
		if isList {
			st.types[fields.Output] = "[]domain." + ExportName(fields.Source)
		} else {
			st.types[fields.Output] = "*domain." + ExportName(fields.Source)
		}
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n", pad, fields.Output+", err", assign, ExportName(fields.Source), method, call))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if fields.Error != "" && !isList {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, fields.Output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", fields.Error)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "db.Query":
		inputArg := ""
		if input != "" {
			inputArg = ", " + input
		}
		var b strings.Builder
		if fields.Output == "" {
			b.WriteString(fmt.Sprintf("%sif _, _qrErr := s.%sRepo.%s(ctx%s); _qrErr != nil {\n", pad, ExportName(fields.Source), ExportName(fields.Method), inputArg))
			b.WriteString(errReturn(st, pad+"\t", "_qrErr"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		assign := ":="
		if st.declared[fields.Output] {
			assign = "="
		}
		st.declared[fields.Output] = true
		st.pointers[fields.Output] = true
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(ctx%s)\n", pad, fields.Output+", err", assign, ExportName(fields.Source), ExportName(fields.Method), inputArg))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		if fields.Error != "" {
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", fields.Error)))
		} else {
			b.WriteString(errReturn(st, pad+"\t", "err"))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if fields.Error != "" {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, fields.Output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"NOT_FOUND\", %q)", fields.Error)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true

	case "db.Insert", "db.Update", "db.Upsert":
		inputArg := input
		if !strings.HasPrefix(input, "&") && !st.pointers[input] {
			inputArg = "&" + input
		}
		method := map[string]string{"db.Insert": "Insert", "db.Update": "Update", "db.Upsert": "Save"}[step.Name]
		return fmt.Sprintf("%sif err := s.%sRepo.%s(ctx, %s); err != nil {\n%s%s}\n", pad, ExportName(fields.Source), method, inputArg, errReturn(st, pad+"\t", "err"), pad), true

	case "db.Delete":
		return fmt.Sprintf("%sif err := s.%sRepo.Delete(ctx, %s); err != nil {\n%s%s}\n", pad, ExportName(fields.Source), input, errReturn(st, pad+"\t", "err"), pad), true

	case "db.Lock", "db.SelectForUpdate":
		assign := ":="
		if st.declared[fields.Output] {
			assign = "="
		}
		st.declared[fields.Output] = true
		st.pointers[fields.Output] = true
		st.types[fields.Output] = "*domain." + ExportName(fields.Source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.LockByID(ctx, %s)\n", pad, fields.Output+", err", assign, ExportName(fields.Source), input))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if fields.Error != "" {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, fields.Output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", fields.Error)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true
	}
	return "", false
}
