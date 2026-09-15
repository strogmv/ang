package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoHoverShowsRepositorySignatureAndTypes(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/port/auditlogrepository.go", "package port\n\nimport (\n\t\"context\"\n\t\"time\"\n)\n\ntype AuditLogRepository interface {\n\tDeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)\n}\n")
	write("internal/port/list_orders.go", "package port\n\ntype ListOrdersResponseData struct {\n\tID string\n}\n")
	write("internal/domain/user.go", "package domain\n\ntype User struct {\n\tID    string\n\tEmail string\n}\n")

	line := `			n, err := s.AuditLogRepo.DeleteOlderThan(ctx, cutoff)`
	value, start, end, ok := goHoverAt(root, line, strings.Index(line, "DeleteOlderThan")+3)
	if !ok || !strings.Contains(value, "DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)") {
		t.Fatalf("repository hover = %q, ok=%v", value, ok)
	}
	if line[start:end] != "s.AuditLogRepo.DeleteOlderThan" {
		t.Fatalf("range covers %q", line[start:end])
	}

	line = `	items := []port.ListOrdersResponseData{}; var u *domain.User`
	if value, _, _, ok := goHoverAt(root, line, strings.Index(line, "ListOrders")); !ok || !strings.Contains(value, "type ListOrdersResponseData struct") {
		t.Fatalf("port type hover = %q", value)
	}
	if value, _, _, ok := goHoverAt(root, line, strings.Index(line, "domain.User")+8); !ok || !strings.Contains(value, "Email string") {
		t.Fatalf("domain type hover = %q", value)
	}
	if _, _, _, ok := goHoverAt(root, `	x := s.MissingRepo.Nope(ctx)`, 10); ok {
		t.Fatal("unknown repository must not hover")
	}

	// A regenerated port file is picked up.
	write("internal/port/auditlogrepository.go", "package port\n\nimport \"context\"\n\ntype AuditLogRepository interface {\n\tDeleteOlderThan(ctx context.Context) (int64, error)\n\tPurge(ctx context.Context) error\n}\n")
	future := filepathTimeBump(t, filepath.Join(root, "internal", "port", "auditlogrepository.go"))
	_ = future
	line = `	err := s.AuditLogRepo.Purge(ctx)`
	if value, _, _, ok := goHoverAt(root, line, strings.Index(line, "Purge")); !ok || !strings.Contains(value, "Purge(ctx context.Context) error") {
		t.Fatalf("hover after regeneration = %q", value)
	}
}

func filepathTimeBump(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(2e9)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	return true
}
