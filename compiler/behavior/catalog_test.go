package behavior

import "testing"

func TestAICatalogRejectsUnboundedOrMissingTasks(t *testing.T) {
	valid := []byte(`{"actions":{"action.done":{"id":"action.done"}},"tasks":{"task.root":{"id":"task.root","methods":[{"id":"method.done","steps":["action.done"]}]}}}`)
	if err := ValidateCatalog(valid, "task.root"); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, data, root string }{
		{"missing root", string(valid), "task.missing"},
		{"cycle", `{"tasks":{"task.root":{"id":"task.root","methods":[{"id":"method.loop","steps":["task.root"]}]}}}`, "task.root"},
		{"unknown step", `{"tasks":{"task.root":{"id":"task.root","methods":[{"id":"method.bad","steps":["action.missing"]}]}}}`, "task.root"},
		{"duplicate method", `{"tasks":{"task.root":{"id":"task.root","methods":[{"id":"method.same","steps":[]},{"id":"method.same","steps":[]}]}}}`, "task.root"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateCatalog([]byte(c.data), c.root); err == nil {
				t.Fatal("accepted invalid AI graph")
			}
		})
	}
}
