package compiler_test

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func TestPipelineReadsHeaderInputs(t *testing.T) {
	_, services, err := runPipelineFiles(t, map[string]string{
		"cue/domain/bot.cue": "package domain\n\nBot: {\n\tid: string\n}\n",
		"cue/api/bots.cue": `package api

TelegramWebhook: {
	service: "bots"
	input: {
		secretToken: string @header(name="X-Telegram-Bot-Api-Secret-Token")
		apiKey?:     string @header("X-Api-Key")
		updateId?:   int
	}
	output: {ok: bool}
}
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range methodByName(services, "TelegramWebhook").Input.Fields {
		got[f.Name] = normalizer.FieldHeaderName(f.Metadata)
	}
	if got["secretToken"] != "X-Telegram-Bot-Api-Secret-Token" || got["apiKey"] != "X-Api-Key" || got["updateId"] != "" {
		t.Fatalf("header names = %v", got)
	}
}

func TestPipelineRefusesBadHeaderInputs(t *testing.T) {
	for name, field := range map[string]string{
		"no name":    `secretToken: string @header()`,
		"bad name":   `secretToken: string @header(name="X Secret")`,
		"not string": `count: int @header(name="X-Count")`,
		"with query": `secretToken: string @query() @header(name="X-Secret")`,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := runPipelineFiles(t, map[string]string{
				"cue/domain/bot.cue": "package domain\n\nBot: {\n\tid: string\n}\n",
				"cue/api/bots.cue":   "package api\n\nTelegramWebhook: {\n\tservice: \"bots\"\n\tinput: {" + field + "}\n\toutput: {ok: bool}\n}\n",
			})
			if err == nil || !strings.Contains(err.Error(), "@header") {
				t.Fatalf("expected a @header error, got %v", err)
			}
		})
	}
}
