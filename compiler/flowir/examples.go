package flowir

import "github.com/strogmv/ang/angir/normalizer"

// actionExamples holds one real step per action for the action catalog
// (ang actions). They were collected from cue/GOLDEN_EXAMPLES.cue, the flow
// tests and cue/GOLDEN_ACTIONS_REFERENCE.cue; TestActionExamplesDecode checks
// that every example decodes with its action's own Decode, so a wrong one fails
// the build of the catalog rather than misleading its reader.
var actionExamples = map[string]normalizer.FlowStep{
	// emitter/service_flow_codegen_infra_dispatch_test.go:50
	"approval.Decide": {Action: "approval.Decide", Args: map[string]any{"actor": "req.UserID", "approvalId": "approvalID", "decision": "\"approved\"", "status": "approvalStatus"}},
	// GOLDEN_EXAMPLES.cue:793
	"approval.Request": {Action: "approval.Request", Args: map[string]any{"approvalId": "approvalID", "approvalKey": "\"order:approve\"", "approvers": []string{"manager@company.com"}, "payload": "req", "policy": "\"any\"", "requestedBy": "req.UserID", "status": "approvalStatus", "title": "\"Order approval\""}},
	// GOLDEN_EXAMPLES.cue:852
	"approval.Wait": {Action: "approval.Wait", Args: map[string]any{"approvalId": "approvalID", "decision": "decision", "status": "approvalStatus"}},
	// emitter/service_flow_codegen_action_matrix_test.go:467
	"archive.ZipDir": {Action: "archive.ZipDir", Args: map[string]any{"output": "zipBytes", "path": "\"./tmp\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:218
	"audit.Log": {Action: "audit.Log", Args: map[string]any{"actor": "req.ID", "company": "req.ID", "event": "\"sample\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:286
	"auth.CheckRole": {Action: "auth.CheckRole", Args: map[string]any{"companyID": "req.CompanyID", "roles": "[]string{\"admin\"}", "user": "currentUser"}},
	// emitter/service_flow_codegen_action_matrix_test.go:278
	"auth.RequireRole": {Action: "auth.RequireRole", Args: map[string]any{"companyID": "req.CompanyID", "output": "currentUser", "roles": "[]string{\"admin\"}", "userID": "req.UserID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:257
	"base64.Decode": {Action: "base64.Decode", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:270
	"base64.Encode": {Action: "base64.Encode", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:283
	"batch.Run": {Action: "batch.Run", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "as": "batch", "from": "items", "size": 1}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:60
	"budget.Check": {Action: "budget.Check", Args: map[string]any{"key": "req.UserID", "limit": 5000}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:61
	"budget.Consume": {Action: "budget.Consume", Args: map[string]any{"key": "req.UserID", "tokens": "reply.TokensUsed"}},
	// hand-written
	"bulkhead.Acquire": {Action: "bulkhead.Acquire", Args: map[string]any{"max": 10, "name": "\"payment-api\"", "throw": "payment service is busy"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:72
	"bulkhead.Run": {Action: "bulkhead.Run", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}, "max": 12, "name": "\"s3-upload\""}},
	// emitter/service_flow_codegen_hardening_test.go:158
	"cache.Del": {Action: "cache.Del", Args: map[string]any{"key": "\"k\""}},
	// GOLDEN_EXAMPLES.cue:317
	"cache.Get": {Action: "cache.Get", Args: map[string]any{"key": "\"dashboard:\"+req.UserID", "output": "cached"}},
	// GOLDEN_EXAMPLES.cue:322
	"cache.Set": {Action: "cache.Set", Args: map[string]any{"key": "\"dashboard:\"+req.UserID", "ttl": "\"5m\"", "value": "dash"}},
	// emitter/service_flow_codegen_action_matrix_test.go:475
	"cast.ToString": {Action: "cast.ToString", Args: map[string]any{"input": "req.UserID", "output": "userIDString"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:71
	"circuit.Breaker": {Action: "circuit.Breaker", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "http.Call", Args: map[string]any{"method": "GET", "output": "body", "url": "\"https://api.test\""}}}, "name": "\"external-api\"", "openTTL": "30*time.Second", "threshold": 3}},
	// emitter/service_flow_codegen_action_matrix_test.go:225
	"circuit.Check": {Action: "circuit.Check", Args: map[string]any{"name": "\"payments\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:241
	"circuit.RecordFailure": {Action: "circuit.RecordFailure", Args: map[string]any{"name": "\"payments\"", "threshold": 3}},
	// emitter/service_flow_codegen_action_matrix_test.go:233
	"circuit.RecordSuccess": {Action: "circuit.RecordSuccess", Args: map[string]any{"name": "\"payments\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:482
	"claude.Chat": {Action: "claude.Chat", Args: map[string]any{"output": "reply", "user_message": "\"hello\""}},
	// hand-written
	"concurrency.Limit": {Action: "concurrency.Limit", Args: map[string]any{"key": "req.CompanyID", "max": 4}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:69
	"concurrency.Run": {Action: "concurrency.Run", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}, "key": "\"build\"", "max": 8}},
	// emitter/service_flow_codegen_action_matrix_test.go:192
	"config.Get": {Action: "config.Get", Args: map[string]any{"default": "\"dev\"", "key": "\"APP_ENV\"", "output": "env"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:62
	"context.Trim": {Action: "context.Trim", Args: map[string]any{"input": "project.CueContent", "max_bytes": 12000, "output": "trimmedCue"}},
	// emitter/service_flow_codegen_action_matrix_test.go:490
	"convert.ToFloat": {Action: "convert.ToFloat", Args: map[string]any{"input": "req.Count", "output": "countFloat"}},
	// emitter/service_flow_codegen_action_matrix_test.go:497
	"convert.ToInt": {Action: "convert.ToInt", Args: map[string]any{"input": "req.Count", "output": "countInt"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:439
	"crypto.Decrypt": {Action: "crypto.Decrypt", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:452
	"crypto.Encrypt": {Action: "crypto.Encrypt", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// GOLDEN_EXAMPLES.cue:874
	"crypto.Hash": {Action: "crypto.Hash", Args: map[string]any{"algo": "bcrypt", "input": "req.Password", "output": "passwordHash"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:66
	"cue.EmitProject": {Action: "cue.EmitProject", Args: map[string]any{"micro_plan": "microPlanDoc", "output": "projectFiles", "usecases": "usecasesDoc"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:67
	"cue.ValidateProject": {Action: "cue.ValidateProject", Args: map[string]any{"files": "projectFiles", "output": "validation"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:68
	"cue.WriteProjectFiles": {Action: "cue.WriteProjectFiles", Args: map[string]any{"files": "projectFiles", "output": "writeResult", "root": "\"/tmp/project\""}},
	// hand-written
	"db.Delete": {Action: "db.Delete", Args: map[string]any{"input": "req.ID", "source": "Order"}},
	// emitter/service_flow_codegen_action_matrix_test.go:512
	"db.Get": {Action: "db.Get", Args: map[string]any{"input": "req.UserID", "output": "user", "source": "User"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:491
	"db.Insert": {Action: "db.Insert", Args: map[string]any{"input": "req.ID", "source": "req.ID"}},
	// hand-written
	"db.List": {Action: "db.List", Args: map[string]any{"input": "req.UserID", "method": "ListByUser", "output": "orders", "source": "Order"}},
	// hand-written
	"db.Lock": {Action: "db.Lock", Args: map[string]any{"error": "order not found", "input": "req.ID", "output": "order", "source": "Order"}},
	// hand-written
	"db.Query": {Action: "db.Query", Args: map[string]any{"input": "req.UserID", "method": "ListOpenByUser", "output": "orders", "source": "Order"}},
	// hand-written
	"db.SelectForUpdate": {Action: "db.SelectForUpdate", Args: map[string]any{"input": "req.ID", "output": "order", "source": "Order"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:556
	"db.Update": {Action: "db.Update", Args: map[string]any{"input": "req.ID", "source": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:569
	"db.Upsert": {Action: "db.Upsert", Args: map[string]any{"input": "req.ID", "source": "req.ID"}},
	// emitter/service_flow_codegen_action_matrix_test.go:216
	"dedupe.Once": {Action: "dedupe.Once", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"done"}}}}, "key": "\"job:1\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:44
	"dlq.Publish": {Action: "dlq.Publish", Args: map[string]any{"payload": "msg", "reason": "\"decode failed\"", "subject": "\"events.test\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:294
	"entity.PatchNonZero": {Action: "entity.PatchNonZero", Args: map[string]any{"fields": "Name,Email", "from": "req", "target": "user"}},
	// hand-written
	"entity.PatchValidated": {Action: "entity.PatchValidated", Args: map[string]any{"fields": map[string]map[string]string{"Email": {"format": "email", "normalize": "lower"}, "TaxID": {"normalize": "trim", "unique": "FindByTaxID"}}, "from": "req", "source": "Company", "target": "company"}},
	// emitter/service_flow_codegen_action_matrix_test.go:519
	"enum.Validate": {Action: "enum.Validate", Args: map[string]any{"allowed": "draft,published", "throw": "invalid status", "value": "req.Status"}},
	// hand-written
	"errors.Map": {Action: "errors.Map", Args: map[string]any{"cases": map[string]map[string]string{"duplicate key": {"code": "conflict", "message": "order already exists", "status": "409"}, "not found": {"code": "not_found", "message": "order not found", "status": "404"}}, "input": "err", "mode": "contains", "output": "mappedErr"}},
	// emitter/service_flow_codegen_action_matrix_test.go:723
	"errors.New": {Action: "errors.New", Args: map[string]any{"code": "\"NOT_FOUND\"", "message": "\"boom\"", "output": "errObj", "status": "404"}},
	// emitter/service_flow_codegen_action_matrix_test.go:731
	"errors.ThrowIf": {Action: "errors.ThrowIf", Args: map[string]any{"code": "NOT_FOUND", "condition": "user == nil", "status": "404", "throw": "user missing"}},
	// emitter/service_flow_codegen_action_matrix_test.go:755
	"errors.Wrap": {Action: "errors.Wrap", Args: map[string]any{"err": "err", "message": "\"save failed\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:647
	"event.Broadcast": {Action: "event.Broadcast", Args: map[string]any{"name": "\"sample\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:762
	"event.EmitIf": {Action: "event.EmitIf", Args: map[string]any{"condition": "req.Notify", "name": "UserRegistered", "payloadMap": map[string]any{"UserID": "user.ID"}}},
	// emitter/service_flow_codegen_action_matrix_test.go:526
	"event.Match": {Action: "event.Match", Args: map[string]any{"event": "evt", "match": "\"order.created\""}},
	// GOLDEN_EXAMPLES.cue:707
	"event.Outbox": {Action: "event.Outbox", Args: map[string]any{"name": "OrderCreated", "payload": "domain.OrderCreated{OrderID: order.ID}"}},
	// GOLDEN_EXAMPLES.cue:263
	"event.Publish": {Action: "event.Publish", Args: map[string]any{"name": "PaymentSucceeded", "payload": "domain.PaymentSucceeded{Raw: req.Payload}"}},
	// emitter/service_flow_codegen_action_matrix_test.go:534
	"event.Subscribe": {Action: "event.Subscribe", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"seen"}}}}, "match": "\"tenant=acme\"", "name": "\"OrderCreated\""}},
	// emitter/service_flow_codegen_hardening_test.go:251
	"event.Wait": {Action: "event.Wait", Args: map[string]any{"into": "map[string]any", "name": "\"OrderCreated\"", "output": "evt"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:699
	"exec.Run": {Action: "exec.Run", Args: map[string]any{"cmd": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:712
	"exec.Stream": {Action: "exec.Stream", Args: map[string]any{"cmd": "req.ID", "output": "streamOut", "timeout": "120 * time.Second"}},
	// emitter/service_flow_codegen_action_matrix_test.go:301
	"field.CopyNonEmpty": {Action: "field.CopyNonEmpty", Args: map[string]any{"fields": "Name,Email", "from": "req", "to": "user"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:738
	"flow.Block": {Action: "flow.Block", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}},
	// flowir/checker_test.go:26
	"flow.Call": {Action: "flow.Call", Args: map[string]any{"args": map[string]any{"id": "requestID"}, "op": "Profile.Get", "output": "profile"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:751
	"flow.Catch": {Action: "flow.Catch", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:764
	"flow.Checkpoint": {Action: "flow.Checkpoint", Args: map[string]any{"name": "\"sample\""}},
	// GOLDEN_EXAMPLES.cue:766
	"flow.Compensate": {Action: "flow.Compensate", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "repo.Delete", Args: map[string]any{"input": "newOrder.ID", "source": "Order"}}}}},
	// emitter/service_flow_codegen_action_matrix_test.go:98
	"flow.Cron": {Action: "flow.Cron", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"inside"}}}}, "window": "Mon-Fri 09:00-17:00"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2393
	"flow.Defer": {Action: "flow.Defer", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "fs.Remove", Args: map[string]any{"path": "workDir"}}}}},
	// emitter/service_flow_codegen_action_matrix_test.go:21
	"flow.Delay": {Action: "flow.Delay", Args: map[string]any{"duration": "5 * time.Second"}},
	// hand-written
	"flow.ExplainError": {Action: "flow.ExplainError", Args: map[string]any{"hint": "check that the order is still open", "output": "explanation"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:816
	"flow.Fallback": {Action: "flow.Fallback", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "_fallback": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}},
	// GOLDEN_EXAMPLES.cue:198
	"flow.For": {Action: "flow.For", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "mapping.Map", Args: map[string]any{"entity": "Item", "output": "newItem"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "newItem.OrderID", "value": "order.ID"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "newItem.Name", "value": "itemReq.Name"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "newItem.Qty", "value": "itemReq.Qty"}}, {Action: "repo.Save", Args: map[string]any{"input": "newItem", "source": "Item"}}}, "as": "itemReq", "each": "req.Items"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:829
	"flow.History.Get": {Action: "flow.History.Get", Args: map[string]any{"output": "result"}},
	// GOLDEN_EXAMPLES.cue:165
	"flow.If": {Action: "flow.If", Args: map[string]any{"_else": []normalizer.FlowStep{{Action: "repo.List", Args: map[string]any{"input": "req.UserID", "method": "ListByUser", "output": "orders", "source": "Order"}}}, "_then": []normalizer.FlowStep{{Action: "repo.List", Args: map[string]any{"method": "ListAll", "output": "orders", "source": "Order"}}}, "condition": "user.Role == \"admin\""}},
	// hand-written
	"flow.Join": {Action: "flow.Join", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"company": []normalizer.FlowStep{{Action: "repo.Get", Args: map[string]any{"input": "req.CompanyID", "output": "company", "source": "Company"}}}, "orders": []normalizer.FlowStep{{Action: "repo.List", Args: map[string]any{"input": "req.CompanyID", "method": "ListByCompany", "output": "orders", "source": "Order"}}}}}},
	// hand-written
	"flow.Parallel": {Action: "flow.Parallel", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"company": []normalizer.FlowStep{{Action: "repo.Get", Args: map[string]any{"input": "req.CompanyID", "output": "company", "source": "Company"}}}, "orders": []normalizer.FlowStep{{Action: "repo.List", Args: map[string]any{"input": "req.CompanyID", "method": "ListByCompany", "output": "orders", "source": "Order"}}}}}},
	// GOLDEN_EXAMPLES.cue:729
	"flow.Race": {Action: "flow.Race", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"cache": []normalizer.FlowStep{{Action: "repo.Query", Args: map[string]any{"args": []string{"req.CompanyID", "req.ItemID"}, "method": "GetByCompanyAndItem", "output": "cached", "source": "PriceCache"}}, {Action: "logic.Check", Args: map[string]any{"condition": "cached != nil", "throw": "cache miss"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "resp.Source", "value": "\"cache\""}}, {Action: "mapping.Assign", Args: map[string]any{"to": "resp.Price", "value": "cached.Price"}}}, "database": []normalizer.FlowStep{{Action: "repo.Query", Args: map[string]any{"args": []string{"req.CompanyID", "req.ItemID"}, "method": "GetByCompanyAndItem", "output": "live", "source": "Price"}}, {Action: "logic.Check", Args: map[string]any{"condition": "live != nil", "throw": "db miss"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "resp.Source", "value": "\"db\""}}, {Action: "mapping.Assign", Args: map[string]any{"to": "resp.Price", "value": "live.Price"}}}}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:868
	"flow.RecordEvent": {Action: "flow.RecordEvent", Args: map[string]any{"name": "\"sample\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:881
	"flow.Replay": {Action: "flow.Replay", Args: map[string]any{"history": "req.ID"}},
	// emitter/service_flow_codegen_hardening_test.go:225
	"flow.Resume": {Action: "flow.Resume", Args: map[string]any{"into": "map[string]any", "name": "\"draft\"", "output": "draft"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:907
	"flow.Retry": {Action: "flow.Retry", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}},
	// emitter/service_flow_codegen_action_matrix_test.go:45
	"flow.Return": {Action: "flow.Return", Args: map[string]any{"set": "resp.Status", "value": "\"ok\""}},
	// GOLDEN_EXAMPLES.cue:770
	"flow.Rollback": {Action: "flow.Rollback", Args: map[string]any{"error": "fmt.Errorf(\"forced rollback\")"}},
	// GOLDEN_EXAMPLES.cue:764
	"flow.Saga": {Action: "flow.Saga", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "repo.Save", Args: map[string]any{"input": "newOrder", "source": "Order"}}, {Action: "flow.Compensate", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "repo.Delete", Args: map[string]any{"input": "newOrder.ID", "source": "Order"}}}}}, {Action: "flow.If", Args: map[string]any{"_then": []normalizer.FlowStep{{Action: "flow.Rollback", Args: map[string]any{"error": "fmt.Errorf(\"forced rollback\")"}}}, "condition": "req.ForceRollback"}}}}},
	// emitter/service_flow_codegen_action_matrix_test.go:29
	"flow.Schedule": {Action: "flow.Schedule", Args: map[string]any{"at": "deadline"}},
	// hand-written
	"flow.SuggestNext": {Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"\"retry\"", "\"contact support\""}, "output": "nextSteps"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:946
	"flow.Switch": {Action: "flow.Switch", Args: map[string]any{"_cases": map[string][]normalizer.FlowStep{"fail": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "ok": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}, "_default": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "value": "req.Mode"}},
	// emitter/service_flow_codegen_action_matrix_test.go:37
	"flow.Tag": {Action: "flow.Tag", Args: map[string]any{"name": "\"stage\"", "value": "\"validate\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:972
	"flow.Timeout": {Action: "flow.Timeout", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "_onTimeout": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "duration": "2 * time.Second"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:985
	"flow.Try": {Action: "flow.Try", Args: map[string]any{"_catch": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:998
	"flow.Validate": {Action: "flow.Validate", Args: map[string]any{"condition": "true"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2245
	"flow.While": {Action: "flow.While", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "mapping.Assign", Args: map[string]any{"to": "i", "value": "i + 1"}}}, "condition": "i < 1"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1011
	"fs.ReadFile": {Action: "fs.ReadFile", Args: map[string]any{"output": "result", "path": "\"sample\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1024
	"fs.Remove": {Action: "fs.Remove", Args: map[string]any{"path": "\"sample\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1037
	"fs.TempDir": {Action: "fs.TempDir", Args: map[string]any{"output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:543
	"fs.WriteFile": {Action: "fs.WriteFile", Args: map[string]any{"data": "\"hello\"", "path": "\"/tmp/out.txt\""}},
	// GOLDEN_EXAMPLES.cue:107
	"fsm.Transition": {Action: "fsm.Transition", Args: map[string]any{"entity": "order", "to": "confirmed"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1063
	"hash.HMAC": {Action: "hash.HMAC", Args: map[string]any{"input": "req.ID", "key": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1076
	"hash.Sum": {Action: "hash.Sum", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:26
	"http.Call": {Action: "http.Call", Args: map[string]any{"body": "\"{}\"", "method": "POST", "output": "httpBody", "statusVar": "httpStatus", "url": "\"https://example.com\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:167
	"http.Paginate": {Action: "http.Paginate", Args: map[string]any{"as": "page", "cursor_expr": "page.NextCursor", "into": "PagedResponse", "items_expr": "page.Items", "output": "items", "output_type": "[]Item", "url": "\"https://example.com/items\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:139
	"http.Request": {Action: "http.Request", Args: map[string]any{"method": "GET", "output": "body", "statusVar": "status", "url": "\"https://example.com\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:158
	"http.RetryPolicy": {Action: "http.RetryPolicy", Args: map[string]any{"attempts": 3, "body": "\"{}\"", "method": "POST", "output": "body", "url": "\"https://example.com\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:148
	"http.SOAP": {Action: "http.SOAP", Args: map[string]any{"into": "VIESResponse", "namespace": "\"urn:test\"", "operation": "\"CheckVat\"", "output": "soapResp", "request": map[string]any{"countryCode": "\"DE\"", "vatNumber": "req.VAT"}, "statusVar": "soapStatus", "url": "\"https://example.com/soap\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:200
	"idem.Check": {Action: "idem.Check", Args: map[string]any{"key": "idemKey"}},
	// hand-written
	"idem.DeriveKey": {Action: "idem.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}},
	// emitter/service_flow_codegen_action_matrix_test.go:208
	"idem.SaveResult": {Action: "idem.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:56
	"idempotency.Check": {Action: "idempotency.Check", Args: map[string]any{"key": "idemKey"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:55
	"idempotency.DeriveKey": {Action: "idempotency.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:57
	"idempotency.SaveResult": {Action: "idempotency.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:34
	"json.Marshal": {Action: "json.Marshal", Args: map[string]any{"input": "req.Payload", "output": "rawJSON"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:33
	"json.Parse": {Action: "json.Parse", Args: map[string]any{"input": "req.Raw", "into": "map[string]any", "output": "parsed"}},
	// emitter/service_flow_codegen_action_matrix_test.go:770
	"json.Stringify": {Action: "json.Stringify", Args: map[string]any{"input": "resp", "output": "raw"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1245
	"jsonpath.Get": {Action: "jsonpath.Get", Args: map[string]any{"input": "req.ID", "output": "result", "path": "\"sample\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1258
	"jsonpath.Set": {Action: "jsonpath.Set", Args: map[string]any{"input": "req.ID", "output": "result", "path": "\"sample\"", "value": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1271
	"jwt.Sign": {Action: "jwt.Sign", Args: map[string]any{"claims": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1284
	"jwt.Verify": {Action: "jwt.Verify", Args: map[string]any{"output": "result", "token": "req.ID"}},
	// emitter/service_flow_codegen_action_matrix_test.go:836
	"list.All": {Action: "list.All", Args: map[string]any{"as": "item", "condition": "item.Active", "from": "items", "output": "allActive"}},
	// emitter/service_flow_codegen_action_matrix_test.go:827
	"list.Any": {Action: "list.Any", Args: map[string]any{"as": "item", "condition": "item.Active", "from": "items", "output": "hasActive"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1297
	"list.Append": {Action: "list.Append", Args: map[string]any{"item": "req.ID", "to": "\"sample\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:551
	"list.Avg": {Action: "list.Avg", Args: map[string]any{"field": "Price", "input": "items", "output": "avgPrice"}},
	// hand-written
	"list.Chunk": {Action: "list.Chunk", Args: map[string]any{"from": "items", "output": "batches", "size": 100}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1323
	"list.Distinct": {Action: "list.Distinct", Args: map[string]any{"from": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:559
	"list.Enrich": {Action: "list.Enrich", Args: map[string]any{"items": "items", "lookupInput": "_item.UserID", "lookupSource": "User", "set": "AuthorName=Name"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1349
	"list.Filter": {Action: "list.Filter", Args: map[string]any{"condition": "true", "from": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:818
	"list.Find": {Action: "list.Find", Args: map[string]any{"as": "item", "condition": "item.ID == req.ID", "found": "matchFound", "from": "items", "into": "Item", "output": "match"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1362
	"list.GroupBy": {Action: "list.GroupBy", Args: map[string]any{"from": "req.ID", "key": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:567
	"list.Len": {Action: "list.Len", Args: map[string]any{"input": "items", "output": "itemCount"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1375
	"list.Map": {Action: "list.Map", Args: map[string]any{"expr": "req.ID", "from": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:574
	"list.New": {Action: "list.New", Args: map[string]any{"cap": "16", "output": "ids", "type": "[]string"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1388
	"list.Paginate": {Action: "list.Paginate", Args: map[string]any{"input": "req.ID", "limit": "req.ID", "offset": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1401
	"list.Reduce": {Action: "list.Reduce", Args: map[string]any{"expr": "req.ID", "from": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1414
	"list.Sort": {Action: "list.Sort", Args: map[string]any{"by": "req.ID", "items": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2422
	"list.Sum": {Action: "list.Sum", Args: map[string]any{"input": "nums", "output": "total"}},
	// hand-written
	"locale.Resolve": {Action: "locale.Resolve", Args: map[string]any{"default": "\"en\"", "output": "locale", "sources": "req.Locale, user.Locale"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:73
	"log.Emit": {Action: "log.Emit", Args: map[string]any{"level": "\"info\"", "message": "\"created project\""}},
	// GOLDEN_EXAMPLES.cue:660
	"logic.Call": {Action: "logic.Call", Args: map[string]any{"args": []string{"ctx", "req.CompanyID"}, "func": "s.companyService.GetCompany", "output": "company"}},
	// GOLDEN_EXAMPLES.cue:260
	"logic.Check": {Action: "logic.Check", Args: map[string]any{"condition": "verifyStripeSignature(req.Payload, req.Signature)", "throw": "Invalid Stripe signature"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:20
	"mail.Send": {Action: "mail.Send", Args: map[string]any{"body": "\"Body\"", "subject": "\"Hello\"", "to": "req.Email"}},
	// emitter/service_flow_codegen_action_matrix_test.go:361
	"map.Build": {Action: "map.Build", Args: map[string]any{"as": "item", "from": "items", "key": "item.ID", "output": "byID", "value": "item.Name", "valueType": "string"}},
	// emitter/service_flow_codegen_action_matrix_test.go:680
	"map.Get": {Action: "map.Get", Args: map[string]any{"default": "\"unknown\"", "found": "ok", "input": "byID", "into": "string", "key": "req.ID", "output": "name"}},
	// emitter/service_flow_codegen_action_matrix_test.go:691
	"map.Has": {Action: "map.Has", Args: map[string]any{"input": "byID", "key": "req.ID", "output": "exists"}},
	// emitter/service_flow_codegen_action_matrix_test.go:706
	"map.Merge": {Action: "map.Merge", Args: map[string]any{"left": "baseLabels", "output": "mergedLabels", "right": "extraLabels"}},
	// emitter/service_flow_codegen_action_matrix_test.go:354
	"map.New": {Action: "map.New", Args: map[string]any{"output": "labels", "type": "map[string]string"}},
	// emitter/service_flow_codegen_action_matrix_test.go:698
	"map.Set": {Action: "map.Set", Args: map[string]any{"input": "labels", "key": "\"status\"", "output": "nextLabels", "value": "\"ready\""}},
	// GOLDEN_EXAMPLES.cue:561
	"mapping.Assign": {Action: "mapping.Assign", Args: map[string]any{"to": "product.DeletedAt", "value": "time.Now().UTC().Format(time.RFC3339)"}},
	// GOLDEN_EXAMPLES.cue:875
	"mapping.Map": {Action: "mapping.Map", Args: map[string]any{"entity": "UserVault", "output": "creds"}},
	// GOLDEN_EXAMPLES.cue:900
	"math.Expr": {Action: "math.Expr", Args: map[string]any{"declare": true, "expr": "(float64(savings) / float64(req.StartPrice)) * 100", "output": "pct"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1466
	"math.Op": {Action: "math.Op", Args: map[string]any{"op": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:74
	"metric.Emit": {Action: "metric.Emit", Args: map[string]any{"kind": "\"counter\"", "name": "\"project.created\"", "value": "1"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:426
	"model.Resolve": {Action: "model.Resolve", Args: map[string]any{"name": "\"Cheap\"", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:845
	"mutex.With": {Action: "mutex.With", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "log.Emit", Args: map[string]any{"message": "\"inside lock\""}}}, "key": "\"jobs:sync\"", "poll": "25 * time.Millisecond", "wait": "2 * time.Second"}},
	// emitter/service_flow_codegen_action_matrix_test.go:377
	"notification.Dispatch": {Action: "notification.Dispatch", Args: map[string]any{"event": "\"invite.sent\"", "payload": "req", "userID": "req.UserID"}},
	// hand-written
	"notify.Dispatch": {Action: "notify.Dispatch", Args: map[string]any{"entityID": "order.ID", "event": "\"order.shipped\"", "payload": "order", "userID": "order.UserID"}},
	// emitter/service_flow_codegen_event_orchestration_test.go:27
	"notify.Email": {Action: "notify.Email", Args: map[string]any{"output": "notificationID", "text": "\"Hello\"", "to": "req.Email"}},
	// GOLDEN_EXAMPLES.cue:795
	"notify.Send": {Action: "notify.Send", Args: map[string]any{"channel": "\"email\"", "text": "\"Approval timeout: fallback executed\"", "to": "\"ops@company.com\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:603
	"num.Add": {Action: "num.Add", Args: map[string]any{"a": "req.A", "b": "req.B", "output": "sum"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2231
	"num.Div": {Action: "num.Div", Args: map[string]any{"a": "req.A", "b": "req.B", "output": "ratio"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2230
	"num.Mul": {Action: "num.Mul", Args: map[string]any{"a": "req.A", "b": "req.B", "output": "prod"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2229
	"num.Sub": {Action: "num.Sub", Args: map[string]any{"a": "req.A", "b": "req.B", "output": "diff"}},
	// hand-written
	"oauth.Google.Exchange": {Action: "oauth.Google.Exchange", Args: map[string]any{"clientID": "cfg.GoogleClientID", "clientSecret": "cfg.GoogleClientSecret", "code": "req.Code", "output": "googleToken", "redirectURL": "cfg.GoogleRedirectURL"}},
	// hand-written
	"oauth.Google.GetURL": {Action: "oauth.Google.GetURL", Args: map[string]any{"clientID": "cfg.GoogleClientID", "output": "authURL", "redirectURL": "cfg.GoogleRedirectURL", "state": "req.State"}},
	// hand-written
	"oauth.Google.UserInfo": {Action: "oauth.Google.UserInfo", Args: map[string]any{"output": "googleUser", "token": "googleToken"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1518
	"oauth2.Refresh": {Action: "oauth2.Refresh", Args: map[string]any{"output": "result", "refreshToken": "req.ID", "tokenURL": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1531
	"oauth2.Token": {Action: "oauth2.Token", Args: map[string]any{"output": "result", "tokenURL": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2365
	"openai.Chat": {Action: "openai.Chat", Args: map[string]any{"max_rounds": 4, "output": "reply", "output_tool_calls": "toolCalls", "output_usage": "usage", "tool_choice": "\"auto\"", "tools": []string{"LookupPost"}, "user_message": "req.Message"}},
	// emitter/service_flow_codegen_action_matrix_test.go:787
	"openai.Embed": {Action: "openai.Embed", Args: map[string]any{"dimensions": 256, "input": "req.Query", "output": "embedding", "output_usage": "usage"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2379
	"openai.Stream": {Action: "openai.Stream", Args: map[string]any{"output": "reply", "user_message": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1544
	"parallel.Run": {Action: "parallel.Run", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"a": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}, "b": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "\"noop\""}}}}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1962
	"path.Base": {Action: "path.Base", Args: map[string]any{"input": "trimmed", "output": "base"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:36
	"pdf.Render": {Action: "pdf.Render", Args: map[string]any{"data": "req.ReportData", "output": "pdfBytes", "template": "\"t\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:64
	"plan.BuildAutomata": {Action: "plan.BuildAutomata", Args: map[string]any{"input": "usecasesDoc", "output": "automataDoc"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:65
	"plan.BuildMicroPlan": {Action: "plan.BuildMicroPlan", Args: map[string]any{"automata": "automataDoc", "output": "microPlanDoc", "usecases": "usecasesDoc"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:52
	"policy.Check": {Action: "policy.Check", Args: map[string]any{"_policyAllowAdminOverride": true, "_policyResolved": true, "_policyRoles": []string{"owner", "admin"}, "_policySameCompany": true, "companyID": "req.CompanyID", "policy": "\"CompanyAdminOnly\"", "user": "currentUser"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:54
	"policy.Decide": {Action: "policy.Decide", Args: map[string]any{"operation": "\"create\"", "output": "decisionObj", "policyKey": "\"project.create\"", "subject": "req.UserID"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:51
	"policy.Evaluate": {Action: "policy.Evaluate", Args: map[string]any{"decision": "policyDecision", "effects": "policyEffects", "operation": "\"create\"", "output": "policyResult", "policyKey": "\"project.create\"", "reason": "policyReason", "subject": "req.UserID"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:53
	"policy.Require": {Action: "policy.Require", Args: map[string]any{"operation": "\"create\"", "policyKey": "\"project.create\"", "subject": "req.UserID", "throw": "\"forbidden\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:63
	"profile.Require": {Action: "profile.Require", Args: map[string]any{"key": "req.UserID", "tier": "\"ops\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1609
	"query.Decode": {Action: "query.Decode", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1622
	"query.Encode": {Action: "query.Encode", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:42
	"queue.Ack": {Action: "queue.Ack", Args: map[string]any{"messageID": "msgID", "subject": "\"events.test\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:41
	"queue.Dequeue": {Action: "queue.Dequeue", Args: map[string]any{"ackToken": "msgID", "attempts": 3, "backoffMs": 50, "jitterMs": 10, "output": "msg", "subject": "\"events.test\"", "timeout": "2*time.Second"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:38
	"queue.Enqueue": {Action: "queue.Enqueue", Args: map[string]any{"payload": "req.Payload", "subject": "\"events.test\"", "timeout": "2*time.Second"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:43
	"queue.Nack": {Action: "queue.Nack", Args: map[string]any{"messageID": "msgID", "reason": "\"decode failed\"", "subject": "\"events.test\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:59
	"quota.Check": {Action: "quota.Check", Args: map[string]any{"key": "req.UserID", "limit": 100, "window": "\"day\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:28
	"rand.Code": {Action: "rand.Code", Args: map[string]any{"length": 6, "output": "otp"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:29
	"rand.Token": {Action: "rand.Token", Args: map[string]any{"bytes": 16, "output": "token"}},
	// hand-written
	"ratelimit.Check": {Action: "ratelimit.Check", Args: map[string]any{"key": "\"vies:\" + req.CompanyID", "rps": 2, "throw": "verification temporarily rate limited"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:58
	"ratelimit.Limit": {Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID", "rps": 20}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1739
	"rbac.CheckPermission": {Action: "rbac.CheckPermission", Args: map[string]any{"permission": "req.ID", "user": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1752
	"regex.Match": {Action: "regex.Match", Args: map[string]any{"input": "req.ID", "output": "result", "pattern": "req.ID"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1765
	"regex.Replace": {Action: "regex.Replace", Args: map[string]any{"input": "req.ID", "output": "result", "pattern": "req.ID", "repl": "req.ID"}},
	// emitter/service_flow_codegen_action_matrix_test.go:673
	"repo.Count": {Action: "repo.Count", Args: map[string]any{"input": "req.AuthorID", "method": "CountByAuthorID", "output": "count", "source": "Post"}},
	// GOLDEN_EXAMPLES.cue:767
	"repo.Delete": {Action: "repo.Delete", Args: map[string]any{"input": "newOrder.ID", "source": "Order"}},
	// emitter/service_flow_codegen_action_matrix_test.go:662
	"repo.Exists": {Action: "repo.Exists", Args: map[string]any{"input": "req.Email", "method": "ExistsByEmail", "output": "exists", "source": "User"}},
	// GOLDEN_EXAMPLES.cue:291
	"repo.Find": {Action: "repo.Find", Args: map[string]any{"error": "Product not found", "input": "req.ProductID", "output": "product", "source": "Product"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2260
	"repo.Get": {Action: "repo.Get", Args: map[string]any{"input": "req.ID", "output": "item", "source": "Tender"}},
	// hand-written
	"repo.GetForUpdate": {Action: "repo.GetForUpdate", Args: map[string]any{"error": "order not found", "input": "req.ID", "output": "order", "source": "Order"}},
	// GOLDEN_EXAMPLES.cue:429
	"repo.List": {Action: "repo.List", Args: map[string]any{"input": "req", "method": "SearchByQueryPriceStatus", "output": "items", "source": "Product"}},
	// GOLDEN_EXAMPLES.cue:614
	"repo.Query": {Action: "repo.Query", Args: map[string]any{"args": []string{"req.CompanyID", "req.OwnerID"}, "method": "SumByCompanyAndOwner", "output": "stats", "source": "Attachment"}},
	// GOLDEN_EXAMPLES.cue:593
	"repo.Save": {Action: "repo.Save", Args: map[string]any{"input": "domain.AuditLog{Entity: \"Order\", EntityID: order.ID, ActorID: req.UserID, Before: before, After: snapshot(order)}", "source": "AuditLog"}},
	// emitter/service_flow_codegen_action_matrix_test.go:369
	"repo.Upsert": {Action: "repo.Upsert", Args: map[string]any{"_ifNew": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"created"}}}}, "find": "req.ID", "input": "reqUser", "output": "user", "source": "User"}},
	// emitter/service_flow_codegen_action_matrix_test.go:185
	"secret.Get": {Action: "secret.Get", Args: map[string]any{"key": "\"SMTP_PASSWORD\"", "output": "smtpPassword"}},
	// flowir/checker_test.go:171
	"service.Call": {Action: "service.Call", Args: map[string]any{"args": []string{"missingRequest"}, "method": "Get", "output": "profile", "service": "Profile"}},
	// emitter/service_flow_codegen_action_matrix_test.go:610
	"session.Get": {Action: "session.Get", Args: map[string]any{"output": "sessionID"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:76
	"slo.Budget": {Action: "slo.Budget", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}, "duration": "2*time.Second", "name": "\"build\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:392
	"state.Delete": {Action: "state.Delete", Args: map[string]any{"key": "\"draft:1\""}},
	// emitter/service_flow_codegen_hardening_test.go:31
	"state.Get": {Action: "state.Get", Args: map[string]any{"default": "map[string]any{}", "into": "map[string]any", "key": "\"draft:1\"", "output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:384
	"state.Set": {Action: "state.Set", Args: map[string]any{"key": "\"draft:1\"", "ttl": "time.Minute", "value": "req"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:24
	"storage.Delete": {Action: "storage.Delete", Args: map[string]any{"key": "\"path/file\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:22
	"storage.Download": {Action: "storage.Download", Args: map[string]any{"key": "\"path/file\"", "output": "blob"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:23
	"storage.GetURL": {Action: "storage.GetURL", Args: map[string]any{"key": "\"path/file\"", "output": "publicURL"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:25
	"storage.List": {Action: "storage.List", Args: map[string]any{"output": "keys", "prefix": "\"path/\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:21
	"storage.Upload": {Action: "storage.Upload", Args: map[string]any{"data": "req.Payload", "key": "\"path/file\"", "output": "url"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2199
	"str.Concat": {Action: "str.Concat", Args: map[string]any{"output": "line", "parts": []string{"\"id=\"", "req.ID"}}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:105
	"str.Format": {Action: "str.Format", Args: map[string]any{"args": []string{"req.UserID", "req.CompanyID"}, "output": "resp.RedirectURL", "template": "\"u:%s/%s\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1975
	"str.Normalize": {Action: "str.Normalize", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:31
	"str.ReplaceAll": {Action: "str.ReplaceAll", Args: map[string]any{"input": "req.Path", "new": "\"/\"", "old": "\"\\\\\"", "output": "normPath"}},
	// emitter/service_flow_codegen_action_matrix_test.go:617
	"str.StripMarkdown": {Action: "str.StripMarkdown", Args: map[string]any{"input": "req.Content", "output": "plain"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:32
	"str.TrimSpace": {Action: "str.TrimSpace", Args: map[string]any{"input": "req.Name", "output": "trimName"}},
	// GOLDEN_ACTIONS_REFERENCE.cue:1831
	"stream.Emit": {Action: "stream.Emit", Args: map[string]any{"data": "\"{\\\"type\\\":\\\"stage\\\"}\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:778
	"template.Render": {Action: "template.Render", Args: map[string]any{"data": "map[string]any{\"Name\": req.Name}", "output": "body", "template": "\"Hello {{.Name}}\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:325
	"time.Add": {Action: "time.Add", Args: map[string]any{"duration": "15*time.Minute", "input": "expiresAt", "output": "extendedAt"}},
	// emitter/service_flow_codegen_action_matrix_test.go:346
	"time.CheckExpiry": {Action: "time.CheckExpiry", Args: map[string]any{"throw": "\"expired\"", "value": "req.ExpiresAt"}},
	// emitter/service_flow_codegen_action_matrix_test.go:339
	"time.Diff": {Action: "time.Diff", Args: map[string]any{"from": "issuedAt", "output": "ttlMinutes", "to": "expiresAt", "unit": "minutes"}},
	// flowir/time_format_test.go:22
	"time.Format": {Action: "time.Format", Args: map[string]any{"input": "createdAt", "output": "formatted", "zero": "empty"}},
	// hand-written
	"time.InZone": {Action: "time.InZone", Args: map[string]any{"input": "order.CreatedAt", "output": "createdLocal", "timezone": "\"Europe/Berlin\""}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2132
	"time.Now": {Action: "time.Now", Args: map[string]any{"output": "now"}},
	// GOLDEN_EXAMPLES.cue:683
	"time.Parse": {Action: "time.Parse", Args: map[string]any{"output": "_parsedTime", "value": "req.StartsAt"}},
	// emitter/service_flow_codegen_action_matrix_test.go:332
	"time.Sub": {Action: "time.Sub", Args: map[string]any{"a": "expiresAt", "b": "issuedAt", "output": "ttl"}},
	// emitter/service_flow_codegen_action_matrix_test.go:797
	"token.Generate": {Action: "token.Generate", Args: map[string]any{"claims": "map[string]any{\"email\": user.Email}", "output": "token", "purpose": "\"verify_email\"", "secret": "\"secret\"", "subject": "user.ID", "ttl": "\"30m\""}},
	// emitter/service_flow_codegen_action_matrix_test.go:808
	"token.Verify": {Action: "token.Verify", Args: map[string]any{"output": "claims", "purpose": "\"verify_email\"", "secret": "\"secret\"", "token": "req.Token"}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:75
	"trace.Span": {Action: "trace.Span", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}, "name": "\"BuildProject\""}},
	// GOLDEN_EXAMPLES.cue:75
	"tx.Block": {Action: "tx.Block", Args: map[string]any{"_do": []normalizer.FlowStep{{Action: "mapping.Assign", Args: map[string]any{"to": "order.Title", "value": "req.Title"}}, {Action: "mapping.Assign", Args: map[string]any{"to": "order.UpdatedAt", "value": "time.Now().UTC().Format(time.RFC3339)"}}, {Action: "repo.Save", Args: map[string]any{"input": "order", "source": "Order"}}}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2014
	"ulid.New": {Action: "ulid.New", Args: map[string]any{"output": "result"}},
	// emitter/service_flow_codegen_action_matrix_test.go:176
	"url.Build": {Action: "url.Build", Args: map[string]any{"base": "\"https://example.com/base\"", "output": "link", "segments": []string{"\"verify\"", "req.Token"}}},
	// GOLDEN_ACTIONS_REFERENCE.cue:2040
	"url.Parse": {Action: "url.Parse", Args: map[string]any{"input": "req.ID", "output": "result"}},
	// flowir/registry_tree_test.go:10
	"uuid.New": {Action: "uuid.New", Args: map[string]any{"output": "id"}},
	// emitter/service_flow_codegen_action_matrix_test.go:714
	"value.Coalesce": {Action: "value.Coalesce", Args: map[string]any{"into": "string", "output": "displayName", "values": []string{"req.DisplayName", "req.Email", "\"anonymous\""}}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:40
	"webhook.Ack": {Action: "webhook.Ack", Args: map[string]any{"body": "\"accepted\"", "status": 202}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:37
	"webhook.Send": {Action: "webhook.Send", Args: map[string]any{"event": "\"evt\"", "payload": "req.Payload", "url": "\"https://hook.example\""}},
	// emitter/service_flow_codegen_infra_dispatch_test.go:39
	"webhook.VerifySignature": {Action: "webhook.VerifySignature", Args: map[string]any{"output": "sigOK", "payload": "req.Body", "signature": "req.Signature"}},
}
