package emitter

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// renderImplSteps renders a minimal Go implementation for the experimental impl_steps DSL.
// Supported kinds: load/assert/call/emit (limited handling). Unknown steps panic in non-prod to surface gaps.
func renderImplSteps(steps []normalizer.ImplStep, serviceName, methodName string) string {
	var b strings.Builder
	b.WriteString("// generated from impl_steps\n")
	for _, st := range steps {
		switch st.Kind {
		case "load":
			into := st.LoadInto
			if into == "" {
				into = "item"
			}
			repo := fmt.Sprintf("s.%sRepo", exportName(st.LoadTarget))
			call := "FindByID"
			arg := "\"\""
			for k, v := range st.LoadBy {
				switch strings.ToLower(k) {
				case "token":
					call = "GetByToken"
				case "id", "_id":
					call = "FindByID"
				}
				arg = v
				break
			}
			b.WriteString(fmt.Sprintf("%s, err := %s(ctx, %s)\n", into, repo+"."+call, arg))
			b.WriteString("if err != nil { return resp, err }\n")
		case "assert":
			cond := st.AssertExpr
			if cond == "" {
				cond = "false"
			}
			errCode := st.AssertError
			if errCode == "" {
				errCode = "NotImplemented"
			}
			b.WriteString(fmt.Sprintf("if !(%s) {\n", cond))
			b.WriteString(fmt.Sprintf("\treturn resp, errors.New(http.StatusUnauthorized, \"Unauthorized\", \"%s failed\")\n", errCode))
			b.WriteString("}\n")
		case "call":
			into := st.CallInto
			if into == "" {
				into = "_"
			}
			switch strings.ToLower(st.CallTarget) {
			case "auth.issueaccesstoken", "auth.issueaccesstoken()":
				uid := valString(st.CallArgsMap, "userID", "\"\"")
				cid := valString(st.CallArgsMap, "companyID", "\"\"")
				roles := valString(st.CallArgsMap, "roles", "nil")
				perms := valString(st.CallArgsMap, "perms", "nil")
				b.WriteString(fmt.Sprintf("%s, err := auth.IssueAccessToken(s.cfg, %s, %s, %s, %s)\n", into, uid, cid, roles, perms))
				b.WriteString("if err != nil { return resp, err }\n")
			case "auth.checkpassword":
				hash := valString(st.CallArgsMap, "hash", "\"\"")
				pass := valString(st.CallArgsMap, "password", "\"\"")
				b.WriteString(fmt.Sprintf("if err := bcrypt.CompareHashAndPassword([]byte(%s), []byte(%s)); err != nil {\n", hash, pass))
				b.WriteString("\treturn resp, errors.New(http.StatusUnauthorized, \"Unauthorized\", \"invalid credentials\")\n}\n")
			case "auth.issuetokens":
				uid := valString(st.CallArgsMap, "userID", "\"\"")
				cid := valString(st.CallArgsMap, "companyID", "\"\"")
				roles := valString(st.CallArgsMap, "roles", "nil")
				perms := valString(st.CallArgsMap, "perms", "nil")
				b.WriteString(fmt.Sprintf("access, err := auth.IssueAccessToken(s.cfg, %s, %s, %s, %s)\n", uid, cid, roles, perms))
				b.WriteString("if err != nil { return resp, err }\n")
				b.WriteString(fmt.Sprintf("refresh, err := auth.IssueRefreshToken(s.cfg, %s)\n", uid))
				b.WriteString("if err != nil { return resp, err }\n")
				b.WriteString("exp := time.Now().Add(24 * time.Hour)\n")
				b.WriteString("if d, err := time.ParseDuration(s.cfg.JWTRefreshTTL); err == nil && d > 0 { exp = time.Now().Add(d) }\n")
				b.WriteString("if s.refreshStore != nil { _ = s.refreshStore.Save(ctx, refresh, " + uid + ", exp) }\n")
				b.WriteString(fmt.Sprintf("%s := struct{Access, Refresh string}{Access: access, Refresh: refresh}\n", into))
			case "auth.rotaterefreshtoken":
				token := valString(st.CallArgsMap, "refreshToken", "req.RefreshToken")
				uid := valString(st.CallArgsMap, "userID", "\"\"")
				b.WriteString("exp := time.Now().Add(24 * time.Hour)\n")
				b.WriteString("if d, err := time.ParseDuration(s.cfg.JWTRefreshTTL); err == nil && d > 0 { exp = time.Now().Add(d) }\n")
				b.WriteString(fmt.Sprintf("%s, err := auth.IssueRefreshToken(s.cfg, %s)\n", into, uid))
				b.WriteString("if err != nil { return resp, err }\n")
				b.WriteString(fmt.Sprintf("if err = s.refreshStore.Rotate(ctx, %s, %s, %s, exp); err != nil { return resp, err }\n", token, into, uid))
			case "auth.verifyemail":
				uid := valString(st.CallArgsMap, "userID", "\"\"")
				b.WriteString(fmt.Sprintf("if s.UserRepo != nil { _ = s.UserRepo.MarkEmailVerified(ctx, %s) }\n", uid))
			case "mapping.assign":
				expr := st.CallArgsExpr
				if expr == "" {
					expr = valString(st.CallArgsMap, "expr", "")
				}
				if expr != "" {
					b.WriteString(expr)
					if !strings.HasSuffix(expr, "\n") {
						b.WriteString("\n")
					}
				}
			case "audit.log":
				actor := valString(st.CallArgsMap, "actor", "\"\"")
				company := valString(st.CallArgsMap, "company", "\"\"")
				action := valString(st.CallArgsMap, "action", "\"\"")
				b.WriteString("if s.AuditLogRepo != nil {\n")
				b.WriteString(fmt.Sprintf("\taudit := &domain.AuditLog{ID: uuid.NewString(), ActorID: %s, CompanyID: %s, Action: %s, CreatedAt: time.Now().UTC()}\n", actor, company, action))
				b.WriteString("\t_ = s.AuditLogRepo.Save(ctx, audit)\n}\n")
			case "auth.logout":
				token := valString(st.CallArgsMap, "refreshToken", "req.RefreshToken")
				b.WriteString("if s.refreshStore != nil { _ = s.refreshStore.Delete(ctx, " + token + ") }\n")
			default:
				if strings.HasPrefix(strings.ToLower(st.CallTarget), "repo.") {
					parts := strings.Split(st.CallTarget, ".")
					if len(parts) >= 3 {
						repoName := exportName(parts[1])
						method := strings.ToLower(parts[2])
						repoVar := fmt.Sprintf("s.%sRepo", repoName)
						b.WriteString(fmt.Sprintf("if %s == nil { return resp, errors.New(http.StatusInternalServerError, \"Repo error\", \"%s not configured\") }\n", repoVar, repoVar))
						switch method {
						case "save":
							inp := valString(st.CallArgsMap, "input", "nil")
							b.WriteString(fmt.Sprintf("if err := %s.Save(ctx, %s); err != nil { return resp, err }\n", repoVar, inp))
						case "findbyid", "get":
							arg := valString(st.CallArgsMap, "id", "\"\"")
							b.WriteString(fmt.Sprintf("%s, err := %s.FindByID(ctx, %s)\n", into, repoVar, arg))
							b.WriteString("if err != nil { return resp, err }\n")
						case "getbytoken":
							arg := valString(st.CallArgsMap, "token", "req.RefreshToken")
							b.WriteString(fmt.Sprintf("%s, err := %s.GetByToken(ctx, %s)\n", into, repoVar, arg))
							b.WriteString("if err != nil { return resp, err }\n")
						case "delete":
							arg := valString(st.CallArgsMap, "id", "\"\"")
							b.WriteString(fmt.Sprintf("if err := %s.Delete(ctx, %s); err != nil { return resp, err }\n", repoVar, arg))
						default:
							b.WriteString(fmt.Sprintf("// TODO impl_steps repo.%s not supported yet\n", method))
							b.WriteString("if os.Getenv(\"APP_ENV\") != \"production\" { panic(\"impl_steps repo call not supported\") }\n")
						}
					} else {
						b.WriteString("// malformed repo call\n")
					}
				} else {
					b.WriteString(fmt.Sprintf("// TODO impl_steps call %s not supported yet\n", st.CallTarget))
					b.WriteString("if os.Getenv(\"APP_ENV\") != \"production\" { panic(\"impl_steps call not supported\") }\n")
				}
			}
		case "emit":
			b.WriteString(fmt.Sprintf("// emit %s not wired yet\n", st.EmitEvent))
			if st.EmitExpr != "" {
				b.WriteString("// payload: " + st.EmitExpr + "\n")
			}
			b.WriteString("if os.Getenv(\"APP_ENV\") != \"production\" { panic(\"impl_steps emit not supported\") }\n")
		default:
			b.WriteString("// unknown impl_step kind\n")
		}
	}
	b.WriteString("return resp, nil\n")
	return b.String()
}

func valString(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return fallback
}

func exportName(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}
