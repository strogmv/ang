package rawgo

import (
	"reflect"
	"testing"
)

func TestUndeclaredUsesFindsTheMethodsErr(t *testing.T) {
	literal := `(func(ctx context.Context) (int, error) {
	var rows []int
	rows, err = load(ctx)
	if err != nil {
		return 0, err
	}
	x := struct{ err error }{err: nil}
	_ = x.err
	return len(rows), nil
})`
	uses, ok := UndeclaredUses(literal, "err", "resp")
	if !ok {
		t.Fatal("the literal parses")
	}
	if want := []NameUse{{Name: "err", Line: 3}}; !reflect.DeepEqual(uses, want) {
		t.Fatalf("uses = %+v, want %+v", uses, want)
	}

	own := `(func(ctx context.Context) (n int, err error) {
	if _, err := load(ctx); err != nil {
		return 0, err
	}
	var resp = struct{ err string }{err: "x"}
	_ = resp.err
	return 1, err
})`
	if uses, _ := UndeclaredUses(own, "err", "resp"); len(uses) != 0 {
		t.Fatalf("a literal that declares err and resp itself: %+v", uses)
	}
}

func TestShadowedResultsFindsNakedReturnsAndDeferredReads(t *testing.T) {
	code := `if req.ID == "" {
	return
}
err := check(req.ID)
defer func() {
	if err != nil {
		log(err)
	}
}()
if err != nil {
	return
}
go func() {
	return
}()
return resp, err`
	uses, ok := ShadowedResults(code)
	if !ok {
		t.Fatal("the code parses")
	}
	want := []NameUse{
		{Name: "err", Line: 6, DeclLine: 4, Deferred: true},
		{Name: "err", Line: 11, DeclLine: 4},
	}
	if !reflect.DeepEqual(uses, want) {
		t.Fatalf("uses = %+v, want %+v", uses, want)
	}

	// Declarations in another scope, and code that never shadows, are fine.
	fine := `if ok {
	resp := 1
	_ = resp
}
defer func() {
	if err != nil {
		err = wrap(err)
	}
}()
token, err2 := acquire()
_ = token
_ = err2
return`
	if uses, _ := ShadowedResults(fine); len(uses) != 0 {
		t.Fatalf("nothing shadows the results here: %+v", uses)
	}
}
