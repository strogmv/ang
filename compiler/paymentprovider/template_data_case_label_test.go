package paymentprovider

import (
	"strings"
	"testing"
)

func TestWrapSwitchCaseLabel_singleName(t *testing.T) {
	got := wrapSwitchCaseLabel([]string{"errCode9100"}, switchCaseWrapWidth)
	if got != "errCode9100" {
		t.Fatalf("got %q", got)
	}
}

func TestWrapSwitchCaseLabel_wrapsLongList(t *testing.T) {
	names := []string{
		"errCode9100", "errCode9101", "errCode9102", "errCode9103", "errCode9104",
		"errCode9105", "errCode9106", "errCode9107", "errCode9108", "errCode9109",
		"errCode9110", "errCode9111", "errCode9112", "errCode9113", "errCode9114",
		"errCode9300", "errCode9301", "errCode9700", "errCode9701", "errCode9702",
	}
	got := wrapSwitchCaseLabel(names, switchCaseWrapWidth)
	if !strings.Contains(got, ",\n\t\t") {
		t.Fatalf("expected wrapped case label, got %q", got)
	}
	if len(got) > switchCaseWrapWidth {
		firstLine := strings.SplitN(got, "\n", 2)[0]
		if len("\tcase "+firstLine) > switchCaseWrapWidth+1 {
			t.Fatalf("first line too long: %q", firstLine)
		}
	}
}

func TestGroupStatusTemplatesByOutcome_mergesSameMappedResult(t *testing.T) {
	items := []StatusTemplate{
		{ConstName: "providerStatusCreated", Code: "created", StatusTitle: "Pending", StatusCode: "SCodeOk", Message: "new"},
		{ConstName: "providerStatusPending", Code: "pending", StatusTitle: "Pending", StatusCode: "SCodeOk", Message: "wait"},
		{ConstName: "providerStatusProcessing", Code: "processing", StatusTitle: "Pending", StatusCode: "SCodeOk"},
		{ConstName: "providerStatusCompleted", Code: "completed", StatusTitle: "Success", StatusCode: "SCodeOk"},
		{ConstName: "providerStatusDeclined", Code: "declined", StatusTitle: "Declined", StatusCode: "SCodeDeclinedByBank"},
	}
	got := groupStatusTemplatesByOutcome(items)
	if len(got) != 3 {
		t.Fatalf("outcomes: got %d groups, want 3", len(got))
	}
	if got[0].CaseLabel() != "providerStatusCreated, providerStatusPending, providerStatusProcessing" {
		t.Fatalf("pending case: %q", got[0].CaseLabel())
	}
	withMsg := groupStatusTemplates(items)
	if len(withMsg) != 5 {
		t.Fatalf("message-sensitive groups: got %d, want 5", len(withMsg))
	}
}

func TestShareStatusMapper(t *testing.T) {
	same := []StatusTemplate{
		{Code: "created", StatusTitle: "Pending", StatusCode: "SCodeOk", ConstName: "providerStatusCreated"},
		{Code: "completed", StatusTitle: "Success", StatusCode: "SCodeOk", ConstName: "providerStatusCompleted"},
	}
	d := &TemplateData{PayinStatuses: same, PayoutStatuses: same}
	if !d.ShareStatusMapper() {
		t.Fatal("expected shared mapper for identical maps")
	}
	different := append([]StatusTemplate(nil), same...)
	different[0].StatusTitle = "Declined"
	d.PayoutStatuses = different
	if d.ShareStatusMapper() {
		t.Fatal("did not expect shared mapper for different outcomes")
	}
}

func TestApplySharedStatuses(t *testing.T) {
	shared := []StatusMapping{{Code: "created", Status: "pending", StatusCode: "SCodeOk"}}
	spec := &ProviderSpec{Statuses: shared}
	applySharedStatuses(spec)
	if len(spec.PayinStatuses) != 1 || len(spec.PayoutStatuses) != 1 {
		t.Fatalf("payin=%d payout=%d", len(spec.PayinStatuses), len(spec.PayoutStatuses))
	}
	spec.PayoutStatuses = nil
	spec.PayinStatuses = []StatusMapping{{Code: "other", Status: "success", StatusCode: "SCodeOk"}}
	applySharedStatuses(spec)
	if spec.PayinStatuses[0].Code != "other" {
		t.Fatal("explicit payin map should win")
	}
	if len(spec.PayoutStatuses) != 1 || spec.PayoutStatuses[0].Code != "created" {
		t.Fatal("empty payout should inherit statuses")
	}
}

func TestPendingRawStatusCaseLabel(t *testing.T) {
	d := &TemplateData{
		PayinStatuses: []StatusTemplate{
			{ConstName: "providerStatusCreated", StatusTitle: "Pending"},
			{ConstName: "providerStatusPending", StatusTitle: "Pending"},
			{ConstName: "providerStatusCompleted", StatusTitle: "Success"},
		},
		PayoutStatuses: []StatusTemplate{
			{ConstName: "providerStatusCreated", StatusTitle: "Pending"},
			{ConstName: "providerStatusProcessing", StatusTitle: "Pending"},
		},
	}
	got := d.PendingRawStatusCaseLabel()
	want := "providerStatusCreated, providerStatusPending, providerStatusProcessing"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
