package emitter

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) getSharedFuncMap() template.FuncMap {
	return template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"InputHash":    func() string { return e.InputHash },
		"CompilerHash": func() string { return e.CompilerHash },
		"GoModule":     func() string { return e.GoModule },
		"SafeAssign": func(scope map[string]bool, name string) string {
			scope[name] = true
			return name + ", err := "
		},

		"Title":      ToTitle,
		"ExportName": ExportName,
		"JSONName":   JSONName,
		"DBName":     DBName,
		"ToLower":    strings.ToLower,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"replace":    strings.ReplaceAll,
		"Split":      strings.Split,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"Indent": func(s string, n int) string {
			indent := strings.Repeat("\t", n)
			return strings.ReplaceAll(s, "\n", "\n"+indent)
		},
		"last": func(i int, list interface{}) bool {
			if list == nil {
				return false
			}
			switch v := list.(type) {
			case []normalizer.Field:
				return i == len(v)-1
			case []string:
				return i == len(v)-1
			case []normalizer.FlowStep:
				return i == len(v)-1
			case []normalizer.Entity:
				return i == len(v)-1
			default:
				return false
			}
		},
		"stringsEqualFold": strings.EqualFold,
		"sortedKeys": func(m interface{}) []string {
			switch v := m.(type) {
			case map[string]string:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			case map[string][]string:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			case map[string]interface{}:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			default:
				return nil
			}
		},
		"mapGet": func(m interface{}, key string) interface{} {
			switch v := m.(type) {
			case map[string]string:
				return v[key]
			case map[string][]string:
				return v[key]
			case map[string]interface{}:
				return v[key]
			default:
				return nil
			}
		},
		"makeMap": func() map[string]bool {
			return make(map[string]bool)
		},
		"mapHas": func(m map[string]bool, key string) bool {
			return m[key]
		},
		"mapSet": func(m map[string]bool, key string, val bool) string {
			m[key] = val
			return ""
		},
		"fail": func(msg string) (string, error) {
			return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s", msg)
		},
		"assert": func(condition bool, msg string) (string, error) {
			if !condition {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s", msg)
			}
			return "", nil
		},
		"assertNotEmpty": func(s string, fieldName string) (string, error) {
			if s == "" {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s is empty", fieldName)
			}
			return "", nil
		},
		"assertHasFields": func(fields []normalizer.Field, entityName string) (string, error) {
			if len(fields) == 0 {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s has no fields - check CUE definition", entityName)
			}
			return "", nil
		},
		"FieldValidateTag": func(f normalizer.Field) string {
			tag := ""
			if f.ValidateTag != "" {
				tag = f.ValidateTag
			} else if v, ok := f.Metadata["validate"].(string); ok {
				tag = v
			}

			if tag != "" {
				if strings.HasPrefix(tag, "rule=") {
					tag = strings.TrimPrefix(tag, "rule=")
					tag = strings.Trim(tag, "\"")
				}
				if f.IsOptional && !strings.Contains(tag, "omitempty") {
					tag = "omitempty," + tag
				}
			}
			return tag
		},
		"lowerFirst": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
		"getRepoEntities": func(s normalizer.Service, entities []normalizer.Entity) []string {
			dtoEntities := make(map[string]bool, len(entities))
			for _, ent := range entities {
				if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
					dtoEntities[ent.Name] = true
				}
			}
			unique := make(map[string]bool)
			var res []string
			var scanSteps func([]normalizer.FlowStep)
			scanSteps = func(steps []normalizer.FlowStep) {
				for _, step := range steps {
					if strings.HasPrefix(step.Action, "repo.") {
						ent := step.Args["source"]
						if entName, ok := ent.(string); ok && entName != "" && !unique[entName] && !dtoEntities[entName] {
							unique[entName] = true
							res = append(res, entName)
						}
					}
					// audit.Log requires AuditLog repository
					if step.Action == "audit.Log" {
						if !unique["AuditLog"] && !dtoEntities["AuditLog"] {
							unique["AuditLog"] = true
							res = append(res, "AuditLog")
						}
					}
					// auth.RequireRole requires User repository
					if step.Action == "auth.RequireRole" {
						if !unique["User"] && !dtoEntities["User"] {
							unique["User"] = true
							res = append(res, "User")
						}
					}
					// entity.PatchValidated may require repository for unique checks
					if step.Action == "entity.PatchValidated" {
						hasUnique := false
						if fields, ok := step.Args["fields"].(map[string]map[string]string); ok {
							for _, cfg := range fields {
								if strings.TrimSpace(cfg["unique"]) != "" {
									hasUnique = true
									break
								}
							}
						}
						if hasUnique {
							repoEntity := ""
							if src, ok := step.Args["source"].(string); ok {
								repoEntity = strings.TrimSpace(src)
							}
							if repoEntity != "" && !unique[repoEntity] && !dtoEntities[repoEntity] {
								unique[repoEntity] = true
								res = append(res, repoEntity)
							}
						}
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							scanSteps(branch)
						}
					}
				}
			}
			for _, m := range s.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] && !dtoEntities[src.Entity] {
						unique[src.Entity] = true
						res = append(res, src.Entity)
					}
				}
				scanSteps(m.Flow)
			}
			sort.Strings(res)
			return res
		},
		"getFlowRepoEntities": func(steps []normalizer.FlowStep) []string {
			unique := make(map[string]bool)
			var res []string
			var scanSteps func([]normalizer.FlowStep)
			scanSteps = func(steps []normalizer.FlowStep) {
				for _, step := range steps {
					if strings.HasPrefix(step.Action, "repo.") {
						if entName, ok := step.Args["source"].(string); ok && entName != "" && !unique[entName] {
							unique[entName] = true
							res = append(res, entName)
						}
					}
					// list.Enrich performs repo lookup via lookupSource
					if step.Action == "list.Enrich" {
						if entName, ok := step.Args["lookupSource"].(string); ok && entName != "" && !unique[entName] {
							unique[entName] = true
							res = append(res, entName)
						}
					}
					// audit.Log requires AuditLog repository
					if step.Action == "audit.Log" && !unique["AuditLog"] {
						unique["AuditLog"] = true
						res = append(res, "AuditLog")
					}
					// auth.RequireRole requires User repository
					if step.Action == "auth.RequireRole" && !unique["User"] {
						unique["User"] = true
						res = append(res, "User")
					}
					if step.Action == "entity.PatchValidated" {
						hasUnique := false
						if fields, ok := step.Args["fields"].(map[string]map[string]string); ok {
							for _, cfg := range fields {
								if strings.TrimSpace(cfg["unique"]) != "" {
									hasUnique = true
									break
								}
							}
						}
						if hasUnique {
							repoEntity := ""
							if src, ok := step.Args["source"].(string); ok {
								repoEntity = strings.TrimSpace(src)
							}
							if repoEntity != "" && !unique[repoEntity] {
								unique[repoEntity] = true
								res = append(res, repoEntity)
							}
						}
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							scanSteps(branch)
						}
					}
				}
			}
			scanSteps(steps)
			sort.Strings(res)
			return res
		},
		"splitFields": func(s interface{}) []string {
			str, ok := s.(string)
			if !ok || str == "" {
				return nil
			}
			parts := strings.Split(str, ",")
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
			return result
		},
		"getServiceDeps": func(s normalizer.Service) []string {
			if len(s.Uses) == 0 {
				return nil
			}
			deps := append([]string{}, s.Uses...)
			sort.Strings(deps)
			return deps
		},
		"ServiceNeedsTx": func(s normalizer.Service) bool {
			var scanSteps func([]normalizer.FlowStep) bool
			scanSteps = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "tx.Block" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if scanSteps(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if scanSteps(m.Flow) {
					return true
				}
				if m.Impl != nil && m.Impl.RequiresTx {
					return true
				}
			}
			return false
		},
		"ServiceHasOutbox": func(s normalizer.Service) bool {
			var hasOutboxStep func([]normalizer.FlowStep) bool
			hasOutboxStep = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "event.Outbox" {
						return true
					}
					for _, childKey := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
						if v, ok := step.Args[childKey].([]normalizer.FlowStep); ok && hasOutboxStep(v) {
							return true
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if hasOutboxStep(branch) {
								return true
							}
						}
					}
					if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range branches {
							if hasOutboxStep(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if m.Outbox {
					return true
				}
				if hasOutboxStep(m.Flow) {
					return true
				}
			}
			return false
		},
		"ServiceHasIdempotency": func(s normalizer.Service) bool {
			for _, m := range s.Methods {
				if m.Idempotency {
					return true
				}
			}
			return false
		},
		"AnyServiceHasIdempotencyOrOutbox": func(services []normalizer.Service) bool {
			var hasOutboxStep func([]normalizer.FlowStep) bool
			hasOutboxStep = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "event.Outbox" {
						return true
					}
					for _, childKey := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
						if v, ok := step.Args[childKey].([]normalizer.FlowStep); ok && hasOutboxStep(v) {
							return true
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if hasOutboxStep(branch) {
								return true
							}
						}
					}
					if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range branches {
							if hasOutboxStep(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, s := range services {
				for _, m := range s.Methods {
					if m.Idempotency || m.Outbox || hasOutboxStep(m.Flow) {
						return true
					}
				}
			}
			return false
		},
		"MethodHasIdempotency": func(m normalizer.Method) bool {
			return m.Idempotency
		},
		"MethodHasOutbox": func(m normalizer.Method) bool {
			return m.Outbox
		},
		"ServiceHasPublishes": func(s normalizer.Service) bool {
			var scanSteps func([]normalizer.FlowStep) bool
			scanSteps = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "event.Publish" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if scanSteps(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if len(m.Publishes) > 0 {
					return true
				}
				if scanSteps(m.Flow) {
					return true
				}
			}
			return false
		},
		"ServiceHasNotificationDispatch": func(s normalizer.Service) bool {
			var scanSteps func([]normalizer.FlowStep) bool
			scanSteps = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "notification.Dispatch" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if scanSteps(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if scanSteps(m.Flow) {
					return true
				}
			}
			return false
		},
		"HasEventMethods": func(s normalizer.Service) bool {
			for _, m := range s.Methods {
				if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
					return true
				}
			}
			return false
		},
		"HasDomainTypes": func(s normalizer.Service) bool {
			hasDomain := func(ent normalizer.Entity) bool {
				for _, f := range ent.Fields {
					if strings.HasPrefix(f.Type, "domain.") || strings.HasPrefix(f.ItemTypeName, "domain.") {
						return true
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if (m.Input.Name != "" && hasDomain(m.Input)) || (m.Output.Name != "" && hasDomain(m.Output)) {
					return true
				}
			}
			return false
		},
		"ServiceNestedTypes": func(s normalizer.Service) []normalizer.Entity {
			typeMap := make(map[string]normalizer.Entity)
			addNested := func(ent normalizer.Entity) {
				for _, f := range ent.Fields {
					if f.ItemTypeName == "" || len(f.ItemFields) == 0 {
						continue
					}
					if _, ok := typeMap[f.ItemTypeName]; ok {
						continue
					}
					typeMap[f.ItemTypeName] = normalizer.Entity{Name: f.ItemTypeName, Fields: f.ItemFields}
				}
			}
			for _, m := range s.Methods {
				if m.Input.Name != "" {
					addNested(m.Input)
				}
				if m.Output.Name != "" {
					addNested(m.Output)
				}
			}
			var res []normalizer.Entity
			for _, v := range typeMap {
				res = append(res, v)
			}
			sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
			return res
		},
		"EventForMethod": func(s normalizer.Service, m normalizer.Method) string {
			if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
				return m.Publishes[0]
			}
			return ""
		},
		"ZeroValue": func(ent normalizer.Entity) string {
			return "port." + ent.Name + "{}"
		},
		"getSteps": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getThen": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getElse": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getIfNew": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getIfExists": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getCases": func(step normalizer.FlowStep) map[string][]normalizer.FlowStep {
			if v, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getCaseKeys": func(step normalizer.FlowStep) []string {
			cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep)
			if !ok || len(cases) == 0 {
				return nil
			}
			keys := make([]string, 0, len(cases))
			for k := range cases {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		},
		"getDefault": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getPatchValidatedFieldKeys": func(step normalizer.FlowStep) []string {
			fields, ok := step.Args["fields"].(map[string]map[string]string)
			if !ok || len(fields) == 0 {
				return nil
			}
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		},
		"getPatchValidatedRule": func(step normalizer.FlowStep, field, rule string) string {
			fields, ok := step.Args["fields"].(map[string]map[string]string)
			if !ok {
				return ""
			}
			cfg, ok := fields[field]
			if !ok {
				return ""
			}
			return strings.TrimSpace(cfg[rule])
		},
		"pointerExpr": func(input string) string {
			if strings.HasPrefix(input, "new") {
				return "&" + input
			}
			return input
		},
		"EndpointType": func(ep normalizer.Endpoint) string {
			method := strings.ToUpper(ep.Method)
			path := ep.Path

			// Action pattern: /api/.../{id}/{action}
			if method == "POST" && strings.HasSuffix(path, "}") == false {
				parts := strings.Split(path, "/")
				if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-2], "{") {
					return "action"
				}
			}

			if strings.HasSuffix(path, "}") {
				switch method {
				case "GET":
					return "get"
				case "PATCH", "PUT":
					return "update"
				case "DELETE":
					return "delete"
				}
			} else {
				switch method {
				case "GET":
					return "list"
				case "POST":
					return "create"
				}
			}
			return "other"
		},
		"getEntityFields": func(entities []normalizer.Entity, name string) []normalizer.Field {
			for _, e := range entities {
				if strings.EqualFold(e.Name, name) {
					return e.Fields
				}
			}
			return nil
		},
		"hasField": func(fields []normalizer.Field, name string) bool {
			for _, f := range fields {
				if strings.EqualFold(f.Name, name) {
					return true
				}
			}
			return false
		},
		"ServiceImplImports": func(s normalizer.Service) []string {
			goMod := e.GoModule
			if goMod == "" {
				goMod = "github.com/strogmv/ang"
			}
			importsMap := map[string]bool{
				goMod + "/internal/domain":      true,
				goMod + "/internal/pkg/errors":  true,
				goMod + "/internal/pkg/helpers": true,
				goMod + "/internal/port":        true,
				"net/http":                      true,
				"os":                            true,
				"github.com/google/uuid":        true,
				"time":                          true,
			}
			for _, m := range s.Methods {
				if m.Impl != nil {
					for _, imp := range m.Impl.Imports {
						importsMap[imp] = true
					}
				}
			}
			result := make([]string, 0, len(importsMap))
			for imp := range importsMap {
				result = append(result, imp)
			}
			sort.Strings(result)
			return result
		},
		"EntityStorage": func(ent normalizer.Entity) string {
			if ent.Metadata != nil {
				if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
					return v
				}
			}
			return "sql"
		},
		"EntityStorageByName": func(entities []normalizer.Entity, name string) string {
			for _, ent := range entities {
				if ent.Name == name {
					if ent.Metadata != nil {
						if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
							return v
						}
					}
					return "sql"
				}
			}
			return "sql"
		},
		"HasMongoRepoEntities": func(entities []normalizer.Entity) bool {
			for _, ent := range entities {
				if ent.Metadata != nil {
					if v, ok := ent.Metadata["storage"].(string); ok && v == "mongo" {
						return true
					}
				}
			}
			return false
		},
		"InitScope": func() map[string]bool {
			return make(map[string]bool)
		},
		"CloneScope": func(scope map[string]bool) map[string]bool {
			newScope := make(map[string]bool)
			for k, v := range scope {
				newScope[k] = v
			}
			return newScope
		},
		"Assign": func(scope map[string]bool, name string) string {
			if strings.Contains(name, ".") || scope[name] {
				return name + ", err ="
			}
			scope[name] = true
			return name + ", err :="
		},
		"AssignSimple": func(scope map[string]bool, name string) string {
			if strings.Contains(name, ".") || scope[name] {
				return name + " ="
			}
			scope[name] = true
			return name + " :="
		},
		"Declare": func(scope map[string]bool, name string) string {
			if scope[name] {
				return ""
			}
			scope[name] = true
			return "var " + name
		},
		"PrepareCodeBlock": func(code string) string {
			lines := strings.Split(code, "\n")
			var result []string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Remove redundant variable declarations (var x T)
				if strings.HasPrefix(trimmed, "var resp ") ||
					strings.HasPrefix(trimmed, "var err error") {
					result = append(result, "// "+line+" // Removed by ANG Emitter")
					continue
				}
				// Convert short assignments (x := ...) to regular assignments (x = ...)
				// if they target the named return variables 'resp' or 'err'
				if strings.HasPrefix(trimmed, "resp :=") || strings.HasPrefix(trimmed, "resp:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				if strings.HasPrefix(trimmed, "err :=") || strings.HasPrefix(trimmed, "err:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				// Fix common "resp, err :=" assignments
				if strings.HasPrefix(trimmed, "resp, err :=") || strings.HasPrefix(trimmed, "resp, err:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				result = append(result, line)
			}
			return strings.Join(result, "\n")
		},
	}
}

type MainContext struct {
	IRSchema                *ir.Schema
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
	EndpointsIR             []ir.Endpoint
	Services                []normalizer.Service
	Entities                []normalizer.Entity
	Endpoints               []normalizer.Endpoint
	HasCache                bool
	HasSQL                  bool
	HasMongo                bool
	HasNats                 bool
	HasS3                   bool
	WebSocketServices       map[string]bool
	HasScheduler            bool
	WSEventMap              map[string]map[string]bool
	EventPayloads           map[string]normalizer.Entity
	EventPayloadsIR         map[string]ir.Entity
	WSRoomField             map[string]string
	AuthService             string
	AuthRefreshStore        string
	HasSession              bool
	SessionCookieName       string
	InputHash               string
	CompilerHash            string
	ANGVersion              string
	EntityOwners            map[string]string
	GoModule                string // Go module path for imports (e.g., "github.com/strog/dealingi-back")
	NotificationMuting      bool   // Enable notification muting decorator
	HasNotificationsService bool
	HasNotificationDispatch bool
}

// AnalyzeContext checks which infrastructure dependencies are required.
