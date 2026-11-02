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

func TestPlayableSceneClientCatalogueExcludesPatrolsAndStock(t *testing.T) {
	data, err := clientResource(Compiled{PlayableScenes: json.RawMessage(`{"z":{"id":"z","scenario":"life:z","level":"deck","name_key":"scene.z","guard_route":[{"x":9}],"sources":[{"secret":"stock"}],"interactions":[{"effect":"take"}],"presentation":{"lights":[{"source_id":"lamp"}]}},"a":{"id":"a","scenario":"life:a"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	var resource struct {
		Scenes []map[string]json.RawMessage `json:"playable_scenes"`
	}
	if err := json.Unmarshal(data, &resource); err != nil {
		t.Fatal(err)
	}
	if len(resource.Scenes) != 2 || string(resource.Scenes[0]["id"]) != `"a"` || string(resource.Scenes[1]["id"]) != `"z"` {
		t.Fatal("scene launch catalogue is not deterministic")
	}
	for _, scene := range resource.Scenes {
		for _, key := range []string{"guard_route", "sources", "interactions"} {
			if _, ok := scene[key]; ok {
				t.Fatalf("server scene data leaked: %s", key)
			}
		}
	}
	if !strings.Contains(string(resource.Scenes[1]["presentation"]), "lamp") {
		t.Fatal("physical lamp presentation missing")
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

func TestUnityCaptureProofPolicyComesFromCompiledCUE(t *testing.T) {
	for _, policy := range [][2]int{{1, 2}, {2, 5}} {
		c := Compiled{}
		c.Inventory.Preparation.MinContactMembers = policy[0]
		c.Inventory.Preparation.SupportDistance = policy[1]
		data, err := clientResource(c)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Config struct {
				Min      int `json:"capture_min_contact_members"`
				Distance int `json:"capture_support_distance_cells"`
			} `json:"config"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Config.Min != policy[0] || got.Config.Distance != policy[1] {
			t.Fatalf("capture proof used stale constants: %s", data)
		}
	}
}
