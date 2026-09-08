package rawbody

import (
	"reflect"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func method(attrs []string, fields ...string) normalizer.Method {
	m := normalizer.Method{Name: "Op"}
	for _, name := range attrs {
		m.Attributes = append(m.Attributes, normalizer.Attribute{Name: name})
	}
	for _, name := range fields {
		m.Input.Fields = append(m.Input.Fields, normalizer.Field{Name: name, Type: "string"})
	}
	return m
}

func TestPassthroughImplicitWebhook(t *testing.T) {
	// The historical shape: a webhook whose only non-body field comes from the path.
	m := method(nil, "botId", "body")
	if !Passthrough(m, "/tg/webhook/{botId}") {
		t.Fatal("expected the webhook shape to keep raw-body passthrough")
	}
	if !Implicit(m, "/tg/webhook/{botId}") {
		t.Fatal("expected the passthrough to be reported as implicit")
	}
}

func TestPassthroughRequiresBodyField(t *testing.T) {
	m := method(nil, "token", "text")
	if Passthrough(m, "/links/{token}/messages") {
		t.Fatal("an input without a body field must decode JSON")
	}
	if Implicit(m, "/links/{token}/messages") {
		t.Fatal("nothing to report without a body field")
	}
}

func TestPassthroughNotTriggeredWhenOtherFieldsAreNotPathParams(t *testing.T) {
	m := method(nil, "body", "kind")
	if Passthrough(m, "/messages") {
		t.Fatal("a body field next to a free field must not switch the decoder")
	}
}

func TestAttributeMakesPassthroughExplicit(t *testing.T) {
	m := method([]string{Attribute}, "body", "kind")
	if !Passthrough(m, "/messages") {
		t.Fatal("@rawBody() must switch the decoder even with extra fields")
	}
	if Implicit(m, "/messages") {
		t.Fatal("a declared passthrough is not implicit")
	}
	if !HasAttribute(m) {
		t.Fatal("expected the attribute to be detected")
	}
}

func TestAttributeMatchIsCaseInsensitive(t *testing.T) {
	if !HasAttribute(method([]string{"RawBody"}, "body")) {
		t.Fatal("attribute matching must ignore case")
	}
	if HasAttribute(method([]string{"table"}, "body")) {
		t.Fatal("unrelated attributes must not match")
	}
}

func TestIgnoredFields(t *testing.T) {
	m := method([]string{Attribute}, "bot_id", "body", "kind", "locale")
	got := IgnoredFields(m.Input, "/tg/webhook/{botId}")
	want := []string{"kind", "locale"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if fields := IgnoredFields(method(nil, "botId", "body").Input, "/tg/webhook/{botId}"); len(fields) != 0 {
		t.Fatalf("expected no ignored fields, got %v", fields)
	}
}
