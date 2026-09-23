package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

func wsAuthCheckFixture(check string) ([]ir.Endpoint, []ir.Service) {
	str := ir.TypeRef{Kind: ir.KindString}
	services := []ir.Service{{
		Name: "Bids",
		Methods: []ir.Method{
			{
				Name: "StreamTenderUpdates",
				Input: &ir.Entity{Name: "StreamTenderUpdatesRequest", Fields: []ir.Field{
					{Name: "tenderID", Type: str},
					{Name: "companyID", Type: str, Optional: true},
				}},
				Output: &ir.Entity{Name: "StreamTenderUpdatesResponse"},
			},
			{
				Name: "CheckTenderAccess",
				Input: &ir.Entity{Name: "CheckTenderAccessRequest", Fields: []ir.Field{
					{Name: "tenderID", Type: str},
					{Name: "companyID", Type: str},
					{Name: "userID", Type: str},
				}},
				Output: &ir.Entity{Name: "CheckTenderAccessResponse", Fields: []ir.Field{{Name: "allowed", Type: ir.TypeRef{Kind: ir.KindBool}}}},
			},
		},
	}}
	endpoints := []ir.Endpoint{{
		Method:    "WS",
		Path:      "/ws/tenders/{tenderId}",
		Service:   "Bids",
		RPC:       "StreamTenderUpdates",
		RoomParam: "tenderId",
		Auth:      &ir.EndpointAuth{Type: "jwt", Check: check},
	}}
	return endpoints, services
}

func emitWS(t *testing.T, endpoints []ir.Endpoint, services []ir.Service) (string, error) {
	t.Helper()
	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.Version = "0.1.0"
	em.GoModule = "example.com/app"
	if err := em.EmitWebSocket(endpoints, services, nil); err != nil {
		return "", err
	}
	src, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "ws_bids.go"))
	if err != nil {
		t.Fatalf("read ws_bids.go: %v", err)
	}
	return string(src), nil
}

// A WS endpoint with auth.check runs the check before the upgrade, on the
// handshake's own identity, with the check's field names (companyID, not
// companyId), and answers 401/403 without upgrading.
func TestEmitWebSocket_AuthCheckRunsBeforeUpgrade(t *testing.T) {
	t.Parallel()
	endpoints, services := wsAuthCheckFixture("CheckTenderAccess")
	src, err := emitWS(t, endpoints, services)
	if err != nil {
		t.Fatalf("EmitWebSocket: %v", err)
	}

	for _, want := range []string{
		`"example.com/app/internal/pkg/errors"`,
		"wsIdent, wsIdentErr := wsHandshakeIdentity(r)",
		"writeSessionFailure(w, r, wsIdentErr)",
		"http.StatusUnauthorized",
		`authReq.TenderID = chi.URLParam(r, "tenderId")`,
		"authReq.CompanyID = wsIdent.CompanyID",
		"authReq.UserID = wsIdent.UserID",
		"svc.CheckTenderAccess(r.Context(), authReq)",
		"http.StatusForbidden",
		"allowDynamicRooms: false",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ws_bids.go lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, "CompanyId") {
		t.Fatalf("auth request field must use the check's Go name CompanyID:\n%s", src)
	}
	check := strings.Index(src, "svc.CheckTenderAccess(")
	upgrade := strings.Index(src, "wsUpgrader.Upgrade(")
	if check < 0 || upgrade < 0 || check > upgrade {
		t.Fatalf("access check must precede the upgrade (check=%d upgrade=%d)", check, upgrade)
	}
	// The identity was settled before the upgrade; the auth frame is not read.
	if strings.Contains(src, "wsReadAuthTokenOrQuery") {
		t.Fatalf("checked endpoint must not re-authenticate after the upgrade:\n%s", src)
	}
}

func TestEmitWebSocket_WithoutAuthCheckKeepsPostUpgradeAuth(t *testing.T) {
	t.Parallel()
	endpoints, services := wsAuthCheckFixture("")
	src, err := emitWS(t, endpoints, services)
	if err != nil {
		t.Fatalf("EmitWebSocket: %v", err)
	}
	if !strings.Contains(src, "_, _, wsAuthErr := wsReadAuthTokenOrQuery(conn, r)") {
		t.Fatalf("unchecked endpoint must keep post-upgrade auth:\n%s", src)
	}
	for _, unwanted := range []string{"wsHandshakeIdentity", "internal/pkg/errors", "authReq"} {
		if strings.Contains(src, unwanted) {
			t.Fatalf("unchecked endpoint must not contain %q:\n%s", unwanted, src)
		}
	}
}

func TestEmitWebSocket_AuthCheckMustExist(t *testing.T) {
	t.Parallel()
	endpoints, services := wsAuthCheckFixture("NoSuchCheck")
	if _, err := emitWS(t, endpoints, services); err == nil || !strings.Contains(err.Error(), "NoSuchCheck") {
		t.Fatalf("expected an error naming the missing check, got %v", err)
	}
}

func TestEmitWSCommon_HandshakeIdentity(t *testing.T) {
	t.Parallel()
	endpoints, services := wsAuthCheckFixture("CheckTenderAccess")
	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.GoModule = "example.com/app"
	if err := em.EmitWebSocket(endpoints, services, nil); err != nil {
		t.Fatalf("EmitWebSocket: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "ws_common.go"))
	if err != nil {
		t.Fatalf("read ws_common.go: %v", err)
	}
	if !strings.Contains(string(src), "func wsHandshakeIdentity(r *http.Request) (wsIdentity, error)") {
		t.Fatalf("ws_common.go lacks wsHandshakeIdentity")
	}
}
