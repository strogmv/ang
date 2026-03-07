package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractEntities_ReadModelContract(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#SupplierRatingView: {
			owner: "tender"
			bounded_context: "tender"
			read_model: {
				source_context: "company"
				refreshOn: ["CompanyReviewCreated", "CompanyCategoryScoreUpdated"]
			}
			companyId: string
			score:     number
		}
	`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	rm := entities[0].ReadModel
	if rm == nil {
		t.Fatalf("expected read model metadata")
	}
	if rm.SourceContext != "company" {
		t.Fatalf("expected source_context=company, got %q", rm.SourceContext)
	}
	if len(rm.RefreshOn) != 2 {
		t.Fatalf("expected 2 refresh events, got %d", len(rm.RefreshOn))
	}

	for _, f := range entities[0].Fields {
		if f.Name == "read_model" || f.Name == "readModel" {
			t.Fatalf("read_model must not be emitted as regular entity field")
		}
	}
}
