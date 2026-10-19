package normalizer

import (
	"strings"
	"testing"
)

func TestConditionsMayCallFunctionsWithoutEffects(t *testing.T) {
	allowed := []string{
		`isValidIBAN(req.Iban)`,
		`!isValidIBAN(req.Iban)`,
		`len(req.Items) > 0 && len(req.Items) <= 100`,
		`strings.HasPrefix(req.Code, "DE")`,
		`!req.StartsAt.IsZero() && req.StartsAt.Before(time.Now().UTC())`,
		`s.CanSubmitQuote(ctx, req.CompanyID)`,
		`s.CompanyService.IsMember(ctx, req.CompanyID, req.UserID)`,
		`utf8.RuneCountInString(strings.TrimSpace(req.Name)) <= 80`,
		`req.Total*100 >= float64(minCents)`,
	}
	for _, condition := range allowed {
		if msg := validateSafeConditionExpr(condition); msg != "" {
			t.Errorf("%s: %s", condition, msg)
		}
	}
	refused := []string{
		`s.QuoteRepo.Delete(ctx, req.ID) == nil`,
		`s.UserRepo.FindByID(ctx, req.ID) != nil`,
		`s.Tx.WithTx(ctx, nil) == nil`,
		`s.txManager.WithTx(ctx, nil) == nil`,
		`func() bool { return true }()`,
		`append(items, x) != nil`,
		`check(func() bool { return true })`,
		`x := 1`,
	}
	for _, condition := range refused {
		if msg := validateSafeConditionExpr(condition); msg == "" {
			t.Errorf("%s must be refused", condition)
		}
	}
}

func TestFlowExprRootsSkipsFunctionsAndPackageCalls(t *testing.T) {
	cases := map[string][]string{
		`isValidIBAN(req.Iban)`: {"req"},
		`utf8.RuneCountInString(strings.TrimSpace(req.Name)) <= limit`: {"limit", "req"},
		`quote.Total > 0 && quote.Valid()`:                             {"quote"},
	}
	for expr, want := range cases {
		if got := flowExprRoots(expr); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: roots %v, want %v", expr, got, want)
		}
	}
}
