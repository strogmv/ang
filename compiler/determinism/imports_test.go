package determinism

import "testing"

func TestGeneratedRNGImports(t *testing.T) {
	for _, path := range []string{"math/rand", "math/rand/v2"} {
		for _, alias := range []string{"", "r ", "_ ", ". "} {
			if CheckImports("generated.go", []byte("package demo\nimport "+alias+"\""+path+"\"\n")) == nil {
				t.Fatalf("accepted %s %s", alias, path)
			}
		}
	}
	if err := CheckImports("generated.go", []byte("package demo\nimport \"math\"\n// math/rand is forbidden\n")); err != nil {
		t.Fatal(err)
	}
}
