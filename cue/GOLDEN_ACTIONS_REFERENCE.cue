// ============================================================================
// GOLDEN ACTIONS REFERENCE (AUTO-GENERATED)
// ============================================================================
// This file provides one minimal reference operation per catalog action that
// is not present in cue/GOLDEN_EXAMPLES.cue. It is contract-oriented guidance
// for AI/codegen prompts, while executable edge cases stay in GOLDEN_EXAMPLES.
// Source: ang actions --json
// DO NOT hand-edit large sections; regenerate when catalog changes.

package examples

import "github.com/strogmv/ang/cue/schema"

// ----------------------------------------------------------------------------
// REF EXAMPLE 1000: approval.Decide
// ----------------------------------------------------------------------------
RefApprovalDecide: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "approval.Decide", actor: "req.ID", approvalId: "result", decision: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 10999: flow.Call
// ----------------------------------------------------------------------------
RefFlowCall: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	uses: ["Tender"]
	flow: [
		{action: "flow.Call", op: "Tender.GetTenderTemplate", args: {id: "req.ID"}, output: "result", ignoreErr: true},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1001: archive.ZipDir
// ----------------------------------------------------------------------------
RefArchiveZipDir: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "archive.ZipDir", output: "result", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1002: audit.Log
// ----------------------------------------------------------------------------
RefAuditLog: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "audit.Log", actor: "req.ID", company: "req.ID", event: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1003: auth.CheckRole
// ----------------------------------------------------------------------------
RefAuthCheckRole: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "auth.CheckRole", roles: "req.ID", user: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1004: auth.RequireRole
// ----------------------------------------------------------------------------
RefAuthRequireRole: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "auth.RequireRole", companyID: "req.ID", roles: "req.ID", userID: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1005: base64.Decode
// ----------------------------------------------------------------------------
RefBase64Decode: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "base64.Decode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1006: base64.Encode
// ----------------------------------------------------------------------------
RefBase64Encode: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "base64.Encode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1007: batch.Run
// ----------------------------------------------------------------------------
RefBatchRun: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "batch.Run", from: "items", size: 1, as: "batch", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1008: bulkhead.Acquire
// ----------------------------------------------------------------------------
RefBulkheadAcquire: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "bulkhead.Acquire", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1009: bulkhead.Run
// ----------------------------------------------------------------------------
RefBulkheadRun: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "bulkhead.Run", name: "\"sample\"", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1010: cache.Del
// ----------------------------------------------------------------------------
RefCacheDel: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "cache.Del", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1011: circuit.Breaker
// ----------------------------------------------------------------------------
RefCircuitBreaker: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "circuit.Breaker", name: "\"sample\"", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1012: circuit.Check
// ----------------------------------------------------------------------------
RefCircuitCheck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "circuit.Check", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1013: circuit.RecordFailure
// ----------------------------------------------------------------------------
RefCircuitRecordFailure: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "circuit.RecordFailure", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1014: circuit.RecordSuccess
// ----------------------------------------------------------------------------
RefCircuitRecordSuccess: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "circuit.RecordSuccess", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1015: concurrency.Limit
// ----------------------------------------------------------------------------
RefConcurrencyLimit: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "concurrency.Limit", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1016: concurrency.Run
// ----------------------------------------------------------------------------
RefConcurrencyRun: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "concurrency.Run", key: "req.ID", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1017: config.Get
// ----------------------------------------------------------------------------
RefConfigGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "config.Get", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1018: crypto.Decrypt
// ----------------------------------------------------------------------------
RefCryptoDecrypt: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "crypto.Decrypt", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1019: crypto.Encrypt
// ----------------------------------------------------------------------------
RefCryptoEncrypt: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "crypto.Encrypt", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1020: db.Delete
// ----------------------------------------------------------------------------
RefDbDelete: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Delete"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1021: db.Get
// ----------------------------------------------------------------------------
RefDbGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Get"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1022: db.Insert
// ----------------------------------------------------------------------------
RefDbInsert: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Insert", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1023: db.List
// ----------------------------------------------------------------------------
RefDbList: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.List"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1024: db.Lock
// ----------------------------------------------------------------------------
RefDbLock: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Lock"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1025: db.Query
// ----------------------------------------------------------------------------
RefDbQuery: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Query", method: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1026: db.SelectForUpdate
// ----------------------------------------------------------------------------
RefDbSelectForUpdate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.SelectForUpdate"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1027: db.Update
// ----------------------------------------------------------------------------
RefDbUpdate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Update", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1028: db.Upsert
// ----------------------------------------------------------------------------
RefDbUpsert: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "db.Upsert", input: "req.ID", source: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1029: dedupe.Once
// ----------------------------------------------------------------------------
RefDedupeOnce: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "dedupe.Once", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1030: dlq.Publish
// ----------------------------------------------------------------------------
RefDlqPublish: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "dlq.Publish", payload: "req.ID", subject: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1031: entity.PatchNonZero
// ----------------------------------------------------------------------------
RefEntityPatchNonZero: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "entity.PatchNonZero", fields: "req.ID", from: "req.ID", target: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1032: entity.PatchValidated
// ----------------------------------------------------------------------------
RefEntityPatchValidated: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "entity.PatchValidated", from: "req.ID", target: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1033: enum.Validate
// ----------------------------------------------------------------------------
RefEnumValidate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "enum.Validate", allowed: "req.ID", throw: "\"sample\"", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1034: event.Broadcast
// ----------------------------------------------------------------------------
RefEventBroadcast: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "event.Broadcast", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1035: event.Match
// ----------------------------------------------------------------------------
RefEventMatch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "event.Match", event: "\"sample\"", match: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1036: event.Subscribe
// ----------------------------------------------------------------------------
RefEventSubscribe: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "event.Subscribe", match: "req.ID", name: "\"sample\"", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1037: event.Wait
// ----------------------------------------------------------------------------
RefEventWait: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "event.Wait", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1038: exec.Run
// ----------------------------------------------------------------------------
RefExecRun: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "exec.Run", cmd: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1039: field.CopyNonEmpty
// ----------------------------------------------------------------------------
RefFieldCopyNonEmpty: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "field.CopyNonEmpty", from: "req.ID", to: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1040: flow.Block
// ----------------------------------------------------------------------------
RefFlowBlock: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Block", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1041: flow.Catch
// ----------------------------------------------------------------------------
RefFlowCatch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Catch", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1042: flow.Checkpoint
// ----------------------------------------------------------------------------
RefFlowCheckpoint: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Checkpoint", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1043: flow.Cron
// ----------------------------------------------------------------------------
RefFlowCron: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Cron", window: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1044: flow.Delay
// ----------------------------------------------------------------------------
RefFlowDelay: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Delay", duration: "5 * time.Second"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1045: flow.ExplainError
// ----------------------------------------------------------------------------
RefFlowExplainError: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.ExplainError"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1046: flow.Fallback
// ----------------------------------------------------------------------------
RefFlowFallback: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Fallback", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], fallback: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1047: flow.History.Get
// ----------------------------------------------------------------------------
RefFlowHistoryGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.History.Get", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1048: flow.Join
// ----------------------------------------------------------------------------
RefFlowJoin: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Join", branches: {a: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], b: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1049: flow.Parallel
// ----------------------------------------------------------------------------
RefFlowParallel: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Parallel", branches: {a: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], b: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1050: flow.RecordEvent
// ----------------------------------------------------------------------------
RefFlowRecordEvent: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.RecordEvent", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1051: flow.Replay
// ----------------------------------------------------------------------------
RefFlowReplay: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Replay", history: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1052: flow.Resume
// ----------------------------------------------------------------------------
RefFlowResume: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Resume", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1053: flow.Retry
// ----------------------------------------------------------------------------
RefFlowRetry: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Retry", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1054: flow.Schedule
// ----------------------------------------------------------------------------
RefFlowSchedule: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Schedule", at: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1055: flow.SuggestNext
// ----------------------------------------------------------------------------
RefFlowSuggestNext: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.SuggestNext"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1056: flow.Switch
// ----------------------------------------------------------------------------
RefFlowSwitch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Switch", value: "req.Mode", cases: {ok: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], fail: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}, default: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1057: flow.Tag
// ----------------------------------------------------------------------------
RefFlowTag: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Tag", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1058: flow.Timeout
// ----------------------------------------------------------------------------
RefFlowTimeout: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Timeout", duration: "2 * time.Second", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], onTimeout: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1059: flow.Try
// ----------------------------------------------------------------------------
RefFlowTry: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Try", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], catch: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1060: flow.Validate
// ----------------------------------------------------------------------------
RefFlowValidate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "flow.Validate", condition: "true"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1061: fs.ReadFile
// ----------------------------------------------------------------------------
RefFsReadFile: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "fs.ReadFile", output: "result", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1062: fs.Remove
// ----------------------------------------------------------------------------
RefFsRemove: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "fs.Remove", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1063: fs.TempDir
// ----------------------------------------------------------------------------
RefFsTempDir: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "fs.TempDir", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1064: fs.WriteFile
// ----------------------------------------------------------------------------
RefFsWriteFile: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "fs.WriteFile", data: "req.ID", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1065: hash.HMAC
// ----------------------------------------------------------------------------
RefHashHMAC: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "hash.HMAC", input: "req.ID", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1066: hash.Sum
// ----------------------------------------------------------------------------
RefHashSum: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "hash.Sum", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1067: http.Call
// ----------------------------------------------------------------------------
RefHttpCall: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "http.Call", method: "\"sample\"", url: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1068: http.Paginate
// ----------------------------------------------------------------------------
RefHttpPaginate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "http.Paginate", as: "req.ID", cursor_expr: "req.ID", into: "req.ID", url: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1069: http.Request
// ----------------------------------------------------------------------------
RefHttpRequest: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "http.Request", method: "\"sample\"", url: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1070: http.RetryPolicy
// ----------------------------------------------------------------------------
RefHttpRetryPolicy: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "http.RetryPolicy", method: "\"sample\"", url: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1071: idem.Check
// ----------------------------------------------------------------------------
RefIdemCheck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idem.Check", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1072: idem.DeriveKey
// ----------------------------------------------------------------------------
RefIdemDeriveKey: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idem.DeriveKey", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1073: idem.SaveResult
// ----------------------------------------------------------------------------
RefIdemSaveResult: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idem.SaveResult", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1074: idempotency.Check
// ----------------------------------------------------------------------------
RefIdempotencyCheck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idempotency.Check", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1075: idempotency.DeriveKey
// ----------------------------------------------------------------------------
RefIdempotencyDeriveKey: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idempotency.DeriveKey", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1076: idempotency.SaveResult
// ----------------------------------------------------------------------------
RefIdempotencySaveResult: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "idempotency.SaveResult", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1077: json.Marshal
// ----------------------------------------------------------------------------
RefJsonMarshal: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "json.Marshal", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1078: json.Parse
// ----------------------------------------------------------------------------
RefJsonParse: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "json.Parse", input: "req.ID", into: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1079: jsonpath.Get
// ----------------------------------------------------------------------------
RefJsonpathGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "jsonpath.Get", input: "req.ID", output: "result", path: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1080: jsonpath.Set
// ----------------------------------------------------------------------------
RefJsonpathSet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "jsonpath.Set", input: "req.ID", output: "result", path: "\"sample\"", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1081: jwt.Sign
// ----------------------------------------------------------------------------
RefJwtSign: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "jwt.Sign", claims: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1082: jwt.Verify
// ----------------------------------------------------------------------------
RefJwtVerify: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "jwt.Verify", output: "result", token: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1083: list.Append
// ----------------------------------------------------------------------------
RefListAppend: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Append", item: "req.ID", to: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1084: list.Chunk
// ----------------------------------------------------------------------------
RefListChunk: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Chunk", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1085: list.Distinct
// ----------------------------------------------------------------------------
RefListDistinct: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Distinct", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1086: list.Enrich
// ----------------------------------------------------------------------------
RefListEnrich: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Enrich", items: "req.ID", lookupInput: "req.ID", lookupSource: "req.ID", set: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1087: list.Filter
// ----------------------------------------------------------------------------
RefListFilter: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Filter", condition: "true", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1088: list.GroupBy
// ----------------------------------------------------------------------------
RefListGroupBy: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.GroupBy", from: "req.ID", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1089: list.Map
// ----------------------------------------------------------------------------
RefListMap: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Map", expr: "req.ID", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1090: list.Paginate
// ----------------------------------------------------------------------------
RefListPaginate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Paginate", input: "req.ID", limit: "req.ID", offset: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1091: list.Reduce
// ----------------------------------------------------------------------------
RefListReduce: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Reduce", expr: "req.ID", from: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1092: list.Sort
// ----------------------------------------------------------------------------
RefListSort: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.Sort", by: "req.ID", items: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1093: log.Emit
// ----------------------------------------------------------------------------
RefLogEmit: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "log.Emit", message: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1094: mail.Send
// ----------------------------------------------------------------------------
RefMailSend: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "mail.Send", body: "req.ID", subject: "\"sample\"", to: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1095: map.Build
// ----------------------------------------------------------------------------
RefMapBuild: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "map.Build", from: "req.ID", key: "req.ID", output: "result", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1096: math.Op
// ----------------------------------------------------------------------------
RefMathOp: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "math.Op", op: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1097: metric.Emit
// ----------------------------------------------------------------------------
RefMetricEmit: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "metric.Emit", name: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1098: notification.Dispatch
// ----------------------------------------------------------------------------
RefNotificationDispatch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "notification.Dispatch"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1099: notify.Dispatch
// ----------------------------------------------------------------------------
RefNotifyDispatch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "notify.Dispatch"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1100: oauth2.Refresh
// ----------------------------------------------------------------------------
RefOauth2Refresh: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "oauth2.Refresh", output: "result", refreshToken: "req.ID", tokenURL: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1101: oauth2.Token
// ----------------------------------------------------------------------------
RefOauth2Token: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "oauth2.Token", output: "result", tokenURL: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1102: parallel.Run
// ----------------------------------------------------------------------------
RefParallelRun: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "parallel.Run", branches: {a: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], b: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]}},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1103: pdf.Render
// ----------------------------------------------------------------------------
RefPdfRender: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "pdf.Render", data: "req.ID", output: "result", template: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1104: policy.Decide
// ----------------------------------------------------------------------------
RefPolicyDecide: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "policy.Decide", output: "result", policyKey: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1105: policy.Evaluate
// ----------------------------------------------------------------------------
RefPolicyEvaluate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "policy.Evaluate", policyKey: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1106: policy.Require
// ----------------------------------------------------------------------------
RefPolicyRequire: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "policy.Require", policyKey: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1107: query.Decode
// ----------------------------------------------------------------------------
RefQueryDecode: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "query.Decode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1108: query.Encode
// ----------------------------------------------------------------------------
RefQueryEncode: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "query.Encode", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1109: queue.Ack
// ----------------------------------------------------------------------------
RefQueueAck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "queue.Ack", messageID: "req.ID", subject: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1110: queue.Dequeue
// ----------------------------------------------------------------------------
RefQueueDequeue: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "queue.Dequeue", output: "result", subject: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1111: queue.Enqueue
// ----------------------------------------------------------------------------
RefQueueEnqueue: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "queue.Enqueue", payload: "req.ID", subject: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1112: queue.Nack
// ----------------------------------------------------------------------------
RefQueueNack: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "queue.Nack", messageID: "req.ID", subject: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1113: rand.Code
// ----------------------------------------------------------------------------
RefRandCode: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "rand.Code", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1114: rand.Token
// ----------------------------------------------------------------------------
RefRandToken: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "rand.Token", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1115: ratelimit.Check
// ----------------------------------------------------------------------------
RefRatelimitCheck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "ratelimit.Check", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1116: ratelimit.Limit
// ----------------------------------------------------------------------------
RefRatelimitLimit: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "ratelimit.Limit", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1117: rbac.CheckPermission
// ----------------------------------------------------------------------------
RefRbacCheckPermission: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "rbac.CheckPermission", permission: "req.ID", user: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1118: regex.Match
// ----------------------------------------------------------------------------
RefRegexMatch: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "regex.Match", input: "req.ID", output: "result", pattern: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1119: regex.Replace
// ----------------------------------------------------------------------------
RefRegexReplace: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "regex.Replace", input: "req.ID", output: "result", pattern: "req.ID", repl: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1120: repo.GetForUpdate
// ----------------------------------------------------------------------------
RefRepoGetForUpdate: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "repo.GetForUpdate"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1121: repo.Upsert
// ----------------------------------------------------------------------------
RefRepoUpsert: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "repo.Upsert", find: "req.ID", input: "req.ID", output: "result", source: "req.ID", ifNew: [{action: "logic.Check", condition: "true", throw: "\"noop\""}], ifExists: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1122: secret.Get
// ----------------------------------------------------------------------------
RefSecretGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "secret.Get", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1123: session.Get
// ----------------------------------------------------------------------------
RefSessionGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "session.Get", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1124: slo.Budget
// ----------------------------------------------------------------------------
RefSloBudget: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "slo.Budget", duration: "5 * time.Second", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1125: state.Delete
// ----------------------------------------------------------------------------
RefStateDelete: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "state.Delete", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1126: state.Get
// ----------------------------------------------------------------------------
RefStateGet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "state.Get", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1127: state.Set
// ----------------------------------------------------------------------------
RefStateSet: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "state.Set", key: "req.ID", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1128: storage.Delete
// ----------------------------------------------------------------------------
RefStorageDelete: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "storage.Delete", key: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1129: storage.Download
// ----------------------------------------------------------------------------
RefStorageDownload: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "storage.Download", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1130: storage.GetURL
// ----------------------------------------------------------------------------
RefStorageGetURL: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "storage.GetURL", key: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1131: storage.List
// ----------------------------------------------------------------------------
RefStorageList: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "storage.List", output: "result", prefix: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1132: str.Format
// ----------------------------------------------------------------------------
RefStrFormat: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "str.Format", output: "result", template: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1133: str.Normalize
// ----------------------------------------------------------------------------
RefStrNormalize: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "str.Normalize", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1134: time.CheckExpiry
// ----------------------------------------------------------------------------
RefTimeCheckExpiry: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "time.CheckExpiry", throw: "\"sample\"", value: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1135: trace.Span
// ----------------------------------------------------------------------------
RefTraceSpan: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "trace.Span", name: "\"sample\"", do: [{action: "logic.Check", condition: "true", throw: "\"noop\""}]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1136: ulid.New
// ----------------------------------------------------------------------------
RefUlidNew: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "ulid.New", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1137: url.Build
// ----------------------------------------------------------------------------
RefUrlBuild: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "url.Build", base: "\"sample\"", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1138: url.Parse
// ----------------------------------------------------------------------------
RefUrlParse: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "url.Parse", input: "req.ID", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1139: uuid.New
// ----------------------------------------------------------------------------
RefUuidNew: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "uuid.New", output: "result"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1140: webhook.Ack
// ----------------------------------------------------------------------------
RefWebhookAck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "webhook.Ack"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1141: webhook.Send
// ----------------------------------------------------------------------------
RefWebhookSend: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "webhook.Send", payload: "req.ID", url: "\"sample\""},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 1142: webhook.VerifySignature
// ----------------------------------------------------------------------------
RefWebhookVerifySignature: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "webhook.VerifySignature", payload: "req.ID", signature: "req.ID"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}


// ----------------------------------------------------------------------------
// REF EXAMPLE 2000: policy.Check
// ----------------------------------------------------------------------------
RefPolicyCheck: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "policy.Check", policy: "\"CompanyAdminOnly\"", user: "currentUser", companyID: "req.CompanyID", output: "policyOK"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2001: service.Call
// ----------------------------------------------------------------------------
RefServiceCall: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "service.Call", service: "company", method: "GetCompany", args: ["ctx", "req.ID"], output: "company"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2002: time.Now
// ----------------------------------------------------------------------------
RefTimeNow: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "time.Now", output: "now"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2003: time.Format
// ----------------------------------------------------------------------------
RefTimeFormat: schema.#Operation & {
	service: "reference"
	input: { id: string }
	output: { ok: bool }
	flow: [
		{action: "time.Now", output: "now"},
		{action: "time.Format", input: "now", output: "nowRFC3339", format: "time.RFC3339"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2004: list.Len
// ----------------------------------------------------------------------------
RefListLen: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.New", output: "items", type: "[]string"},
		{action: "list.Len", input: "items", output: "count"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2005: list.New
// ----------------------------------------------------------------------------
RefListNew: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "list.New", output: "items", type: "[]string"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2006: map.New
// ----------------------------------------------------------------------------
RefMapNew: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "map.New", output: "bag", type: "map[string]string"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2007: str.Concat
// ----------------------------------------------------------------------------
RefStrConcat: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "str.Concat", parts: ["\"id=\"", "req.ID"], output: "line"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2008: cast.ToString
// ----------------------------------------------------------------------------
RefCastToString: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "cast.ToString", input: "req.ID", output: "s"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2009: num.Add/num.Sub/num.Mul/num.Div
// ----------------------------------------------------------------------------
RefNumOps: schema.#Operation & {
	service: "reference"
	input: {
		a: int
		b: int
	}
	output: { ok: bool }
	flow: [
		{action: "num.Add", a: "req.A", b: "req.B", output: "sum"},
		{action: "num.Sub", a: "req.A", b: "req.B", output: "diff"},
		{action: "num.Mul", a: "req.A", b: "req.B", output: "prod"},
		{action: "num.Div", a: "req.A", b: "req.B", output: "ratio"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2010: flow.While
// ----------------------------------------------------------------------------
RefFlowWhile: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "mapping.Assign", to: "i", declare: true, value: "0"},
		{action: "flow.While", condition: "i < 1", do: [
			{action: "mapping.Assign", to: "i", value: "i + 1"},
		]},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// ----------------------------------------------------------------------------
// REF EXAMPLE 2011: repo.Get
// ----------------------------------------------------------------------------
RefRepoGet: schema.#Operation & {
	service: "reference"
	input:  { id: string }
	output: { ok: bool }
	flow: [
		{action: "repo.Get", source: "Tender", input: "req.ID", output: "item"},
		{action: "mapping.Assign", to: "resp.Ok", value: "item != nil"},
	]
}
