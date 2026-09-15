package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractEndpoints_ReadsFrontendMetadata(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	v := ctx.CompileString(`package api

ListPresenceFeed: {
  service: "realtime"
}

HTTP: {
  ListPresenceFeed: {
    method: "GET"
    path: "/api/presence/feed"
    meta: {
      frontend: {
        queryProfile: "realtime"
        cachePolicy:  "realtime"
      }
    }
  }
}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	eps, err := n.ExtractEndpoints(v)
	if err != nil {
		t.Fatalf("extract endpoints: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
	frontend, ok := eps[0].Metadata["frontend"].(map[string]any)
	if !ok {
		t.Fatalf("expected frontend metadata map, got %#v", eps[0].Metadata)
	}
	if got := frontend["queryProfile"]; got != "realtime" {
		t.Fatalf("expected queryProfile realtime, got %#v", got)
	}
	if got := frontend["cachePolicy"]; got != "realtime" {
		t.Fatalf("expected cachePolicy realtime, got %#v", got)
	}
}

func TestExtractEndpoints_RequestBody(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	v := ctx.CompileString(`package api

AttachmentUploadMultipart: { service: "attachments" }

HTTP: {
	AttachmentUploadMultipart: {
		method: "POST"
		path: "/api/attachments/upload"
		request_body: "multipart_form"
	}
}`)
	if err := v.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	eps, err := n.ExtractEndpoints(v)
	if err != nil {
		t.Fatalf("extract endpoints: %v", err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
	if got, _ := eps[0].Metadata["request_body"].(string); got != "multipart_form" {
		t.Fatalf("expected request_body multipart_form, got %#v", eps[0].Metadata)
	}
}
