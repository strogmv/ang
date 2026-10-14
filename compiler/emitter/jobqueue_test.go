package emitter

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/jackc/pgx/v5"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// queueTestEntity runs a queue entity through the normalizer (fields and
// indexes) and into the IR, the way a build does.
func queueTestEntity(t *testing.T, attr string) ir.Entity {
	t.Helper()
	src := `
import "time"

Job: {
	` + attr + `
	id:        string @db(type="UUID", primary_key=true)
	companyId: string @db(type="UUID")
	source:    string
	updatedAt: time.Time
}
`
	val := cuecontext.New().CompileString(src, cue.Filename("cue/domain/job.cue"))
	if val.Err() != nil {
		t.Fatal(val.Err())
	}
	n := normalizer.New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.AddQueueFields(entities); err != nil {
		t.Fatal(err)
	}
	services := []normalizer.Service{{Name: "Catalog", Methods: []normalizer.Method{{Name: "RunJob", Input: normalizer.Entity{Fields: []normalizer.Field{{Name: "jobId", Type: "string"}, {Name: "source", Type: "string"}}}}}}}
	if _, err := normalizer.ResolveQueueHandlers(entities, services, nil); err != nil {
		t.Fatal(err)
	}
	return ir.ConvertEntity(entities[0])
}

// Item 26: a queue entity gets internal/jobqueue (worker loop + its store)
// and a worker in the main; a project without one gets neither.
func TestEmitQueues(t *testing.T) {
	ent := queueTestEntity(t, `@queue(handler="RunJob", key="companyId", unique="companyId,source", concurrency=2, when="cfg.JobsEnabled")`)
	dir := t.TempDir()
	em := New(dir, "sdk", "templates")
	em.GoModule = "github.com/acme/demo"
	if err := em.EmitQueuesFromIR(&ir.Schema{Entities: []ir.Entity{ent}}); err != nil {
		t.Fatal(err)
	}
	store, err := os.ReadFile(filepath.Join(dir, "internal", "jobqueue", "job_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func NewJobStore(db *pgxpool.Pool, repo port.JobRepository) *JobStore",
		"func EnqueueJob(ctx context.Context, repo port.JobRepository, job *domain.Job) (bool, error)",
		`pgErr.ConstraintName == "uq_jobs_queue_active"`,
		"pg_advisory_xact_lock(hashtext('jobqueue:jobs'))",
		"FOR UPDATE OF q SKIP LOCKED",
		", updatedat = now()",
	} {
		if !strings.Contains(string(store), want) {
			t.Errorf("job_store.go misses %q", want)
		}
	}
	for _, f := range []string{"jobqueue.go", "postgres.go"} {
		if _, err := os.Stat(filepath.Join(dir, "internal", "jobqueue", f)); err != nil {
			t.Error(err)
		}
	}

	ctx := runtimeTestContext()
	ctx.EntitiesIR = []ir.Entity{ent}
	ctx.ServicesIR = append(ctx.ServicesIR, ir.Service{Name: "Catalog", Methods: []ir.Method{{Name: "RunJob"}}})
	out, err := emitTestMain(t, ctx, jobsConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"github.com/acme/demo/internal/jobqueue"`,
		"if runWorkers && (cfg.JobsEnabled) {",
		"store := jobqueue.NewJobStore(pgPool, pgrepo.NewJobRepository(pgPool))",
		"Concurrency: 2,",
		"Heartbeat:   30 * time.Second,",
		"Stale:       5 * time.Minute,",
		"runtimeContainer.SvcCatalog.RunJob(ctx, port.RunJobRequest{",
		"JobID:  job.ID,",
		"abortJobs()",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("main.go misses %q", want)
		}
	}
	if t.Failed() {
		t.Log(string(store))
		t.Log(out)
	}

	plain := t.TempDir()
	if err := New(plain, "sdk", "templates").EmitQueuesFromIR(&ir.Schema{Entities: []ir.Entity{{Name: "Customer"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(plain, "internal", "jobqueue")); !os.IsNotExist(err) {
		t.Fatalf("internal/jobqueue without a queue entity: %v", err)
	}
}

// The generated worker loop runs against an in-memory store: completion,
// retry with backoff, dead after the last attempt, Permanent, a job lost to
// the stale requeue, and release at shutdown.
func TestGeneratedJobqueueRun(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test on generated code")
	}
	dir := t.TempDir()
	src, err := ReadTemplateByPath("templates/jobqueue.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	writeGenFile(t, filepath.Join(dir, "go.mod"), "module example.com/jobqueue\n\ngo 1.22\n")
	writeGenFile(t, filepath.Join(dir, "jobqueue.go"), string(src))
	writeGenFile(t, filepath.Join(dir, "jobqueue_test.go"), jobqueueRuntimeTest)
	cmd := exec.Command("go", "test", "-race", "-count=1", "-v", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test of the generated jobqueue.go: %v\n%s", err, out)
	}
	for _, name := range []string{"TestRunCompletesRetriesAndBuries", "TestLostJobWritesNothing", "TestAbortReleases", "TestBackoff"} {
		if !strings.Contains(string(out), "--- PASS: "+name) {
			t.Fatalf("%s did not pass:\n%s", name, out)
		}
	}
}

const jobqueueRuntimeTest = `package jobqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type job struct {
	ID       string
	Attempts int
	Fail     int  // fail this many attempts
	Perm     bool // fail permanently
	Block    bool // run until cancelled
}

type memStore struct {
	mu        sync.Mutex
	jobs      map[string]*job
	status    map[string]string
	owner     map[string]string
	runAt     map[string]time.Time
	lastError map[string]string
	keepLease bool
}

func newMem(jobs ...*job) *memStore {
	s := &memStore{jobs: map[string]*job{}, status: map[string]string{}, owner: map[string]string{}, runAt: map[string]time.Time{}, lastError: map[string]string{}, keepLease: true}
	for _, j := range jobs {
		s.jobs[j.ID] = j
		s.status[j.ID] = "queued"
	}
	return s
}

func (s *memStore) Claim(ctx context.Context, worker string, limit int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, st := range s.status {
		if len(ids) == limit {
			break
		}
		if st == "queued" && !time.Now().Before(s.runAt[id]) {
			s.status[id] = "running"
			s.owner[id] = worker
			s.jobs[id].Attempts++
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *memStore) Load(ctx context.Context, id string) (*job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := *s.jobs[id]
	return &j, nil
}

func (s *memStore) held(id, worker string) bool { return s.status[id] == "running" && s.owner[id] == worker }

func (s *memStore) Heartbeat(ctx context.Context, id, worker string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.keepLease {
		s.status[id] = "queued" // the janitor took it
		s.owner[id] = "other"
	}
	return s.held(id, worker), nil
}

func (s *memStore) set(id, worker, status, lastError string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held(id, worker) {
		s.status[id] = status
		s.lastError[id] = lastError
		s.owner[id] = ""
	}
}

func (s *memStore) Complete(ctx context.Context, id, worker string) error { s.set(id, worker, "done", ""); return nil }
func (s *memStore) Bury(ctx context.Context, id, worker, e string) error  { s.set(id, worker, "dead", e); return nil }
func (s *memStore) Retry(ctx context.Context, id, worker, e string, at time.Time) error {
	s.mu.Lock()
	s.runAt[id] = at
	s.mu.Unlock()
	s.set(id, worker, "queued", e)
	return nil
}
func (s *memStore) Release(ctx context.Context, id, worker string) error {
	s.mu.Lock()
	s.jobs[id].Attempts--
	s.mu.Unlock()
	s.set(id, worker, "released", "")
	return nil
}
func (s *memStore) RequeueStale(ctx context.Context, before time.Time, max int) (int, int, error) {
	return 0, 0, nil
}

func (s *memStore) get(id string) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[id], s.jobs[id].Attempts
}

func handler(ctx context.Context, j *job) error {
	if j.Block {
		<-ctx.Done()
		return ctx.Err()
	}
	if j.Perm {
		return Permanent(errors.New("bad input"))
	}
	if j.Attempts <= j.Fail {
		return errors.New("try again")
	}
	return nil
}

func opts() Options {
	return Options{Name: "test", Concurrency: 3, Poll: 5 * time.Millisecond, Heartbeat: 10 * time.Millisecond, Stale: time.Second, MaxAttempts: 3, Backoff: time.Millisecond, MaxBackoff: 4 * time.Millisecond}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRunCompletesRetriesAndBuries(t *testing.T) {
	s := newMem(&job{ID: "ok"}, &job{ID: "flaky", Fail: 2}, &job{ID: "broken", Fail: 99}, &job{ID: "perm", Perm: true})
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Run[job](ctx, context.Background(), s, opts(), func(j *job) int { return j.Attempts }, handler); close(done) }()
	waitFor(t, func() bool {
		a, _ := s.get("ok")
		b, _ := s.get("flaky")
		c, _ := s.get("broken")
		d, _ := s.get("perm")
		return a == "done" && b == "done" && c == "dead" && d == "dead"
	})
	stop()
	<-done
	if _, n := s.get("flaky"); n != 3 {
		t.Errorf("flaky ran %d times, want 3", n)
	}
	if _, n := s.get("broken"); n != 3 {
		t.Errorf("broken ran %d times, want 3 (MaxAttempts)", n)
	}
	if _, n := s.get("perm"); n != 1 {
		t.Errorf("a permanent error was retried: %d runs", n)
	}
}

func TestLostJobWritesNothing(t *testing.T) {
	s := newMem(&job{ID: "slow", Block: true})
	s.keepLease = false
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Run[job](ctx, context.Background(), s, opts(), func(j *job) int { return j.Attempts }, handler); close(done) }()
	// The heartbeat finds the job taken, cancels the handler and writes
	// nothing over the other owner.
	waitFor(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.owner["slow"] == "other"
	})
	time.Sleep(30 * time.Millisecond)
	stop()
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner["slow"] != "other" || s.lastError["slow"] != "" {
		t.Fatalf("lost job was written: owner %q lastError %q", s.owner["slow"], s.lastError["slow"])
	}
}

func TestAbortReleases(t *testing.T) {
	s := newMem(&job{ID: "long", Block: true})
	ctx, stop := context.WithCancel(context.Background())
	abort, cancelJobs := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Run[job](ctx, abort, s, opts(), func(j *job) int { return j.Attempts }, handler); close(done) }()
	waitFor(t, func() bool { st, _ := s.get("long"); return st == "running" })
	stop() // the drain: the job keeps running
	select {
	case <-done:
		t.Fatal("Run returned while a job was running")
	case <-time.After(50 * time.Millisecond):
	}
	cancelJobs() // the drain timed out
	<-done
	if st, n := s.get("long"); st != "released" || n != 0 {
		t.Fatalf("aborted job: status %q attempts %d, want released with the attempt undone", st, n)
	}
}

func TestBackoff(t *testing.T) {
	for n, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 10: 8 * time.Second} {
		got := Backoff(time.Second, 8*time.Second, n)
		if got < want || got > want+want/5 {
			t.Errorf("Backoff(n=%d) = %s, want %s plus at most 20%%", n, got, want)
		}
	}
}
`

// The claim, heartbeat, retry and stale requeue statements against
// PostgreSQL, on the table the generated schema.sql creates.
func TestQueueSQLOnPostgres(t *testing.T) {
	base := os.Getenv("ANG_TEST_POSTGRES_URL")
	if base == "" {
		t.Skip("ANG_TEST_POSTGRES_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	db := fmt.Sprintf("angtest_queue_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE IF EXISTS "+db+" WITH (FORCE)")
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db
	connect := func() *pgx.Conn {
		t.Helper()
		c, err := pgx.Connect(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	pool := connect()
	defer pool.Close(ctx)

	ent := queueTestEntity(t, `@queue(handler="RunJob", key="companyId", unique="companyId,source")`)
	schema := emitSchemaSQL(t, New(t.TempDir(), "", "templates"), ent)
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("%v\n%s", err, schema)
	}
	q := queueSQL(ent)
	const a, b = "aaaaaaaa-0000-4000-8000-000000000000", "bbbbbbbb-0000-4000-8000-000000000000"
	insert := func(id, company, source string, priority int) error {
		_, err := pool.Exec(ctx, `INSERT INTO jobs (id, companyid, source, updatedat, priority, runat) VALUES ($1, $2, $3, now(), $4, now() - interval '1 second')`, id, company, source, priority)
		return err
	}
	for i, j := range []struct {
		company, source string
		priority        int
	}{{a, "a1", 0}, {a, "a2", 5}, {a, "a3", 0}, {b, "b1", 0}, {b, "b2", 0}} {
		if err := insert(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", i), j.company, j.source, j.priority); err != nil {
			t.Fatal(err)
		}
	}
	if err := insert("00000000-0000-4000-8000-000000000009", a, "a1", 0); err == nil || !strings.Contains(err.Error(), "uq_jobs_queue_active") {
		t.Fatalf("a duplicate active job: %v", err)
	}

	claim := func(worker string, limit int) []string {
		t.Helper()
		conn := connect()
		defer conn.Close(ctx)
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('jobqueue:jobs'))`); err != nil {
			t.Fatal(err)
		}
		rows, err := tx.Query(ctx, q.Claim, worker, limit, 1)
		if err != nil {
			t.Fatal(err)
		}
		var ids, sources []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			var src string
			if err := tx.QueryRow(ctx, `SELECT source FROM jobs WHERE id = $1`, id).Scan(&src); err != nil {
				t.Fatal(err)
			}
			sources = append(sources, src)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return sources
	}
	// One per company, the higher priority first.
	got := claim("w1", 10)
	if strings.Join(sortedCopy(got), ",") != "a2,b1" {
		t.Fatalf("first claim = %v, want a2 and b1", got)
	}
	if got := claim("w2", 10); len(got) != 0 {
		t.Fatalf("a second claim took %v while both companies run a job", got)
	}
	// Concurrent claims never take the same row.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'done', lockedby = NULL WHERE source IN ('a2', 'b1')`); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var all []string
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			got := claim(fmt.Sprintf("c%d", w), 10)
			mu.Lock()
			all = append(all, got...)
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	if len(all) != 2 {
		t.Fatalf("concurrent claims took %v, want one job of each company", all)
	}

	var heldBy string
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT lockedby, attempts FROM jobs WHERE source = $1`, all[0]).Scan(&heldBy, &attempts); err != nil || attempts != 1 {
		t.Fatalf("claimed row: worker %q attempts %d err %v", heldBy, attempts, err)
	}
	// Stale requeue: a running job with an old heartbeat goes back to queued,
	// one that used its last attempt goes dead.
	if _, err := pool.Exec(ctx, `UPDATE jobs SET heartbeatat = now() - interval '1 hour' WHERE status = 'running'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jobs SET attempts = 3 WHERE source = $1`, all[1]); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, q.Requeue, time.Now().Add(-time.Minute), 3)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]int{}
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			t.Fatal(err)
		}
		statuses[st]++
	}
	rows.Close()
	if statuses["queued"] != 1 || statuses["dead"] != 1 {
		t.Fatalf("requeue = %v, want one queued and one dead", statuses)
	}
	var lastError string
	if err := pool.QueryRow(ctx, `SELECT lasterror FROM jobs WHERE source = $1`, all[0]).Scan(&lastError); err != nil || !strings.Contains(lastError, "heartbeat lost: worker c") {
		t.Fatalf("lasterror = %q (%v)", lastError, err)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
