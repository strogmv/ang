package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestEmitGRPCProtoAndTransportFromIR(t *testing.T) {
	root := t.TempDir()
	schema := &ir.Schema{
		Services: []ir.Service{
			{
				Name: "Auth",
				Methods: []ir.Method{
					{
						Name: "Login",
						Input: &ir.Entity{
							Name: "LoginRequest",
							Fields: []ir.Field{
								{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}},
								{Name: "attempt", Type: ir.TypeRef{Kind: ir.KindInt}},
							},
						},
						Output: &ir.Entity{
							Name: "LoginResponse",
							Fields: []ir.Field{
								{Name: "token", Type: ir.TypeRef{Kind: ir.KindString}},
							},
						},
					},
				},
			},
		},
	}

	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.GoModule = "github.com/acme/demo"

	if err := em.EmitGRPCProtoFromIR(schema); err != nil {
		t.Fatalf("EmitGRPCProtoFromIR failed: %v", err)
	}
	if err := em.EmitGRPCTransportFromIR(schema); err != nil {
		t.Fatalf("EmitGRPCTransportFromIR failed: %v", err)
	}

	protoPath := filepath.Join(root, "api", "grpc", "service.proto")
	serverPath := filepath.Join(root, "internal", "transport", "grpc", "server.go")
	readmePath := filepath.Join(root, "internal", "transport", "grpc", "README.md")

	protoBytes, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read proto file: %v", err)
	}
	serverBytes, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("read grpc server file: %v", err)
	}
	readmeBytes, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read grpc readme: %v", err)
	}

	protoText := string(protoBytes)
	if !strings.Contains(protoText, `syntax = "proto3";`) {
		t.Fatalf("proto missing syntax header")
	}
	if !strings.Contains(protoText, `service AuthService`) {
		t.Fatalf("proto missing service declaration")
	}
	if !strings.Contains(protoText, `rpc Login (LoginRequest) returns (LoginResponse);`) {
		t.Fatalf("proto missing rpc declaration")
	}
	if !strings.Contains(protoText, `option go_package = "github.com/acme/demo/internal/transport/grpc/pb;pb";`) {
		t.Fatalf("proto missing go_package option")
	}

	serverText := string(serverBytes)
	if !strings.Contains(serverText, "type Server struct") {
		t.Fatalf("grpc server scaffold missing Server type")
	}
	if !strings.Contains(serverText, `"Auth"`) {
		t.Fatalf("grpc server scaffold missing service list")
	}

	if !strings.Contains(string(readmeBytes), "gRPC Transport Scaffold") {
		t.Fatalf("grpc readme missing title")
	}
}
