package emitter

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestDeterministicCoreAcrossTenRuns(t *testing.T) {
	schema := &ir.Schema{
		Project: ir.Project{Name: "deterministic-core"},
		Entities: []ir.Entity{
			{Name: "Zoo", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
			{Name: "Alpha", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
		},
		Services: []ir.Service{
			{
				Name: "Company",
				Methods: []ir.Method{
					{
						Name:  "ListCompanies",
						Input: &ir.Entity{Name: "ListCompaniesRequest"},
						Output: &ir.Entity{
							Name: "ListCompaniesResponse",
							Fields: []ir.Field{
								{Name: "data", Type: ir.TypeRef{Kind: ir.KindList, ItemType: &ir.TypeRef{Kind: ir.KindEntity, Name: "Alpha"}}},
							},
						},
					},
				},
			},
			{
				Name: "Auth",
				Methods: []ir.Method{
					{
						Name: "Login",
						Input: &ir.Entity{
							Name: "LoginRequest",
							Fields: []ir.Field{
								{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}},
							},
						},
						Output: &ir.Entity{
							Name: "LoginResponse",
							Fields: []ir.Field{
								{Name: "token", Type: ir.TypeRef{Kind: ir.KindString}},
							},
						},
					},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/api/companies", Service: "Company", RPC: "ListCompanies"},
			{Method: "POST", Path: "/api/auth/login", Service: "Auth", RPC: "Login"},
		},
	}

	var baseline string
	for i := 0; i < 10; i++ {
		root := filepath.Join(t.TempDir(), "run")
		em := New(root, filepath.Join(root, "sdk"), "templates")
		if err := em.EmitManifest(schema); err != nil {
			t.Fatalf("run %d: EmitManifest failed: %v", i+1, err)
		}
		if err := em.EmitServiceFromIR(schema); err != nil {
			t.Fatalf("run %d: EmitServiceFromIR failed: %v", i+1, err)
		}
		if err := em.EmitServiceImplFromIR(schema, nil); err != nil {
			t.Fatalf("run %d: EmitServiceImplFromIR failed: %v", i+1, err)
		}
		if err := em.EmitHTTPFromIR(schema, nil); err != nil {
			t.Fatalf("run %d: EmitHTTPFromIR failed: %v", i+1, err)
		}

		h, err := hashTree(root)
		if err != nil {
			t.Fatalf("run %d: hashTree failed: %v", i+1, err)
		}
		if i == 0 {
			baseline = h
			continue
		}
		if h != baseline {
			t.Fatalf("non-deterministic generation detected on run %d: baseline=%s current=%s", i+1, baseline, h)
		}
	}
}

func hashTree(root string) (string, error) {
	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(files)

	sum := sha256.New()
	for _, rel := range files {
		sum.Write([]byte(rel))
		sum.Write([]byte{0})
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		sum.Write(data)
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
