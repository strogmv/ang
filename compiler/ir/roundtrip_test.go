package ir_test

import (
	"reflect"
	"testing"

	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestIRRoundTrip(t *testing.T) {
	t.Run("Endpoint", func(t *testing.T) {
		original := normalizer.Endpoint{
			Method:           "POST",
			Path:             "/api/test",
			ServiceName:      "TestService",
			RPC:              "DoSomething",
			Description:      "Test endpoint",
			Messages:         []string{"Msg1", "Msg2"},
			RoomParam:        "roomID",
			AuthType:         "jwt",
			Permission:       "create_tender",
			AuthRoles:        []string{"admin", "user"},
			AuthCheck:        "req.ID == user.ID",
			AuthInject:       []string{"userID", "companyID"},
			CacheTTL:         "5m",
			CacheTags:        []string{"tag1"},
			Invalidate:       []string{"RPC1"},
			OptimisticUpdate: "GetRelated",
			Timeout:          "30s",
			MaxBodySize:      1024 * 1024,
			Idempotency:      true,
			DedupeKey:        "X-Dedupe-ID",
			Errors:           []string{"NOT_FOUND", "FORBIDDEN"},
			View:             "public",
			Pagination: &normalizer.PaginationDef{
				Type:         "offset",
				DefaultLimit: 20,
				MaxLimit:     100,
			},
			RateLimit: &normalizer.RateLimitDef{
				RPS:   10,
				Burst: 20,
			},
			CircuitBreaker: &normalizer.CircuitBreakerDef{
				Threshold:   5,
				Timeout:     "1m",
				HalfOpenMax: 3,
			},
			SLO: normalizer.SLODef{
				Latency: "200ms",
				Success: "99.9%",
			},
			TestHints: &normalizer.TestHints{
				HappyPath:  "should work",
				ErrorCases: []string{"error1", "error2"},
			},
			Metadata: map[string]any{"custom": "value"},
			Source:   "test.cue",
		}

		assertAllFieldsSet(t, original)

		irEndpoint := ir.ConvertEndpoint(original)
		results := emitter.IREndpointsToNormalizer([]ir.Endpoint{irEndpoint})
		result := results[0]

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Endpoint")
		}
	})

	t.Run("Entity", func(t *testing.T) {
		original := normalizer.Entity{
			Name:        "TestEntity",
			Description: "Test entity desc",
			Owner:       "TestService",
			Fields:      []normalizer.Field{},
			Metadata:    map[string]any{"e": "v"},
			Indexes:     []normalizer.IndexDef{},
			Source:      "test.cue",
		}

		assertAllFieldsSet(t, original)

		irEntity := ir.ConvertEntity(original)
		result := emitter.IREntityToNormalizer(irEntity)

		if result.Indexes == nil {
			result.Indexes = []normalizer.IndexDef{}
		}
		if result.Fields == nil {
			result.Fields = []normalizer.Field{}
		}

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Entity")
		}
	})

	t.Run("Field", func(t *testing.T) {
		original := normalizer.Field{
			Name:        "testField",
			Type:        "string",
			Default:     "defaultVal",
			IsOptional:  true,
			IsList:      false,
			IsSecret:    true,
			IsPII:       true,
			SkipDomain:  true,
			ValidateTag: "required,email",
			Constraints: &normalizer.Constraints{
				Regex: "^[^@]+@[^@]+$",
			},
			EnvVar:   "TEST_VAR",
			Metadata: map[string]any{"sql_type": "text"},
			UI: &normalizer.UIHints{
				Type:        "email",
				Label:       "Email",
				Placeholder: "Enter email",
				HelperText:  "Helper",
				Order:       1,
				Hidden:      true,
				Disabled:    true,
				FullWidth:   true,
				Rows:        3,
				Currency:    "USD",
				Source:      "DataSource",
				Options:     []string{"Opt1", "Opt2"},
				Multiple:    true,
				Accept:      "image/*",
				MaxSize:     5000,
			},
			DB: normalizer.DBMeta{
				Type:       "varchar(255)",
				PrimaryKey: true,
				Unique:     true,
				Index:      true,
			},
			FileMeta: &normalizer.FileMeta{
				Kind:      "image",
				Thumbnail: true,
			},
			Source: "test.cue",
		}

		assertAllFieldsSet(t, original)

		irField := ir.ConvertField(original)
		result := emitter.IRFieldToNormalizer(irField)

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Field")
		}
	})

	t.Run("Service", func(t *testing.T) {
		original := normalizer.Service{
			Name:          "TestService",
			Description:   "Test description",
			Publishes:     []string{"Event1"},
			Subscribes:    map[string]string{"Event2": "HandleEvent2"},
			Uses:          []string{"Chat"},
			Metadata:      map[string]any{"key": "val"},
			RequiresSQL:   true,
			RequiresMongo: true,
			RequiresRedis: true,
			RequiresNats:  true,
			RequiresS3:    true,
			Source:        "test.cue",
			Methods: []normalizer.Method{
				{
					Name:        "TestMethod",
					Description: "Method desc",
					CacheTTL:    "1h",
					CacheTags:   []string{"tag1"},
					Idempotency: true,
					DedupeKey:   "key",
					Outbox:      true,
					Throws:      []string{"ERR1"},
					Publishes:   []string{"EVT1"},
					Broadcasts:  []string{"BRD1"},
					Pagination: &normalizer.PaginationDef{
						Type:         "cursor",
						DefaultLimit: 50,
						MaxLimit:     100,
					},
					Metadata: map[string]any{"m": "v"},
					Source:   "test.cue",
					Input: normalizer.Entity{
						Name:     "Input",
						Metadata: make(map[string]any),
						Fields:   []normalizer.Field{},
						Indexes:  []normalizer.IndexDef{},
					},
					Output: normalizer.Entity{
						Name:     "Output",
						Metadata: make(map[string]any),
						Fields:   []normalizer.Field{},
						Indexes:  []normalizer.IndexDef{},
					},
					Sources: []normalizer.Source{},
					Flow:    []normalizer.FlowStep{},
				},
			},
		}

		assertAllFieldsSet(t, original)

		irSvc := ir.ConvertService(original)
		result := emitter.IRServiceToNormalizer(irSvc)

		if len(original.Methods) != len(result.Methods) {
			t.Fatal("Method count mismatch")
		}

		for i := range original.Methods {
			m1 := original.Methods[i]
			m2 := result.Methods[i]

			if m2.Input.Indexes == nil {
				m2.Input.Indexes = []normalizer.IndexDef{}
			}
			if m2.Output.Indexes == nil {
				m2.Output.Indexes = []normalizer.IndexDef{}
			}
			if m2.Input.Fields == nil {
				m2.Input.Fields = []normalizer.Field{}
			}
			if m2.Output.Fields == nil {
				m2.Output.Fields = []normalizer.Field{}
			}
			if m2.Sources == nil {
				m2.Sources = []normalizer.Source{}
			}
			if m2.Flow == nil {
				m2.Flow = []normalizer.FlowStep{}
			}

			m1.Pagination = nil
			m2.Pagination = nil

			if !reflect.DeepEqual(m1, m2) {
				t.Error("Method mismatch")
			}
		}

		original.Methods = nil
		result.Methods = nil

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Service")
		}
	})

	t.Run("Event", func(t *testing.T) {
		original := normalizer.EventDef{
			Name: "TestEvent",
			Fields: []normalizer.Field{
				{Name: "f1", Type: "string", Metadata: make(map[string]any)},
			},
			Metadata: map[string]any{"ev": "v"},
			Source:   "test.cue",
		}

		assertAllFieldsSet(t, original)

		irEvent := ir.ConvertEvent(original)
		result := emitter.IREventsToNormalizer([]ir.Event{irEvent})[0]

		for i := range result.Fields {
			if result.Fields[i].Metadata == nil {
				result.Fields[i].Metadata = make(map[string]any)
			}
		}

		if !reflect.DeepEqual(original, result) {
			t.Logf("Original: %+v", original)
			t.Logf("Result:   %+v", result)
			t.Error("Round-trip failed for Event")
		}
	})

	t.Run("Error", func(t *testing.T) {
		original := normalizer.ErrorDef{
			Name:       "TEST_ERROR",
			Code:       1001,
			HTTPStatus: 400,
			Message:    "Test error message",
			Source:     "test.cue",
		}

		assertAllFieldsSet(t, original)

		irErr := ir.Error{
			Name:       original.Name,
			Code:       original.Code,
			HTTPStatus: original.HTTPStatus,
			Message:    original.Message,
			Source:     original.Source,
		}
		result := emitter.IRErrorsToNormalizer([]ir.Error{irErr})[0]

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Error")
		}
	})

	t.Run("Repository", func(t *testing.T) {
		original := normalizer.Repository{
			Name:   "TestRepo",
			Entity: "TestEntity",
			Source: "test.cue",
			Finders: []normalizer.RepositoryFinder{
				{
					Name:      "FindActive",
					Action:    "find",
					Returns:   "[]Entity",
					Select:    []string{"id", "name"},
					OrderBy:   "created_at DESC",
					Limit:     10,
					ForUpdate: true,
					Source:    "test.cue",
					Where: []normalizer.FinderWhere{
						{Field: "status", Op: "eq", Param: "active", ParamType: "string"},
					},
				},
			},
		}

		assertAllFieldsSet(t, original)

		irRepo := ir.ConvertRepository(original)
		result := emitter.IRReposToNormalizer([]ir.Repository{irRepo})[0]

		if len(original.Finders) != len(result.Finders) {
			t.Fatal("Finder count mismatch")
		}
		for i := range original.Finders {
			if result.Finders[i].Where == nil {
				result.Finders[i].Where = []normalizer.FinderWhere{}
			}
		}

		if !reflect.DeepEqual(original, result) {
			t.Error("Round-trip failed for Repository")
		}
	})
}

func assertAllFieldsSet(t *testing.T, v interface{}) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			t.Error("Pointer is nil")
			return
		}
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldName := typ.Field(i).Name

		if fieldName == "ItemFields" || fieldName == "ItemTypeName" || fieldName == "Impl" || fieldName == "FSM" || fieldName == "UI" || fieldName == "Indexes" || fieldName == "CRUD" {
			continue
		}

		if field.Kind() == reflect.Slice || field.Kind() == reflect.Map {
			if field.IsNil() {
				t.Error("Field is nil:", fieldName)
			}
			continue
		}

		if field.Kind() == reflect.Bool {
			continue
		}

		if field.IsZero() {
			t.Error("Field is zero:", fieldName)
		}
	}
}
