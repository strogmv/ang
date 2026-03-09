package emitter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

func renderServiceImplTypeDecl(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
	repoEntities := serviceImplRepoEntities(svc, entities)
	deps := serviceImplServiceDeps(svc)

	fields := make([]*ast.Field, 0, len(repoEntities)+len(deps)+8)
	for _, ent := range repoEntities {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(ExportName(ent) + "Repo")},
			Type:  mustParseExpr("port." + ExportName(ent) + "Repository"),
		})
	}
	for _, dep := range deps {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(ExportName(dep) + "Service")},
			Type:  mustParseExpr("port." + ExportName(dep)),
		})
	}
	if serviceImplNeedsTx(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("txManager")},
			Type:  mustParseExpr("port.TxManager"),
		})
	}
	if auth != nil && auth.Service == svc.Name {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("cfg")},
			Type:  mustParseExpr("*config.Config"),
		})
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("refreshStore")},
			Type:  mustParseExpr("port.RefreshTokenStore"),
		})
	}
	if serviceImplHasPublishes(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("publisher")},
			Type:  mustParseExpr("port.Publisher"),
		})
	}
	if serviceImplHasIdempotency(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("idempotency")},
			Type:  mustParseExpr("port.IdempotencyStore"),
		})
	}
	if serviceImplHasOutbox(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("outbox")},
			Type:  mustParseExpr("port.OutboxRepository"),
		})
	}
	if svc.RequiresS3 || serviceImplHasStorageActions(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("storage")},
			Type:  mustParseExpr("port.FileStorage"),
		})
	}
	if serviceImplHasNotificationDispatch(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("dispatcher")},
			Type:  mustParseExpr("port.NotificationDispatcher"),
		})
	}
	if serviceImplHasCacheActions(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("cache")},
			Type:  mustParseExpr("*redis.Client"),
		})
	}
	if serviceImplHasMailSend(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("mailer")},
			Type:  mustParseExpr("port.Mailer"),
		})
	}
	if serviceImplHasQueueDeliveryActions(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("queuePublisher")},
			Type:  mustParseExpr("port.QueuePublisher"),
		})
	}
	if serviceImplHasStateActions(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("stateStore")},
			Type:  mustParseExpr("port.StateStore"),
		})
	}
	if serviceImplHasPolicyActions(svc) {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("policyEngine")},
			Type:  mustParseExpr("port.PolicyEngine"),
		})
	}

	gen := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(svc.Name + "Impl"),
				Type: &ast.StructType{
					Fields: &ast.FieldList{List: fields},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), gen); err != nil {
		return "", fmt.Errorf("format service impl type %s: %w", svc.Name, err)
	}
	return buf.String(), nil
}

func renderServiceImplConstructorDecl(svc normalizer.Service, entities []normalizer.Entity, auth *normalizer.AuthDef) (string, error) {
	repoEntities := serviceImplRepoEntities(svc, entities)
	deps := serviceImplServiceDeps(svc)

	params := make([]*ast.Field, 0, len(repoEntities)+len(deps)+8)
	elts := make([]ast.Expr, 0, len(repoEntities)+len(deps)+8)

	for _, ent := range repoEntities {
		paramName := lowerFirst(ExportName(ent)) + "Repo"
		fieldName := ExportName(ent) + "Repo"
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(paramName)},
			Type:  mustParseExpr("port." + ExportName(ent) + "Repository"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent(fieldName), Value: ast.NewIdent(paramName)})
	}
	for _, dep := range deps {
		paramName := lowerFirst(ExportName(dep)) + "Service"
		fieldName := ExportName(dep) + "Service"
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(paramName)},
			Type:  mustParseExpr("port." + ExportName(dep)),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent(fieldName), Value: ast.NewIdent(paramName)})
	}
	if serviceImplNeedsTx(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("txManager")},
			Type:  mustParseExpr("port.TxManager"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("txManager"), Value: ast.NewIdent("txManager")})
	}
	if auth != nil && auth.Service == svc.Name {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("cfg")},
			Type:  mustParseExpr("*config.Config"),
		})
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("refreshStore")},
			Type:  mustParseExpr("port.RefreshTokenStore"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("cfg"), Value: ast.NewIdent("cfg")})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("refreshStore"), Value: ast.NewIdent("refreshStore")})
	}
	if serviceImplHasPublishes(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("publisher")},
			Type:  mustParseExpr("port.Publisher"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("publisher"), Value: ast.NewIdent("publisher")})
	}
	if serviceImplHasIdempotency(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("idempotency")},
			Type:  mustParseExpr("port.IdempotencyStore"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("idempotency"), Value: ast.NewIdent("idempotency")})
	}
	if serviceImplHasOutbox(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("outbox")},
			Type:  mustParseExpr("port.OutboxRepository"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("outbox"), Value: ast.NewIdent("outbox")})
	}
	if svc.RequiresS3 || serviceImplHasStorageActions(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("storage")},
			Type:  mustParseExpr("port.FileStorage"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("storage"), Value: ast.NewIdent("storage")})
	}
	if serviceImplHasNotificationDispatch(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("dispatcher")},
			Type:  mustParseExpr("port.NotificationDispatcher"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("dispatcher"), Value: ast.NewIdent("dispatcher")})
	}
	if serviceImplHasCacheActions(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("cache")},
			Type:  mustParseExpr("*redis.Client"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("cache"), Value: ast.NewIdent("cache")})
	}
	if serviceImplHasMailSend(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("mailer")},
			Type:  mustParseExpr("port.Mailer"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("mailer"), Value: ast.NewIdent("mailer")})
	}
	if serviceImplHasQueueDeliveryActions(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("queuePublisher")},
			Type:  mustParseExpr("port.QueuePublisher"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("queuePublisher"), Value: ast.NewIdent("queuePublisher")})
	}
	if serviceImplHasStateActions(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("stateStore")},
			Type:  mustParseExpr("port.StateStore"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("stateStore"), Value: ast.NewIdent("stateStore")})
	}
	if serviceImplHasPolicyActions(svc) {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("policyEngine")},
			Type:  mustParseExpr("port.PolicyEngine"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("policyEngine"), Value: ast.NewIdent("policyEngine")})
	}

	fd := &ast.FuncDecl{
		Name: ast.NewIdent("New" + svc.Name + "Impl"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: params},
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: mustParseExpr("*" + svc.Name + "Impl")},
			}},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.UnaryExpr{
							Op: token.AND,
							X: &ast.CompositeLit{
								Type: ast.NewIdent(svc.Name + "Impl"),
								Elts: elts,
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), fd); err != nil {
		return "", fmt.Errorf("format service impl constructor %s: %w", svc.Name, err)
	}
	return buf.String(), nil
}

func serviceImplRepoEntities(s normalizer.Service, entities []normalizer.Entity) []string {
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
				if src, ok := step.Args["source"].(string); ok && src != "" && !unique[src] && !dtoEntities[src] {
					unique[src] = true
					res = append(res, src)
				}
			}
			if step.Action == "list.Enrich" {
				if src, ok := step.Args["lookupSource"].(string); ok && src != "" && !unique[src] && !dtoEntities[src] {
					unique[src] = true
					res = append(res, src)
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
			if src.Entity == "" || unique[src.Entity] || dtoEntities[src.Entity] {
				continue
			}
			unique[src.Entity] = true
			res = append(res, src.Entity)
		}
		scanSteps(m.Flow)
	}

	sort.Strings(res)
	return res
}

func serviceImplServiceDeps(s normalizer.Service) []string {
	if len(s.Uses) == 0 {
		return nil
	}
	deps := append([]string{}, s.Uses...)
	sort.Strings(deps)
	return deps
}

func serviceImplNeedsTx(s normalizer.Service) bool {
	var scanSteps func([]normalizer.FlowStep) bool
	scanSteps = func(steps []normalizer.FlowStep) bool {
		for _, step := range steps {
			if step.Action == "tx.Block" {
				return true
			}
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
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
}

func serviceImplHasPublishes(s normalizer.Service) bool {
	var scanSteps func([]normalizer.FlowStep) bool
	scanSteps = func(steps []normalizer.FlowStep) bool {
		for _, step := range steps {
			if step.Action == "event.Publish" || step.Action == "event.Broadcast" || step.Action == "event.Wait" || step.Action == "event.Subscribe" {
				return true
			}
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_ifNew"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_ifExists"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
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
		if len(m.Publishes) > 0 || scanSteps(m.Flow) {
			return true
		}
	}
	return false
}

func serviceImplHasNotificationDispatch(s normalizer.Service) bool {
	var scanSteps func([]normalizer.FlowStep) bool
	scanSteps = func(steps []normalizer.FlowStep) bool {
		for _, step := range steps {
			if step.Action == "notification.Dispatch" || step.Action == "notify.Dispatch" || step.Action == "notify.Send" {
				return true
			}
			for _, childKey := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if v, ok := step.Args[childKey].([]normalizer.FlowStep); ok && scanSteps(v) {
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
}

func serviceImplHasIdempotency(s normalizer.Service) bool {
	for _, m := range s.Methods {
		if m.Idempotency {
			return true
		}
	}
	return false
}

func serviceImplHasOutbox(s normalizer.Service) bool {
	for _, m := range s.Methods {
		if m.Outbox {
			return true
		}
	}
	return serviceImplHasFlowAction(s, "event.Outbox")
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// serviceImplHasFlowAction returns true if any method flow contains the given action (recursive).
func serviceImplHasFlowAction(s normalizer.Service, action string) bool {
	var scanSteps func([]normalizer.FlowStep) bool
	scanSteps = func(steps []normalizer.FlowStep) bool {
		for _, step := range steps {
			if step.Action == action {
				return true
			}
			for _, childKey := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if v, ok := step.Args[childKey].([]normalizer.FlowStep); ok && scanSteps(v) {
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
			if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
				for _, branch := range branches {
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
}

func serviceImplHasCacheActions(s normalizer.Service) bool {
	return serviceImplHasFlowAction(s, "cache.Get") ||
		serviceImplHasFlowAction(s, "cache.Set") ||
		serviceImplHasFlowAction(s, "cache.Del")
}

func serviceImplHasStorageActions(s normalizer.Service) bool {
	return serviceImplHasFlowAction(s, "storage.Upload") ||
		serviceImplHasFlowAction(s, "storage.Download") ||
		serviceImplHasFlowAction(s, "storage.GetURL") ||
		serviceImplHasFlowAction(s, "storage.Delete") ||
		serviceImplHasFlowAction(s, "storage.List")
}

func serviceImplHasMailSend(s normalizer.Service) bool {
	return serviceImplHasFlowAction(s, "mail.Send")
}

func serviceImplHasQueueDeliveryActions(s normalizer.Service) bool {
	return serviceImplHasFlowActionWithPrefix(s, "queue.") || serviceImplHasFlowAction(s, "dlq.Publish")
}

func serviceImplHasStateActions(s normalizer.Service) bool {
	if serviceImplHasFlowAction(s, "state.Get") ||
		serviceImplHasFlowAction(s, "state.Set") ||
		serviceImplHasFlowAction(s, "state.Delete") {
		return true
	}
	// Reliability primitives all use stateStore internally
	for _, prefix := range []string{"idem.", "idempotency.", "dedupe.", "ratelimit.", "quota.", "budget.", "profile.", "concurrency.", "circuit.", "bulkhead.", "approval."} {
		if serviceImplHasFlowActionWithPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func serviceImplHasPolicyActions(s normalizer.Service) bool {
	return serviceImplHasFlowActionWithPrefix(s, "policy.")
}

// serviceImplHasFlowActionWithPrefix returns true if any method flow contains an action
// that starts with the given prefix (recursive).
func serviceImplHasFlowActionWithPrefix(s normalizer.Service, prefix string) bool {
	var scanSteps func([]normalizer.FlowStep) bool
	scanSteps = func(steps []normalizer.FlowStep) bool {
		for _, step := range steps {
			if strings.HasPrefix(step.Action, prefix) {
				return true
			}
			for _, childKey := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if v, ok := step.Args[childKey].([]normalizer.FlowStep); ok && scanSteps(v) {
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
			if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok {
				for _, branch := range branches {
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
}
