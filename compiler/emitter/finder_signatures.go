package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// FinderSignature is shared finder method metadata used by repository emitters.
type FinderSignature struct {
	Name        string
	ParamsSig   string
	ArgsCSV     string
	ArgNames    string
	ReturnType  string
	ReturnZero  string
	ReturnSlice bool
	HasTime     bool
}

// ComputeFinderSignature computes canonical finder signature parts once.
// fallbackReturnType is used only when finder.Returns is empty and no other rule matches.
func ComputeFinderSignature(entity string, finder normalizer.RepositoryFinder, fallbackReturnType string) FinderSignature {
	sig := FinderSignature{
		Name: ExportName(finder.Name),
	}

	var params []string
	var args []string
	for _, w := range finder.Where {
		pType, isTime := finderParamType(w.ParamType)
		if isTime {
			sig.HasTime = true
		}
		params = append(params, fmt.Sprintf("%s %s", w.Param, pType))
		args = append(args, w.Param)
	}
	sig.ParamsSig = strings.Join(params, ", ")
	sig.ArgsCSV = strings.Join(args, ", ")
	sig.ArgNames = sig.ArgsCSV

	sig.ReturnType, sig.ReturnZero, sig.ReturnSlice = resolveFinderReturnType(entity, finder, fallbackReturnType)
	return sig
}

// ComputeIRFinderSignature is the IR counterpart used by IR-based emitters.
func ComputeIRFinderSignature(entity string, finder ir.Finder, fallbackReturnType string) FinderSignature {
	sig := FinderSignature{
		Name: ExportName(finder.Name),
	}

	var params []string
	var args []string
	for _, w := range finder.Where {
		pType, isTime := finderParamType(w.ParamType)
		if isTime {
			sig.HasTime = true
		}
		params = append(params, fmt.Sprintf("%s %s", w.Param, pType))
		args = append(args, w.Param)
	}
	sig.ParamsSig = strings.Join(params, ", ")
	sig.ArgsCSV = strings.Join(args, ", ")
	sig.ArgNames = sig.ArgsCSV

	nFinder := normalizer.RepositoryFinder{
		Name:       finder.Name,
		Action:     finder.Action,
		Returns:    finder.Returns,
		ReturnType: finder.ReturnType,
	}
	sig.ReturnType, sig.ReturnZero, sig.ReturnSlice = resolveFinderReturnType(entity, nFinder, fallbackReturnType)
	return sig
}

func finderParamType(paramType string) (goType string, isTime bool) {
	pType := strings.TrimSpace(paramType)
	if pType == "time" || pType == "time.Time" {
		return "time.Time", true
	}
	return pType, false
}

func resolveFinderReturnType(entity string, finder normalizer.RepositoryFinder, fallbackReturnType string) (retType, zero string, slice bool) {
	if finder.ReturnType != "" {
		return finder.ReturnType, "nil", strings.HasPrefix(finder.ReturnType, "[]")
	}
	if finder.Action == "delete" {
		return "int64", "0", false
	}
	switch finder.Returns {
	case "one", entity, "*" + entity:
		return "*domain." + entity, "nil", false
	case "many", "[]" + entity:
		return "[]domain." + entity, "nil", true
	case "count":
		return "int64", "0", false
	case "":
		if fallbackReturnType != "" {
			return fallbackReturnType, "nil", strings.HasPrefix(fallbackReturnType, "[]")
		}
		return "", "nil", false
	default:
		return finder.Returns, "nil", strings.HasPrefix(finder.Returns, "[]")
	}
}
