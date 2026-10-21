package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAIHashIgnoresMapOrderAndKeepsExactIntegers(t *testing.T) {
	a, err := canonicalJSONBytes([]byte(`{"b":9007199254740993,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalJSONBytes([]byte(`{"a":1,"b":9007199254740993}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) || !strings.Contains(string(a), "9007199254740993") {
		t.Fatalf("unstable or lossy canonical JSON: %s %s", a, b)
	}
}
func TestUnityResourceContainsNoServerAIOrActorDefinitions(t *testing.T) {
	data, err := clientResource(Compiled{Config: Config{TickHz: 10, InputAckBudgetMS: 30}, AI: json.RawMessage(`{"private":"server"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["ai"]; ok {
		t.Fatal("server AI leaked into Unity resource")
	}
	config := got["config"].(map[string]any)
	if _, ok := config["capture_demo"]; ok {
		t.Fatal("NPC definitions leaked into Unity resource")
	}
	if config["tick_hz"] != float64(10) || config["input_ack_budget_ms"] != float64(30) {
		t.Fatal("client parameters missing")
	}
}

func TestAIFixtureFactsCannotInventKnowledgeOrCompleteMission(t *testing.T) {
	data := []byte(`{"facts":{"security.absent":{},"custody.handed_over":{}}}`)
	if err := validateAIFixtureFacts(data, map[string]bool{"security.absent": true}); err != nil {
		t.Fatal(err)
	}
	for _, facts := range []map[string]bool{{"unknown.fact": true}, {"custody.handed_over": true}} {
		if err := validateAIFixtureFacts(data, facts); err == nil {
			t.Fatal("accepted invalid fixture fact")
		}
	}
}
