package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

const runtimeOpsCUE = `package api

SweepUsers: {
	service: "user"
	input: {}
	output: {ok: bool}
	subscribes: {UsersSweepRequested: "SweepUsers"}
	subscribe: {when: "cfg.JobsEnabled"}
}

WarmUsers: {
	service: "user"
	hook: "OnStart"
	input: {}
	output: {ok: bool}
}

FlushUsers: {
	service: "user"
	hook: {on: "OnShutdown", role: "worker", when: "cfg.JobsEnabled"}
	input: {}
	output: {ok: bool}
}
`

// Items 24 and 25: subscribe.when and hook: reach the method metadata the
// generated main reads.
func TestSemanticPhasesReadRuntimeKeysOfOperations(t *testing.T) {
	base := copyMinimalProject(t)
	writeFixtureFile(t, filepath.Join(base, "cue", "api", "runtime.cue"), runtimeOpsCUE)
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(normalizer.Warning) {}})
	if err != nil {
		t.Fatal(err)
	}
	meta := map[string]map[string]any{}
	for _, s := range out.Services {
		for _, m := range s.Methods {
			meta[m.Name] = m.Metadata
		}
	}
	if got := meta["SweepUsers"][normalizer.MetaSubscribeWhen]; got != "cfg.JobsEnabled" {
		t.Errorf("SweepUsers subscribe_when = %v", got)
	}
	if got := meta["WarmUsers"][normalizer.MetaHookOn]; got != "OnStart" {
		t.Errorf("WarmUsers hook_on = %v", got)
	}
	if got := meta["WarmUsers"][normalizer.MetaHookRole]; got != "all" {
		t.Errorf("WarmUsers hook_role = %v", got)
	}
	f := meta["FlushUsers"]
	if f[normalizer.MetaHookOn] != "OnShutdown" || f[normalizer.MetaHookRole] != "worker" || f[normalizer.MetaHookWhen] != "cfg.JobsEnabled" {
		t.Errorf("FlushUsers metadata = %v", f)
	}
}

func TestSemanticPhasesRejectBadRuntimeKeys(t *testing.T) {
	cases := map[string]struct{ op, want string }{
		"when not Go":    {`Op: {service: "user", input: {}, output: {}, subscribe: {when: "cfg.Jobs ==="}}`, "subscribe.when \"cfg.Jobs ===\": is not a Go expression"},
		"hook name":      {`Op: {service: "user", input: {}, output: {}, hook: "OnBoot"}`, `hook.on is "OnBoot"`},
		"hook role":      {`Op: {service: "user", input: {}, output: {}, hook: {on: "OnStart", role: "cron"}}`, `hook.role is "cron"`},
		"hook has input": {`Op: {service: "user", input: {id: string}, output: {}, hook: "OnStart"}`, "its input must be empty (has id)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base := copyMinimalProject(t)
			writeFixtureFile(t, filepath.Join(base, "cue", "api", "runtime.cue"), "package api\n\n"+tc.op+"\n")
			_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(normalizer.Warning) {}})
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "runtime.cue:3") {
				t.Fatalf("err = %v, want %q at runtime.cue:3", err, tc.want)
			}
		})
	}
}
