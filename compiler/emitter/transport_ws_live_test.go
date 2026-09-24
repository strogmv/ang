package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func liveTestFixture() (WsEndpointView, map[string]normalizer.Method, map[string]normalizer.Entity) {
	fields := func(names ...string) normalizer.Entity {
		ent := normalizer.Entity{}
		for _, n := range names {
			ent.Fields = append(ent.Fields, normalizer.Field{Name: n, Type: "string"})
		}
		return ent
	}
	view := WsEndpointView{
		Endpoint: normalizer.Endpoint{
			Method: "WS", Path: "/ws/rooms/{roomId}", RPC: "StreamRoom", ServiceName: "rooms", AuthCheck: "CheckRoom",
			Live: &normalizer.LiveRoomDef{State: "BuildRoomState", View: "ViewRoom", Triggers: []string{"RoomChanged", "RoomChanged"}},
		},
		RoomParam:             "roomId",
		AuthCheckCompanyField: "CompanyID",
	}
	methods := map[string]normalizer.Method{
		"BuildRoomState": {Name: "BuildRoomState", Input: fields("roomID"), Output: fields("state")},
		"ViewRoom":       {Name: "ViewRoom", Input: fields("state", "companyIds", "now"), Output: fields("snapshots", "denied", "refreshAt")},
	}
	events := map[string]normalizer.Entity{"RoomChanged": fields("roomId", "at")}
	return view, methods, events
}

func TestResolveWSLiveRoom(t *testing.T) {
	view, methods, events := liveTestFixture()
	if err := resolveWSLiveRoom(&view, methods, events); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !view.IsLive || view.LiveName != "streamroom" || view.LiveStateRoomField != "RoomID" {
		t.Fatalf("unexpected view: live=%v name=%q roomField=%q", view.IsLive, view.LiveName, view.LiveStateRoomField)
	}
	if view.LiveViewCompaniesField != "CompanyIds" || view.LiveViewRefreshField != "RefreshAt" {
		t.Fatalf("unexpected view fields: %q %q", view.LiveViewCompaniesField, view.LiveViewRefreshField)
	}
	if len(view.LiveTriggers) != 1 || view.LiveTriggers[0].RoomField != "RoomID" || view.LiveTriggers[0].GoType != "RoomChanged" {
		t.Fatalf("triggers: %+v", view.LiveTriggers)
	}
}

func TestResolveWSLiveRoomRejectsBrokenContracts(t *testing.T) {
	cases := map[string]struct {
		mutate func(*WsEndpointView, map[string]normalizer.Method, map[string]normalizer.Entity)
		want   string
	}{
		"no auth check": {func(v *WsEndpointView, _ map[string]normalizer.Method, _ map[string]normalizer.Entity) {
			v.AuthCheck, v.AuthCheckCompanyField = "", ""
		}, "needs auth.check"},
		"unknown view op": {func(v *WsEndpointView, m map[string]normalizer.Method, _ map[string]normalizer.Entity) {
			delete(m, "ViewRoom")
		}, `view "ViewRoom" is not a method`},
		"view without refreshAt": {func(v *WsEndpointView, m map[string]normalizer.Method, _ map[string]normalizer.Entity) {
			op := m["ViewRoom"]
			op.Output.Fields = op.Output.Fields[:2]
			m["ViewRoom"] = op
		}, `no field "refreshAt"`},
		"trigger without room field": {func(v *WsEndpointView, _ map[string]normalizer.Method, e map[string]normalizer.Entity) {
			e["RoomChanged"] = normalizer.Entity{Fields: []normalizer.Field{{Name: "at"}}}
		}, `has no field "roomId"`},
		"unknown trigger": {func(v *WsEndpointView, _ map[string]normalizer.Method, e map[string]normalizer.Entity) {
			delete(e, "RoomChanged")
		}, "is not an event"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			view, methods, events := liveTestFixture()
			tc.mutate(&view, methods, events)
			err := resolveWSLiveRoom(&view, methods, events)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
