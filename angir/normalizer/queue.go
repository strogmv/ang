package normalizer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// QueueDef makes an entity a job queue: @queue(handler="RunImport", ...) on
// the entity. Its rows are jobs; the generated worker claims them with
// SELECT … FOR UPDATE SKIP LOCKED, keeps a heartbeat, retries with backoff and
// moves a job that keeps failing to status "dead" (the dead letter).
type QueueDef struct {
	// Handler is the operation that runs one job; its input fields are
	// filled from the job's fields of the same name (jobId is the job's id).
	Handler string
	// Service is the handler's service (resolved by ApplyQueues).
	Service string
	// Key is the field whose value limits concurrency: at most PerKey jobs
	// with the same key run at once (a company's imports one at a time).
	Key    string
	PerKey int
	// Attempts is how many times a job runs before it goes dead.
	Attempts int
	// Backoff is the delay before the first retry, doubled each attempt up
	// to MaxBackoff.
	Backoff    string
	MaxBackoff string
	// Heartbeat is how often a running job's row is touched; a running job
	// not touched for Stale is requeued (or dead after Attempts).
	Heartbeat string
	Stale     string
	// Concurrency is the number of jobs one process runs at once.
	Concurrency int
	// Poll is how long an idle worker waits before it looks again.
	Poll string
	// Unique fields get a partial unique index over the active jobs
	// (queued, running): enqueueing a duplicate is a no-op.
	Unique []string
	// When is a Go bool expression over cfg; the worker starts only when it
	// holds (and, with server roles, only in worker|all).
	When string
	// Inputs maps the handler's input fields to job fields (resolved by
	// ApplyQueues).
	Inputs []QueueInput
	Source string
}

// QueueInput is one handler input field and the job field it is filled from.
type QueueInput struct {
	Input string
	Field string
}

// Queue defaults.
const (
	QueueDefaultAttempts    = 5
	QueueDefaultBackoff     = "30s"
	QueueDefaultMaxBackoff  = "1h"
	QueueDefaultHeartbeat   = "30s"
	QueueDefaultStale       = "5m"
	QueueDefaultConcurrency = 1
	QueueDefaultPoll        = "2s"
)

// The statuses of a job row.
const (
	QueueStatusQueued  = "queued"
	QueueStatusRunning = "running"
	QueueStatusDone    = "done"
	QueueStatusDead    = "dead"
)

// queueSystemFieldsCUE are the fields every queue entity gets. Declared in CUE
// so they are read exactly like fields written by hand.
const queueSystemFieldsCUE = `
import "time"

status:       "queued" | "running" | "done" | "dead" | *"queued" @db(type="TEXT")
priority:     int | *0     @db(type="INT")
attempts:     int | *0     @db(type="INT")
runAt:        time.Time    @db(type="TIMESTAMPTZ")
lockedBy?:    string       @db(type="TEXT")
heartbeatAt?: time.Time    @db(type="TIMESTAMPTZ")
lastError:    string | *"" @db(type="TEXT")
finishedAt?:  time.Time    @db(type="TIMESTAMPTZ")
`

func parseEntityQueue(val cue.Value) (*QueueDef, error) {
	var found *cue.Attribute
	for _, attr := range val.Attributes(cue.ValueAttr | cue.FieldAttr | cue.DeclAttr) {
		if attr.Name() == "queue" {
			a := attr
			found = &a
			break
		}
	}
	if found == nil {
		return nil, nil
	}
	pos := formatPos(val)
	q := &QueueDef{Source: pos}
	known := map[string]bool{}
	str := func(key string) string {
		known[key] = true
		v, ok, _ := found.Lookup(0, key)
		if !ok {
			return ""
		}
		return strings.TrimSpace(strings.Trim(v, `"`))
	}
	var errs []string
	num := func(key string) int {
		s := str(key)
		if s == "" {
			return 0
		}
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			errs = append(errs, fmt.Sprintf("%s=%q is not a positive integer", key, s))
			return 0
		}
		return n
	}
	q.Handler = str("handler")
	q.Key = str("key")
	q.PerKey = num("perKey")
	q.Attempts = num("attempts")
	q.Backoff = str("backoff")
	q.MaxBackoff = str("maxBackoff")
	q.Heartbeat = str("heartbeat")
	q.Stale = str("stale")
	q.Concurrency = num("concurrency")
	q.Poll = str("poll")
	q.When = str("when")
	for _, f := range strings.Split(str("unique"), ",") {
		if f = strings.TrimSpace(f); f != "" {
			q.Unique = append(q.Unique, f)
		}
	}
	for i := 0; i < found.NumArgs(); i++ {
		key, _ := found.Arg(i)
		if key == "" {
			errs = append(errs, "every argument is key=value")
			continue
		}
		if !known[key] {
			errs = append(errs, fmt.Sprintf("unknown key %q (handler, key, perKey, attempts, backoff, maxBackoff, heartbeat, stale, concurrency, poll, unique, when)", key))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s: @queue: %s", pos, strings.Join(errs, "; "))
	}
	return q, nil
}

// AddQueueFields completes every queue entity right after the entities are
// read: it checks the @queue options and fills their defaults, adds the
// queue fields (status, priority, attempts, runAt, lockedBy, heartbeatAt,
// lastError, finishedAt) a flow can then use, and the indexes the claim, the
// stale scan, the per-key limit and the unique key need.
func (n *Normalizer) AddQueueFields(entities []Entity) error {
	var errs []string
	for i := range entities {
		e := &entities[i]
		q := e.Queue
		if q == nil {
			continue
		}
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Sprintf("%s: queue %s: %s", q.Source, e.Name, fmt.Sprintf(format, args...)))
		}
		if storage, _ := e.Metadata["storage"].(string); storage != "" && !strings.EqualFold(storage, "postgres") && !strings.EqualFold(storage, "sql") {
			fail("a queue lives in PostgreSQL; @storage(%q) is not supported", storage)
			continue
		}
		if dto, _ := e.Metadata["dto"].(bool); dto {
			fail("a DTO has no table")
			continue
		}
		fieldByName := map[string]Field{}
		for _, f := range e.Fields {
			fieldByName[strings.ToLower(f.Name)] = f
		}
		if _, ok := fieldByName["id"]; !ok {
			fail("needs an id field")
			continue
		}
		if q.Handler == "" {
			fail("handler= (the operation that runs one job) is required")
		}
		if q.Attempts == 0 {
			q.Attempts = QueueDefaultAttempts
		}
		if q.Concurrency == 0 {
			q.Concurrency = QueueDefaultConcurrency
		}
		for _, d := range []struct {
			name string
			val  *string
			def  string
		}{
			{"backoff", &q.Backoff, QueueDefaultBackoff},
			{"maxBackoff", &q.MaxBackoff, QueueDefaultMaxBackoff},
			{"heartbeat", &q.Heartbeat, QueueDefaultHeartbeat},
			{"stale", &q.Stale, QueueDefaultStale},
			{"poll", &q.Poll, QueueDefaultPoll},
		} {
			if *d.val == "" {
				*d.val = d.def
			}
			if v, err := time.ParseDuration(*d.val); err != nil || v <= 0 {
				fail("%s=%q is not a positive duration such as \"30s\"", d.name, *d.val)
			}
		}
		hb, _ := time.ParseDuration(q.Heartbeat)
		stale, _ := time.ParseDuration(q.Stale)
		if hb > 0 && stale > 0 && stale < 2*hb {
			fail("stale=%s must be at least twice heartbeat=%s, or a live job is taken from its worker", q.Stale, q.Heartbeat)
		}
		if q.Key != "" {
			if f, ok := fieldByName[strings.ToLower(q.Key)]; ok {
				q.Key = f.Name
			} else {
				fail("key=%q is not a field", q.Key)
			}
			if q.PerKey == 0 {
				q.PerKey = 1
			}
		} else if q.PerKey != 0 {
			fail("perKey needs key=")
		}
		for j, u := range q.Unique {
			if f, ok := fieldByName[strings.ToLower(u)]; ok {
				q.Unique[j] = f.Name
			} else {
				fail("unique field %q is not a field", u)
			}
		}
		if q.When != "" {
			if err := checkRuntimeCondition(q.When); err != nil {
				fail("when=%q %v", q.When, err)
			}
		}

		sys, err := n.queueSystemFields(q.Source)
		if err != nil {
			return err
		}
		for _, f := range sys {
			if have, ok := fieldByName[strings.ToLower(f.Name)]; ok {
				if have.Type != f.Type {
					fail("field %s is %s; the queue needs it as %s (or leave it out and the queue adds it)", have.Name, have.Type, f.Type)
				}
				if f.Name == "status" && have.Constraints != nil && len(have.Constraints.Enum) > 0 {
					for _, st := range []string{QueueStatusQueued, QueueStatusRunning, QueueStatusDone, QueueStatusDead} {
						if !containsString(have.Constraints.Enum, st) {
							fail("field status must allow %q (the queue's statuses are queued, running, done, dead)", st)
							break
						}
					}
				}
				continue
			}
			e.Fields = append(e.Fields, f)
		}

		// Indexes, in the entity's index syntax (docs/sql-schema.md).
		table := strings.ToLower(e.Name) + "s"
		e.Indexes = append(e.Indexes,
			IndexDef{Name: "idx_" + table + "_queue_ready", Expr: "priority DESC, runat, id", Where: "status = '" + QueueStatusQueued + "'", Source: q.Source},
			IndexDef{Name: "idx_" + table + "_queue_running", Fields: []string{"heartbeatAt"}, Where: "status = '" + QueueStatusRunning + "'", Source: q.Source},
		)
		if q.Key != "" {
			// perKey=1 is also a constraint, not only a claim rule.
			e.Indexes = append(e.Indexes, IndexDef{Name: "idx_" + table + "_queue_key", Fields: []string{q.Key}, Unique: q.PerKey == 1, Where: "status = '" + QueueStatusRunning + "'", Source: q.Source})
		}
		if len(q.Unique) > 0 {
			e.Indexes = append(e.Indexes, IndexDef{Name: QueueUniqueIndexName(e.Name), Fields: append([]string(nil), q.Unique...), Unique: true,
				Where: fmt.Sprintf("status IN ('%s', '%s')", QueueStatusQueued, QueueStatusRunning), Source: q.Source})
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// ResolveQueueHandlers finds each queue's handler operation, maps its input
// fields to job fields, and gives the queue entity a repository when it has
// none.
func ResolveQueueHandlers(entities []Entity, services []Service, repos []Repository) ([]Repository, error) {
	var errs []string
	for i := range entities {
		e := &entities[i]
		q := e.Queue
		if q == nil || q.Handler == "" {
			continue
		}
		fail := func(format string, args ...any) {
			errs = append(errs, fmt.Sprintf("%s: queue %s: %s", q.Source, e.Name, fmt.Sprintf(format, args...)))
		}
		fieldByName := map[string]Field{}
		for _, f := range e.Fields {
			fieldByName[strings.ToLower(f.Name)] = f
		}
		var owners []string
		var method Method
		for _, svc := range services {
			for _, m := range svc.Methods {
				if m.Name == q.Handler {
					owners = append(owners, svc.Name)
					method = m
				}
			}
		}
		switch len(owners) {
		case 0:
			fail("handler %s is not an operation", q.Handler)
			continue
		case 1:
			q.Service = owners[0]
		default:
			fail("handler %s exists in services %s", q.Handler, strings.Join(owners, ", "))
			continue
		}
		q.Inputs = nil
		for _, in := range method.Input.Fields {
			name := strings.ToLower(in.Name)
			field, ok := fieldByName[name]
			if !ok && (name == "jobid" || name == "job_id") {
				field, ok = fieldByName["id"]
			}
			if !ok {
				fail("handler %s input %q is no field of the job (name it like the field, or jobId for the id)", q.Handler, in.Name)
				continue
			}
			if in.Type != field.Type || in.IsList != field.IsList {
				fail("handler %s input %q is %s, the job field %s is %s", q.Handler, in.Name, in.Type, field.Name, field.Type)
				continue
			}
			q.Inputs = append(q.Inputs, QueueInput{Input: in.Name, Field: field.Name})
		}
		hasRepo := false
		for _, r := range repos {
			if r.Entity == e.Name {
				hasRepo = true
				break
			}
		}
		if !hasRepo {
			repos = append(repos, Repository{Name: e.Name + "Repository", Entity: e.Name})
		}
	}
	if len(errs) > 0 {
		return repos, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return repos, nil
}

// QueueUniqueIndexName is the partial unique index over the active jobs of a
// queue entity; Enqueue treats a violation of it as "already queued".
func QueueUniqueIndexName(entity string) string {
	return "uq_" + strings.ToLower(entity) + "s_queue_active"
}

func (n *Normalizer) queueSystemFields(source string) ([]Field, error) {
	v := cuecontext.New().CompileString(queueSystemFieldsCUE)
	if v.Err() != nil {
		return nil, fmt.Errorf("queue fields: %w", v.Err())
	}
	ent, err := n.parseEntity("QueueFields", v)
	if err != nil {
		return nil, fmt.Errorf("queue fields: %w", err)
	}
	for i := range ent.Fields {
		ent.Fields[i].Source = source
	}
	return ent.Fields, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
