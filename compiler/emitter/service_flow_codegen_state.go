package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func renderFlowStepState(st *flowRenderState, step normalizer.FlowStep, indent int, sfx string, arg func(string) string, child func(string) []normalizer.FlowStep) (string, bool) {
	pad := strings.Repeat("\t", indent)

	switch step.Action {
	case "state.Get":
		key := arg("key")
		output := arg("output")
		defVal := arg("default")
		if key == "" || output == "" {
			return renderInvalidFlowStepConfig(st, pad, "state.Get", "state.Get requires key and output"), true
		}
		into := arg("into")

		rawVar := "_stateRaw" + sfx
		errVar := "_stateErr" + sfx

		outputType := strings.TrimSpace(into)
		if outputType == "" && st.declared[output] {
			outputType = strings.TrimSpace(st.types[output])
		}
		if outputType == "" {
			outputType = "any"
		}
		declareOutput := !st.declared[output]
		st.declared[output] = true
		st.pointers[output] = false
		st.types[output] = outputType

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// state.Get: %s\n", pad, key))
		b.WriteString(fmt.Sprintf("%s%s, %s := s.stateStore.Get(ctx, %s)\n", pad, rawVar, errVar, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"state.Get: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		if declareOutput {
			b.WriteString(fmt.Sprintf("%svar %s %s\n", pad, output, outputType))
		}
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, rawVar))
		b.WriteString(fmt.Sprintf("%s\tif err := json.Unmarshal(%s, &%s); err != nil {\n", pad, rawVar, output))
		b.WriteString(errReturn(st, pad+"\t\t", "fmt.Errorf(\"state.Get unmarshal: %w\", err)"))
		b.WriteString(fmt.Sprintf("%s\t}\n", pad))
		if defVal != "" {
			b.WriteString(fmt.Sprintf("%s} else {\n", pad))
			b.WriteString(fmt.Sprintf("%s\t%s = %s\n", pad, output, defVal))
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		return b.String(), true

	case "state.Set":
		key := arg("key")
		value := arg("value")
		ttl := arg("ttl")
		if key == "" || value == "" {
			return renderInvalidFlowStepConfig(st, pad, "state.Set", "state.Set requires key and value"), true
		}

		if ttl == "" {
			ttl = "0"
		}

		rawVar := "_stateData" + sfx
		errVar := "_stateErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// state.Set: %s\n", pad, key))
		b.WriteString(fmt.Sprintf("%s%s, _ := json.Marshal(%s)\n", pad, rawVar, value))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Set(ctx, %s, %s, %s)\n", pad, errVar, key, rawVar, ttl))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"state.Set: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		return b.String(), true

	case "state.Delete":
		key := arg("key")
		if key == "" {
			return renderInvalidFlowStepConfig(st, pad, "state.Delete", "state.Delete requires key"), true
		}

		errVar := "_stateErr" + sfx

		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s// state.Delete: %s\n", pad, key))
		b.WriteString(fmt.Sprintf("%s%s := s.stateStore.Delete(ctx, %s)\n", pad, errVar, key))
		b.WriteString(fmt.Sprintf("%sif %s != nil {\n", pad, errVar))
		b.WriteString(errReturn(st, pad+"\t", fmt.Sprintf("fmt.Errorf(\"state.Delete: %%w\", %s)", errVar)))
		b.WriteString(fmt.Sprintf("%s}\n", pad))

		return b.String(), true
	}

	return "", false
}
