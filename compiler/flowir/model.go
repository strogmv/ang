package flowir

import (
	"go/token"
)

type TypeKind string

const (
	TypeUnknown  TypeKind = "unknown"
	TypeString   TypeKind = "string"
	TypeBool     TypeKind = "bool"
	TypeInt      TypeKind = "int"
	TypeBytes    TypeKind = "bytes"
	TypeEntity   TypeKind = "entity"
	TypeDTO      TypeKind = "dto"
	TypeList     TypeKind = "list"
	TypePointer  TypeKind = "pointer"
	TypeTime     TypeKind = "time"
	TypeDuration TypeKind = "duration"
	TypeFloat    TypeKind = "float"
	TypeMap      TypeKind = "map"
	TypeError    TypeKind = "error"
)

type TypeRef struct {
	Kind TypeKind `json:"kind"`
	Name string   `json:"name,omitempty"`
	Elem *TypeRef `json:"elem,omitempty"`
}

type Expression struct {
	Source string  `json:"source"`
	Type   TypeRef `json:"type"`
}

type Variable struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

type Action interface {
	ActionName() string
	DeclaredVariables() []Variable
}

type FlowTry struct {
	Retries, BackoffMS int
}

func (FlowTry) ActionName() string            { return "flow.Try" }
func (FlowTry) DeclaredVariables() []Variable { return nil }

type FlowRetry struct {
	Attempts, BackoffMS int
}

func (FlowRetry) ActionName() string            { return "flow.Retry" }
func (FlowRetry) DeclaredVariables() []Variable { return nil }

type FlowTimeout struct {
	Duration Expression
}

func (FlowTimeout) ActionName() string            { return "flow.Timeout" }
func (FlowTimeout) DeclaredVariables() []Variable { return nil }

type FlowFallback struct{}

func (FlowFallback) ActionName() string            { return "flow.Fallback" }
func (FlowFallback) DeclaredVariables() []Variable { return nil }

type FlowParallel struct{}
type FlowJoin struct{}
type FlowRace struct{}
type ParallelRun struct {
	MaxConcurrency int
}
type FlowDelay struct{ Duration Expression }
type FlowSchedule struct{ At Expression }
type FlowCron struct {
	Window, Timezone string
}
type FlowSaga struct{}
type FlowCompensate struct{}
type FlowRollback struct{ Error Expression }
type FlowCheckpoint struct {
	Name string
	Data Expression
}
type FlowResume struct {
	Name, Output, Into string
}
type FlowRecordEvent struct {
	Name, Payload Expression
	Output        string
}
type FlowHistoryGet struct {
	Name, Limit Expression
	Output      string
}
type FlowReplay struct {
	History Expression
	Output  string
}
type FlowValidate struct {
	Condition           Expression
	Message, Hint, Code string
	Status              Expression
}
type FlowCatch struct{}
type FlowDefer struct{}
type FlowSuggestNext struct {
	Options []string
	Output  string
}
type FlowExplainError struct {
	Error                 Expression
	Output, Message, Hint string
}
type FlowTag struct{ Name, Value Expression }
type FlowReturn struct{ Set, Value Expression }
type FlowCall struct {
	Operation, Output, IgnoreErrReason string
	Arguments                          map[string]Expression
	IgnoreError                        bool
}

func (FlowParallel) ActionName() string              { return "flow.Parallel" }
func (FlowParallel) DeclaredVariables() []Variable   { return nil }
func (FlowJoin) ActionName() string                  { return "flow.Join" }
func (FlowJoin) DeclaredVariables() []Variable       { return nil }
func (FlowRace) ActionName() string                  { return "flow.Race" }
func (FlowRace) DeclaredVariables() []Variable       { return nil }
func (ParallelRun) ActionName() string               { return "parallel.Run" }
func (ParallelRun) DeclaredVariables() []Variable    { return nil }
func (FlowDelay) ActionName() string                 { return "flow.Delay" }
func (FlowDelay) DeclaredVariables() []Variable      { return nil }
func (FlowSchedule) ActionName() string              { return "flow.Schedule" }
func (FlowSchedule) DeclaredVariables() []Variable   { return nil }
func (FlowCron) ActionName() string                  { return "flow.Cron" }
func (FlowCron) DeclaredVariables() []Variable       { return nil }
func (FlowSaga) ActionName() string                  { return "flow.Saga" }
func (FlowSaga) DeclaredVariables() []Variable       { return nil }
func (FlowCompensate) ActionName() string            { return "flow.Compensate" }
func (FlowCompensate) DeclaredVariables() []Variable { return nil }
func (FlowRollback) ActionName() string              { return "flow.Rollback" }
func (FlowRollback) DeclaredVariables() []Variable   { return nil }
func (FlowCheckpoint) ActionName() string            { return "flow.Checkpoint" }
func (FlowCheckpoint) DeclaredVariables() []Variable { return nil }
func (FlowResume) ActionName() string                { return "flow.Resume" }
func (a FlowResume) DeclaredVariables() []Variable {
	return outputVariable(a.Output, parseTypeHint(a.Into))
}
func (FlowRecordEvent) ActionName() string { return "flow.RecordEvent" }
func (a FlowRecordEvent) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}
func (FlowHistoryGet) ActionName() string { return "flow.History.Get" }
func (a FlowHistoryGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}
func (FlowReplay) ActionName() string { return "flow.Replay" }
func (a FlowReplay) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}
func (FlowValidate) ActionName() string            { return "flow.Validate" }
func (FlowValidate) DeclaredVariables() []Variable { return nil }
func (FlowCatch) ActionName() string               { return "flow.Catch" }
func (FlowCatch) DeclaredVariables() []Variable    { return nil }
func (FlowDefer) ActionName() string               { return "flow.Defer" }
func (FlowDefer) DeclaredVariables() []Variable    { return nil }
func (FlowSuggestNext) ActionName() string         { return "flow.SuggestNext" }
func (a FlowSuggestNext) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeString}})
}
func (FlowExplainError) ActionName() string { return "flow.ExplainError" }
func (a FlowExplainError) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (FlowTag) ActionName() string               { return "flow.Tag" }
func (FlowTag) DeclaredVariables() []Variable    { return nil }
func (FlowReturn) ActionName() string            { return "flow.Return" }
func (FlowReturn) DeclaredVariables() []Variable { return nil }
func (FlowCall) ActionName() string              { return "flow.Call" }
func (a FlowCall) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeDTO, Name: a.Operation + "Response"})
}

type Source struct {
	File    string
	Line    int
	Column  int
	CUEPath string
}

type CallOptions struct {
	Output          string
	IgnoreError     bool
	IgnoreErrReason string
}

type LogicCall struct {
	Function  Expression
	Arguments []Expression
	CallOptions
}

func (LogicCall) ActionName() string { return "logic.Call" }
func (a LogicCall) DeclaredVariables() []Variable {
	if a.Output == "" {
		return nil
	}
	return []Variable{{Name: a.Output, Type: TypeRef{Kind: TypeUnknown}}}
}

type ServiceCall struct {
	Service   string
	Method    string
	Arguments []Expression
	CallOptions
}

func (ServiceCall) ActionName() string { return "service.Call" }
func (a ServiceCall) DeclaredVariables() []Variable {
	if a.Output == "" {
		return nil
	}
	return []Variable{{Name: a.Output, Type: TypeRef{Kind: TypeDTO, Name: a.Service + "." + a.Method + "Response"}}}
}

type RepositoryOperation string

const (
	RepoSave         RepositoryOperation = "repo.Save"
	RepoFind         RepositoryOperation = "repo.Find"
	RepoDelete       RepositoryOperation = "repo.Delete"
	RepoList         RepositoryOperation = "repo.List"
	RepoQuery        RepositoryOperation = "repo.Query"
	RepoGet          RepositoryOperation = "repo.Get"
	RepoGetForUpdate RepositoryOperation = "repo.GetForUpdate"
	RepoExists       RepositoryOperation = "repo.Exists"
	RepoCount        RepositoryOperation = "repo.Count"
	RepoUpsert       RepositoryOperation = "repo.Upsert"
)

type RepositoryCall struct {
	Operation RepositoryOperation
	Entity    string
	Input     Expression
	Output    string
	Method    string
	Error     string
	Required  bool
	Arguments []Expression
	List      bool
	Find      Expression
}

type MappingAssign struct {
	Target  Expression
	Value   Expression
	Declare bool
	Type    TypeRef
}

func (MappingAssign) ActionName() string { return "mapping.Assign" }
func (a MappingAssign) DeclaredVariables() []Variable {
	if !a.Declare || !token.IsIdentifier(a.Target.Source) {
		return nil
	}
	return []Variable{{Name: a.Target.Source, Type: a.Type}}
}

type MappingMap struct {
	Input  Expression
	Output string
	Entity string
}

func (MappingMap) ActionName() string { return "mapping.Map" }
func (a MappingMap) DeclaredVariables() []Variable {
	typ := TypeRef{Kind: TypeUnknown}
	if a.Entity != "" {
		typ = TypeRef{Kind: TypeEntity, Name: a.Entity}
	}
	return outputVariable(a.Output, typ)
}

type LogicCheck struct {
	Condition Expression
	Throw     string
	Params    []Expression
}

func (LogicCheck) ActionName() string            { return "logic.Check" }
func (LogicCheck) DeclaredVariables() []Variable { return nil }

type FlowIf struct {
	Condition Expression
}

func (FlowIf) ActionName() string            { return "flow.If" }
func (FlowIf) DeclaredVariables() []Variable { return nil }

type FlowBlock struct {
	Transactional bool
}

func (a FlowBlock) ActionName() string {
	if a.Transactional {
		return "tx.Block"
	}
	return "flow.Block"
}
func (FlowBlock) DeclaredVariables() []Variable { return nil }

type EventPublish struct {
	Event      string
	Payload    Expression
	PayloadMap map[string]Expression
	Broadcast  bool
}

func (a EventPublish) ActionName() string {
	if a.Broadcast {
		return "event.Broadcast"
	}
	return "event.Publish"
}
func (EventPublish) DeclaredVariables() []Variable { return nil }

type CacheGet struct {
	Key      Expression
	Output   string
	Optional bool
}

func (CacheGet) ActionName() string { return "cache.Get" }
func (a CacheGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type CacheSet struct{ Key, Value, TTL Expression }

func (CacheSet) ActionName() string            { return "cache.Set" }
func (CacheSet) DeclaredVariables() []Variable { return nil }

type CacheDelete struct{ Key Expression }

func (CacheDelete) ActionName() string            { return "cache.Del" }
func (CacheDelete) DeclaredVariables() []Variable { return nil }

type StateGet struct {
	Key, Default Expression
	Output       string
	Into         string
}

func (StateGet) ActionName() string { return "state.Get" }
func (a StateGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

type StateSet struct{ Key, Value, TTL Expression }

func (StateSet) ActionName() string            { return "state.Set" }
func (StateSet) DeclaredVariables() []Variable { return nil }

type StateDelete struct{ Key Expression }

func (StateDelete) ActionName() string            { return "state.Delete" }
func (StateDelete) DeclaredVariables() []Variable { return nil }

type StorageUpload struct {
	Key, Data, ContentType Expression
	Output                 string
}

func (StorageUpload) ActionName() string { return "storage.Upload" }
func (a StorageUpload) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StorageDownload struct {
	Key    Expression
	Output string
}

func (StorageDownload) ActionName() string { return "storage.Download" }
func (a StorageDownload) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBytes})
}

type StorageDelete struct{ Key Expression }

func (StorageDelete) ActionName() string            { return "storage.Delete" }
func (StorageDelete) DeclaredVariables() []Variable { return nil }

type StorageList struct {
	Prefix Expression
	Output string
}

func (StorageList) ActionName() string { return "storage.List" }
func (a StorageList) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeString}})
}

type StorageGetURL struct {
	Key    Expression
	Output string
}

type RegexMatch struct {
	Input, Pattern Expression
	Output         string
}

func (RegexMatch) ActionName() string { return "regex.Match" }
func (a RegexMatch) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type RegexReplace struct {
	Input, Pattern, Replacement Expression
	Output                      string
}

func (RegexReplace) ActionName() string { return "regex.Replace" }
func (a RegexReplace) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type UUIDNew struct{ Output string }

func (UUIDNew) ActionName() string { return "uuid.New" }
func (a UUIDNew) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type ULIDNew struct{ Output string }

func (ULIDNew) ActionName() string { return "ulid.New" }
func (a ULIDNew) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type RandomCode struct {
	Length int
	Output string
}

func (RandomCode) ActionName() string { return "rand.Code" }
func (a RandomCode) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type RandomToken struct {
	Bytes  int
	Output string
}

func (RandomToken) ActionName() string { return "rand.Token" }
func (a RandomToken) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringFormat struct {
	Template  Expression
	Arguments []Expression
	Output    string
}

func (StringFormat) ActionName() string { return "str.Format" }
func (a StringFormat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringConcat struct {
	Parts     []Expression
	Separator Expression
	Output    string
}

func (StringConcat) ActionName() string { return "str.Concat" }
func (a StringConcat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringStripMarkdown struct {
	Input  Expression
	Output string
}

func (StringStripMarkdown) ActionName() string { return "str.StripMarkdown" }
func (a StringStripMarkdown) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringReplaceAll struct {
	Input, Old, New Expression
	Output          string
}

func (StringReplaceAll) ActionName() string { return "str.ReplaceAll" }
func (a StringReplaceAll) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringTrimSpace struct {
	Input  Expression
	Output string
}

func (StringTrimSpace) ActionName() string { return "str.TrimSpace" }
func (a StringTrimSpace) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type StringNormalize struct {
	Input        Expression
	Mode, Output string
}

func (StringNormalize) ActionName() string { return "str.Normalize" }
func (a StringNormalize) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type TimeNow struct{ Output, Format string }

func (TimeNow) ActionName() string { return "time.Now" }
func (a TimeNow) DeclaredVariables() []Variable {
	k := TypeTime
	if a.Format != "" {
		k = TypeString
	}
	return outputVariable(a.Output, TypeRef{Kind: k})
}

type TimeParse struct {
	Value          Expression
	Format, Output string
}

func (TimeParse) ActionName() string { return "time.Parse" }
func (a TimeParse) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeTime})
}

type TimeFormat struct {
	Input                    Expression
	Format, Timezone, Output string
}

func (TimeFormat) ActionName() string { return "time.Format" }
func (a TimeFormat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type TimeInZone struct {
	Input            Expression
	Timezone, Output string
}

func (TimeInZone) ActionName() string { return "time.InZone" }
func (a TimeInZone) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeTime})
}

type TimeAdd struct {
	Input, Duration Expression
	Output          string
}

func (TimeAdd) ActionName() string { return "time.Add" }
func (a TimeAdd) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeTime})
}

type TimeSub struct {
	A, B   Expression
	Output string
}

func (TimeSub) ActionName() string { return "time.Sub" }
func (a TimeSub) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeDuration})
}

type TimeDiff struct {
	From, To     Expression
	Unit, Output string
}

func (TimeDiff) ActionName() string { return "time.Diff" }
func (a TimeDiff) DeclaredVariables() []Variable {
	kind := TypeUnknown
	if a.Unit == "" || a.Unit == "duration" {
		kind = TypeDuration
	}
	return outputVariable(a.Output, TypeRef{Kind: kind})
}

type TimeCheckExpiry struct {
	Value         Expression
	Throw, MustBe string
}

func (TimeCheckExpiry) ActionName() string            { return "time.CheckExpiry" }
func (TimeCheckExpiry) DeclaredVariables() []Variable { return nil }

type ListAppend struct{ Target, Item Expression }

func (ListAppend) ActionName() string            { return "list.Append" }
func (ListAppend) DeclaredVariables() []Variable { return nil }

type ListLen struct {
	Input  Expression
	Output string
}

func (ListLen) ActionName() string { return "list.Len" }
func (a ListLen) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeInt})
}

type ListNew struct {
	Output   string
	Type     TypeRef
	GoType   string
	Capacity Expression
}

func (ListNew) ActionName() string              { return "list.New" }
func (a ListNew) DeclaredVariables() []Variable { return outputVariable(a.Output, a.Type) }

type ListFilter struct {
	From      Expression
	As        string
	Condition Expression
	Output    string
}

func (ListFilter) ActionName() string { return "list.Filter" }
func (a ListFilter) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}

type ListPaginate struct {
	Input, Offset, Limit Expression
	DefaultLimit         int
	Output, Total        string
}

func (ListPaginate) ActionName() string { return "list.Paginate" }
func (a ListPaginate) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}

type ListFind struct {
	From                Expression
	As                  string
	Condition           Expression
	Output, Into, Found string
}

func (ListFind) ActionName() string { return "list.Find" }
func (a ListFind) DeclaredVariables() []Variable {
	v := outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
	return append(v, outputVariable(a.Found, TypeRef{Kind: TypeBool})...)
}

type ListAny struct {
	From      Expression
	As        string
	Condition Expression
	Output    string
}

func (ListAny) ActionName() string { return "list.Any" }
func (a ListAny) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type ListAll struct {
	From      Expression
	As        string
	Condition Expression
	Output    string
}

func (ListAll) ActionName() string { return "list.All" }
func (a ListAll) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type ListMap struct {
	From   Expression
	As     string
	Value  Expression
	Output string
}

func (ListMap) ActionName() string { return "list.Map" }
func (a ListMap) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}

type ListReduce struct {
	From           Expression
	As             string
	Value, Initial Expression
	Output         string
}

func (ListReduce) ActionName() string { return "list.Reduce" }
func (a ListReduce) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

type ListGroupBy struct {
	From   Expression
	As     string
	Key    Expression
	Output string
}

func (ListGroupBy) ActionName() string { return "list.GroupBy" }
func (a ListGroupBy) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

type ListDistinct struct {
	From   Expression
	As     string
	Key    Expression
	Output string
}

func (ListDistinct) ActionName() string { return "list.Distinct" }
func (a ListDistinct) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList})
}

type ListChunk struct {
	From, Size Expression
	Output     string
}

type BatchRun struct {
	From, Size Expression
	As         string
}

type ExecCommand struct {
	Alias                      string
	Command, Stdin, Timeout    Expression
	Arguments                  []Expression
	TimeoutMS                  int
	Output, ExitCodeVar, Throw string
	FailOnError                bool
}
type FSTempDir struct {
	Pattern Expression
	Output  string
}
type FSWriteFile struct{ Path, Data Expression }
type FSReadFile struct {
	Path     Expression
	Output   string
	Optional bool
}
type FSRemove struct{ Path Expression }
type ArchiveZipDir struct {
	Path   Expression
	Output string
}
type ClaudeChat struct {
	System, SystemContext, UserMessage, History, Model, Locale, Timezone Expression
	Output                                                               string
	MaxTokens                                                            int
}
type OpenAIChat struct {
	System, SystemContext, UserMessage, History, Model, ToolChoice Expression
	ResponseJSONSchema, ResponseJSONName                           Expression
	Output, OutputUsage, OutputToolCalls, OutputJSON               string
	Tools                                                          []string
	MaxTokens, MaxRounds                                           int
	ResponseJSONStrict                                             bool
}
type OpenAIEmbed struct {
	Input, Model        Expression
	Output, OutputUsage string
	Dimensions          int
}
type OpenAIStream struct {
	System, SystemContext, UserMessage, History, Model Expression
	Output                                             string
	MaxTokens                                          int
}
type PlanBuildAutomata struct {
	Input  Expression
	Output string
}
type PlanBuildMicroPlan struct {
	Usecases, Automata Expression
	Output             string
}
type CueEmitProject struct {
	Usecases, MicroPlan, Layout Expression
	Output                      string
}
type CueValidateProject struct {
	Files, Binary Expression
	Output        string
}
type CueWriteProjectFiles struct {
	Root, Files, Mode Expression
	Prefixes          []string
	Output            string
}
type AuditLog struct{ Actor, Company, Event Expression }
type RBACCheckPermission struct {
	User, Permission, Status Expression
	Output, Throw, Code      string
}
type SecretGet struct {
	Key, Default Expression
	Output       string
}
type ConfigGet struct {
	Key, Default Expression
	Output       string
}
type ModelResolve struct {
	Name, Default Expression
	Output        string
}
type StreamEmit struct{ Data Expression }
type LocaleResolve struct {
	Sources string
	Default Expression
	Output  string
}
type TemplateRender struct {
	Template, Data Expression
	Output         string
}
type PDFRender struct {
	Template, Data Expression
	Output         string
}
type SessionGet struct{ Output string }
type DBFields struct {
	Source, Method, Output, Error string
	Input                         Expression
}
type DBGet struct{ DBFields }
type DBList struct{ DBFields }
type DBQuery struct{ DBFields }
type DBInsert struct{ DBFields }
type DBUpdate struct{ DBFields }
type DBUpsert struct{ DBFields }
type DBDelete struct{ DBFields }
type DBLock struct{ DBFields }
type DBSelectForUpdate struct{ DBFields }
type EventEmitIf struct {
	Condition  Expression
	Event      string
	Payload    Expression
	PayloadMap map[string]Expression
}
type EventOutbox struct{ Name, Payload, ID Expression }
type EventWait struct {
	Name, Timeout, Match Expression
	Output, Into         string
}
type EventSubscribe struct {
	Name, Match Expression
}
type EventMatch struct {
	Event, Match Expression
	Throw        string
}
type EntityPatchNonZero struct {
	Target, From Expression
	Fields       []string
}
type FieldCopyNonEmpty struct {
	From, To Expression
	Fields   []string
}
type PatchFieldRule struct{ Normalize, Format, Unique string }
type EntityPatchValidated struct {
	Target, From Expression
	Source       string
	Fields       map[string]PatchFieldRule
}
type EnumValidate struct {
	Value   Expression
	Allowed []string
	Throw   string
}
type FSMTransition struct {
	Entity Expression
	To     string
}
type EnrichField struct{ Target, Source string }
type ListEnrich struct {
	Items, LookupInput Expression
	As, LookupSource   string
	Fields             []EnrichField
}
type OAuthGoogleGetURL struct {
	ClientID, RedirectURL, State, Scopes Expression
	Output                               string
}
type OAuthGoogleExchange struct {
	ClientID, ClientSecret, RedirectURL, Code, Scopes Expression
	Output                                            string
}
type OAuthGoogleUserInfo struct {
	Token  Expression
	Output string
}

func (FSMTransition) ActionName() string            { return "fsm.Transition" }
func (FSMTransition) DeclaredVariables() []Variable { return nil }
func (ListEnrich) ActionName() string               { return "list.Enrich" }
func (ListEnrich) DeclaredVariables() []Variable    { return nil }
func (OAuthGoogleGetURL) ActionName() string        { return "oauth.Google.GetURL" }
func (a OAuthGoogleGetURL) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (OAuthGoogleExchange) ActionName() string { return "oauth.Google.Exchange" }
func (a OAuthGoogleExchange) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeUnknown, Name: "oauth2.Token"}})
}
func (OAuthGoogleUserInfo) ActionName() string { return "oauth.Google.UserInfo" }
func (a OAuthGoogleUserInfo) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown, Name: "GoogleUserInfo"})
}

func (EntityPatchNonZero) ActionName() string              { return "entity.PatchNonZero" }
func (EntityPatchNonZero) DeclaredVariables() []Variable   { return nil }
func (FieldCopyNonEmpty) ActionName() string               { return "field.CopyNonEmpty" }
func (FieldCopyNonEmpty) DeclaredVariables() []Variable    { return nil }
func (EntityPatchValidated) ActionName() string            { return "entity.PatchValidated" }
func (EntityPatchValidated) DeclaredVariables() []Variable { return nil }
func (EnumValidate) ActionName() string                    { return "enum.Validate" }
func (EnumValidate) DeclaredVariables() []Variable         { return nil }

func (EventEmitIf) ActionName() string            { return "event.EmitIf" }
func (EventEmitIf) DeclaredVariables() []Variable { return nil }
func (EventOutbox) ActionName() string            { return "event.Outbox" }
func (EventOutbox) DeclaredVariables() []Variable { return nil }
func (EventWait) ActionName() string              { return "event.Wait" }
func (a EventWait) DeclaredVariables() []Variable {
	return outputVariable(a.Output, parseTypeHint(a.Into))
}
func (EventSubscribe) ActionName() string            { return "event.Subscribe" }
func (EventSubscribe) DeclaredVariables() []Variable { return nil }
func (EventMatch) ActionName() string                { return "event.Match" }
func (EventMatch) DeclaredVariables() []Variable     { return nil }

func (AuditLog) ActionName() string            { return "audit.Log" }
func (AuditLog) DeclaredVariables() []Variable { return nil }
func (RBACCheckPermission) ActionName() string { return "rbac.CheckPermission" }
func (a RBACCheckPermission) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}
func (SecretGet) ActionName() string { return "secret.Get" }
func (a SecretGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (ConfigGet) ActionName() string { return "config.Get" }
func (a ConfigGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (ModelResolve) ActionName() string { return "model.Resolve" }
func (a ModelResolve) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (StreamEmit) ActionName() string            { return "stream.Emit" }
func (StreamEmit) DeclaredVariables() []Variable { return nil }
func (LocaleResolve) ActionName() string         { return "locale.Resolve" }
func (a LocaleResolve) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (TemplateRender) ActionName() string { return "template.Render" }
func (a TemplateRender) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (PDFRender) ActionName() string { return "pdf.Render" }
func (a PDFRender) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBytes})
}
func (SessionGet) ActionName() string { return "session.Get" }
func (a SessionGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (DBGet) ActionName() string { return "db.Get" }
func (a DBGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: a.Source}})
}
func (DBList) ActionName() string { return "db.List" }
func (a DBList) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeEntity, Name: a.Source}})
}
func (DBQuery) ActionName() string { return "db.Query" }
func (a DBQuery) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}
func (DBInsert) ActionName() string            { return "db.Insert" }
func (DBInsert) DeclaredVariables() []Variable { return nil }
func (DBUpdate) ActionName() string            { return "db.Update" }
func (DBUpdate) DeclaredVariables() []Variable { return nil }
func (DBUpsert) ActionName() string            { return "db.Upsert" }
func (DBUpsert) DeclaredVariables() []Variable { return nil }
func (DBDelete) ActionName() string            { return "db.Delete" }
func (DBDelete) DeclaredVariables() []Variable { return nil }
func (DBLock) ActionName() string              { return "db.Lock" }
func (a DBLock) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: a.Source}})
}
func (DBSelectForUpdate) ActionName() string { return "db.SelectForUpdate" }
func (a DBSelectForUpdate) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: a.Source}})
}

func (PlanBuildAutomata) ActionName() string { return "plan.BuildAutomata" }
func (a PlanBuildAutomata) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}
func (PlanBuildMicroPlan) ActionName() string { return "plan.BuildMicroPlan" }
func (a PlanBuildMicroPlan) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}
func (CueEmitProject) ActionName() string { return "cue.EmitProject" }
func (a CueEmitProject) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}
func (CueValidateProject) ActionName() string { return "cue.ValidateProject" }
func (a CueValidateProject) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}
func (CueWriteProjectFiles) ActionName() string { return "cue.WriteProjectFiles" }
func (a CueWriteProjectFiles) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

func (ClaudeChat) ActionName() string { return "claude.Chat" }
func (a ClaudeChat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (OpenAIChat) ActionName() string { return "openai.Chat" }
func (a OpenAIChat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (OpenAIEmbed) ActionName() string { return "openai.Embed" }
func (a OpenAIEmbed) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeFloat}})
}
func (OpenAIStream) ActionName() string { return "openai.Stream" }
func (a OpenAIStream) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

func (a ExecCommand) ActionName() string { return a.Alias }
func (a ExecCommand) DeclaredVariables() []Variable {
	v := outputVariable(a.Output, TypeRef{Kind: TypeString})
	return append(v, outputVariable(a.ExitCodeVar, TypeRef{Kind: TypeInt})...)
}
func (FSTempDir) ActionName() string { return "fs.TempDir" }
func (a FSTempDir) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (FSWriteFile) ActionName() string            { return "fs.WriteFile" }
func (FSWriteFile) DeclaredVariables() []Variable { return nil }
func (FSReadFile) ActionName() string             { return "fs.ReadFile" }
func (a FSReadFile) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}
func (FSRemove) ActionName() string            { return "fs.Remove" }
func (FSRemove) DeclaredVariables() []Variable { return nil }
func (ArchiveZipDir) ActionName() string       { return "archive.ZipDir" }
func (a ArchiveZipDir) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBytes})
}

func (BatchRun) ActionName() string            { return "batch.Run" }
func (BatchRun) DeclaredVariables() []Variable { return nil }

func (ListChunk) ActionName() string { return "list.Chunk" }
func (a ListChunk) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeList}})
}

type ListSort struct {
	Items      Expression
	Order      Expression
	By         string
	Descending bool
}

func (ListSort) ActionName() string            { return "list.Sort" }
func (ListSort) DeclaredVariables() []Variable { return nil }

type ListAggregate struct {
	Operation     string
	Input         Expression
	Field, Output string
}

type ValueCoalesce struct {
	Values             []Expression
	Output, Mode, Into string
}

func (ValueCoalesce) ActionName() string { return "value.Coalesce" }
func (a ValueCoalesce) DeclaredVariables() []Variable {
	return outputVariable(a.Output, parseTypeHint(a.Into))
}

type MapBuild struct {
	From              Expression
	As                string
	Key, Value        Expression
	Output, ValueType string
}

func (MapBuild) ActionName() string { return "map.Build" }
func (a MapBuild) DeclaredVariables() []Variable {
	v := parseTypeHint(a.ValueType)
	return outputVariable(a.Output, TypeRef{Kind: TypeMap, Elem: &v})
}

type MapNew struct{ Output, GoType string }

func (MapNew) ActionName() string { return "map.New" }
func (a MapNew) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap, Name: a.GoType})
}

type MapGet struct {
	Input, Key, Default Expression
	Output, Into, Found string
}

func (MapGet) ActionName() string { return "map.Get" }
func (a MapGet) DeclaredVariables() []Variable {
	v := outputVariable(a.Output, parseTypeHint(a.Into))
	return append(v, outputVariable(a.Found, TypeRef{Kind: TypeBool})...)
}

type MapHas struct {
	Input, Key Expression
	Output     string
}

func (MapHas) ActionName() string { return "map.Has" }
func (a MapHas) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type MapSet struct {
	Input, Key, Value Expression
	Output            string
}

func (MapSet) ActionName() string { return "map.Set" }
func (a MapSet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type MapMerge struct {
	Left, Right Expression
	Output      string
}

func (MapMerge) ActionName() string { return "map.Merge" }
func (a MapMerge) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type CastToString struct {
	Input          Expression
	Format, Output string
}

func (CastToString) ActionName() string { return "cast.ToString" }
func (a CastToString) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type ConvertToFloat struct {
	Input  Expression
	Output string
}

func (ConvertToFloat) ActionName() string { return "convert.ToFloat" }
func (a ConvertToFloat) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeFloat})
}

type ConvertToInt struct {
	Input  Expression
	Output string
}

func (ConvertToInt) ActionName() string { return "convert.ToInt" }
func (a ConvertToInt) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeInt})
}

type JSONParse struct {
	Input        Expression
	Into, Output string
}

func (JSONParse) ActionName() string { return "json.Parse" }
func (a JSONParse) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown, Name: a.Into})
}

type JSONMarshal struct {
	Input     Expression
	Output    string
	Stringify bool
}

type Base64Encode struct {
	Input  Expression
	Output string
}

func (Base64Encode) ActionName() string { return "base64.Encode" }
func (a Base64Encode) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type Base64Decode struct {
	Input  Expression
	Output string
}

func (Base64Decode) ActionName() string { return "base64.Decode" }
func (a Base64Decode) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type URLParse struct {
	Input  Expression
	Output string
}

func (URLParse) ActionName() string { return "url.Parse" }
func (a URLParse) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeUnknown, Name: "url.URL"}})
}

type URLBuild struct {
	Base, Path Expression
	Segments   []Expression
	Query      map[string]Expression
	Output     string
}

func (URLBuild) ActionName() string { return "url.Build" }
func (a URLBuild) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type PathBase struct {
	Input  Expression
	Output string
}

func (PathBase) ActionName() string { return "path.Base" }
func (a PathBase) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type QueryEncode struct {
	Input  Expression
	Output string
}

func (QueryEncode) ActionName() string { return "query.Encode" }
func (a QueryEncode) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type QueryDecode struct {
	Input  Expression
	Output string
}

func (QueryDecode) ActionName() string { return "query.Decode" }
func (a QueryDecode) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap, Name: "url.Values"})
}

type HashSum struct {
	Input, Algorithm Expression
	Output           string
}

func (HashSum) ActionName() string { return "hash.Sum" }
func (a HashSum) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type HashHMAC struct {
	Input, Key, Algorithm Expression
	Output                string
}

func (HashHMAC) ActionName() string { return "hash.HMAC" }
func (a HashHMAC) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type NumberBinary struct {
	Operation string
	A, B      Expression
	Output    string
}

func (a NumberBinary) ActionName() string { return a.Operation }
func (a NumberBinary) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeFloat})
}

type MathExpression struct {
	Value   Expression
	Output  string
	Declare bool
}

func (MathExpression) ActionName() string { return "math.Expr" }
func (a MathExpression) DeclaredVariables() []Variable {
	if !a.Declare {
		return nil
	}
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

type MathOperation struct {
	Operation             Expression
	A, B, Value, Min, Max Expression
	Precision             int
	Output                string
}

type JSONPathGet struct {
	Input, Path Expression
	Output      string
}

func (JSONPathGet) ActionName() string { return "jsonpath.Get" }
func (a JSONPathGet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

type JSONPathSet struct {
	Input, Path, Value Expression
	Output             string
}

func (JSONPathSet) ActionName() string { return "jsonpath.Set" }
func (a JSONPathSet) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type ErrorNew struct {
	Message, Status, Code Expression
	Output                string
	Throw                 bool
}

func (ErrorNew) ActionName() string { return "errors.New" }
func (a ErrorNew) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeError})
}

type ErrorThrowIf struct {
	Condition    Expression
	Throw        string
	Status, Code Expression
}

func (ErrorThrowIf) ActionName() string            { return "errors.ThrowIf" }
func (ErrorThrowIf) DeclaredVariables() []Variable { return nil }

type ErrorWrap struct {
	Error, Message Expression
	Output         string
}

func (ErrorWrap) ActionName() string { return "errors.Wrap" }
func (a ErrorWrap) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeError})
}

type ErrorMapCase struct{ Status, Code, Message string }
type ErrorMap struct {
	Input                                                    Expression
	Cases                                                    map[string]ErrorMapCase
	Mode, Output, DefaultMessage, DefaultCode, DefaultStatus string
}

type AuthRequireRole struct {
	UserID, CompanyID, Roles Expression
	Output                   string
	AdminBypass              bool
}

func (AuthRequireRole) ActionName() string { return "auth.RequireRole" }
func (a AuthRequireRole) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: "User"}})
}

type AuthCheckRole struct{ User, Roles, CompanyID Expression }

func (AuthCheckRole) ActionName() string            { return "auth.CheckRole" }
func (AuthCheckRole) DeclaredVariables() []Variable { return nil }

type JWTSign struct {
	Claims, Secret, Algorithm, TTL Expression
	Output                         string
}

func (JWTSign) ActionName() string { return "jwt.Sign" }
func (a JWTSign) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type JWTVerify struct {
	Token, Secret Expression
	Output        string
}

func (JWTVerify) ActionName() string { return "jwt.Verify" }
func (a JWTVerify) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type TokenGenerate struct {
	Subject, Purpose, Claims, Secret, TTL Expression
	Output                                string
}

func (TokenGenerate) ActionName() string { return "token.Generate" }
func (a TokenGenerate) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type TokenVerify struct {
	Token, Purpose, Secret Expression
	Output                 string
}

func (TokenVerify) ActionName() string { return "token.Verify" }
func (a TokenVerify) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type CryptoHash struct {
	Input             Expression
	Algorithm, Output string
}

func (CryptoHash) ActionName() string { return "crypto.Hash" }
func (a CryptoHash) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type CryptoCipher struct {
	Decrypt         bool
	Input, Key, AAD Expression
	Output          string
}

type OAuth2Fields struct {
	TokenURL, ClientID, ClientSecret, Scope, Audience, GrantType, Username, Password, Code, RedirectURI, RefreshToken Expression
	Output                                                                                                            string
}
type OAuth2Token struct{ OAuth2Fields }

func (OAuth2Token) ActionName() string { return "oauth2.Token" }
func (a OAuth2Token) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type OAuth2Refresh struct{ OAuth2Fields }

func (OAuth2Refresh) ActionName() string { return "oauth2.Refresh" }
func (a OAuth2Refresh) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeMap})
}

type WebhookSend struct {
	URL, Payload, Event Expression
	Retries             int
}

func (WebhookSend) ActionName() string            { return "webhook.Send" }
func (WebhookSend) DeclaredVariables() []Variable { return nil }

type WebhookVerifySignature struct {
	Payload, Signature, Secret, Algorithm, Throw Expression
	Strict                                       bool
	Output                                       string
}

func (WebhookVerifySignature) ActionName() string { return "webhook.VerifySignature" }
func (a WebhookVerifySignature) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type WebhookAck struct {
	Status int
	Body   Expression
}

func (WebhookAck) ActionName() string            { return "webhook.Ack" }
func (WebhookAck) DeclaredVariables() []Variable { return nil }

type QueueEnqueue struct{ Subject, Payload, Timeout Expression }

func (QueueEnqueue) ActionName() string            { return "queue.Enqueue" }
func (QueueEnqueue) DeclaredVariables() []Variable { return nil }

type QueueDequeue struct {
	Subject, Timeout              Expression
	Output, AckToken              string
	Attempts, BackoffMS, JitterMS int
}

func (QueueDequeue) ActionName() string { return "queue.Dequeue" }
func (a QueueDequeue) DeclaredVariables() []Variable {
	v := outputVariable(a.Output, TypeRef{Kind: TypeBytes})
	return append(v, outputVariable(a.AckToken, TypeRef{Kind: TypeString})...)
}

type QueueAck struct{ Subject, MessageID Expression }

func (QueueAck) ActionName() string            { return "queue.Ack" }
func (QueueAck) DeclaredVariables() []Variable { return nil }

type QueueNack struct{ Subject, MessageID, Reason Expression }

func (QueueNack) ActionName() string            { return "queue.Nack" }
func (QueueNack) DeclaredVariables() []Variable { return nil }

type DLQPublish struct{ Subject, Payload, Reason Expression }

func (DLQPublish) ActionName() string            { return "dlq.Publish" }
func (DLQPublish) DeclaredVariables() []Variable { return nil }

type MailSend struct{ To, Subject, Body, HTML Expression }

func (MailSend) ActionName() string            { return "mail.Send" }
func (MailSend) DeclaredVariables() []Variable { return nil }

type NotifySend struct {
	Channel, To, Template, Text, Subject, HTML, Data Expression
	Output                                           string
}

func (NotifySend) ActionName() string { return "notify.Send" }
func (a NotifySend) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type NotifyEmail struct {
	To, Template, Text, Subject, HTML, Data, Locale Expression
	Output                                          string
}

func (NotifyEmail) ActionName() string { return "notify.Email" }
func (a NotifyEmail) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type NotificationDispatch struct {
	Alias                                            string
	Event, Type, UserID, EntityID, Payload, Template Expression
}

func (a NotificationDispatch) ActionName() string          { return a.Alias }
func (NotificationDispatch) DeclaredVariables() []Variable { return nil }

type PolicyCheck struct {
	Policy                               string
	User, CompanyID, Status, Code, Throw Expression
	Output                               string
	Resolved                             bool
	Roles                                []string
	SameCompany, AllowAdminOverride      bool
}

func (PolicyCheck) ActionName() string { return "policy.Check" }
func (a PolicyCheck) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeBool})
}

type PolicyDecisionAction struct {
	Alias                                                                                string
	PolicyKey, Subject, Resource, Operation, Tenant, Attrs, Context, Status, Code, Throw Expression
	Decision, Reason, Effects, Output                                                    string
}

type IdempotencyDeriveKey struct {
	Alias  string
	From   []Expression
	Output string
	Prefix Expression
}

func (a IdempotencyDeriveKey) ActionName() string { return a.Alias }
func (a IdempotencyDeriveKey) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type IdempotencyCheck struct {
	Alias string
	Key   Expression
}

func (a IdempotencyCheck) ActionName() string          { return a.Alias }
func (IdempotencyCheck) DeclaredVariables() []Variable { return nil }

type IdempotencySaveResult struct {
	Alias    string
	Key, TTL Expression
}

func (a IdempotencySaveResult) ActionName() string          { return a.Alias }
func (IdempotencySaveResult) DeclaredVariables() []Variable { return nil }

type DedupeOnce struct {
	Key, TTL Expression
}

func (DedupeOnce) ActionName() string            { return "dedupe.Once" }
func (DedupeOnce) DeclaredVariables() []Variable { return nil }

type RateLimit struct {
	Alias string
	Key   Expression
	RPS   int
	Throw string
}

func (a RateLimit) ActionName() string          { return a.Alias }
func (RateLimit) DeclaredVariables() []Variable { return nil }

type QuotaCheck struct {
	Key           Expression
	Limit         int
	Window, Throw string
}

func (QuotaCheck) ActionName() string            { return "quota.Check" }
func (QuotaCheck) DeclaredVariables() []Variable { return nil }

type BudgetCheck struct {
	Key   Expression
	Limit int
	Throw string
}

func (BudgetCheck) ActionName() string            { return "budget.Check" }
func (BudgetCheck) DeclaredVariables() []Variable { return nil }

type BudgetConsume struct{ Key, Tokens, TTL Expression }

func (BudgetConsume) ActionName() string            { return "budget.Consume" }
func (BudgetConsume) DeclaredVariables() []Variable { return nil }

func (a PolicyDecisionAction) ActionName() string { return a.Alias }
func (a PolicyDecisionAction) DeclaredVariables() []Variable {
	v := outputVariable(a.Decision, TypeRef{Kind: TypeString})
	v = append(v, outputVariable(a.Reason, TypeRef{Kind: TypeString})...)
	v = append(v, outputVariable(a.Effects, TypeRef{Kind: TypeMap})...)
	return append(v, outputVariable(a.Output, TypeRef{Kind: TypeUnknown, Name: "port.PolicyDecision"})...)
}

type ContextTrim struct {
	Input    Expression
	Output   string
	MaxBytes int
	Strategy Expression
}

func (ContextTrim) ActionName() string { return "context.Trim" }
func (a ContextTrim) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

type ProfileRequire struct {
	Key, Tier Expression
	Throw     string
}

func (ProfileRequire) ActionName() string            { return "profile.Require" }
func (ProfileRequire) DeclaredVariables() []Variable { return nil }

type ConcurrencyLimit struct {
	Key   Expression
	Max   int
	Throw string
}

func (ConcurrencyLimit) ActionName() string            { return "concurrency.Limit" }
func (ConcurrencyLimit) DeclaredVariables() []Variable { return nil }

type ConcurrencyRun struct {
	ConcurrencyLimit
}

func (ConcurrencyRun) ActionName() string            { return "concurrency.Run" }
func (ConcurrencyRun) DeclaredVariables() []Variable { return nil }

type MutexWith struct {
	Key, Wait, Poll Expression
	Throw           string
}

func (MutexWith) ActionName() string            { return "mutex.With" }
func (MutexWith) DeclaredVariables() []Variable { return nil }

type CircuitAction struct {
	Alias         string
	Name, OpenTTL Expression
	Threshold     int
	Throw         string
}

func (a CircuitAction) ActionName() string          { return a.Alias }
func (CircuitAction) DeclaredVariables() []Variable { return nil }

type BulkheadAction struct {
	Alias string
	Name  Expression
	Max   int
	Throw string
}
type LogEmit struct {
	Message Expression
	Level   string
	Fields  map[string]Expression
}

func (LogEmit) ActionName() string            { return "log.Emit" }
func (LogEmit) DeclaredVariables() []Variable { return nil }

type MetricEmit struct {
	Name   Expression
	Kind   string
	Value  Expression
	Labels map[string]Expression
}

func (MetricEmit) ActionName() string            { return "metric.Emit" }
func (MetricEmit) DeclaredVariables() []Variable { return nil }

type TraceSpan struct {
	Name       Expression
	Attributes map[string]Expression
}

func (TraceSpan) ActionName() string            { return "trace.Span" }
func (TraceSpan) DeclaredVariables() []Variable { return nil }

type SLOBudget struct {
	Name     string
	Duration Expression
}
type HTTPCall struct {
	Method             string
	URL, Body, Timeout Expression
	Headers            map[string]Expression
	Output, StatusVar  string
	FailOnError        bool
	Attempts           int
	BackoffMS          int
}

func (HTTPCall) ActionName() string { return "http.Call" }
func (a HTTPCall) DeclaredVariables() []Variable {
	v := outputVariable(a.Output, TypeRef{Kind: TypeString})
	return append(v, outputVariable(a.StatusVar, TypeRef{Kind: TypeInt})...)
}

type HTTPRequest struct {
	Method                  string
	URL, Body, Timeout      Expression
	Headers, Query          map[string]Expression
	Auth                    string
	Into, Output, StatusVar string
	FailOnError             bool
}
type HTTPRetryPolicy struct {
	HTTPRequest
	Attempts, BackoffMS int
	RetryOn             []int
}
type FlowFor struct {
	Each Expression
	As   string
}

func (FlowFor) ActionName() string            { return "flow.For" }
func (FlowFor) DeclaredVariables() []Variable { return nil }

type FlowWhile struct {
	Condition Expression
}

func (FlowWhile) ActionName() string            { return "flow.While" }
func (FlowWhile) DeclaredVariables() []Variable { return nil }

type FlowSwitch struct {
	Value Expression
	Match string
}

func (FlowSwitch) ActionName() string            { return "flow.Switch" }
func (FlowSwitch) DeclaredVariables() []Variable { return nil }

func (HTTPRetryPolicy) ActionName() string              { return "http.RetryPolicy" }
func (a HTTPRetryPolicy) DeclaredVariables() []Variable { return a.HTTPRequest.DeclaredVariables() }

type HTTPPaginate struct {
	URL                             Expression
	Method, Into, As                string
	Cursor, Items                   Expression
	CursorParam, Output, OutputType string
	MaxPages                        int
	Headers                         map[string]Expression
	Auth                            string
}

func (HTTPPaginate) ActionName() string { return "http.Paginate" }
func (a HTTPPaginate) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeList, Name: a.OutputType})
}

type HTTPSOAP struct {
	URL, Namespace, Operation, SOAPAction, Timeout Expression
	Request, Headers                               map[string]Expression
	Into, Output, StatusVar                        string
	FailOnError                                    bool
}

func (HTTPSOAP) ActionName() string { return "http.SOAP" }
func (a HTTPSOAP) DeclaredVariables() []Variable {
	typ := TypeRef{Kind: TypeString}
	if a.Into != "" {
		typ = TypeRef{Kind: TypeUnknown, Name: a.Into}
	}
	v := outputVariable(a.Output, typ)
	return append(v, outputVariable(a.StatusVar, TypeRef{Kind: TypeInt})...)
}

func (HTTPRequest) ActionName() string { return "http.Request" }
func (a HTTPRequest) DeclaredVariables() []Variable {
	typ := TypeRef{Kind: TypeString}
	if a.Into != "" {
		typ = TypeRef{Kind: TypeUnknown, Name: a.Into}
	}
	v := outputVariable(a.Output, typ)
	return append(v, outputVariable(a.StatusVar, TypeRef{Kind: TypeInt})...)
}

func (SLOBudget) ActionName() string            { return "slo.Budget" }
func (SLOBudget) DeclaredVariables() []Variable { return nil }

func (a BulkheadAction) ActionName() string          { return a.Alias }
func (BulkheadAction) DeclaredVariables() []Variable { return nil }

type ApprovalRequest struct {
	ApprovalKey, Title, Description, RequestedBy, Policy, Payload, Deadline, TTL Expression
	Approvers                                                                    []Expression
	ApproversList                                                                bool
	ApprovalID, Status                                                           string
}

func (ApprovalRequest) ActionName() string { return "approval.Request" }
func (a ApprovalRequest) DeclaredVariables() []Variable {
	v := outputVariable(a.ApprovalID, TypeRef{Kind: TypeString})
	return append(v, outputVariable(a.Status, TypeRef{Kind: TypeString})...)
}

type ApprovalWait struct {
	ApprovalID, Timeout, TimeoutMode               Expression
	Decision, Status, DecidedBy, DecidedAt, Reason string
}

func (ApprovalWait) ActionName() string { return "approval.Wait" }
func (a ApprovalWait) DeclaredVariables() []Variable {
	var v []Variable
	for _, n := range []string{a.Decision, a.Status, a.DecidedBy, a.DecidedAt, a.Reason} {
		v = append(v, outputVariable(n, TypeRef{Kind: TypeString})...)
	}
	return v
}

type ApprovalDecide struct {
	ApprovalID, Decision, Actor, Reason Expression
	Status                              string
}

func (ApprovalDecide) ActionName() string { return "approval.Decide" }
func (a ApprovalDecide) DeclaredVariables() []Variable {
	return outputVariable(a.Status, TypeRef{Kind: TypeString})
}

func (a CryptoCipher) ActionName() string {
	if a.Decrypt {
		return "crypto.Decrypt"
	}
	return "crypto.Encrypt"
}
func (a CryptoCipher) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

func (ErrorMap) ActionName() string { return "errors.Map" }
func (a ErrorMap) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeError})
}

func (MathOperation) ActionName() string { return "math.Op" }
func (a MathOperation) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeFloat})
}

func (a JSONMarshal) ActionName() string {
	if a.Stringify {
		return "json.Stringify"
	}
	return "json.Marshal"
}
func (a JSONMarshal) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

func (a ListAggregate) ActionName() string { return a.Operation }
func (a ListAggregate) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeUnknown})
}

func (StorageGetURL) ActionName() string { return "storage.GetURL" }
func (a StorageGetURL) DeclaredVariables() []Variable {
	return outputVariable(a.Output, TypeRef{Kind: TypeString})
}

func outputVariable(name string, typ TypeRef) []Variable {
	if name == "" || !token.IsIdentifier(name) || token.Lookup(name).IsKeyword() {
		return nil
	}
	return []Variable{{Name: name, Type: typ}}
}

func (a RepositoryCall) ActionName() string { return string(a.Operation) }
func (a RepositoryCall) DeclaredVariables() []Variable {
	if a.Output == "" {
		return nil
	}
	typeRef := TypeRef{Kind: TypePointer, Elem: &TypeRef{Kind: TypeEntity, Name: a.Entity}}
	switch a.Operation {
	case RepoList:
		typeRef = TypeRef{Kind: TypeList, Elem: &TypeRef{Kind: TypeEntity, Name: a.Entity}}
	case RepoQuery:
		typeRef = TypeRef{Kind: TypeUnknown}
	case RepoExists:
		typeRef = TypeRef{Kind: TypeBool}
	case RepoCount:
		typeRef = TypeRef{Kind: TypeInt}
	}
	return []Variable{{Name: a.Output, Type: typeRef}}
}
