package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
)

func TestEmitGDPR_GeneratesOwnerAwareListAllHelpers(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, filepath.Join(tmp, "sdk"), "templates")
	em.Version = "0.1.139"

	entities := []ir.Entity{
		{
			Name:  "Notification",
			Owner: "notifications",
			GDPRPolicy: &ir.GDPRPolicy{
				Erasable:   true,
				Exportable: true,
				Retention:  "90d",
				OwnerField: "userId",
			},
			Fields: []ir.Field{
				{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "body", Type: ir.TypeRef{Kind: ir.KindString}, IsPII: true},
				{Name: "createdAt", Type: ir.TypeRef{Kind: ir.KindTime}},
			},
		},
	}

	if err := em.EmitGDPR(entities); err != nil {
		t.Fatalf("EmitGDPR failed: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(tmp, "internal", "service", "gdpr.gen.go"))
	if err != nil {
		t.Fatalf("read gdpr.gen.go: %v", err)
	}
	text := string(out)

	for _, want := range []string{
		"func (s *NotificationsImpl) EraseNotificationPersonalData",
		"items, err := s.NotificationRepo.ListAll(ctx, offset, pageSize)",
		"if item.UserID != ownerID {",
		"item.Body = \"\"",
		"func (s *NotificationsImpl) ExportNotificationPersonalData",
		"func (s *NotificationsImpl) PurgeNotificationExpired",
		"retention, err := parseGDPRRetention(\"90d\")",
		"if err := s.NotificationRepo.Delete(ctx, id); err != nil {",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated gdpr helper to contain %q, got:\n%s", want, text)
		}
	}

	for _, forbidden := range []string{
		"NotificationServiceImpl",
		"GetByUser",
		"ListByUser",
		"DeleteCreatedBefore",
		"time.ParseDuration(\"90d\")",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("expected generated gdpr helper to avoid %q, got:\n%s", forbidden, text)
		}
	}
}

func TestEmitGDPR_RemovesStaleFileWhenPoliciesDisappear(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, filepath.Join(tmp, "sdk"), "templates")
	em.Version = "0.1.139"

	withPolicy := []ir.Entity{
		{
			Name:  "User",
			Owner: "user",
			GDPRPolicy: &ir.GDPRPolicy{
				Erasable: true,
			},
			Fields: []ir.Field{
				{Name: "ID", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "userId", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}, IsPII: true},
			},
		},
	}
	if err := em.EmitGDPR(withPolicy); err != nil {
		t.Fatalf("EmitGDPR with policy failed: %v", err)
	}

	outPath := filepath.Join(tmp, "internal", "service", "gdpr.gen.go")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected gdpr.gen.go to exist: %v", err)
	}

	if err := em.EmitGDPR(nil); err != nil {
		t.Fatalf("EmitGDPR without policy failed: %v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("expected gdpr.gen.go to be removed when policies disappear, got err=%v", err)
	}
}
