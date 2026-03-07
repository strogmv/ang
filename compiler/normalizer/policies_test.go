package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractPolicies(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#Policies: {
			CompanyAdminOnly: {
				description: "owner/admin inside same company"
				roles: ["owner", "admin"]
				sameCompany: true
				allowAdminOverride: true
			}
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	policies, err := n.ExtractPolicies(val)
	if err != nil {
		t.Fatalf("ExtractPolicies failed: %v", err)
	}
	p, ok := policies["CompanyAdminOnly"]
	if !ok {
		t.Fatalf("policy CompanyAdminOnly not found")
	}
	if !p.SameCompany {
		t.Fatalf("expected SameCompany=true")
	}
	if !p.AllowAdminOverride {
		t.Fatalf("expected AllowAdminOverride=true")
	}
	if len(p.Roles) != 2 || p.Roles[0] != "owner" || p.Roles[1] != "admin" {
		t.Fatalf("unexpected roles: %+v", p.Roles)
	}
}
