package emitter

import "testing"

// cleanImplCode rewrites a logger named l to slog. It used to replace every
// "l." in the line, which turned html.EscapeString into htmslog.EscapeString
// and url.Parse into urslog.Parse.
func TestCleanImplCodeRewritesOnlyTheLoggerIdentifier(t *testing.T) {
	code := "escaped := html.EscapeString(raw)\n" +
		"parsed, err := url.Parse(link)\n" +
		"name := model.Name + channel.Title\n" +
		"l.Info(\"done\", \"id\", req.UserId)"
	want := "escaped := html.EscapeString(raw)\n" +
		"parsed, err := url.Parse(link)\n" +
		"name := model.Name + channel.Title\n" +
		"slog.Info(\"done\", \"id\", req.UserID)"
	if got := cleanImplCode(code, ""); got != want {
		t.Fatalf("cleanImplCode:\n%s\nwant:\n%s", got, want)
	}
}
