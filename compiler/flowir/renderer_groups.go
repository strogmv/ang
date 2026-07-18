package flowir

// RendererGroup is target-neutral routing metadata for an action. It belongs
// to Flow IR so schema, validation, documentation and targets share the same
// action catalogue without importing an emitter implementation.
type RendererGroup string

const (
	RendererGroupUnrouted            RendererGroup = "unrouted"
	RendererGroupState               RendererGroup = "state"
	RendererGroupCache               RendererGroup = "cache"
	RendererGroupStorageSimple       RendererGroup = "storage_simple"
	RendererGroupStorageData         RendererGroup = "storage_data"
	RendererGroupMail                RendererGroup = "mail"
	RendererGroupSecretConfig        RendererGroup = "secret_config"
	RendererGroupCore                RendererGroup = "core"
	RendererGroupCall                RendererGroup = "call"
	RendererGroupRepositoryBasic     RendererGroup = "repository_basic"
	RendererGroupRepositoryAdvanced  RendererGroup = "repository_advanced"
	RendererGroupDB                  RendererGroup = "db"
	RendererGroupMapping             RendererGroup = "mapping"
	RendererGroupJSON                RendererGroup = "json"
	RendererGroupHTTPCall            RendererGroup = "http_call"
	RendererGroupHTTPAdvanced        RendererGroup = "http_advanced"
	RendererGroupString              RendererGroup = "string"
	RendererGroupDataTransform       RendererGroup = "data_transform"
	RendererGroupSecurity            RendererGroup = "security"
	RendererGroupConcurrencyDelivery RendererGroup = "concurrency_delivery"
	RendererGroupEventOrchestration  RendererGroup = "event_orchestration"
	RendererGroupPolicy              RendererGroup = "policy"
	RendererGroupReliability         RendererGroup = "reliability"
	RendererGroupOAuthGoogle         RendererGroup = "oauth_google"
	RendererGroupControlResilience   RendererGroup = "control_resilience"
	RendererGroupControlFlowBasic    RendererGroup = "control_flow_basic"
	RendererGroupControlFlowStateful RendererGroup = "control_flow_stateful"
	RendererGroupControlParallel     RendererGroup = "control_parallel"
	RendererGroupSaga                RendererGroup = "saga"
	RendererGroupControlScheduling   RendererGroup = "control_scheduling"
	RendererGroupCollections         RendererGroup = "collections"
	RendererGroupDomainErrors        RendererGroup = "domain_errors"
	RendererGroupDomainAuth          RendererGroup = "domain_auth"
	RendererGroupDomainPrimitives    RendererGroup = "domain_primitives"
	RendererGroupDomainTime          RendererGroup = "domain_time"
	RendererGroupDomainValidation    RendererGroup = "domain_validation"
	RendererGroupListEnrich          RendererGroup = "list_enrich"
	RendererGroupDomainSpecial       RendererGroup = "domain_special"
	RendererGroupRBAC                RendererGroup = "rbac"
	RendererGroupLogicCheck          RendererGroup = "logic_check"
	RendererGroupDomainComputed      RendererGroup = "domain_computed"
	RendererGroupExecFS              RendererGroup = "exec_fs"
	RendererGroupInfrastructure      RendererGroup = "infrastructure"
)

var actionRendererGroups = buildActionRendererGroups()

// RendererGroupFor returns Unrouted for a registered action that deliberately
// has no implementation for a target yet. Targets must surface that condition
// as a diagnostic instead of guessing a renderer.
func RendererGroupFor(action string) RendererGroup {
	if group, ok := actionRendererGroups[action]; ok {
		return group
	}
	return RendererGroupUnrouted
}

func buildActionRendererGroups() map[string]RendererGroup {
	groups := map[RendererGroup][]string{
		RendererGroupState:               {"state.Get", "state.Set", "state.Delete"},
		RendererGroupCache:               {"cache.Get", "cache.Set", "cache.Del"},
		RendererGroupStorageSimple:       {"storage.Delete", "storage.List", "storage.GetURL"},
		RendererGroupStorageData:         {"storage.Upload", "storage.Download"},
		RendererGroupMail:                {"mail.Send"},
		RendererGroupSecretConfig:        {"secret.Get", "config.Get", "model.Resolve"},
		RendererGroupCore:                {"stream.Emit", "session.Get", "event.Publish", "event.EmitIf", "rand.Token", "rand.Code", "str.ReplaceAll", "str.TrimSpace", "cast.ToString", "template.Render"},
		RendererGroupCall:                {"logic.Call", "service.Call", "flow.Call"},
		RendererGroupRepositoryBasic:     {"repo.Exists", "repo.Count", "repo.Get", "repo.Find", "repo.GetForUpdate", "repo.List", "repo.Save", "repo.Delete"},
		RendererGroupRepositoryAdvanced:  {"repo.Query", "repo.Upsert"},
		RendererGroupDB:                  {"db.Get", "db.List", "db.Query", "db.Insert", "db.Update", "db.Upsert", "db.Delete", "db.Lock", "db.SelectForUpdate"},
		RendererGroupMapping:             {"mapping.Assign", "mapping.Map"},
		RendererGroupJSON:                {"json.Parse", "json.Marshal", "json.Stringify"},
		RendererGroupHTTPCall:            {"http.Call"},
		RendererGroupHTTPAdvanced:        {"http.Request", "http.SOAP", "http.RetryPolicy", "http.Paginate"},
		RendererGroupString:              {"str.Format", "str.Concat", "str.StripMarkdown"},
		RendererGroupDataTransform:       {"regex.Match", "regex.Replace", "base64.Encode", "base64.Decode", "url.Parse", "path.Base", "url.Build", "query.Encode", "query.Decode", "hash.Sum", "hash.HMAC", "uuid.New", "ulid.New", "time.Now", "time.Format", "time.InZone", "math.Op", "num.Add", "num.Sub", "num.Mul", "num.Div", "jsonpath.Get", "jsonpath.Set"},
		RendererGroupSecurity:            {"jwt.Sign", "jwt.Verify", "token.Generate", "token.Verify", "crypto.Hash", "oauth2.Token", "oauth2.Refresh", "crypto.Encrypt", "crypto.Decrypt"},
		RendererGroupConcurrencyDelivery: {"parallel.Run", "pdf.Render", "webhook.Send", "webhook.VerifySignature", "webhook.Ack", "queue.Enqueue", "queue.Dequeue", "queue.Ack", "queue.Nack", "dlq.Publish"},
		RendererGroupEventOrchestration:  {"notify.Send", "notify.Email", "approval.Request", "approval.Wait", "approval.Decide", "event.Broadcast", "event.Outbox", "event.Wait", "event.Subscribe", "event.Match"},
		RendererGroupPolicy:              {"policy.Check", "policy.Evaluate", "policy.Require", "policy.Decide"},
		RendererGroupReliability:         {"idem.DeriveKey", "idempotency.DeriveKey", "idem.Check", "idempotency.Check", "idem.SaveResult", "idempotency.SaveResult", "dedupe.Once", "ratelimit.Check", "ratelimit.Limit", "quota.Check", "budget.Check", "budget.Consume", "context.Trim", "profile.Require", "concurrency.Limit", "concurrency.Run", "mutex.With", "circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker", "bulkhead.Acquire", "bulkhead.Run", "log.Emit", "metric.Emit", "trace.Span", "slo.Budget"},
		RendererGroupOAuthGoogle:         {"oauth.Google.GetURL", "oauth.Google.Exchange", "oauth.Google.UserInfo"},
		RendererGroupControlResilience:   {"flow.Try", "flow.Retry", "flow.Timeout", "flow.Fallback"},
		RendererGroupControlFlowBasic:    {"flow.If", "flow.For", "flow.Block", "tx.Block", "flow.Switch", "flow.While"},
		RendererGroupControlFlowStateful: {"flow.Checkpoint", "flow.Resume", "flow.RecordEvent", "flow.History.Get", "flow.Replay", "flow.Validate", "flow.Catch", "flow.Defer", "flow.SuggestNext", "flow.ExplainError"},
		RendererGroupControlParallel:     {"flow.Parallel", "flow.Join", "flow.Race"},
		RendererGroupSaga:                {"flow.Saga", "flow.Compensate", "flow.Rollback"},
		RendererGroupControlScheduling:   {"flow.Delay", "flow.Schedule", "flow.Cron", "flow.Tag", "flow.Return"},
		RendererGroupCollections:         {"value.Coalesce", "list.Filter", "list.Paginate", "list.Append", "list.Sort", "list.Map", "list.Reduce", "list.GroupBy", "list.Distinct", "list.Chunk", "list.Find", "list.Any", "list.All", "list.Sum", "list.Avg", "batch.Run", "str.Normalize"},
		RendererGroupDomainErrors:        {"errors.New", "errors.ThrowIf", "errors.Wrap", "errors.Map"},
		RendererGroupDomainAuth:          {"auth.RequireRole", "auth.CheckRole"},
		RendererGroupDomainPrimitives:    {"list.Len", "list.New", "convert.ToFloat", "convert.ToInt", "map.New", "map.Get", "map.Has", "map.Set", "map.Merge"},
		RendererGroupDomainTime:          {"time.Parse", "time.Add", "time.Sub", "time.Diff", "time.CheckExpiry"},
		RendererGroupDomainValidation:    {"entity.PatchNonZero", "field.CopyNonEmpty", "entity.PatchValidated", "enum.Validate"},
		RendererGroupListEnrich:          {"list.Enrich"},
		RendererGroupDomainSpecial:       {"audit.Log", "fsm.Transition", "notification.Dispatch", "notify.Dispatch"},
		RendererGroupRBAC:                {"rbac.CheckPermission"},
		RendererGroupLogicCheck:          {"logic.Check"},
		RendererGroupDomainComputed:      {"map.Build", "math.Expr"},
		RendererGroupExecFS:              {"exec.Run", "exec.Stream", "fs.TempDir", "fs.WriteFile", "fs.ReadFile", "fs.Remove", "archive.ZipDir"},
		RendererGroupInfrastructure:      {"claude.Chat", "openai.Chat", "openai.Embed", "openai.Stream", "plan.BuildAutomata", "plan.BuildMicroPlan", "cue.EmitProject", "cue.ValidateProject", "cue.WriteProjectFiles", "locale.Resolve"},
	}
	out := make(map[string]RendererGroup, 256)
	for group, actions := range groups {
		for _, action := range actions {
			if previous, exists := out[action]; exists {
				panic("flowir: action " + action + " assigned to renderer groups " + string(previous) + " and " + string(group))
			}
			out[action] = group
		}
	}
	return out
}
