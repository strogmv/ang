package emitter

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/compiler"
)

// finderTestDomain is the entity the finder tests query: a uuid key, a
// unique (companyId, sku) index, a timestamp.
const finderTestDomain = `package domain

import "time"

Product: {
	id:        string    @db(type="UUID", primary_key=true)
	companyId: string    @db(type="UUID")
	sku:       string    @db(type="TEXT")
	name:      string    @db(type="TEXT")
	price:     int       @db(type="BIGINT")
	createdAt: time.Time @db(type="TIMESTAMPTZ")
	indexes: [{fields: ["companyId", "sku"], unique: true}]
}

// A scan target whose string field is stored as a date.
ProductStamp: {
	name:  string @db(type="TEXT")
	stamp: string @db(type="DATE")
} @dto(only="true")
`

// writeFinderProject copies angir/testdata/minimal into a temp dir and adds
// the Product entity and repoCUE as cue/repo/products.cue.
func writeFinderProject(t *testing.T, repoCUE string) string {
	t.Helper()
	src := filepath.Join("..", "..", "angir", "testdata", "minimal")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "cue", "domain", "product.cue"), []byte(finderTestDomain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "cue", "repo", "products.cue"), []byte(repoCUE), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// finderSchema runs the semantic and IR phases over a finder project.
func finderSchema(t *testing.T, repoCUE string) *ir.Schema {
	t.Helper()
	base := writeFinderProject(t, repoCUE)
	out, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	if err != nil {
		t.Fatalf("semantic phases: %v", err)
	}
	irOut, err := compiler.RunIRPhase(compiler.IRPhaseInput{Normalized: out, BasePath: base})
	if err != nil {
		t.Fatalf("IR phase: %v", err)
	}
	return irOut.Schema
}

// finderValidationError runs the semantic phases and returns their error.
func finderValidationError(t *testing.T, repoCUE string) error {
	t.Helper()
	base := writeFinderProject(t, repoCUE)
	_, err := compiler.RunSemanticPhasesWithOptions(base, compiler.PipelineOptions{})
	return err
}

// emitProductRepo generates the Product port and Postgres repository and
// returns both sources.
func emitProductRepo(t *testing.T, schema *ir.Schema) (port, repo string) {
	t.Helper()
	em := New(t.TempDir(), t.TempDir(), "templates")
	em.GoModule = "example.com/app"
	if err := em.EmitRepository(schema.Repos, schema.Entities); err != nil {
		t.Fatalf("EmitRepository: %v", err)
	}
	if err := em.EmitPostgresRepo(schema.Repos, schema.Entities); err != nil {
		t.Fatalf("EmitPostgresRepo: %v", err)
	}
	read := func(parts ...string) string {
		data, err := os.ReadFile(filepath.Join(append([]string{em.OutputDir}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	return read("internal", "port", "productrepository.go"), read("internal", "adapter", "repository", "postgres", "productrepository.go")
}

func requireFinderError(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", compiler.ErrCodeRepoFinderInvalid)
	}
	var ce *compiler.ContractError
	if !errors.As(err, &ce) || ce.Code != compiler.ErrCodeRepoFinderInvalid {
		t.Fatalf("expected ContractError %s, got %v", compiler.ErrCodeRepoFinderInvalid, err)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}
