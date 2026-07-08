package flowir

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestDecodeStepsBuildsTypedTree(t *testing.T) {
	rawChild := normalizer.FlowStep{Action: "uuid.New", Args: map[string]any{"output": "id"}}
	raw := normalizer.FlowStep{Action: "flow.If", Args: map[string]any{
		"condition": "true",
		"_then":     []normalizer.FlowStep{rawChild},
	}}

	steps, err := DecodeSteps([]normalizer.FlowStep{raw})
	if err != nil {
		t.Fatalf("DecodeSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("got %d root steps", len(steps))
	}
	if _, ok := steps[0].Action.(FlowIf); !ok {
		t.Fatalf("root action decoded as %T", steps[0].Action)
	}
	children := steps[0].Children["_then"]
	if len(children) != 1 {
		t.Fatalf("got %d children", len(children))
	}
	if _, ok := children[0].Action.(UUIDNew); !ok {
		t.Fatalf("child action decoded as %T", children[0].Action)
	}
}

func TestDecodeStepsReportsNestedError(t *testing.T) {
	raw := normalizer.FlowStep{Action: "flow.If", Args: map[string]any{
		"condition": "true",
		"_then":     []normalizer.FlowStep{{Action: "uuid.New", Args: map[string]any{}}},
	}}

	steps, err := DecodeSteps([]normalizer.FlowStep{raw})
	if err == nil {
		t.Fatal("expected nested decode error")
	}
	if steps[0].Children["_then"][0].DecodeError == nil {
		t.Fatal("nested TypedStep did not retain decode error")
	}
}

func TestDecodeStepsRetainsTypedScalarCompatibilityArgs(t *testing.T) {
	steps, err := DecodeSteps([]normalizer.FlowStep{{Action: "http.Call", Args: map[string]any{
		"method": "GET", "url": `"https://example.test"`, "attempts": 4, "failOnError": false,
	}}})
	if err != nil {
		t.Fatalf("DecodeSteps: %v", err)
	}
	args := steps[0].ScalarArgs
	if args["attempts"].Kind != ScalarInt || args["attempts"].Source() != "4" {
		t.Fatalf("attempts scalar = %#v", args["attempts"])
	}
	if args["failOnError"].Kind != ScalarBool || args["failOnError"].Source() != "false" {
		t.Fatalf("failOnError scalar = %#v", args["failOnError"])
	}
}
