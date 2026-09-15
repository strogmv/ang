package flowsem

import "testing"

func TestIsSafeMappingAssignValueMatchesNormalizerRules(t *testing.T) {
	for value, want := range map[string]bool{
		"req.UserID":                         true,
		`"active"`:                           true,
		"_webhookBuyer || _webhookSeller":    true,
		"!offer.Archived && offer.Total > 0": true,
		"time.Time{}":                        true,
		"port.ReleaseTenderStockRequest{CompanyID: award.WinnerCompanyID, AwardID: award.ID}": true,
		"uuid.NewString()":             true,
		"strings.TrimSpace(req.Name)":  false,
		"port.X{ID: uuid.NewString()}": false,
		"func() int { return 1 }()":    false,
		"a + b":                        false,
	} {
		if got := isSafeMappingAssignValue(value); got != want {
			t.Errorf("isSafeMappingAssignValue(%q) = %v, want %v", value, got, want)
		}
	}
}
