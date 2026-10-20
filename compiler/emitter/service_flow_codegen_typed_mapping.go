package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderTypedStepMapping(st *flowRenderState, step flowir.TypedStep, indent int) (string, bool) {
	pad := strings.Repeat("\t", indent)
	switch step.Name {
	case "mapping.Assign":
		typed, err := typedActionAs[flowir.MappingAssign](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		target := normalizeFlowExpr(typed.Target.Source)
		value := normalizeFlowExpr(typed.Value.Source)
		if typed.Declare && !st.declared[target] {
			st.declared[target] = true
			st.pointers[target] = false
			return fmt.Sprintf("%s%s := %s\n", pad, target, value), true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, target, value))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true

	case "mapping.Map":
		typed, err := typedActionAs[flowir.MappingMap](step)
		if err != nil {
			return renderInvalidFlowStepConfig(st, pad, step.Name, err.Error()), true
		}
		input := normalizeFlowExpr(typed.Input.Source)
		output := typed.Output
		outputRef := output
		if strings.Contains(output, ".") {
			parts := strings.Split(output, ".")
			parts[len(parts)-1] = ExportName(parts[len(parts)-1])
			outputRef = strings.Join(parts, ".")
		}
		if typed.Entity != "" && !st.declared[output] && !strings.Contains(output, ".") {
			st.declared[output] = true
			st.pointers[output] = false
			var b strings.Builder
			b.WriteString(fmt.Sprintf("%svar %s domain.%s\n", pad, output, typed.Entity))
			if input == "" {
				return b.String(), true
			}
			b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, output, input))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		var b strings.Builder
		// A body mapped onto a row that is already loaded (AdminUpdateX)
		// must not write its secret or @managed() fields: the row keeps them.
		if keep := bodyProtectedFields(st, typed.Entity, output); len(keep) > 0 {
			quoted := make([]string, len(keep))
			for i, name := range keep {
				quoted[i] = fmt.Sprintf("%q", ExportName(name))
			}
			b.WriteString(fmt.Sprintf("%sif err := helpers.AssignKeeping(&%s, %s, %s); err != nil {\n", pad, outputRef, input, strings.Join(quoted, ", ")))
			b.WriteString(errReturn(st, pad+"\t", "err"))
			b.WriteString(fmt.Sprintf("%s}\n", pad))
			return b.String(), true
		}
		b.WriteString(fmt.Sprintf("%sif err := helpers.Assign(&%s, %s); err != nil {\n", pad, outputRef, input))
		b.WriteString(errReturn(st, pad+"\t", "err"))
		b.WriteString(fmt.Sprintf("%s}\n", pad))
		return b.String(), true
	}
	return "", false
}

// bodyProtectedFields: the fields mapping.Map onto an already loaded entity
// variable keeps (normalizer.BodyProtectedFields); none for a new variable,
// a nested target or an input that is not mapped from outside the row.
func bodyProtectedFields(st *flowRenderState, entity, output string) []string {
	if entity == "" || strings.Contains(output, ".") || !st.declared[output] {
		return nil
	}
	ent, ok := st.entityDefsByName[entity]
	if !ok {
		return nil
	}
	return normalizer.BodyProtectedFields(ent)
}
