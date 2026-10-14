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

// Item 26: a queue entity in cue/domain and its handler in cue/api come out
// of the normalize phase with the queue fields, the indexes, the handler's
// input mapped to job fields, and a repository.
func TestSemanticPhasesQueueEntity(t *testing.T) {
	base := copyMinimalProject(t)
	writeFixtureFile(t, filepath.Join(base, "cue", "domain", "jobs.cue"), `package domain

ExportJob: {
	@queue(handler="RunExport", key="userId", unique="userId,format")
	id:     string
	userId: string
	format: string
}
`)
	writeFixtureFile(t, filepath.Join(base, "cue", "api", "jobs.cue"), `package api

RunExport: {
	service: "user"
	input: {jobId: string, format: string}
	output: {ok: bool}
}
`)
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(normalizer.Warning) {}})
	if err != nil {
		t.Fatal(err)
	}
	var job *normalizer.Entity
	for i := range out.Entities {
		if out.Entities[i].Name == "ExportJob" {
			job = &out.Entities[i]
		}
	}
	if job == nil || job.Queue == nil {
		t.Fatalf("no queue entity in %v", out.Entities)
	}
	if job.Queue.Service != "User" || len(job.Queue.Inputs) != 2 || job.Queue.Inputs[0].Field != "id" {
		t.Fatalf("queue = %+v", *job.Queue)
	}
	fields := map[string]bool{}
	for _, f := range job.Fields {
		fields[f.Name] = true
	}
	if !fields["status"] || !fields["heartbeatAt"] || len(job.Indexes) != 4 {
		t.Fatalf("fields %v indexes %v", fields, job.Indexes)
	}
	hasRepo := false
	for _, r := range out.Repos {
		hasRepo = hasRepo || r.Entity == "ExportJob"
	}
	if !hasRepo {
		t.Fatal("the queue entity has no repository")
	}
}
