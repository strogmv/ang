package normalizer

import (
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// ExtractPolicies reads typed policy registry from cue/policy (optional).
// Expected shape:
//
//	#Policies: {
//	  CompanyAdminOnly: {
//	    roles: ["owner", "admin"]
//	    sameCompany: true
//	    allowAdminOverride: true
//	  }
//	}
func (n *Normalizer) ExtractPolicies(val cue.Value) (map[string]PolicyDef, error) {
	if !val.Exists() {
		return map[string]PolicyDef{}, nil
	}

	lookup := func(name string) cue.Value {
		v := val.LookupPath(cue.ParsePath(name))
		if v.Exists() {
			return v
		}
		return cue.Value{}
	}

	policiesVal := lookup("#Policies")
	if !policiesVal.Exists() {
		policiesVal = lookup("Policies")
	}
	if !policiesVal.Exists() {
		return map[string]PolicyDef{}, nil
	}

	out := make(map[string]PolicyDef)
	it, err := policiesVal.Fields()
	if err != nil {
		return nil, err
	}
	for it.Next() {
		name := strings.TrimSpace(it.Selector().String())
		if name == "" {
			continue
		}
		v := it.Value()
		p := PolicyDef{
			Name:               name,
			AllowAdminOverride: true,
			Source:             formatPos(v),
		}
		if desc, err := v.LookupPath(cue.ParsePath("description")).String(); err == nil {
			p.Description = strings.TrimSpace(desc)
		}
		if rolesVal := v.LookupPath(cue.ParsePath("roles")); rolesVal.Exists() && rolesVal.Kind() == cue.ListKind {
			list, _ := rolesVal.List()
			for list.Next() {
				if s, err := list.Value().String(); err == nil {
					s = strings.TrimSpace(s)
					if s != "" {
						p.Roles = append(p.Roles, s)
					}
				}
			}
		}
		if b, err := v.LookupPath(cue.ParsePath("sameCompany")).Bool(); err == nil {
			p.SameCompany = b
		}
		if b, err := v.LookupPath(cue.ParsePath("allowAdminOverride")).Bool(); err == nil {
			p.AllowAdminOverride = b
		}
		out[p.Name] = p
	}
	return out, nil
}

func sortedPolicyNames(policies map[string]PolicyDef) []string {
	out := make([]string, 0, len(policies))
	for name := range policies {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
