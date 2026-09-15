package emitter

import (
	"slices"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func TestUsageCapabilitiesFollowWhatTheProjectUses(t *testing.T) {
	if got := UsageCapabilities(MainContext{}, &ir.Schema{}); len(got) != 0 {
		t.Fatalf("empty project uses %v", got)
	}
	cases := []struct {
		name   string
		ctx    MainContext
		schema *ir.Schema
		want   string
	}{
		{"cache", MainContext{HasCache: true}, &ir.Schema{}, "uses_redis_client"},
		{"redis refresh store", MainContext{AuthService: "Auth", AuthRefreshStore: "redis"}, &ir.Schema{}, "uses_redis_client"},
		{"state actions", MainContext{}, &ir.Schema{Services: []ir.Service{{Name: "S", Methods: []ir.Method{{Name: "M", Flow: []ir.FlowStep{{Action: "ratelimit.Check"}}}}}}}, "uses_redis_client"},
		{"mongo entity", MainContext{}, &ir.Schema{Entities: []ir.Entity{{Name: "Chat", Metadata: map[string]any{"storage": "mongo"}}}}, "uses_mongo"},
		{"storage step", MainContext{}, &ir.Schema{Services: []ir.Service{{Name: "S", Methods: []ir.Method{{Name: "M", Flow: []ir.FlowStep{{Action: "storage.Upload"}}}}}}}, "uses_s3"},
		{"memory store", MainContext{AuthService: "Auth", AuthRefreshStore: "memory"}, &ir.Schema{}, "uses_refresh_store_memory"},
		{"hybrid store", MainContext{AuthService: "Auth", AuthRefreshStore: "hybrid"}, &ir.Schema{}, "uses_refresh_store_hybrid"},
	}
	for _, c := range cases {
		if got := UsageCapabilities(c.ctx, c.schema); !slices.Contains(got, c.want) {
			t.Errorf("%s: %v lacks %s", c.name, got, c.want)
		}
	}
	if got := UsageCapabilities(MainContext{AuthService: "Auth", AuthRefreshStore: "memory"}, &ir.Schema{}); slices.Contains(got, "uses_redis_client") || slices.Contains(got, "uses_refresh_store_postgres") {
		t.Fatalf("memory refresh store must not need redis or postgres: %v", got)
	}
}
