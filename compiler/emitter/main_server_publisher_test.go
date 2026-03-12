package emitter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestMainServerTemplate_UsesPublisherInterfaceForRuntimeContainer(t *testing.T) {
	t.Parallel()

	em := New(t.TempDir(), "sdk", "templates")
	tpl, err := em.parseMainServerTemplate()
	if err != nil {
		t.Fatalf("parseMainServerTemplate: %v", err)
	}

	ctx := MainContext{
		ServicesIR: []ir.Service{
			{Name: "Sandbox"},
		},
		EntitiesIR: []ir.Entity{
			{
				Name: "Project",
				Fields: []ir.Field{
					{Name: "id", Type: ir.TypeRef{Kind: ir.KindUUID}},
				},
			},
		},
		HasNats:           true,
		HasSQL:            true,
		WebSocketServices: map[string]bool{},
		WSEventMap:        map[string]map[string]bool{},
		EventPayloadsIR:   map[string]ir.Entity{},
		GoModule:          "github.com/acme/demo",
		ANGVersion:        "test",
		InputHash:         "test",
		CompilerHash:      "test",
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "main_server_root", buildMainServerTemplateData(ctx)); err != nil {
		t.Fatalf("execute main_server_root: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "var publisher port.Publisher") {
		t.Fatalf("expected publisher variable declaration in generated main.go:\n%s", out)
	}
	if !strings.Contains(out, "publisher = natsClient") {
		t.Fatalf("expected publisher assignment from natsClient in generated main.go:\n%s", out)
	}

	callStart := strings.Index(out, "bootstrap.NewRuntimeContainer(")
	if callStart == -1 {
		t.Fatalf("expected bootstrap.NewRuntimeContainer call in generated main.go:\n%s", out)
	}
	callRest := out[callStart:]
	callEnd := strings.Index(callRest, ")\n")
	if callEnd == -1 {
		t.Fatalf("failed to locate end of bootstrap.NewRuntimeContainer call in generated main.go")
	}
	call := callRest[:callEnd]

	if !strings.Contains(call, "publisher,") {
		t.Fatalf("expected runtime container call to pass publisher interface, got:\n%s", call)
	}
	if strings.Contains(call, "natsClient,") {
		t.Fatalf("runtime container call must not pass typed natsClient directly, got:\n%s", call)
	}
}
