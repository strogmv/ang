package emitter

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderRepositoryPortAST_GeneratesValidGo(t *testing.T) {
	t.Parallel()

	em := New(".", ".", "templates")
	em.GoModule = "github.com/acme/project"

	repo := normalizer.Repository{
		Name:   "UserRepository",
		Entity: "User",
		Finders: []normalizer.RepositoryFinder{
			{
				Name: "FindByEmail",
				Where: []normalizer.FinderWhere{
					{Field: "email", Param: "email", ParamType: "string"},
				},
				Returns: "one",
			},
			{
				Name: "FindRecent",
				Where: []normalizer.FinderWhere{
					{Field: "createdAt", Param: "createdAfter", ParamType: "time.Time"},
				},
				Returns: "many",
			},
		},
	}

	src, err := em.renderRepositoryPortAST(repo)
	if err != nil {
		t.Fatalf("renderRepositoryPortAST failed: %v", err)
	}

	if !strings.Contains(string(src), "type UserRepository interface") {
		t.Fatalf("expected UserRepository interface, got:\n%s", string(src))
	}
	if !strings.Contains(string(src), "\"time\"") {
		t.Fatalf("expected time import for time-based finder, got:\n%s", string(src))
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "userrepository.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, string(src))
	}
}
