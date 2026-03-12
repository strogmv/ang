package normalizer

import (
	"strings"
	"time"

	"cuelang.org/go/cue"
)

func (n *Normalizer) LoadCodegenConfig(val cue.Value) error {
	codegen := val.LookupPath(cue.ParsePath("#Codegen"))
	if !codegen.Exists() {
		return nil
	}

	mappingVal := codegen.LookupPath(cue.ParsePath("type_mapping"))
	if mappingVal.Exists() {
		iter, _ := mappingVal.Fields()
		for iter.Next() {
			typeName := iter.Selector().String()
			v := iter.Value()

			cfg := TypeConfig{
				GoType:     getString(v, "type"),
				Package:    getString(v, "pkg"),
				SQLType:    getString(v, "sql"),
				NullHelper: getString(v, "null"),
			}
			n.TypeMapping[typeName] = cfg
		}
	}
	return nil
}

// ExtractConfig parses #AppConfig.
func (n *Normalizer) ExtractConfig(val cue.Value) (*ConfigDef, error) {
	cfgVal := val.LookupPath(cue.ParsePath("#AppConfig"))
	if !cfgVal.Exists() {
		return nil, nil
	}

	ent, err := n.parseEntity("AppConfig", cfgVal)
	if err != nil {
		return nil, err
	}
	return &ConfigDef{Fields: ent.Fields}, nil
}

// ExtractAuth parses #Auth definition from infra.
func (n *Normalizer) ExtractAuth(val cue.Value) (*AuthDef, error) {
	authVal := val.LookupPath(cue.ParsePath("#Auth"))
	if !authVal.Exists() {
		return nil, nil
	}

	jwtVal := authVal.LookupPath(cue.ParsePath("jwt"))
	if !jwtVal.Exists() {
		return nil, nil
	}

	getClaim := func(name, def string) string {
		v := jwtVal.LookupPath(cue.ParsePath("claims." + name + ".field"))
		s, _ := v.String()
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		return s
	}

	alg, _ := jwtVal.LookupPath(cue.ParsePath("alg")).String()
	alg = strings.TrimSpace(alg)
	issuer, _ := jwtVal.LookupPath(cue.ParsePath("issuer")).String()
	issuer = strings.TrimSpace(issuer)
	audience, _ := jwtVal.LookupPath(cue.ParsePath("audience")).String()
	audience = strings.TrimSpace(audience)
	accessTTL, _ := jwtVal.LookupPath(cue.ParsePath("tokens.access_ttl")).String()
	refreshTTL, _ := jwtVal.LookupPath(cue.ParsePath("tokens.refresh_ttl")).String()
	rotation, _ := jwtVal.LookupPath(cue.ParsePath("tokens.rotation")).Bool()
	refreshStore, _ := jwtVal.LookupPath(cue.ParsePath("tokens.store")).String()
	if alg == "" {
		alg = "RS256"
	}

	service, _ := authVal.LookupPath(cue.ParsePath("service")).String()
	service = normalizeServiceName(strings.TrimSpace(service))
	loginOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.op")).String()
	loginAccess, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.access_field")).String()
	loginRefresh, _ := jwtVal.LookupPath(cue.ParsePath("ops.login.refresh_field")).String()
	refreshOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.op")).String()
	refreshTokenField, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.token_field")).String()
	refreshAccess, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.access_field")).String()
	refreshRefresh, _ := jwtVal.LookupPath(cue.ParsePath("ops.refresh.refresh_field")).String()
	logoutOp, _ := jwtVal.LookupPath(cue.ParsePath("ops.logout.op")).String()
	logoutTokenField, _ := jwtVal.LookupPath(cue.ParsePath("ops.logout.token_field")).String()

	return &AuthDef{
		Alg:                 alg,
		Issuer:              issuer,
		Audience:            audience,
		UserIDClaim:         getClaim("userId", "sub"),
		CompanyIDClaim:      getClaim("companyId", "cid"),
		RolesClaim:          getClaim("roles", "roles"),
		PermissionsClaim:    getClaim("perms", "perms"),
		AccessTTL:           strings.TrimSpace(accessTTL),
		RefreshTTL:          strings.TrimSpace(refreshTTL),
		Rotation:            rotation,
		RefreshStore:        strings.TrimSpace(refreshStore),
		Service:             service,
		LoginOp:             strings.TrimSpace(loginOp),
		LoginAccessField:    strings.TrimSpace(loginAccess),
		LoginRefreshField:   strings.TrimSpace(loginRefresh),
		RefreshOp:           strings.TrimSpace(refreshOp),
		RefreshTokenField:   strings.TrimSpace(refreshTokenField),
		RefreshAccessField:  strings.TrimSpace(refreshAccess),
		RefreshRefreshField: strings.TrimSpace(refreshRefresh),
		LogoutOp:            strings.TrimSpace(logoutOp),
		LogoutTokenField:    strings.TrimSpace(logoutTokenField),
	}, nil
}

// ExtractSession parses #Session definition from infra.
// CUE shape:
//
//	#Session: {
//	    cookieName: "sendbox_session"   // default "session_id"
//	    ttl:        "8760h"             // optional, default 365 days
//	}
func (n *Normalizer) ExtractSession(val cue.Value) (*SessionDef, error) {
	sessVal := val.LookupPath(cue.ParsePath("#Session"))
	if !sessVal.Exists() {
		return nil, nil
	}

	cookieName, _ := sessVal.LookupPath(cue.ParsePath("cookieName")).String()
	cookieName = strings.TrimSpace(cookieName)
	if cookieName == "" {
		cookieName = "session_id"
	}

	ttlSeconds := 365 * 24 * 3600 // default 1 year
	if ttlStr, err := sessVal.LookupPath(cue.ParsePath("ttl")).String(); err == nil {
		ttlStr = strings.TrimSpace(ttlStr)
		if d, err := time.ParseDuration(ttlStr); err == nil && d > 0 {
			ttlSeconds = int(d.Seconds())
		}
	}

	return &SessionDef{
		CookieName: cookieName,
		TTLSeconds: ttlSeconds,
	}, nil
}

// ExtractRBAC parses #RBAC.
func (n *Normalizer) ExtractRBAC(val cue.Value) (*RBACDef, error) {
	rbacVal := val.LookupPath(cue.ParsePath("#RBAC"))
	if !rbacVal.Exists() {
		return n.extractRBACFromPolicy(val)
	}

	rbac := &RBACDef{
		Roles:       make(map[string][]string),
		Permissions: make(map[string]string),
	}

	rolesVal := rbacVal.LookupPath(cue.ParsePath("roles"))
	iter, _ := rolesVal.Fields()
	for iter.Next() {
		roleName := iter.Selector().String()
		var perms []string
		list, _ := iter.Value().List()
		for list.Next() {
			p, _ := list.Value().String()
			perms = append(perms, strings.Trim(p, ""))
		}
		rbac.Roles[roleName] = perms
	}

	permsVal := rbacVal.LookupPath(cue.ParsePath("permissions"))
	if permsVal.Exists() {
		pit, _ := permsVal.Fields()
		for pit.Next() {
			permName := strings.TrimSpace(strings.Trim(pit.Selector().String(), "\""))
			desc, _ := pit.Value().String()
			rbac.Permissions[permName] = strings.TrimSpace(desc)
		}
	}

	return rbac, nil
}

func (n *Normalizer) extractRBACFromPolicy(val cue.Value) (*RBACDef, error) {
	rolesVal := val.LookupPath(cue.ParsePath("Roles"))
	actionsVal := val.LookupPath(cue.ParsePath("Actions"))
	policiesVal := val.LookupPath(cue.ParsePath("Policies"))
	if !rolesVal.Exists() && !actionsVal.Exists() && !policiesVal.Exists() {
		return nil, nil
	}

	rbac := &RBACDef{
		Roles:       make(map[string][]string),
		Permissions: make(map[string]string),
	}

	allPerms := make(map[string]bool)
	actionByResource := make(map[string][]string)

	if actionsVal.Exists() {
		rIter, _ := actionsVal.Fields()
		for rIter.Next() {
			resource := strings.TrimSpace(rIter.Selector().String())
			aIter, _ := rIter.Value().Fields()
			for aIter.Next() {
				action := strings.TrimSpace(aIter.Selector().String())
				perm := resource + "." + action
				allPerms[perm] = true
				actionByResource[resource] = append(actionByResource[resource], perm)
			}
		}
	}

	if policiesVal.Exists() {
		pIter, _ := policiesVal.Fields()
		for pIter.Next() {
			roleName := strings.TrimSpace(pIter.Selector().String())
			allowVal := pIter.Value().LookupPath(cue.ParsePath("allow"))
			if !allowVal.Exists() {
				continue
			}
			list, _ := allowVal.List()
			for list.Next() {
				raw, _ := list.Value().String()
				pattern := strings.TrimSpace(raw)
				if pattern == "*" {
					for perm := range allPerms {
						rbac.Roles[roleName] = append(rbac.Roles[roleName], perm)
					}
					continue
				}
				if strings.HasSuffix(pattern, ".*") {
					resource := strings.TrimSuffix(pattern, ".*")
					for _, perm := range actionByResource[resource] {
						rbac.Roles[roleName] = append(rbac.Roles[roleName], perm)
					}
					continue
				}
				rbac.Roles[roleName] = append(rbac.Roles[roleName], pattern)
				allPerms[pattern] = true
			}
		}
	}

	for perm := range allPerms {
		rbac.Permissions[perm] = ""
	}

	return rbac, nil
}

// ExtractNotificationMuting parses #NotificationMuting from infra.
