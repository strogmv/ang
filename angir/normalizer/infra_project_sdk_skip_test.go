package normalizer

import (
	"reflect"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractTargetReadsFrontendSDKSkip(t *testing.T) {
	val := cuecontext.New().CompileString(`
state: target: {
	lang: "go"
	frontend_app_dir: "../front/src/@sdk"
	frontend_sdk_skip: ["mocks", " format ", ""]
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(td.FrontendSDKSkip, []string{"mocks", "format"}) {
		t.Fatalf("FrontendSDKSkip = %#v", td.FrontendSDKSkip)
	}
	empty := cuecontext.New().CompileString(`state: target: lang: "go"`)
	if td, _ := New().ExtractTarget(empty); len(td.FrontendSDKSkip) != 0 {
		t.Fatalf("no list must mean nothing skipped, got %#v", td.FrontendSDKSkip)
	}
}

// A project whose SDK is generated straight into the application repository
// says so in CUE, not in the build command.
func TestExtractTargetReadsFrontendDir(t *testing.T) {
	val := cuecontext.New().CompileString(`
state: target: {
	lang: "go"
	frontend_dir: " ../dealingi-front/src/@sdk "
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	if td.FrontendDir != "../dealingi-front/src/@sdk" {
		t.Fatalf("FrontendDir = %q", td.FrontendDir)
	}
	empty := cuecontext.New().CompileString(`state: target: lang: "go"`)
	if td, _ := New().ExtractTarget(empty); td.FrontendDir != "" {
		t.Fatalf("no frontend_dir must stay empty, got %q", td.FrontendDir)
	}
}
