// ============================================================================
// GOLDEN ACTIONS REFERENCE (GENERATED — DO NOT EDIT)
// ============================================================================
// One minimal operation per Typed Flow IR action that cue/GOLDEN_EXAMPLES.cue
// does not already show. Each step is the action's catalog example
// (compiler/flowir/examples.go), which a test decodes with the action's own
// decoder. Regenerate: ANG_UPDATE_GOLDEN=1 go test ./cmd/ang -run TestGoldenActionsReferenceInSync

package examples

import "github.com/strogmv/ang/cue/schema"

// approval.Decide
RefApprovalDecide: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "approval.Decide", actor: "req.UserID", approvalId: "approvalID", decision: "\"approved\"", status: "approvalStatus"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// archive.ZipDir
RefArchiveZipDir: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "archive.ZipDir", output: "zipBytes", path: "\"./tmp\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// audit.Log
RefAuditLog: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "audit.Log", actor: "req.ID", company: "req.ID", event: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// auth.CheckRole
RefAuthCheckRole: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "auth.CheckRole", companyID: "req.CompanyID", roles: "[]string{\"admin\"}", user: "currentUser"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// auth.RequireRole
RefAuthRequireRole: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "auth.RequireRole", companyID: "req.CompanyID", output: "currentUser", roles: "[]string{\"admin\"}", userID: "req.UserID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// base64.Decode
RefBase64Decode: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "base64.Decode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// base64.Encode
RefBase64Encode: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "base64.Encode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// batch.Run
RefBatchRun: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "batch.Run", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], as: "batch", from: "items", size: 1},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// budget.Check
RefBudgetCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "budget.Check", key: "req.UserID", limit: 5000},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// budget.Consume
RefBudgetConsume: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "budget.Consume", key: "req.UserID", tokens: "reply.TokensUsed"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// bulkhead.Acquire
RefBulkheadAcquire: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "bulkhead.Acquire", max: 10, name: "\"payment-api\"", throw: "payment service is busy"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// bulkhead.Run
RefBulkheadRun: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "bulkhead.Run", do: [{action: "logic.Check", condition: "true", throw: "ok"}], max: 12, name: "\"s3-upload\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// cache.Del
RefCacheDel: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "cache.Del", key: "\"k\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// cast.ToString
RefCastToString: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "cast.ToString", input: "req.UserID", output: "userIDString"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// circuit.Breaker
RefCircuitBreaker: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "circuit.Breaker", do: [{action: "http.Call", method: "GET", output: "body", url: "\"https://api.test\""}], name: "\"external-api\"", openTTL: "30*time.Second", threshold: 3},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// circuit.Check
RefCircuitCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "circuit.Check", name: "\"payments\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// circuit.RecordFailure
RefCircuitRecordFailure: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "circuit.RecordFailure", name: "\"payments\"", threshold: 3},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// circuit.RecordSuccess
RefCircuitRecordSuccess: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "circuit.RecordSuccess", name: "\"payments\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// claude.Chat
RefClaudeChat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "claude.Chat", output: "reply", user_message: "\"hello\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// concurrency.Limit
RefConcurrencyLimit: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "concurrency.Limit", key: "req.CompanyID", max: 4},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// concurrency.Run
RefConcurrencyRun: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "concurrency.Run", do: [{action: "logic.Check", condition: "true", throw: "ok"}], key: "\"build\"", max: 8},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// config.Get
RefConfigGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "config.Get", default: "\"dev\"", key: "\"APP_ENV\"", output: "env"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// context.Trim
RefContextTrim: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "context.Trim", input: "project.CueContent", max_bytes: 12000, output: "trimmedCue"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// convert.ToFloat
RefConvertToFloat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "convert.ToFloat", input: "req.Count", output: "countFloat"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// convert.ToInt
RefConvertToInt: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "convert.ToInt", input: "req.Count", output: "countInt"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// crypto.Decrypt
RefCryptoDecrypt: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "crypto.Decrypt", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// crypto.Encrypt
RefCryptoEncrypt: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "crypto.Encrypt", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// cue.EmitProject
RefCueEmitProject: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "cue.EmitProject", micro_plan: "microPlanDoc", output: "projectFiles", usecases: "usecasesDoc"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// cue.ValidateProject
RefCueValidateProject: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "cue.ValidateProject", files: "projectFiles", output: "validation"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// cue.WriteProjectFiles
RefCueWriteProjectFiles: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "cue.WriteProjectFiles", files: "projectFiles", output: "writeResult", root: "\"/tmp/project\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Delete
RefDbDelete: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Delete", input: "req.ID", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Get
RefDbGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Get", input: "req.UserID", output: "user", source: "User"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Insert
RefDbInsert: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Insert", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.List
RefDbList: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.List", input: "req.UserID", method: "ListByUser", output: "orders", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Lock
RefDbLock: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Lock", error: "order not found", input: "req.ID", output: "order", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Query
RefDbQuery: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Query", input: "req.UserID", method: "ListOpenByUser", output: "orders", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.SelectForUpdate
RefDbSelectForUpdate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.SelectForUpdate", input: "req.ID", output: "order", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Update
RefDbUpdate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Update", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// db.Upsert
RefDbUpsert: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "db.Upsert", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// dedupe.Once
RefDedupeOnce: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "dedupe.Once", do: [{action: "flow.SuggestNext", options: ["done"]}], key: "\"job:1\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// dlq.Publish
RefDlqPublish: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "dlq.Publish", payload: "msg", reason: "\"decode failed\"", subject: "\"events.test\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// entity.PatchNonZero
RefEntityPatchNonZero: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "entity.PatchNonZero", fields: "Name,Email", from: "req", target: "user"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// entity.PatchValidated
RefEntityPatchValidated: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "entity.PatchValidated", fields: {Email: {format: "email", normalize: "lower"}, TaxID: {normalize: "trim", unique: "FindByTaxID"}}, from: "req", source: "Company", target: "company"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// enum.Validate
RefEnumValidate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "enum.Validate", allowed: "draft,published", throw: "invalid status", value: "req.Status"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// errors.Map
RefErrorsMap: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "errors.Map", cases: {"duplicate key": {code: "conflict", message: "order already exists", status: "409"}, "not found": {code: "not_found", message: "order not found", status: "404"}}, input: "err", mode: "contains", output: "mappedErr"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// errors.New
RefErrorsNew: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "errors.New", code: "\"NOT_FOUND\"", message: "\"boom\"", output: "errObj", status: "404"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// errors.ThrowIf
RefErrorsThrowIf: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "errors.ThrowIf", code: "NOT_FOUND", condition: "user == nil", status: "404", throw: "user missing"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// errors.Wrap
RefErrorsWrap: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "errors.Wrap", err: "err", message: "\"save failed\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// event.Broadcast
RefEventBroadcast: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "event.Broadcast", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// event.EmitIf
RefEventEmitIf: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "event.EmitIf", condition: "req.Notify", name: "UserRegistered", payloadMap: {UserID: "user.ID"}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// event.Match
RefEventMatch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "event.Match", event: "evt", match: "\"order.created\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// event.Subscribe
RefEventSubscribe: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "event.Subscribe", do: [{action: "flow.SuggestNext", options: ["seen"]}], match: "\"tenant=acme\"", name: "\"OrderCreated\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// event.Wait
RefEventWait: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "event.Wait", into: "map[string]any", name: "\"OrderCreated\"", output: "evt"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// exec.Run
RefExecRun: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "exec.Run", cmd: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// exec.Stream
RefExecStream: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "exec.Stream", cmd: "req.ID", output: "streamOut", timeout: "120 * time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// field.CopyNonEmpty
RefFieldCopyNonEmpty: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "field.CopyNonEmpty", fields: "Name,Email", from: "req", to: "user"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Block
RefFlowBlock: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Block", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Call
RefFlowCall: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Call", args: {id: "requestID"}, op: "Profile.Get", output: "profile"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Catch
RefFlowCatch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Catch", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Checkpoint
RefFlowCheckpoint: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Checkpoint", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Cron
RefFlowCron: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Cron", do: [{action: "flow.SuggestNext", options: ["inside"]}], window: "Mon-Fri 09:00-17:00"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Defer
RefFlowDefer: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Defer", do: [{action: "fs.Remove", path: "workDir"}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Delay
RefFlowDelay: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Delay", duration: "5 * time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.ExplainError
RefFlowExplainError: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.ExplainError", hint: "check that the order is still open", output: "explanation"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Fallback
RefFlowFallback: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Fallback", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], fallback: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.History.Get
RefFlowHistoryGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.History.Get", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Join
RefFlowJoin: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Join", branches: {company: [{action: "repo.Get", input: "req.CompanyID", output: "company", source: "Company"}], orders: [{action: "repo.List", input: "req.CompanyID", method: "ListByCompany", output: "orders", source: "Order"}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Parallel
RefFlowParallel: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Parallel", branches: {company: [{action: "repo.Get", input: "req.CompanyID", output: "company", source: "Company"}], orders: [{action: "repo.List", input: "req.CompanyID", method: "ListByCompany", output: "orders", source: "Order"}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.RecordEvent
RefFlowRecordEvent: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.RecordEvent", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Replay
RefFlowReplay: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Replay", history: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Resume
RefFlowResume: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Resume", into: "map[string]any", name: "\"draft\"", output: "draft"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Retry
RefFlowRetry: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Retry", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Return
RefFlowReturn: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Return", set: "resp.Status", value: "\"ok\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Schedule
RefFlowSchedule: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Schedule", at: "deadline"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.SuggestNext
RefFlowSuggestNext: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.SuggestNext", options: ["\"retry\"", "\"contact support\""], output: "nextSteps"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Switch
RefFlowSwitch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Switch", cases: {fail: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], ok: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}, default: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], value: "req.Mode"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Tag
RefFlowTag: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Tag", name: "\"stage\"", value: "\"validate\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Timeout
RefFlowTimeout: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Timeout", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], onTimeout: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], duration: "2 * time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Try
RefFlowTry: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Try", catch: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.Validate
RefFlowValidate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.Validate", condition: "true"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// flow.While
RefFlowWhile: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "flow.While", do: [{action: "mapping.Assign", to: "i", value: "i + 1"}], condition: "i < 1"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// fs.ReadFile
RefFsReadFile: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "fs.ReadFile", output: "result", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// fs.Remove
RefFsRemove: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "fs.Remove", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// fs.TempDir
RefFsTempDir: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "fs.TempDir", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// fs.WriteFile
RefFsWriteFile: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "fs.WriteFile", data: "\"hello\"", path: "\"/tmp/out.txt\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// hash.HMAC
RefHashHmac: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "hash.HMAC", input: "req.ID", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// hash.Sum
RefHashSum: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "hash.Sum", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// http.Call
RefHTTPCall: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "http.Call", body: "\"{}\"", method: "POST", output: "httpBody", statusVar: "httpStatus", url: "\"https://example.com\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// http.Paginate
RefHTTPPaginate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "http.Paginate", as: "page", cursor_expr: "page.NextCursor", into: "PagedResponse", items_expr: "page.Items", output: "items", output_type: "[]Item", url: "\"https://example.com/items\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// http.Request
RefHTTPRequest: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "http.Request", method: "GET", output: "body", statusVar: "status", url: "\"https://example.com\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// http.RetryPolicy
RefHTTPRetryPolicy: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "http.RetryPolicy", attempts: 3, body: "\"{}\"", method: "POST", output: "body", url: "\"https://example.com\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// http.SOAP
RefHTTPSoap: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "http.SOAP", into: "VIESResponse", namespace: "\"urn:test\"", operation: "\"CheckVat\"", output: "soapResp", request: {countryCode: "\"DE\"", vatNumber: "req.VAT"}, statusVar: "soapStatus", url: "\"https://example.com/soap\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idem.Check
RefIdemCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idem.Check", key: "idemKey"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idem.DeriveKey
RefIdemDeriveKey: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idem.DeriveKey", from: ["req.UserID", "req.OrderID"], output: "idemKey"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idem.SaveResult
RefIdemSaveResult: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idem.SaveResult", key: "idemKey", ttl: "24*time.Hour"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idempotency.Check
RefIdempotencyCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idempotency.Check", key: "idemKey"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idempotency.DeriveKey
RefIdempotencyDeriveKey: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idempotency.DeriveKey", from: ["req.UserID", "req.OrderID"], output: "idemKey"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// idempotency.SaveResult
RefIdempotencySaveResult: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "idempotency.SaveResult", key: "idemKey", ttl: "24*time.Hour"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// json.Marshal
RefJSONMarshal: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "json.Marshal", input: "req.Payload", output: "rawJSON"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// json.Parse
RefJSONParse: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "json.Parse", input: "req.Raw", into: "map[string]any", output: "parsed"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// json.Stringify
RefJSONStringify: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "json.Stringify", input: "resp", output: "raw"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// jsonpath.Get
RefJsonpathGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "jsonpath.Get", input: "req.ID", output: "result", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// jsonpath.Set
RefJsonpathSet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "jsonpath.Set", input: "req.ID", output: "result", path: "\"sample\"", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// jwt.Sign
RefJWTSign: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "jwt.Sign", claims: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// jwt.Verify
RefJWTVerify: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "jwt.Verify", output: "result", token: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.All
RefListAll: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.All", as: "item", condition: "item.Active", from: "items", output: "allActive"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Any
RefListAny: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Any", as: "item", condition: "item.Active", from: "items", output: "hasActive"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Append
RefListAppend: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Append", item: "req.ID", to: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Avg
RefListAvg: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Avg", field: "Price", input: "items", output: "avgPrice"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Chunk
RefListChunk: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Chunk", from: "items", output: "batches", size: 100},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Distinct
RefListDistinct: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Distinct", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Enrich
RefListEnrich: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Enrich", items: "items", lookupInput: "_item.UserID", lookupSource: "User", set: "AuthorName=Name"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Filter
RefListFilter: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Filter", condition: "true", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Find
RefListFind: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Find", as: "item", condition: "item.ID == req.ID", found: "matchFound", from: "items", into: "Item", output: "match"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.GroupBy
RefListGroupBy: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.GroupBy", from: "req.ID", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Len
RefListLen: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Len", input: "items", output: "itemCount"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Map
RefListMap: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Map", expr: "req.ID", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.New
RefListNew: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.New", cap: "16", output: "ids", type: "[]string"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Paginate
RefListPaginate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Paginate", input: "req.ID", limit: "req.ID", offset: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Reduce
RefListReduce: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Reduce", expr: "req.ID", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Sort
RefListSort: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Sort", by: "req.ID", items: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// list.Sum
RefListSum: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "list.Sum", input: "nums", output: "total"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// locale.Resolve
RefLocaleResolve: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "locale.Resolve", default: "\"en\"", output: "locale", sources: "req.Locale, user.Locale"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// log.Emit
RefLogEmit: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "log.Emit", level: "\"info\"", message: "\"created project\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.Build
RefMapBuild: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.Build", as: "item", from: "items", key: "item.ID", output: "byID", value: "item.Name", valueType: "string"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.Get
RefMapGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.Get", default: "\"unknown\"", found: "ok", input: "byID", into: "string", key: "req.ID", output: "name"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.Has
RefMapHas: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.Has", input: "byID", key: "req.ID", output: "exists"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.Merge
RefMapMerge: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.Merge", left: "baseLabels", output: "mergedLabels", right: "extraLabels"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.New
RefMapNew: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.New", output: "labels", type: "map[string]string"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// map.Set
RefMapSet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "map.Set", input: "labels", key: "\"status\"", output: "nextLabels", value: "\"ready\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// math.Op
RefMathOp: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "math.Op", op: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// metric.Emit
RefMetricEmit: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "metric.Emit", kind: "\"counter\"", name: "\"project.created\"", value: "1"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// model.Resolve
RefModelResolve: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "model.Resolve", name: "\"Cheap\"", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// mutex.With
RefMutexWith: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "mutex.With", do: [{action: "log.Emit", message: "\"inside lock\""}], key: "\"jobs:sync\"", poll: "25 * time.Millisecond", wait: "2 * time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// notification.Dispatch
RefNotificationDispatch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "notification.Dispatch", event: "\"invite.sent\"", payload: "req", userID: "req.UserID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// notify.Dispatch
RefNotifyDispatch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "notify.Dispatch", entityID: "order.ID", event: "\"order.shipped\"", payload: "order", userID: "order.UserID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// notify.Email
RefNotifyEmail: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "notify.Email", output: "notificationID", text: "\"Hello\"", to: "req.Email"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// notify.Send
RefNotifySend: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "notify.Send", channel: "\"email\"", text: "\"Approval timeout: fallback executed\"", to: "\"ops@company.com\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// num.Add
RefNumAdd: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "num.Add", a: "req.A", b: "req.B", output: "sum"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// num.Div
RefNumDiv: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "num.Div", a: "req.A", b: "req.B", output: "ratio"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// num.Mul
RefNumMul: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "num.Mul", a: "req.A", b: "req.B", output: "prod"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// num.Sub
RefNumSub: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "num.Sub", a: "req.A", b: "req.B", output: "diff"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// oauth.Google.Exchange
RefOauthGoogleExchange: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "oauth.Google.Exchange", clientID: "cfg.GoogleClientID", clientSecret: "cfg.GoogleClientSecret", code: "req.Code", output: "googleToken", redirectURL: "cfg.GoogleRedirectURL"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// oauth.Google.GetURL
RefOauthGoogleGetURL: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "oauth.Google.GetURL", clientID: "cfg.GoogleClientID", output: "authURL", redirectURL: "cfg.GoogleRedirectURL", state: "req.State"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// oauth.Google.UserInfo
RefOauthGoogleUserInfo: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "oauth.Google.UserInfo", output: "googleUser", token: "googleToken"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// oauth2.Refresh
RefOauth2Refresh: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "oauth2.Refresh", output: "result", refreshToken: "req.ID", tokenURL: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// oauth2.Token
RefOauth2Token: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "oauth2.Token", output: "result", tokenURL: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// openai.Chat
RefOpenaiChat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "openai.Chat", max_rounds: 4, output: "reply", output_tool_calls: "toolCalls", output_usage: "usage", tool_choice: "\"auto\"", tools: ["LookupPost"], user_message: "req.Message"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// openai.Embed
RefOpenaiEmbed: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "openai.Embed", dimensions: 256, input: "req.Query", output: "embedding", output_usage: "usage"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// openai.Stream
RefOpenaiStream: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "openai.Stream", output: "reply", user_message: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// parallel.Run
RefParallelRun: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "parallel.Run", branches: {a: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], b: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// path.Base
RefPathBase: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "path.Base", input: "trimmed", output: "base"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// pdf.Render
RefPdfRender: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "pdf.Render", data: "req.ReportData", output: "pdfBytes", template: "\"t\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// plan.BuildAutomata
RefPlanBuildAutomata: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "plan.BuildAutomata", input: "usecasesDoc", output: "automataDoc"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// plan.BuildMicroPlan
RefPlanBuildMicroPlan: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "plan.BuildMicroPlan", automata: "automataDoc", output: "microPlanDoc", usecases: "usecasesDoc"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// policy.Check
RefPolicyCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "policy.Check", policyAllowAdminOverride: true, policyResolved: true, policyRoles: ["owner", "admin"], policySameCompany: true, companyID: "req.CompanyID", policy: "\"CompanyAdminOnly\"", user: "currentUser"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// policy.Decide
RefPolicyDecide: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "policy.Decide", operation: "\"create\"", output: "decisionObj", policyKey: "\"project.create\"", subject: "req.UserID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// policy.Evaluate
RefPolicyEvaluate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "policy.Evaluate", decision: "policyDecision", effects: "policyEffects", operation: "\"create\"", output: "policyResult", policyKey: "\"project.create\"", reason: "policyReason", subject: "req.UserID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// policy.Require
RefPolicyRequire: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "policy.Require", operation: "\"create\"", policyKey: "\"project.create\"", subject: "req.UserID", throw: "\"forbidden\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// profile.Require
RefProfileRequire: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "profile.Require", key: "req.UserID", tier: "\"ops\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// query.Decode
RefQueryDecode: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "query.Decode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// query.Encode
RefQueryEncode: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "query.Encode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// queue.Ack
RefQueueAck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "queue.Ack", messageID: "msgID", subject: "\"events.test\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// queue.Dequeue
RefQueueDequeue: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "queue.Dequeue", ackToken: "msgID", attempts: 3, backoffMs: 50, jitterMs: 10, output: "msg", subject: "\"events.test\"", timeout: "2*time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// queue.Enqueue
RefQueueEnqueue: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "queue.Enqueue", payload: "req.Payload", subject: "\"events.test\"", timeout: "2*time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// queue.Nack
RefQueueNack: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "queue.Nack", messageID: "msgID", reason: "\"decode failed\"", subject: "\"events.test\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// quota.Check
RefQuotaCheck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "quota.Check", key: "req.UserID", limit: 100, window: "\"day\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// rand.Code
RefRandCode: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "rand.Code", length: 6, output: "otp"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// rand.Token
RefRandToken: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "rand.Token", bytes: 16, output: "token"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ratelimit.Limit
RefRatelimitLimit: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "ratelimit.Limit", key: "req.UserID", rps: 20},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// rbac.CheckPermission
RefRbacCheckPermission: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "rbac.CheckPermission", permission: "req.ID", user: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// regex.Match
RefRegexMatch: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "regex.Match", input: "req.ID", output: "result", pattern: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// regex.Replace
RefRegexReplace: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "regex.Replace", input: "req.ID", output: "result", pattern: "req.ID", repl: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// repo.Count
RefRepoCount: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "repo.Count", input: "req.AuthorID", method: "CountByAuthorID", output: "count", source: "Post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// repo.Exists
RefRepoExists: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "repo.Exists", input: "req.Email", method: "ExistsByEmail", output: "exists", source: "User"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// repo.Get
RefRepoGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "repo.Get", input: "req.ID", output: "item", source: "Tender"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// repo.GetForUpdate
RefRepoGetForUpdate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "repo.GetForUpdate", error: "order not found", input: "req.ID", output: "order", source: "Order"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// repo.Upsert
RefRepoUpsert: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "repo.Upsert", ifNew: [{action: "flow.SuggestNext", options: ["created"]}], find: "req.ID", input: "reqUser", output: "user", source: "User"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// secret.Get
RefSecretGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "secret.Get", key: "\"SMTP_PASSWORD\"", output: "smtpPassword"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// service.Call
RefServiceCall: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "service.Call", args: ["missingRequest"], method: "Get", output: "profile", service: "Profile"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// session.Get
RefSessionGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "session.Get", output: "sessionID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// slo.Budget
RefSloBudget: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "slo.Budget", do: [{action: "logic.Check", condition: "true", throw: "ok"}], duration: "2*time.Second", name: "\"build\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// state.Delete
RefStateDelete: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "state.Delete", key: "\"draft:1\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// state.Get
RefStateGet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "state.Get", default: "map[string]any{}", into: "map[string]any", key: "\"draft:1\"", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// state.Set
RefStateSet: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "state.Set", key: "\"draft:1\"", ttl: "time.Minute", value: "req"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// storage.Delete
RefStorageDelete: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "storage.Delete", key: "\"path/file\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// storage.Download
RefStorageDownload: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "storage.Download", key: "\"path/file\"", output: "blob"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// storage.GetURL
RefStorageGetURL: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "storage.GetURL", key: "\"path/file\"", output: "publicURL"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// storage.List
RefStorageList: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "storage.List", output: "keys", prefix: "\"path/\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.Concat
RefStrConcat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.Concat", output: "line", parts: ["\"id=\"", "req.ID"]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.Format
RefStrFormat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.Format", args: ["req.UserID", "req.CompanyID"], output: "resp.RedirectURL", template: "\"u:%s/%s\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.Normalize
RefStrNormalize: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.Normalize", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.ReplaceAll
RefStrReplaceAll: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.ReplaceAll", input: "req.Path", new: "\"/\"", old: "\"\\\\\"", output: "normPath"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.StripMarkdown
RefStrStripMarkdown: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.StripMarkdown", input: "req.Content", output: "plain"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// str.TrimSpace
RefStrTrimSpace: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "str.TrimSpace", input: "req.Name", output: "trimName"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// stream.Emit
RefStreamEmit: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "stream.Emit", data: "\"{\\\"type\\\":\\\"stage\\\"}\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// template.Render
RefTemplateRender: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "template.Render", data: "map[string]any{\"Name\": req.Name}", output: "body", template: "\"Hello {{.Name}}\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.Add
RefTimeAdd: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.Add", duration: "15*time.Minute", input: "expiresAt", output: "extendedAt"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.CheckExpiry
RefTimeCheckExpiry: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.CheckExpiry", throw: "\"expired\"", value: "req.ExpiresAt"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.Diff
RefTimeDiff: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.Diff", from: "issuedAt", output: "ttlMinutes", to: "expiresAt", unit: "minutes"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.Format
RefTimeFormat: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.Format", input: "createdAt", output: "formatted", zero: "empty"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.InZone
RefTimeInZone: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.InZone", input: "order.CreatedAt", output: "createdLocal", timezone: "\"Europe/Berlin\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.Now
RefTimeNow: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.Now", output: "now"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// time.Sub
RefTimeSub: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "time.Sub", a: "expiresAt", b: "issuedAt", output: "ttl"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// token.Generate
RefTokenGenerate: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "token.Generate", claims: "map[string]any{\"email\": user.Email}", output: "token", purpose: "\"verify_email\"", secret: "\"secret\"", subject: "user.ID", ttl: "\"30m\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// token.Verify
RefTokenVerify: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "token.Verify", output: "claims", purpose: "\"verify_email\"", secret: "\"secret\"", token: "req.Token"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// trace.Span
RefTraceSpan: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "trace.Span", do: [{action: "logic.Check", condition: "true", throw: "ok"}], name: "\"BuildProject\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ulid.New
RefUlidNew: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "ulid.New", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// url.Build
RefURLBuild: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "url.Build", base: "\"https://example.com/base\"", output: "link", segments: ["\"verify\"", "req.Token"]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// url.Parse
RefURLParse: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "url.Parse", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// uuid.New
RefUUIDNew: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "uuid.New", output: "id"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// value.Coalesce
RefValueCoalesce: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "value.Coalesce", into: "string", output: "displayName", values: ["req.DisplayName", "req.Email", "\"anonymous\""]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// webhook.Ack
RefWebhookAck: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "webhook.Ack", body: "\"accepted\"", status: 202},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// webhook.Send
RefWebhookSend: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "webhook.Send", event: "\"evt\"", payload: "req.Payload", url: "\"https://hook.example\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// webhook.VerifySignature
RefWebhookVerifySignature: schema.#Operation & {
	service: "reference"
	input: {id: string}
	output: {ok: bool}
	flow: [
		{action: "webhook.VerifySignature", output: "sigOK", payload: "req.Body", signature: "req.Signature"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
