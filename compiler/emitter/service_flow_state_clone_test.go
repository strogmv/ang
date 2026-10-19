package emitter

import (
	"reflect"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// cloneFlowState must carry every field: a field it forgot (the service
// catalog) silently broke steps inside nested blocks.
func TestCloneFlowStateCopiesEveryField(t *testing.T) {
	n := 1
	typed := flowir.TypedStep{Name: "flow.If"}
	st := &flowRenderState{
		declared:          map[string]bool{"a": true},
		pointers:          map[string]bool{"a": true},
		types:             map[string]string{"a": "int"},
		goroutineMode:     true,
		stepN:             &n,
		returnErrOnly:     true,
		deferMode:         true,
		concurrMode:       "race",
		concurrVarPfx:     "_fr_0",
		sagaCompVar:       "_comp",
		serviceName:       "Svc",
		opName:            "Svc.Op",
		warningSink:       func(normalizer.Warning) {},
		eventDefsByName:   map[string]normalizer.EventDef{"E": {}},
		entityDefsByName:  map[string]normalizer.Entity{"X": {}},
		serviceDefsByName: map[string]normalizer.Service{"Svc": {}},
		isStreaming:       true,
		infraValues:       map[string]any{"k": 1},
		currentTyped:      &typed,
	}
	orig := reflect.ValueOf(st).Elem()
	for i := 0; i < orig.NumField(); i++ {
		if orig.Field(i).IsZero() {
			t.Fatalf("test state leaves %s unset; set it so the clone is checked", orig.Type().Field(i).Name)
		}
	}
	cp := reflect.ValueOf(cloneFlowState(st)).Elem()
	for i := 0; i < cp.NumField(); i++ {
		if cp.Field(i).IsZero() {
			t.Errorf("cloneFlowState drops %s", cp.Type().Field(i).Name)
		}
	}
}
