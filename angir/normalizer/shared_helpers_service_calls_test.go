package normalizer

import (
	"strings"
	"testing"
)

// A method helper may call an operation of the service that reaches it and
// the services it uses; what a copy of another service cannot compile is an
// error naming the fix (backlog 46).
func TestResolveHelpersChecksWhatAMethodHelperCallsOnS(t *testing.T) {
	helper := func(name, code string) Helper {
		return Helper{Name: name, Kind: HelperMethod, Code: code, Source: "cue/api/helpers.cue:1"}
	}
	helpers := []Helper{
		helper("submitFor", `func (s *Service) submitFor(ctx context.Context, req port.SubmitQuoteRequest) error {
	_, err := s.SubmitQuote(ctx, req)
	return err
}`),
		helper("reserve", `func (s *Service) reserve(ctx context.Context) error {
	_, err := s.StockService.Reserve(ctx, port.ReserveRequest{})
	return err
}`),
	}
	op := func(name, code string) Method {
		return Method{Name: name, Impl: &MethodImpl{Code: code}}
	}
	services := []Service{
		{Name: "Quote", Uses: []string{"Stock"}, Methods: []Method{
			op("SubmitQuote", "return resp, nil"),
			op("ResubmitQuote", "return resp, s.submitFor(ctx, port.SubmitQuoteRequest{})"),
			op("ReserveQuote", "return resp, s.reserve(ctx)"),
		}},
		{Name: "Order", Methods: []Method{
			op("ConvertQuote", "return resp, s.submitFor(ctx, port.SubmitQuoteRequest{})"),
			op("ReserveOrder", "return resp, s.reserve(ctx)"),
		}},
		{Name: "Stock", Methods: []Method{op("Reserve", "return resp, nil")}},
	}
	_, err := ResolveHelpers(helpers, services)
	if err == nil {
		t.Fatal("expected the Order copies of both helpers to be refused")
	}
	msg := err.Error()
	for _, want := range []string{
		"helper submitFor calls s.SubmitQuote(…), an operation of Quote, not of Order (Order.ConvertQuote reaches it): call s.QuoteService.SubmitQuote(…)",
		"helper reserve uses s.StockService, but service Order does not use Stock (Order.ReserveOrder reaches it)",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not say %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "not of Quote") || strings.Contains(msg, "service Quote does not use") {
		t.Errorf("the Quote copies compile and must pass:\n%s", msg)
	}

	// Only the Quote service reaches them: fine.
	if _, err := ResolveHelpers(helpers, services[:1]); err != nil {
		t.Fatalf("Quote's own operations and uses are allowed: %v", err)
	}
}
