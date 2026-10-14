package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

// QueueSQL holds the statements of one queue table. They are generated here,
// not in the template, so tests can run them against PostgreSQL.
type QueueSQL struct {
	Table string
	// Touch is ", updatedat = now()" when the entity has updatedAt.
	Touch string
	// Claim: $1 worker, $2 limit (and $3 perKey with a key).
	Claim string
	// Requeue: $1 stale before, $2 max attempts; returns each row's status.
	Requeue string
}

// queueSQL builds the claim and stale-requeue statements of a queue entity.
func queueSQL(e ir.Entity) QueueSQL {
	table := e.TableName()
	touch := ""
	for _, f := range e.Fields {
		if strings.EqualFold(f.Name, "updatedAt") {
			touch = ", updatedat = now()"
		}
	}
	update := fmt.Sprintf(`UPDATE %[1]s AS t SET status = 'running', attempts = t.attempts + 1, lockedby = $1, heartbeatat = now()%[2]s
FROM picked WHERE t.id = picked.id
RETURNING t.id::text`, table, touch)
	var claim string
	if key := e.Queue.Key; key != "" {
		col := DBName(key)
		claim = fmt.Sprintf(`WITH running AS (
	SELECT %[2]s AS k, count(*) AS n FROM %[1]s WHERE status = 'running' GROUP BY %[2]s
), ranked AS (
	SELECT q.id, row_number() OVER (PARTITION BY q.%[2]s ORDER BY q.priority DESC, q.runat, q.id) + COALESCE(r.n, 0) AS slot
	FROM %[1]s q LEFT JOIN running r ON r.k IS NOT DISTINCT FROM q.%[2]s
	WHERE q.status = 'queued' AND q.runat <= now()
), picked AS (
	SELECT q.id FROM %[1]s q JOIN ranked ON ranked.id = q.id
	WHERE ranked.slot <= $3
	ORDER BY q.priority DESC, q.runat, q.id
	LIMIT $2
	FOR UPDATE OF q SKIP LOCKED
)
%[3]s`, table, col, update)
	} else {
		claim = fmt.Sprintf(`WITH picked AS (
	SELECT id FROM %[1]s
	WHERE status = 'queued' AND runat <= now()
	ORDER BY priority DESC, runat, id
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
%[2]s`, table, update)
	}
	requeue := fmt.Sprintf(`UPDATE %[1]s SET
	status = CASE WHEN attempts >= $2 THEN 'dead' ELSE 'queued' END,
	finishedat = CASE WHEN attempts >= $2 THEN now() ELSE finishedat END,
	runat = CASE WHEN attempts >= $2 THEN runat ELSE now() END,
	lasterror = 'heartbeat lost: worker ' || COALESCE(lockedby, '?') || ' stopped',
	lockedby = NULL, heartbeatat = NULL%[2]s
WHERE status = 'running' AND (heartbeatat IS NULL OR heartbeatat < $1)
RETURNING status`, table, touch)
	return QueueSQL{Table: table, Touch: touch, Claim: claim, Requeue: requeue}
}

// queueEntities are the entities with @queue, in declaration order.
func queueEntities(entities []ir.Entity) []ir.Entity {
	var out []ir.Entity
	for _, e := range entities {
		if e.Queue != nil {
			out = append(out, e)
		}
	}
	return out
}

// EmitQueuesFromIR writes internal/jobqueue: the worker loop and a store per
// queue entity. A project without queues gets nothing.
func (e *Emitter) EmitQueuesFromIR(schema *ir.Schema) error {
	queues := queueEntities(schema.Entities)
	if len(queues) == 0 {
		return nil
	}
	dir := e.outDir("internal", "jobqueue")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range []struct{ tmpl, out string }{
		{"templates/jobqueue.tmpl", "jobqueue.go"},
		{"templates/jobqueue_pg.tmpl", "postgres.go"},
	} {
		src, err := ReadTemplateByPath(f.tmpl)
		if err != nil {
			return err
		}
		formatted, err := formatGoStrict(src, "internal/jobqueue/"+f.out)
		if err != nil {
			return err
		}
		if err := WriteFileIfChanged(filepath.Join(dir, f.out), formatted, 0o644); err != nil {
			return err
		}
	}
	tmplSrc, err := ReadTemplateByPath("templates/jobqueue_store.tmpl")
	if err != nil {
		return err
	}
	t, err := template.New("jobqueue_store").Parse(string(tmplSrc))
	if err != nil {
		return err
	}
	for _, ent := range queues {
		q := queueSQL(ent)
		data := map[string]any{
			"GoModule":     e.GoModule,
			"Type":         ExportName(ent.Name),
			"Table":        q.Table,
			"Touch":        q.Touch,
			"ClaimSQL":     q.Claim,
			"RequeueSQL":   q.Requeue,
			"Key":          ent.Queue.Key,
			"KeyField":     ent.Queue.Key,
			"PerKey":       ent.Queue.PerKey,
			"UniqueIndex":  "",
			"UniqueFields": strings.Join(ent.Queue.Unique, ", "),
		}
		if len(ent.Queue.Unique) > 0 {
			data["UniqueIndex"] = normalizer.QueueUniqueIndexName(ent.TableName())
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("queue %s: %w", ent.Name, err)
		}
		unit := "internal/jobqueue/" + DBName(ent.Name) + "_store.go"
		formatted, err := formatGoStrict(buf.Bytes(), unit)
		if err != nil {
			return err
		}
		if err := WriteFileIfChanged(filepath.Join(dir, DBName(ent.Name)+"_store.go"), formatted, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// mainQueues are the queue workers the main starts.
func mainQueues(entities []ir.Entity) []MainQueue {
	var out []MainQueue
	for _, ent := range queueEntities(entities) {
		q := ent.Queue
		mq := MainQueue{
			Entity:      ent.Name,
			Type:        ExportName(ent.Name),
			Service:     q.Service,
			Handler:     q.Handler,
			When:        q.When,
			Source:      q.Source,
			Concurrency: q.Concurrency,
			Attempts:    q.Attempts,
			Poll:        q.Poll,
			Heartbeat:   q.Heartbeat,
			Stale:       q.Stale,
			Backoff:     q.Backoff,
			MaxBackoff:  q.MaxBackoff,
		}
		for _, in := range q.Inputs {
			mq.Inputs = append(mq.Inputs, MainQueueInput{Request: ExportName(in.Input), Job: ExportName(in.Field)})
		}
		out = append(out, mq)
	}
	return out
}
