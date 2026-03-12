package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func emitSelectProjectionDiagnostics(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityFieldMap map[string]map[string]string,
	opts PipelineOptions,
) {
	fields, ok := entityFieldMap[entity]
	if !ok || len(fields) == 0 {
		return
	}
	total := len(fields)

	for _, f := range finders {
		if len(f.Select) == 0 {
			continue
		}
		if !finderReturnsEntity(f, entity) {
			continue
		}

		selected := make(map[string]struct{}, len(f.Select))
		for _, col := range f.Select {
			key := canonicalFieldToken(col)
			if key != "" {
				selected[key] = struct{}{}
			}
		}

		var missing []string
		for fieldName := range fields {
			if _, ok := selected[canonicalFieldToken(fieldName)]; !ok {
				missing = append(missing, fieldName)
			}
		}
		if len(missing) == 0 {
			continue
		}

		file, line := parseSourcePos(f.Source)
		msg := fmt.Sprintf(
			"Finder '%s.%s' returns domain.%s but select has %d/%d fields; partial entity select is forbidden",
			entity, f.Name, entity, total-len(missing), total,
		)
		hint := "Use full select for entity return, or set return_type to a projection DTO/custom type"
		diag := normalizer.Warning{
			Kind:     "architecture",
			Code:     "ENTITY_PARTIAL_SELECT_ERROR",
			Severity: "error",
			Message:  msg,
			File:     file,
			Line:     line,
			Hint:     hint,
		}
		recordPipelineDiagnostic(diag, opts)
	}
}

func synthesizeImplicitProjections(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityByName map[string]normalizer.Entity,
	projNameByKey map[string]string,
) ([]normalizer.RepositoryFinder, []normalizer.Entity) {
	src, ok := entityByName[entity]
	if !ok {
		return finders, nil
	}

	lookup := make(map[string]normalizer.Field)
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		lookup[canonicalFieldToken(f.Name)] = f
	}

	var projections []normalizer.Entity
	for i := range finders {
		f := &finders[i]
		if len(f.Select) == 0 || !finderReturnsEntity(*f, entity) {
			continue
		}
		if strings.TrimSpace(f.ReturnType) != "" {
			continue // Explicit return_type must remain explicit; validator will enforce compatibility.
		}

		fields, orderedCols, key, ok := projectionFieldsForSelect(entity, f.Select, src)
		if !ok {
			continue
		}
		if len(fields) == len(lookup) {
			continue // Full select is allowed for domain entity return.
		}

		projName, ok := projNameByKey[key]
		if !ok {
			projName = projectionName(entity, orderedCols)
			projNameByKey[key] = projName
			projections = append(projections, normalizer.Entity{
				Name:   projName,
				Owner:  src.Owner,
				Fields: fields,
				Metadata: map[string]any{
					"projection": true,
					"source":     entity,
				},
				Source: f.Source,
			})
		}

		f.Select = append([]string(nil), orderedCols...)
		if finderReturnsMany(*f, entity) {
			f.ReturnType = "[]domain." + projName
		} else {
			f.ReturnType = "*domain." + projName
		}
	}
	return finders, projections
}

func projectionFieldsForSelect(
	entity string,
	selectCols []string,
	src normalizer.Entity,
) ([]normalizer.Field, []string, string, bool) {
	keys := make([]string, 0, len(selectCols))
	seen := make(map[string]struct{}, len(selectCols))
	for _, col := range selectCols {
		k := canonicalFieldToken(col)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, nil, "", false
	}
	keyTokens := append([]string(nil), keys...)
	sort.Strings(keyTokens)

	fields := make([]normalizer.Field, 0, len(keys))
	orderedCols := make([]string, 0, len(keys))
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		k := canonicalFieldToken(f.Name)
		if _, ok := seen[k]; !ok {
			continue
		}
		delete(seen, k)
		fields = append(fields, f)
		orderedCols = append(orderedCols, f.Name)
	}
	if len(seen) > 0 {
		return nil, nil, "", false
	}
	return fields, orderedCols, entity + "|" + strings.Join(keyTokens, ","), true
}

func projectionName(entity string, sortedCols []string) string {
	parts := make([]string, 0, len(sortedCols))
	for _, c := range sortedCols {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		parts = append(parts, strings.ToLower(ToSnakeCase(s)))
	}
	if len(parts) == 0 {
		return entity + "_Proj"
	}
	return entity + "_" + strings.Join(parts, "_") + "_Proj"
}

func finderReturnsEntity(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "count") || strings.EqualFold(f.Action, "delete") {
		return false
	}
	if f.ReturnType == "" {
		return true
	}

	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	rt = strings.TrimPrefix(rt, "[]")
	rt = strings.TrimPrefix(rt, "*")
	rt = strings.TrimPrefix(rt, "domain.")
	return rt == strings.ToLower(entity)
}

func finderReturnsMany(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "many") || strings.EqualFold(f.Returns, "[]"+entity) {
		return true
	}
	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	return strings.HasPrefix(rt, "[]")
}

func canonicalFieldToken(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	s = strings.Trim(s, "`\"")
	s = strings.SplitN(s, " ", 2)[0]
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, "_", "")
}
