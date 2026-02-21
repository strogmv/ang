package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestParseImplSteps_Basic(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
[
  {load: "TenderRepository", by: {id: "req.ID"}, into: "t"},
  {assert: "t.Status == \"active\"", error: "ErrTenderInactive"},
  {call: "BidRepository.Create", with: {bid: "req.Bid"}, into: "resp"},
  {emit: "BidPlaced", payload: {id: "resp.ID"}}
]`)
	if val.Err() != nil {
		t.Fatalf("compile: %v", val.Err())
	}
	n := New()
	n.RepoNames = map[string]struct{}{"TenderRepository": {}, "BidRepository": {}}
	n.EventNames = map[string]struct{}{"BidPlaced": {}}

	steps, err := n.parseImplSteps(val)
	if err != nil {
		t.Fatalf("parseImplSteps: %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}
	if steps[0].Kind != "load" || steps[0].LoadInto != "t" {
		t.Fatalf("unexpected load step: %+v", steps[0])
	}
	if steps[1].Kind != "assert" || steps[1].AssertError != "ErrTenderInactive" {
		t.Fatalf("unexpected assert step: %+v", steps[1])
	}
	if steps[2].Kind != "call" || steps[2].CallTarget != "BidRepository.Create" {
		t.Fatalf("unexpected call step: %+v", steps[2])
	}
	if steps[3].Kind != "emit" || steps[3].EmitEvent != "BidPlaced" {
		t.Fatalf("unexpected emit step: %+v", steps[3])
	}
}

func TestParseImplSteps_Warnings(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`[{load: "MissingRepo", into: "x"}, {emit: "UnknownEvent"}]`)
	if val.Err() != nil {
		t.Fatalf("compile: %v", val.Err())
	}
	var warns []Warning
	n := New()
	n.WarningSink = func(w Warning) { warns = append(warns, w) }
	n.RepoNames = map[string]struct{}{"OtherRepo": {}}
	n.EventNames = map[string]struct{}{}

	_, _ = n.parseImplSteps(val)

	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %+v", len(warns), warns)
	}
}

// FuzzParseImplSteps_NoPanic ensures arbitrary impl_steps input does not panic.
func FuzzParseImplSteps_NoPanic(f *testing.F) {
	ctx := cuecontext.New()
	seeds := []string{
		`[]`,
		`[{load:"Repo",into:"x"}]`,
		`[{assert:"true",error:"Err"}]`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		val := ctx.CompileString(src)
		n := New()
		n.parseImplSteps(val) // ignore error; just ensure no panic
	})
}
