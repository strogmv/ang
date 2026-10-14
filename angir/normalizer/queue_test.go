package normalizer

import (
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const queueEntityCUE = `
import "time"

ImportJob: {
	@queue(handler="RunImport", key="companyId", attempts=4, backoff="10s", stale="2m", heartbeat="20s", concurrency=3, unique="companyId,sourceHash", when="cfg.JobsEnabled")
	id:         string @db(type="UUID", primary_key=true)
	companyId:  string @db(type="UUID")
	sourceHash: string
	createdAt:  time.Time
}
`

func queueTestEntities(t *testing.T, src string) ([]Entity, error) {
	t.Helper()
	val := cuecontext.New().CompileString(src, cue.Filename("cue/domain/jobs.cue"))
	if val.Err() != nil {
		t.Fatal(val.Err())
	}
	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		return nil, err
	}
	return entities, n.AddQueueFields(entities)
}

// Item 26 of docs/tech-debt-backlog.md: @queue on an entity adds the queue
// fields and the indexes the claim, the stale scan, the per-key limit and the
// unique key need.
func TestAddQueueFields(t *testing.T) {
	entities, err := queueTestEntities(t, queueEntityCUE)
	if err != nil {
		t.Fatal(err)
	}
	e := entities[0]
	q := e.Queue
	if q == nil {
		t.Fatal("no queue")
	}
	if q.Handler != "RunImport" || q.Key != "companyId" || q.PerKey != 1 || q.Attempts != 4 || q.Backoff != "10s" || q.MaxBackoff != QueueDefaultMaxBackoff ||
		q.Heartbeat != "20s" || q.Stale != "2m" || q.Concurrency != 3 || q.Poll != QueueDefaultPoll || q.When != "cfg.JobsEnabled" ||
		!reflect.DeepEqual(q.Unique, []string{"companyId", "sourceHash"}) || !strings.HasPrefix(q.Source, "cue/domain/jobs.cue:") {
		t.Fatalf("queue = %+v", *q)
	}
	types := map[string]string{}
	optional := map[string]bool{}
	for _, f := range e.Fields {
		types[f.Name] = f.Type
		optional[f.Name] = f.IsOptional
	}
	for name, typ := range map[string]string{"status": "string", "priority": "int", "attempts": "int", "runAt": "time.Time", "lockedBy": "string", "heartbeatAt": "time.Time", "lastError": "string", "finishedAt": "time.Time"} {
		if types[name] != typ {
			t.Errorf("field %s = %q, want %q", name, types[name], typ)
		}
	}
	if !optional["heartbeatAt"] || !optional["lockedBy"] || optional["runAt"] {
		t.Errorf("optional fields: %v", optional)
	}
	var names []string
	for _, idx := range e.Indexes {
		names = append(names, idx.Name)
	}
	want := []string{"idx_importjobs_queue_ready", "idx_importjobs_queue_running", "idx_importjobs_queue_key", "uq_importjobs_queue_active"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("indexes = %v", names)
	}
	if k := e.Indexes[2]; !k.Unique || k.Where != "status = 'running'" || !reflect.DeepEqual(k.Fields, []string{"companyId"}) {
		t.Errorf("key index = %+v", k)
	}
	if u := e.Indexes[3]; !u.Unique || u.Where != "status IN ('queued', 'running')" {
		t.Errorf("unique index = %+v", u)
	}
}

func TestAddQueueFieldsRejects(t *testing.T) {
	cases := map[string]struct{ attr, fields, want string }{
		"no handler":    {`@queue(key="companyId")`, "", "handler= (the operation that runs one job) is required"},
		"unknown key":   {`@queue(handler="X", retries=3)`, "", `unknown key "retries"`},
		"bad duration":  {`@queue(handler="X", backoff="soon")`, "", `backoff="soon" is not a positive duration`},
		"stale short":   {`@queue(handler="X", heartbeat="30s", stale="45s")`, "", "stale=45s must be at least twice heartbeat=30s"},
		"key missing":   {`@queue(handler="X", key="tenant")`, "", `key="tenant" is not a field`},
		"perKey alone":  {`@queue(handler="X", perKey=2)`, "", "perKey needs key="},
		"bad number":    {`@queue(handler="X", attempts=0)`, "", `attempts="0" is not a positive integer`},
		"status type":   {`@queue(handler="X")`, "status: int", "field status is int; the queue needs it as string"},
		"status enum":   {`@queue(handler="X")`, `status: "new" | "queued" | "running" | "done"`, `field status must allow "dead"`},
		"bad when":      {`@queue(handler="X", when="cfg.(")`, "", "is not a Go expression"},
		"unique absent": {`@queue(handler="X", unique="companyId,hash")`, "", `unique field "hash" is not a field`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := "Job: {\n\t" + tc.attr + "\n\tid: string\n\tcompanyId: string\n\t" + tc.fields + "\n}\n"
			_, err := queueTestEntities(t, src)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "cue/domain/jobs.cue:") {
				t.Fatalf("err = %v, want %q with the position", err, tc.want)
			}
		})
	}
}

func TestResolveQueueHandlers(t *testing.T) {
	entities, err := queueTestEntities(t, queueEntityCUE)
	if err != nil {
		t.Fatal(err)
	}
	services := []Service{{Name: "Catalog", Methods: []Method{{Name: "RunImport", Input: Entity{Fields: []Field{
		{Name: "jobId", Type: "string"}, {Name: "companyId", Type: "string"}, {Name: "attempts", Type: "int"},
	}}}}}}
	repos, err := ResolveQueueHandlers(entities, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	q := entities[0].Queue
	if q.Service != "Catalog" || !reflect.DeepEqual(q.Inputs, []QueueInput{{"jobId", "id"}, {"companyId", "companyId"}, {"attempts", "attempts"}}) {
		t.Fatalf("queue = %+v", *q)
	}
	if len(repos) != 1 || repos[0].Entity != "ImportJob" {
		t.Fatalf("repos = %+v", repos)
	}

	for name, tc := range map[string]struct {
		input []Field
		want  string
	}{
		"unknown input": {[]Field{{Name: "file", Type: "string"}}, `input "file" is no field of the job`},
		"wrong type":    {[]Field{{Name: "attempts", Type: "string"}}, `input "attempts" is string, the job field attempts is int`},
	} {
		t.Run(name, func(t *testing.T) {
			entities, _ := queueTestEntities(t, queueEntityCUE)
			services := []Service{{Name: "Catalog", Methods: []Method{{Name: "RunImport", Input: Entity{Fields: tc.input}}}}}
			if _, err := ResolveQueueHandlers(entities, services, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	entities, _ = queueTestEntities(t, queueEntityCUE)
	if _, err := ResolveQueueHandlers(entities, nil, nil); err == nil || !strings.Contains(err.Error(), "handler RunImport is not an operation") {
		t.Fatalf("err = %v", err)
	}
}
