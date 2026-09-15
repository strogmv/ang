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
