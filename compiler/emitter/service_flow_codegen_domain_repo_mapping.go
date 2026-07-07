package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderFlowStepDomainRepoMapping(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)
	var repositoryCall flowir.RepositoryCall
	if strings.HasPrefix(step.Action, "repo.") {
		decoded, err := flowir.DecodeAs[flowir.RepositoryCall](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Action, err.Error()), true
		}
		repositoryCall = decoded
	}

	switch step.Action {
	case "repo.Exists":
		source := repositoryCall.Entity
		input := repositoryCall.Input.Source
		output := repositoryCall.Output
		method := repositoryCall.Method
		if source == "" || input == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "repo.Exists", "repo.Exists requires source, input, and output"), true
		}
		if method == "" {
			method = "FindByID"
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "bool"
		var b strings.Builder
		if strings.HasPrefix(method, "Exists") || strings.HasPrefix(method, "Has") {
			b.WriteString(fmt.Sprintf("%s%s, err %s s.%sRepo.%s(ctx, %s)\n", pad, output, assign, ExportName(source), method, input))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		b.WriteString(fmt.Sprintf("%s_repoExists%s, err := s.%sRepo.%s(ctx, %s)\n", pad, sfx, ExportName(source), method, input))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		b.WriteString(fmt.Sprintf("%s%s %s _repoExists%s != nil\n", pad, output, assign, sfx))
		return b.String(), true

	case "repo.Count":
		source := repositoryCall.Entity
		method := repositoryCall.Method
		output := repositoryCall.Output
		input := repositoryCall.Input.Source
		if source == "" || method == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "repo.Count", "repo.Count requires source, method, and output"), true
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = "int"
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
		source := repositoryCall.Entity
		if source == "" {
			return "", true
		}
		method := repositoryCall.Method
		input := repositoryCall.Input.Source
		output := repositoryCall.Output
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
			if errMsg := repositoryCall.Error; errMsg != "" && step.Action != "repo.List" {
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
		source := repositoryCall.Entity
		if source == "" {
			return "", true
		}
		method := repositoryCall.Method
		if method == "" {
			if step.Action == "repo.Save" {
				method = "Save"
			} else {
				method = "Delete"
			}
		}
		input := repositoryCall.Input.Source
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
		typed, err := flowir.DecodeAs[flowir.MappingAssign](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, "mapping.Assign", err.Error()), true
		}
		to := normalizeFlowExpr(typed.Target.Source)
		val := normalizeFlowExpr(typed.Value.Source)
		if to == "" || val == "" {
			return "", true
		}
		declare := typed.Declare
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

	case "repo.Query":
		source := repositoryCall.Entity
		method := repositoryCall.Method
		input := repositoryCall.Input.Source
		output := repositoryCall.Output
		errMsg := repositoryCall.Error
		if source == "" || method == "" {
			return "", true
		}
		var b strings.Builder
		inputArg := ""
		if input != "" {
			inputArg = ", " + input
		}
		// Multi-arg support: args: ["req.TenderID", "req.CompanyID"]
		if inputArg == "" && len(repositoryCall.Arguments) > 0 {
			args := make([]string, 0, len(repositoryCall.Arguments))
			for _, expression := range repositoryCall.Arguments {
				args = append(args, expression.Source)
			}
			inputArg = ", " + strings.Join(args, ", ")
		}
		// list:true → output is a slice, not a pointer
		isList := repositoryCall.List
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

	case "repo.Upsert":
		source := repositoryCall.Entity
		find := repositoryCall.Find.Source
		input := repositoryCall.Input.Source
		output := repositoryCall.Output
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

	// -------------------------------------------------------------------------
	// db.* — explicit DB primitives (aliases and new methods)
	// -------------------------------------------------------------------------

	case "db.Get", "db.List":
		// Aliases for repo.Get / repo.List
		source := arg("source")
		if source == "" {
			return renderInvalidFlowStepConfig(st, pad, step.Action, step.Action+" requires source"), true
		}
		method := arg("method")
		input := arg("input")
		output := arg("output")
		isList := step.Action == "db.List"
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
		if output != "" {
			assign := ":="
			if st.declared[output] {
				assign = "="
			}
			st.declared[output] = true
			st.pointers[output] = !isList
			if isList {
				st.types[output] = "[]domain." + ExportName(source)
			} else {
				st.types[output] = "*domain." + ExportName(source)
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.%s(%s)\n", pad, output+", err", assign, ExportName(source), method, call))
			b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			if errMsg := arg("error"); errMsg != "" && !isList {
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

	case "db.Query":
		// Alias for repo.Query
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

	case "db.Insert":
		// Pure INSERT — error on duplicate PK
		source := arg("source")
		input := arg("input")
		if source == "" || input == "" {
			return "", true
		}
		inputArg := input
		if !strings.HasPrefix(input, "&") && !st.pointers[input] {
			inputArg = "&" + input
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Insert(ctx, %s); err != nil {\n", pad, ExportName(source), inputArg))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "db.Update":
		// UPDATE only — error if 0 rows affected
		source := arg("source")
		input := arg("input")
		if source == "" || input == "" {
			return "", true
		}
		inputArg := input
		if !strings.HasPrefix(input, "&") && !st.pointers[input] {
			inputArg = "&" + input
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Update(ctx, %s); err != nil {\n", pad, ExportName(source), inputArg))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "db.Upsert":
		// Simple alias for Save (no find+branch)
		source := arg("source")
		input := arg("input")
		if source == "" || input == "" {
			return "", true
		}
		inputArg := input
		if !strings.HasPrefix(input, "&") && !st.pointers[input] {
			inputArg = "&" + input
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Save(ctx, %s); err != nil {\n", pad, ExportName(source), inputArg))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "db.Delete":
		// Alias for repo.Delete
		source := arg("source")
		input := arg("input")
		if source == "" || input == "" {
			return "", true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := s.%sRepo.Delete(ctx, %s); err != nil {\n", pad, ExportName(source), input))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "db.Lock", "db.SelectForUpdate":
		// SELECT FOR UPDATE — must be inside tx.Block
		source := arg("source")
		input := arg("input")
		output := arg("output")
		if source == "" || input == "" {
			return "", true
		}
		if output == "" {
			output = strings.ToLower(source[:1]) + source[1:]
		}
		assign := ":="
		if st.declared[output] {
			assign = "="
		}
		st.declared[output] = true
		st.pointers[output] = true
		st.types[output] = "*domain." + ExportName(source)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s%s %s s.%sRepo.LockByID(ctx, %s)\n", pad, output+", err", assign, ExportName(source), input))
		b.WriteString(fmt.Sprintf("%sif err != nil {\n", pad))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		if errMsg := arg("error"); errMsg != "" {
			b.WriteString(fmt.Sprintf("%sif %s == nil {\n", pad, output))
			b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("errors.New(http.StatusNotFound, \"Not Found\", %q)", errMsg)))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
		}
		return b.String(), true
	}

	return "", false
}
