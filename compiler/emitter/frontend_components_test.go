package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestBuildTableData_SkipsAuthInjectedFields(t *testing.T) {
	t.Parallel()

	method := normalizer.Method{
		Name: "ListAllChatsForTheCurrentUser",
		Input: normalizer.Entity{
			Name: "ListAllChatsForTheCurrentUserRequest",
			Fields: []normalizer.Field{
				{Name: "CompanyID", Type: "string"},
				{Name: "UserID", Type: "string"},
				{Name: "Limit", Type: "int"},
				{Name: "Offset", Type: "int"},
			},
		},
	}
	endpoints := []normalizer.Endpoint{
		{
			ServiceName: "Chat",
			RPC:         "ListAllChatsForTheCurrentUser",
			AuthInject:  []string{"companyId", "userId"},
		},
	}

	got := buildTableData("Chat", method, nil, endpoints)

	if len(got.ExtraProps) != 0 {
		t.Fatalf("expected auth-injected fields to be omitted from table props, got %#v", got.ExtraProps)
	}
	if len(got.QueryParams) != 2 {
		t.Fatalf("expected only pagination params to remain, got %#v", got.QueryParams)
	}
	if got.QueryParams[0].Name != "limit" || got.QueryParams[1].Name != "offset" {
		t.Fatalf("expected only limit/offset query params, got %#v", got.QueryParams)
	}
}
