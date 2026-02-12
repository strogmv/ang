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
	if svc.Name != "Audit" {
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("auditService")},
			Type:  mustParseExpr("port.Audit"),
		})
	}
	if svc.RequiresS3 {
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
	if svc.Name != "Audit" {
		params = append(params, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("auditService")},
			Type:  mustParseExpr("port.Audit"),
		})
		elts = append(elts, &ast.KeyValueExpr{Key: ast.NewIdent("auditService"), Value: ast.NewIdent("auditService")})
	}
	if svc.RequiresS3 {
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
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
				scanSteps(v)
			}
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
				scanSteps(v)
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
				scanSteps(v)
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
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
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
			if step.Action == "event.Publish" {
				return true
			}
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
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
			if step.Action == "notification.Dispatch" {
				return true
			}
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
			}
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && scanSteps(v) {
				return true
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
	return false
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}
