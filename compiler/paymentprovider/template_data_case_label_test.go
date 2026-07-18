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
