package flowir

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

type Issue struct {
	Code    string
	Message string
	Source  Source
}

type Program struct {
	Services     []normalizer.Service
	Entities     []normalizer.Entity
	Repositories []normalizer.Repository
	Events       []normalizer.EventDef
}

type checker struct {
	services map[string]normalizer.Service
	entities map[string]normalizer.Entity
	repos    map[string]normalizer.Repository
	events   map[string]normalizer.EventDef
}

func Check(program Program) []Issue {
	c := checker{services: map[string]normalizer.Service{}, entities: map[string]normalizer.Entity{}, repos: map[string]normalizer.Repository{}, events: map[string]normalizer.EventDef{}}
	for _, service := range program.Services {
		c.services[strings.ToLower(service.Name)] = service
	}
	for _, entity := range program.Entities {
		c.entities[strings.ToLower(entity.Name)] = entity
	}
	for _, repo := range program.Repositories {
		name := repo.Entity
		if name == "" {
			name = repo.Name
		}
		c.repos[strings.ToLower(name)] = repo
	}
	for _, event := range program.Events {
		c.events[strings.ToLower(event.Name)] = event
	}
	var issues []Issue
	for _, service := range program.Services {
		for _, method := range service.Methods {
			env := map[string]TypeRef{
				"req":  {Kind: TypeDTO, Name: method.Input.Name},
				"resp": {Kind: TypeDTO, Name: method.Output.Name},
				"out":  {Kind: TypeDTO, Name: method.Output.Name},
				"ctx":  {Kind: TypeUnknown, Name: "context.Context"},
			}
			issues = append(issues, c.checkSteps(service, method, method.Flow, env)...)
		}
	}
	return issues
}

func (c checker) checkSteps(service normalizer.Service, method normalizer.Method, steps []normalizer.FlowStep, env map[string]TypeRef) []Issue {
	var issues []Issue
	for _, step := range steps {
		spec, registered := Lookup(step.Action)
		if registered {
			action, err := spec.Decode(step)
			if err != nil {
				issues = append(issues, issue(step, "FLOW_TYPED_DECODE", err.Error()))
			} else {
				issues = append(issues, c.checkAction(service, method, step, action, env)...)
				for _, variable := range action.DeclaredVariables() {
					if typed, ok := action.(RepositoryCall); ok && typed.Method != "" {
						if resolved := c.repositoryOutputType(typed); resolved.Kind != TypeUnknown {
							variable.Type = resolved
						}
					}
					if variable.Type.Kind == TypeUnknown {
						switch typed := action.(type) {
						case MappingAssign:
							variable.Type = c.inferExpression(typed.Value.Source, method, env)
						case MappingMap:
							variable.Type = c.inferExpression(typed.Input.Source, method, env)
						}
					}
					env[variable.Name] = variable.Type
				}
			}
		}
		for _, nested := range nestedStepGroups(step) {
			issues = append(issues, c.checkSteps(service, method, nested, cloneEnv(env))...)
		}
	}
	return issues
}

func (c checker) repositoryOutputType(call RepositoryCall) TypeRef {
	if call.Method == "" {
		return TypeRef{Kind: TypeUnknown}
	}
	repo, ok := c.repos[strings.ToLower(call.Entity)]
	if !ok {
		return TypeRef{Kind: TypeUnknown}
	}
	for _, finder := range repo.Finders {
		if !strings.EqualFold(finder.Name, call.Method) {
			continue
		}
		returns := strings.ToLower(strings.TrimSpace(finder.Returns))
		returnType := strings.TrimSpace(finder.ReturnType)
		if returns == "count" || strings.Contains(strings.ToLower(returnType), "int") || strings.HasPrefix(strings.ToLower(finder.Name), "count") {
			return TypeRef{Kind: TypeInt}
		}
		if returns == "exists" || strings.Contains(strings.ToLower(returnType), "bool") {
			return TypeRef{Kind: TypeBool}
		}
		if strings.HasPrefix(returnType, "[]") || returns == "many" || returns == "list" {
			return TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeEntity, Name: strings.TrimPrefix(strings.TrimPrefix(returnType, "[]"), "domain.")}}
		}
		if returnType != "" {
			name := strings.TrimPrefix(strings.TrimPrefix(returnType, "*"), "domain.")
			return TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: name}}
		}
	}
	return TypeRef{Kind: TypeUnknown}
}

func (c checker) checkAction(service normalizer.Service, method normalizer.Method, step normalizer.FlowStep, action Action, env map[string]TypeRef) []Issue {
	switch typed := action.(type) {
	case LogicCall:
		for _, argument := range typed.Arguments {
			_ = c.inferExpression(argument.Source, method, env)
		}
	case ServiceCall:
		return c.checkServiceCall(service, method, step, typed, env)
	case RepositoryCall:
		return c.checkRepositoryCall(method, step, typed, env)
	case MappingAssign:
		actual := c.inferExpression(typed.Value.Source, method, env)
		expected := c.inferExpression(typed.Target.Source, method, env)
		if actual.Kind != TypeUnknown && expected.Kind != TypeUnknown && !mappingAssignable(actual, expected) {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("cannot assign %s to %s (%s)", displayType(actual), typed.Target.Source, displayType(expected)))}
		}
	case MappingMap:
		if typed.Entity != "" {
			if _, ok := c.entities[strings.ToLower(typed.Entity)]; !ok {
				return []Issue{issue(step, "FLOW_ENTITY_UNKNOWN", fmt.Sprintf("mapping.Map references unknown entity %q", typed.Entity))}
			}
		}
	case LogicCheck:
		conditionType := c.inferExpression(typed.Condition.Source, method, env)
		if conditionType.Kind != TypeUnknown && conditionType.Kind != TypeBool {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("logic.Check condition has type %s, expected bool", displayType(conditionType)))}
		}
	case FlowIf:
		conditionType := c.inferExpression(typed.Condition.Source, method, env)
		if conditionType.Kind != TypeUnknown && conditionType.Kind != TypeBool {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("flow.If condition has type %s, expected bool", displayType(conditionType)))}
		}
	case EventPublish:
		return c.checkEventPublish(method, step, typed, env)
	case CacheGet:
		return c.expectExpression(method, step, "cache.Get key", typed.Key, TypeRef{Kind: TypeString}, env)
	case CacheSet:
		return c.expectExpression(method, step, "cache.Set key", typed.Key, TypeRef{Kind: TypeString}, env)
	case CacheDelete:
		return c.expectExpression(method, step, "cache.Del key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StateGet:
		return c.expectExpression(method, step, "state.Get key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StateSet:
		return c.expectExpression(method, step, "state.Set key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StateDelete:
		return c.expectExpression(method, step, "state.Delete key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StorageUpload:
		issues := c.expectExpression(method, step, "storage.Upload key", typed.Key, TypeRef{Kind: TypeString}, env)
		actualData := c.inferExpression(typed.Data.Source, method, env)
		if actualData.Kind != TypeUnknown && actualData.Kind != TypeString && actualData.Kind != TypeBytes {
			issues = append(issues, issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("storage.Upload data has type %s, expected string or bytes", displayType(actualData))))
		}
		return issues
	case StorageDownload:
		return c.expectExpression(method, step, "storage.Download key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StorageDelete:
		return c.expectExpression(method, step, "storage.Delete key", typed.Key, TypeRef{Kind: TypeString}, env)
	case StorageList:
		return c.expectExpression(method, step, "storage.List prefix", typed.Prefix, TypeRef{Kind: TypeString}, env)
	case StorageGetURL:
		return c.expectExpression(method, step, "storage.GetURL key", typed.Key, TypeRef{Kind: TypeString}, env)
	case RegexMatch:
		return c.expectStrings(method, step, env, "regex.Match", typed.Input, typed.Pattern)
	case RegexReplace:
		return c.expectStrings(method, step, env, "regex.Replace", typed.Input, typed.Pattern, typed.Replacement)
	case StringFormat:
		return c.expectExpression(method, step, "str.Format template", typed.Template, TypeRef{Kind: TypeString}, env)
	case StringStripMarkdown:
		return c.expectStrings(method, step, env, "str.StripMarkdown", typed.Input)
	case StringReplaceAll:
		return c.expectStrings(method, step, env, "str.ReplaceAll", typed.Input, typed.Old, typed.New)
	case StringTrimSpace:
		return c.expectStrings(method, step, env, "str.TrimSpace", typed.Input)
	case StringNormalize:
		return c.expectStrings(method, step, env, "str.Normalize", typed.Input)
	case TimeParse:
		return c.expectExpression(method, step, "time.Parse value", typed.Value, TypeRef{Kind: TypeString}, env)
	case TimeFormat:
		return c.expectExpression(method, step, "time.Format input", typed.Input, TypeRef{Kind: TypeTime}, env)
	case TimeInZone:
		return c.expectExpression(method, step, "time.InZone input", typed.Input, TypeRef{Kind: TypeTime}, env)
	case TimeAdd:
		return c.expectExpression(method, step, "time.Add input", typed.Input, TypeRef{Kind: TypeTime}, env)
	case TimeSub:
		issues := c.expectExpression(method, step, "time.Sub a", typed.A, TypeRef{Kind: TypeTime}, env)
		return append(issues, c.expectExpression(method, step, "time.Sub b", typed.B, TypeRef{Kind: TypeTime}, env)...)
	case TimeDiff:
		issues := c.expectExpression(method, step, "time.Diff from", typed.From, TypeRef{Kind: TypeTime}, env)
		return append(issues, c.expectExpression(method, step, "time.Diff to", typed.To, TypeRef{Kind: TypeTime}, env)...)
	case ListAppend:
		listType := c.inferExpression(typed.Target.Source, method, env)
		itemType := c.inferExpression(typed.Item.Source, method, env)
		if listType.Kind != TypeUnknown && listType.Kind != TypeList {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("list.Append target has type %s, expected list", displayType(listType)))}
		}
		if listType.Kind == TypeList && listType.Elem != nil && itemType.Kind != TypeUnknown && !assignable(itemType, *listType.Elem) {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("list.Append item has type %s, expected %s", displayType(itemType), displayType(*listType.Elem)))}
		}
	case ListLen:
		actual := c.inferExpression(typed.Input.Source, method, env)
		if actual.Kind != TypeUnknown && actual.Kind != TypeList && actual.Kind != TypeString {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("list.Len input has type %s, expected list or string", displayType(actual)))}
		}
	case ListFilter:
		return c.checkListPredicate(method, step, typed.From, typed.Condition, env)
	case ListFind:
		return c.checkListPredicate(method, step, typed.From, typed.Condition, env)
	case ListAny:
		return c.checkListPredicate(method, step, typed.From, typed.Condition, env)
	case ListAll:
		return c.checkListPredicate(method, step, typed.From, typed.Condition, env)
	case ListMap:
		return c.expectList(method, step, typed.From, env)
	case ListReduce:
		return c.expectList(method, step, typed.From, env)
	case ListGroupBy:
		return c.expectList(method, step, typed.From, env)
	case ListDistinct:
		return c.expectList(method, step, typed.From, env)
	case ListChunk:
		issues := c.expectList(method, step, typed.From, env)
		return append(issues, c.expectExpression(method, step, "list.Chunk size", typed.Size, TypeRef{Kind: TypeInt}, env)...)
	case ListSort:
		return c.expectList(method, step, typed.Items, env)
	case ListPaginate:
		issues := c.expectList(method, step, typed.Input, env)
		issues = append(issues, c.expectExpression(method, step, "list.Paginate offset", typed.Offset, TypeRef{Kind: TypeInt}, env)...)
		return append(issues, c.expectExpression(method, step, "list.Paginate limit", typed.Limit, TypeRef{Kind: TypeInt}, env)...)
	case ListAggregate:
		return c.expectList(method, step, typed.Input, env)
	case ValueCoalesce:
		expected := parseTypeHint(typed.Into)
		var issues []Issue
		if expected.Kind != TypeUnknown {
			for _, v := range typed.Values {
				issues = append(issues, c.expectExpression(method, step, "value.Coalesce value", v, expected, env)...)
			}
		}
		return issues
	case MapBuild:
		return c.expectList(method, step, typed.From, env)
	case MapGet:
		return c.expectMap(method, step, typed.Input, env)
	case MapHas:
		return c.expectMap(method, step, typed.Input, env)
	case MapSet:
		return c.expectMap(method, step, typed.Input, env)
	case MapMerge:
		issues := c.expectMap(method, step, typed.Left, env)
		return append(issues, c.expectMap(method, step, typed.Right, env)...)
	case JSONParse:
		return c.expectExpression(method, step, "json.Parse input", typed.Input, TypeRef{Kind: TypeString}, env)
	case ConvertToFloat:
		return c.expectNumber(method, step, typed.Input, env)
	case ConvertToInt:
		return c.expectNumber(method, step, typed.Input, env)
	case Base64Encode:
		actual := c.inferExpression(typed.Input.Source, method, env)
		if actual.Kind != TypeUnknown && actual.Kind != TypeString && actual.Kind != TypeBytes {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("base64.Encode input has type %s, expected string or bytes", displayType(actual)))}
		}
	case Base64Decode:
		return c.expectExpression(method, step, "base64.Decode input", typed.Input, TypeRef{Kind: TypeString}, env)
	case URLParse:
		return c.expectExpression(method, step, "url.Parse input", typed.Input, TypeRef{Kind: TypeString}, env)
	case PathBase:
		return c.expectExpression(method, step, "path.Base input", typed.Input, TypeRef{Kind: TypeString}, env)
	case URLBuild:
		issues := c.expectExpression(method, step, "url.Build base", typed.Base, TypeRef{Kind: TypeString}, env)
		if typed.Path.Source != "" {
			issues = append(issues, c.expectExpression(method, step, "url.Build path", typed.Path, TypeRef{Kind: TypeString}, env)...)
		}
		for _, v := range typed.Segments {
			issues = append(issues, c.expectExpression(method, step, "url.Build segment", v, TypeRef{Kind: TypeString}, env)...)
		}
		for _, v := range typed.Query {
			issues = append(issues, c.expectExpression(method, step, "url.Build query value", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case QueryEncode:
		return c.expectMap(method, step, typed.Input, env)
	case QueryDecode:
		return c.expectExpression(method, step, "query.Decode input", typed.Input, TypeRef{Kind: TypeString}, env)
	case HashSum:
		return c.expectStrings(method, step, env, "hash.Sum", typed.Input, typed.Algorithm)
	case HashHMAC:
		return c.expectStrings(method, step, env, "hash.HMAC", typed.Input, typed.Key, typed.Algorithm)
	case NumberBinary:
		issues := c.expectNumber(method, step, typed.A, env)
		return append(issues, c.expectNumber(method, step, typed.B, env)...)
	case MathOperation:
		issues := c.expectExpression(method, step, "math.Op op", typed.Operation, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.A, typed.B, typed.Value, typed.Min, typed.Max} {
			if v.Source != "" {
				issues = append(issues, c.expectNumber(method, step, v, env)...)
			}
		}
		return issues
	case JSONPathGet:
		return c.expectExpression(method, step, "jsonpath.Get path", typed.Path, TypeRef{Kind: TypeString}, env)
	case JSONPathSet:
		return c.expectExpression(method, step, "jsonpath.Set path", typed.Path, TypeRef{Kind: TypeString}, env)
	case ErrorThrowIf:
		return c.expectExpression(method, step, "errors.ThrowIf condition", typed.Condition, TypeRef{Kind: TypeBool}, env)
	case ErrorWrap:
		return c.expectExpression(method, step, "errors.Wrap err", typed.Error, TypeRef{Kind: TypeError}, env)
	case ErrorMap:
		return c.expectExpression(method, step, "errors.Map input", typed.Input, TypeRef{Kind: TypeError}, env)
	case AuthRequireRole:
		issues := c.expectExpression(method, step, "auth.RequireRole userID", typed.UserID, TypeRef{Kind: TypeString}, env)
		return append(issues, c.expectExpression(method, step, "auth.RequireRole companyID", typed.CompanyID, TypeRef{Kind: TypeString}, env)...)
	case JWTSign:
		issues := c.expectMap(method, step, typed.Claims, env)
		for _, v := range []Expression{typed.Secret, typed.Algorithm, typed.TTL} {
			if v.Source != "" {
				issues = append(issues, c.expectExpression(method, step, "jwt.Sign option", v, TypeRef{Kind: TypeString}, env)...)
			}
		}
		return issues
	case JWTVerify:
		issues := c.expectExpression(method, step, "jwt.Verify token", typed.Token, TypeRef{Kind: TypeString}, env)
		if typed.Secret.Source != "" {
			issues = append(issues, c.expectExpression(method, step, "jwt.Verify secret", typed.Secret, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case TokenGenerate:
		issues := c.expectExpression(method, step, "token.Generate subject", typed.Subject, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.Purpose, typed.Secret, typed.TTL} {
			if v.Source != "" {
				issues = append(issues, c.expectExpression(method, step, "token.Generate option", v, TypeRef{Kind: TypeString}, env)...)
			}
		}
		if typed.Claims.Source != "" {
			issues = append(issues, c.expectMap(method, step, typed.Claims, env)...)
		}
		return issues
	case TokenVerify:
		issues := c.expectExpression(method, step, "token.Verify token", typed.Token, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.Purpose, typed.Secret} {
			if v.Source != "" {
				issues = append(issues, c.expectExpression(method, step, "token.Verify option", v, TypeRef{Kind: TypeString}, env)...)
			}
		}
		return issues
	case CryptoHash:
		return c.expectExpression(method, step, "crypto.Hash input", typed.Input, TypeRef{Kind: TypeString}, env)
	case CryptoCipher:
		issues := c.expectExpression(method, step, typed.ActionName()+" input", typed.Input, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.Key, typed.AAD} {
			if v.Source != "" {
				issues = append(issues, c.expectExpression(method, step, typed.ActionName()+" option", v, TypeRef{Kind: TypeString}, env)...)
			}
		}
		return issues
	case OAuth2Token:
		return c.checkOAuth2(method, step, typed.OAuth2Fields, env)
	case OAuth2Refresh:
		return c.checkOAuth2(method, step, typed.OAuth2Fields, env)
	case WebhookSend:
		issues := c.expectExpression(method, step, "webhook.Send url", typed.URL, TypeRef{Kind: TypeString}, env)
		if typed.Event.Source != "" {
			issues = append(issues, c.expectExpression(method, step, "webhook.Send event", typed.Event, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case WebhookVerifySignature:
		issues := c.expectExpression(method, step, "webhook.VerifySignature signature", typed.Signature, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.Secret, typed.Algorithm, typed.Throw} {
			issues = append(issues, c.expectExpression(method, step, "webhook.VerifySignature argument", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case QueueEnqueue:
		return c.expectExpression(method, step, "queue.Enqueue subject", typed.Subject, TypeRef{Kind: TypeString}, env)
	case QueueDequeue:
		return c.expectExpression(method, step, "queue.Dequeue subject", typed.Subject, TypeRef{Kind: TypeString}, env)
	case QueueAck:
		return c.expectStrings(method, step, env, "queue.Ack", typed.Subject, typed.MessageID)
	case QueueNack:
		return c.expectStrings(method, step, env, "queue.Nack", typed.Subject, typed.MessageID, typed.Reason)
	case DLQPublish:
		issues := c.expectExpression(method, step, "dlq.Publish subject", typed.Subject, TypeRef{Kind: TypeString}, env)
		return append(issues, c.expectExpression(method, step, "dlq.Publish reason", typed.Reason, TypeRef{Kind: TypeString}, env)...)
	case MailSend:
		values := []Expression{typed.To, typed.Subject, typed.Body}
		if typed.HTML.Source != "" {
			values = append(values, typed.HTML)
		}
		return c.expectStrings(method, step, env, "mail.Send", values...)
	case NotifySend:
		values := []Expression{typed.Channel, typed.To}
		for _, v := range []Expression{typed.Template, typed.Text, typed.Subject, typed.HTML} {
			if v.Source != "" {
				values = append(values, v)
			}
		}
		return c.expectStrings(method, step, env, "notify.Send", values...)
	case NotifyEmail:
		values := []Expression{typed.To}
		for _, v := range []Expression{typed.Template, typed.Text, typed.Subject, typed.HTML, typed.Locale} {
			if v.Source != "" {
				values = append(values, v)
			}
		}
		return c.expectStrings(method, step, env, "notify.Email", values...)
	case NotificationDispatch:
		values := []Expression{typed.Event}
		for _, v := range []Expression{typed.Type, typed.UserID, typed.EntityID, typed.Template} {
			if v.Source != "" {
				values = append(values, v)
			}
		}
		return c.expectStrings(method, step, env, typed.ActionName(), values...)
	case ApprovalRequest:
		values := []Expression{typed.ApprovalKey, typed.Title, typed.RequestedBy, typed.Policy}
		if typed.Description.Source != "" {
			values = append(values, typed.Description)
		}
		return c.expectStrings(method, step, env, "approval.Request", values...)
	case ApprovalWait:
		return c.expectExpression(method, step, "approval.Wait approvalId", typed.ApprovalID, TypeRef{Kind: TypeString}, env)
	case ApprovalDecide:
		values := []Expression{typed.ApprovalID, typed.Decision, typed.Actor}
		if typed.Reason.Source != "" {
			values = append(values, typed.Reason)
		}
		return c.expectStrings(method, step, env, "approval.Decide", values...)
	case PolicyCheck:
		if typed.SameCompany && typed.CompanyID.Source == "" {
			return []Issue{issue(step, "FLOW_POLICY_SIGNATURE", "policy.Check resolved policy requires companyID")}
		}
	case PolicyDecisionAction:
		issues := c.expectExpression(method, step, typed.ActionName()+" policyKey", typed.PolicyKey, TypeRef{Kind: TypeString}, env)
		for _, v := range []Expression{typed.Subject, typed.Resource, typed.Operation, typed.Tenant} {
			if v.Source != "" {
				issues = append(issues, c.expectExpression(method, step, typed.ActionName()+" argument", v, TypeRef{Kind: TypeString}, env)...)
			}
		}
		for _, v := range []Expression{typed.Attrs, typed.Context} {
			if v.Source != "" {
				issues = append(issues, c.expectMap(method, step, v, env)...)
			}
		}
		return issues
	case IdempotencyCheck:
		return c.expectExpression(method, step, typed.ActionName()+" key", typed.Key, TypeRef{Kind: TypeString}, env)
	case IdempotencySaveResult:
		return c.expectExpression(method, step, typed.ActionName()+" key", typed.Key, TypeRef{Kind: TypeString}, env)
	case DedupeOnce:
		return c.expectExpression(method, step, "dedupe.Once key", typed.Key, TypeRef{Kind: TypeString}, env)
	case RateLimit:
		return c.expectExpression(method, step, typed.ActionName()+" key", typed.Key, TypeRef{Kind: TypeString}, env)
	case QuotaCheck:
		return c.expectExpression(method, step, "quota.Check key", typed.Key, TypeRef{Kind: TypeString}, env)
	case BudgetCheck:
		return c.expectExpression(method, step, "budget.Check key", typed.Key, TypeRef{Kind: TypeString}, env)
	case BudgetConsume:
		issues := c.expectExpression(method, step, "budget.Consume key", typed.Key, TypeRef{Kind: TypeString}, env)
		return append(issues, c.expectExpression(method, step, "budget.Consume tokens", typed.Tokens, TypeRef{Kind: TypeInt}, env)...)
	case ContextTrim:
		return c.expectExpression(method, step, "context.Trim input", typed.Input, TypeRef{Kind: TypeString}, env)
	case ProfileRequire:
		return c.expectStrings(method, step, env, "profile.Require", typed.Key, typed.Tier)
	case ConcurrencyLimit:
		return c.expectExpression(method, step, "concurrency.Limit key", typed.Key, TypeRef{Kind: TypeString}, env)
	case ConcurrencyRun:
		return c.expectExpression(method, step, "concurrency.Run key", typed.Key, TypeRef{Kind: TypeString}, env)
	case MutexWith:
		return c.expectExpression(method, step, "mutex.With key", typed.Key, TypeRef{Kind: TypeString}, env)
	case CircuitAction:
		return c.expectExpression(method, step, typed.ActionName()+" name", typed.Name, TypeRef{Kind: TypeString}, env)
	case BulkheadAction:
		return c.expectExpression(method, step, typed.ActionName()+" name", typed.Name, TypeRef{Kind: TypeString}, env)
	case LogEmit:
		return c.expectExpression(method, step, "log.Emit message", typed.Message, TypeRef{Kind: TypeString}, env)
	case MetricEmit:
		return c.expectExpression(method, step, "metric.Emit name", typed.Name, TypeRef{Kind: TypeString}, env)
	case TraceSpan:
		return c.expectExpression(method, step, "trace.Span name", typed.Name, TypeRef{Kind: TypeString}, env)
	case HTTPCall:
		issues := c.expectExpression(method, step, "http.Call url", typed.URL, TypeRef{Kind: TypeString}, env)
		for _, v := range typed.Headers {
			issues = append(issues, c.expectExpression(method, step, "http.Call header", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case HTTPRequest:
		issues := c.expectExpression(method, step, "http.Request url", typed.URL, TypeRef{Kind: TypeString}, env)
		for _, v := range typed.Headers {
			issues = append(issues, c.expectExpression(method, step, "http.Request header", v, TypeRef{Kind: TypeString}, env)...)
		}
		for _, v := range typed.Query {
			issues = append(issues, c.expectExpression(method, step, "http.Request query", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case HTTPRetryPolicy:
		issues := c.expectExpression(method, step, "http.RetryPolicy url", typed.URL, TypeRef{Kind: TypeString}, env)
		for _, v := range typed.Headers {
			issues = append(issues, c.expectExpression(method, step, "http.RetryPolicy header", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case HTTPPaginate:
		issues := c.expectExpression(method, step, "http.Paginate url", typed.URL, TypeRef{Kind: TypeString}, env)
		for _, v := range typed.Headers {
			issues = append(issues, c.expectExpression(method, step, "http.Paginate header", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case HTTPSOAP:
		issues := c.expectStrings(method, step, env, "http.SOAP", typed.URL, typed.Namespace, typed.Operation, typed.SOAPAction)
		for _, v := range typed.Headers {
			issues = append(issues, c.expectExpression(method, step, "http.SOAP header", v, TypeRef{Kind: TypeString}, env)...)
		}
		return issues
	case FlowFor:
		return c.expectList(method, step, typed.Each, env)
	case FlowWhile:
		return c.expectExpression(method, step, "flow.While condition", typed.Condition, TypeRef{Kind: TypeBool}, env)
	}
	return nil
}

func (c checker) checkOAuth2(method normalizer.Method, step normalizer.FlowStep, fields OAuth2Fields, env map[string]TypeRef) []Issue {
	var issues []Issue
	for _, v := range []Expression{fields.TokenURL, fields.ClientID, fields.ClientSecret, fields.Scope, fields.Audience, fields.GrantType, fields.Username, fields.Password, fields.Code, fields.RedirectURI, fields.RefreshToken} {
		if v.Source != "" {
			issues = append(issues, c.expectExpression(method, step, step.Action+" argument", v, TypeRef{Kind: TypeString}, env)...)
		}
	}
	return issues
}

func (c checker) expectMap(method normalizer.Method, step normalizer.FlowStep, expression Expression, env map[string]TypeRef) []Issue {
	return c.expectExpression(method, step, step.Action+" input", expression, TypeRef{Kind: TypeMap}, env)
}

func (c checker) expectNumber(method normalizer.Method, step normalizer.FlowStep, expression Expression, env map[string]TypeRef) []Issue {
	actual := c.inferExpression(expression.Source, method, env)
	if actual.Kind == TypeUnknown || actual.Kind == TypeInt || actual.Kind == TypeFloat {
		return nil
	}
	return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("%s input has type %s, expected number", step.Action, displayType(actual)))}
}

func (c checker) expectList(method normalizer.Method, step normalizer.FlowStep, expression Expression, env map[string]TypeRef) []Issue {
	return c.expectExpression(method, step, step.Action+" input", expression, TypeRef{Kind: TypeList}, env)
}

func (c checker) checkListPredicate(method normalizer.Method, step normalizer.FlowStep, from, condition Expression, env map[string]TypeRef) []Issue {
	issues := c.expectList(method, step, from, env)
	actual := c.inferExpression(condition.Source, method, env)
	if actual.Kind != TypeUnknown && actual.Kind != TypeBool {
		issues = append(issues, issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("%s condition has type %s, expected bool", step.Action, displayType(actual))))
	}
	return issues
}

func (c checker) expectStrings(method normalizer.Method, step normalizer.FlowStep, env map[string]TypeRef, label string, expressions ...Expression) []Issue {
	var issues []Issue
	for _, expression := range expressions {
		issues = append(issues, c.expectExpression(method, step, label, expression, TypeRef{Kind: TypeString}, env)...)
	}
	return issues
}

func (c checker) expectExpression(method normalizer.Method, step normalizer.FlowStep, label string, expression Expression, expected TypeRef, env map[string]TypeRef) []Issue {
	actual := c.inferExpression(expression.Source, method, env)
	if actual.Kind == TypeUnknown || assignable(actual, expected) {
		return nil
	}
	return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("%s has type %s, expected %s", label, displayType(actual), displayType(expected)))}
}

func (c checker) checkEventPublish(method normalizer.Method, step normalizer.FlowStep, publish EventPublish, env map[string]TypeRef) []Issue {
	event, ok := c.events[strings.ToLower(publish.Event)]
	if !ok {
		return []Issue{issue(step, "FLOW_EVENT_UNKNOWN", fmt.Sprintf("%s references unknown event %q", publish.ActionName(), publish.Event))}
	}
	fields := make(map[string]normalizer.Field, len(event.Fields))
	for _, field := range event.Fields {
		fields[strings.ToLower(field.Name)] = field
	}
	for name, expression := range publish.PayloadMap {
		field, exists := fields[strings.ToLower(name)]
		if !exists {
			return []Issue{issue(step, "FLOW_EVENT_FIELD_UNKNOWN", fmt.Sprintf("event %s has no field %q", event.Name, name))}
		}
		actual := c.inferExpression(expression.Source, method, env)
		expected := fieldType(field)
		if actual.Kind != TypeUnknown && !assignable(actual, expected) {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("event %s field %s has type %s, got %s", event.Name, field.Name, displayType(expected), displayType(actual)))}
		}
	}
	return nil
}

func (c checker) checkServiceCall(owner normalizer.Service, method normalizer.Method, step normalizer.FlowStep, call ServiceCall, env map[string]TypeRef) []Issue {
	target, ok := c.services[strings.ToLower(call.Service)]
	if !ok {
		return []Issue{issue(step, "FLOW_SERVICE_UNKNOWN", fmt.Sprintf("service.Call references unknown service %q", call.Service))}
	}
	if !strings.EqualFold(owner.Name, target.Name) && !containsFold(owner.Uses, target.Name) {
		return []Issue{issue(step, "FLOW_SERVICE_DEPENDENCY", fmt.Sprintf("service %q must declare uses: %q before service.Call", owner.Name, target.Name))}
	}
	var targetMethod *normalizer.Method
	for index := range target.Methods {
		if strings.EqualFold(target.Methods[index].Name, call.Method) {
			targetMethod = &target.Methods[index]
			break
		}
	}
	if targetMethod == nil {
		return []Issue{issue(step, "FLOW_SERVICE_METHOD_UNKNOWN", fmt.Sprintf("service %q has no method %q", target.Name, call.Method))}
	}
	args := call.Arguments
	if len(args) > 0 && strings.TrimSpace(args[0].Source) == "ctx" {
		args = args[1:]
	}
	if len(args) != 1 {
		return []Issue{issue(step, "FLOW_SERVICE_SIGNATURE", fmt.Sprintf("%s.%s expects one %s request argument after ctx, got %d", target.Name, targetMethod.Name, targetMethod.Input.Name, len(args)))}
	}
	actual := c.inferExpression(args[0].Source, method, env)
	expected := TypeRef{Kind: TypeDTO, Name: targetMethod.Input.Name}
	if actual.Kind != TypeUnknown && !assignable(actual, expected) {
		return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("%s.%s argument has type %s, expected %s", target.Name, targetMethod.Name, displayType(actual), displayType(expected)))}
	}
	return nil
}

func (c checker) checkRepositoryCall(method normalizer.Method, step normalizer.FlowStep, call RepositoryCall, env map[string]TypeRef) []Issue {
	entity, exists := c.entities[strings.ToLower(call.Entity)]
	if !exists {
		return []Issue{issue(step, "FLOW_REPOSITORY_ENTITY_UNKNOWN", fmt.Sprintf("%s references unknown entity %q", call.Operation, call.Entity))}
	}
	if call.Method != "" && !isBuiltinRepoMethod(call.Operation, call.Method) {
		repo, ok := c.repos[strings.ToLower(entity.Name)]
		if !ok || !repoHasFinder(repo, call.Method) {
			return []Issue{issue(step, "FLOW_REPOSITORY_METHOD_UNKNOWN", fmt.Sprintf("repository for %s has no finder %q", entity.Name, call.Method))}
		}
	}
	if call.Input.Source == "" && call.Operation != RepoList && call.Operation != RepoCount {
		return []Issue{issue(step, "FLOW_REPOSITORY_SIGNATURE", fmt.Sprintf("%s requires input", call.Operation))}
	}
	if call.Output == "" {
		switch call.Operation {
		case RepoFind, RepoGet, RepoGetForUpdate, RepoList, RepoQuery, RepoExists, RepoCount, RepoUpsert:
			return []Issue{issue(step, "FLOW_REPOSITORY_SIGNATURE", fmt.Sprintf("%s requires output", call.Operation))}
		}
	}
	if call.Operation == RepoSave && call.Input.Source != "" {
		actual := c.inferExpression(call.Input.Source, method, env)
		expected := TypeRef{Kind: TypeEntity, Name: entity.Name}
		if actual.Kind == TypePointer && actual.Elem != nil {
			actual = *actual.Elem
		}
		if actual.Kind != TypeUnknown && !assignable(actual, expected) {
			return []Issue{issue(step, "FLOW_TYPE_MISMATCH", fmt.Sprintf("repo.Save %s input has type %s, expected %s", entity.Name, displayType(actual), displayType(expected)))}
		}
	}
	return nil
}

func (c checker) inferExpression(source string, method normalizer.Method, env map[string]TypeRef) TypeRef {
	expr, err := parser.ParseExpr(strings.TrimSpace(source))
	if err != nil {
		return TypeRef{Kind: TypeUnknown}
	}
	return c.inferAST(expr, method, env)
}

func (c checker) inferAST(expr ast.Expr, method normalizer.Method, env map[string]TypeRef) TypeRef {
	switch value := expr.(type) {
	case *ast.Ident:
		if value.Name == "true" || value.Name == "false" {
			return TypeRef{Kind: TypeBool}
		}
		if typ, ok := env[value.Name]; ok {
			return typ
		}
	case *ast.BasicLit:
		switch value.Kind {
		case token.STRING, token.CHAR:
			return TypeRef{Kind: TypeString}
		case token.INT:
			return TypeRef{Kind: TypeInt}
		}
	case *ast.SelectorExpr:
		root, ok := value.X.(*ast.Ident)
		if !ok {
			return TypeRef{Kind: TypeUnknown}
		}
		rootType, ok := env[root.Name]
		if !ok {
			return TypeRef{Kind: TypeUnknown}
		}
		if rootType.Kind == TypePointer && rootType.Elem != nil {
			rootType = *rootType.Elem
		}
		var entity normalizer.Entity
		switch rootType.Kind {
		case TypeDTO:
			if rootType.Name == method.Input.Name {
				entity = method.Input
			} else if rootType.Name == method.Output.Name {
				entity = method.Output
			}
		case TypeEntity:
			entity = c.entities[strings.ToLower(rootType.Name)]
		}
		for _, field := range entity.Fields {
			if strings.EqualFold(field.Name, value.Sel.Name) || strings.EqualFold(goFieldName(field.Name), value.Sel.Name) {
				return fieldType(field)
			}
		}
	case *ast.UnaryExpr:
		return c.inferAST(value.X, method, env)
	case *ast.BinaryExpr:
		switch value.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ, token.LAND, token.LOR:
			return TypeRef{Kind: TypeBool}
		}
	}
	return TypeRef{Kind: TypeUnknown}
}

func fieldType(field normalizer.Field) TypeRef {
	name := strings.ToLower(strings.TrimSpace(field.Type))
	if strings.Contains(name, "time.time") || name == "time" || name == "date" || name == "datetime" {
		return TypeRef{Kind: TypeUnknown, Name: field.Type}
	}
	name = strings.TrimPrefix(name, "[]")
	var typ TypeRef
	switch name {
	case "string", "uuid", "email":
		typ = TypeRef{Kind: TypeString}
	case "bool", "boolean":
		typ = TypeRef{Kind: TypeBool}
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		typ = TypeRef{Kind: TypeInt}
	default:
		typeName := strings.TrimPrefix(field.Type, "[]")
		if field.ItemTypeName != "" {
			typeName = field.ItemTypeName
		}
		typ = TypeRef{Kind: TypeEntity, Name: typeName}
	}
	if field.IsList {
		return TypeRef{Kind: TypeList, Elem: &typ}
	}
	return typ
}

func mappingAssignable(actual, expected TypeRef) bool {
	if assignable(actual, expected) {
		return true
	}
	if actual.Kind == TypePointer && actual.Elem != nil {
		return mappingAssignable(*actual.Elem, expected)
	}
	if expected.Kind == TypePointer && expected.Elem != nil {
		return mappingAssignable(actual, *expected.Elem)
	}
	if actual.Kind == TypeList && expected.Kind == TypeList && actual.Elem != nil && expected.Elem != nil {
		return mappingAssignable(*actual.Elem, *expected.Elem)
	}
	structured := func(kind TypeKind) bool { return kind == TypeEntity || kind == TypeDTO }
	return structured(actual.Kind) && structured(expected.Kind)
}

func goFieldName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for index := range parts {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, "")
}

func assignable(actual, expected TypeRef) bool {
	if actual.Kind == TypeUnknown || expected.Kind == TypeUnknown {
		return true
	}
	if actual.Kind != expected.Kind {
		return false
	}
	if expected.Name != "" && !strings.EqualFold(actual.Name, expected.Name) {
		return false
	}
	if expected.Elem != nil {
		return actual.Elem != nil && assignable(*actual.Elem, *expected.Elem)
	}
	return true
}

func displayType(typ TypeRef) string {
	if typ.Kind == TypePointer && typ.Elem != nil {
		return "*" + displayType(*typ.Elem)
	}
	if typ.Kind == TypeList && typ.Elem != nil {
		return "[]" + displayType(*typ.Elem)
	}
	if typ.Name != "" {
		return string(typ.Kind) + "(" + typ.Name + ")"
	}
	return string(typ.Kind)
}

func nestedStepGroups(step normalizer.FlowStep) [][]normalizer.FlowStep {
	var out [][]normalizer.FlowStep
	for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
		if nested, ok := step.Args[key].([]normalizer.FlowStep); ok {
			out = append(out, nested)
		}
	}
	for _, key := range []string{"_cases", "_branches"} {
		if groups, ok := step.Args[key].(map[string][]normalizer.FlowStep); ok {
			for _, nested := range groups {
				out = append(out, nested)
			}
		}
	}
	return out
}

func cloneEnv(env map[string]TypeRef) map[string]TypeRef {
	out := make(map[string]TypeRef, len(env))
	for key, value := range env {
		out[key] = value
	}
	return out
}

func issue(step normalizer.FlowStep, code, message string) Issue {
	return Issue{Code: code, Message: message, Source: SourceOf(step)}
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func repoHasFinder(repo normalizer.Repository, method string) bool {
	for _, finder := range repo.Finders {
		if strings.EqualFold(finder.Name, method) {
			return true
		}
	}
	return false
}

func isBuiltinRepoMethod(operation RepositoryOperation, method string) bool {
	defaults := map[RepositoryOperation][]string{
		RepoSave: {"Save"}, RepoDelete: {"Delete"}, RepoFind: {"FindByID"}, RepoGet: {"GetByID"},
		RepoGetForUpdate: {"GetByIDForUpdate"}, RepoList: {"FindAll"}, RepoExists: {"Exists"}, RepoCount: {"Count"}, RepoUpsert: {"Upsert"},
	}
	return containsFold(defaults[operation], method)
}
