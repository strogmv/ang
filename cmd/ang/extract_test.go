package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Go extractor tests
// ---------------------------------------------------------------------------

func TestExtractGoFacts_Struct(t *testing.T) {
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "domain")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}
	code := `package domain

import "github.com/google/uuid"

type User struct {
	ID        uuid.UUID ` + "`" + `json:"id" db:"id"` + "`" + `
	Email     string    ` + "`" + `json:"email" db:"email"` + "`" + `
	CreatedAt string    ` + "`" + `json:"created_at" db:"created_at"` + "`" + `
}
`
	if err := os.WriteFile(filepath.Join(domainDir, "user.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractGoFacts(dir)
	if err != nil {
		t.Fatalf("extractGoFacts: %v", err)
	}
	if len(env.Entities) == 0 {
		t.Fatal("expected at least one entity, got none")
	}
	var user *FactEntity
	for i := range env.Entities {
		if env.Entities[i].Name == "User" {
			user = &env.Entities[i]
			break
		}
	}
	if user == nil {
		t.Fatalf("entity User not found; got: %v", entityNames(env.Entities))
	}
	if user.TableHint != "users" {
		t.Errorf("table_hint = %q, want %q", user.TableHint, "users")
	}
	fieldMap := make(map[string]FactField)
	for _, f := range user.Fields {
		fieldMap[f.Name] = f
	}
	if _, ok := fieldMap["ID"]; !ok {
		t.Error("field ID not found")
	}
	if fieldMap["ID"].CueTypeHint != "string" {
		t.Errorf("ID cue_type_hint = %q, want string", fieldMap["ID"].CueTypeHint)
	}
	if fieldMap["ID"].DBTag != "id" {
		t.Errorf("ID db_tag = %q, want id", fieldMap["ID"].DBTag)
	}
	if fieldMap["Email"].JSONTag != "email" {
		t.Errorf("Email json_tag = %q, want email", fieldMap["Email"].JSONTag)
	}
}

func TestExtractGoFacts_RepoInterface(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	code := `package repo

import "context"

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*User, error)
	List(ctx context.Context, offset, limit int) ([]User, error)
	Count(ctx context.Context) (int64, error)
	Save(ctx context.Context, u *User) error
}
`
	if err := os.WriteFile(filepath.Join(repoDir, "user_repo.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractGoFacts(dir)
	if err != nil {
		t.Fatalf("extractGoFacts: %v", err)
	}
	if len(env.Repositories) == 0 {
		t.Fatal("expected at least one repository, got none")
	}
	var repo *FactRepo
	for i := range env.Repositories {
		if env.Repositories[i].Entity == "User" {
			repo = &env.Repositories[i]
			break
		}
	}
	if repo == nil {
		t.Fatalf("repo User not found; got: %v", repoEntities(env.Repositories))
	}
	methodMap := make(map[string]FactRepoMethod)
	for _, m := range repo.Methods {
		methodMap[m.Name] = m
	}
	if methodMap["GetByID"].Returns != "one" {
		t.Errorf("GetByID returns = %q, want one", methodMap["GetByID"].Returns)
	}
	if methodMap["List"].Returns != "many" {
		t.Errorf("List returns = %q, want many", methodMap["List"].Returns)
	}
	if methodMap["Count"].Returns != "count" {
		t.Errorf("Count returns = %q, want count", methodMap["Count"].Returns)
	}
	if methodMap["Save"].Returns != "none" {
		t.Errorf("Save returns = %q, want none", methodMap["Save"].Returns)
	}
}

func TestExtractGoFacts_ServiceInterface(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}
	code := `package service

import "context"

type UserService interface {
	CreateUser(ctx context.Context, email string, name string) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	DeleteUser(ctx context.Context, id string) error
}
`
	if err := os.WriteFile(filepath.Join(svcDir, "user_service.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractGoFacts(dir)
	if err != nil {
		t.Fatalf("extractGoFacts: %v", err)
	}
	if len(env.Operations) == 0 {
		t.Fatal("expected at least one operation, got none")
	}
	opMap := make(map[string]FactOp)
	for _, op := range env.Operations {
		opMap[op.Name] = op
	}
	if _, ok := opMap["CreateUser"]; !ok {
		t.Fatalf("op CreateUser not found; got: %v", opNames(env.Operations))
	}
	if opMap["CreateUser"].ServiceHint != "user" {
		t.Errorf("CreateUser service_hint = %q, want user", opMap["CreateUser"].ServiceHint)
	}
	// context.Context should be filtered from inputs
	for _, f := range opMap["CreateUser"].InputFields {
		if f.GoType == "context.Context" {
			t.Error("context.Context should not appear in input_fields")
		}
	}
	// error should be filtered from outputs
	for _, f := range opMap["CreateUser"].OutputFields {
		if f.GoType == "error" {
			t.Error("error should not appear in output_fields")
		}
	}
}

// ---------------------------------------------------------------------------
// SQL extractor tests
// ---------------------------------------------------------------------------

func TestExtractSQLFacts(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE users (
	id         UUID PRIMARY KEY,
	email      VARCHAR(255) NOT NULL,
	age        INT,
	balance    DECIMAL(10,2),
	is_active  BOOLEAN,
	created_at TIMESTAMP WITH TIME ZONE,
	metadata   JSONB
);

CREATE TABLE order_items (
	id         BIGSERIAL PRIMARY KEY,
	order_id   UUID REFERENCES orders(id),
	quantity   INT NOT NULL
);
`
	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractSQLFacts(dir)
	if err != nil {
		t.Fatalf("extractSQLFacts: %v", err)
	}
	if len(env.Entities) < 2 {
		t.Fatalf("expected 2 entities, got %d: %v", len(env.Entities), entityNames(env.Entities))
	}
	entityMap := make(map[string]FactEntity)
	for _, e := range env.Entities {
		entityMap[e.Name] = e
	}
	user, ok := entityMap["User"]
	if !ok {
		t.Fatalf("entity User not found; got: %v", entityNames(env.Entities))
	}
	fieldMap := make(map[string]FactField)
	for _, f := range user.Fields {
		fieldMap[f.Name] = f
	}
	if fieldMap["Id"].CueTypeHint != "string" {
		t.Errorf("Id cue_type_hint = %q, want string", fieldMap["Id"].CueTypeHint)
	}
	if fieldMap["Email"].CueTypeHint != "string" {
		t.Errorf("Email cue_type_hint = %q, want string", fieldMap["Email"].CueTypeHint)
	}
	if fieldMap["Age"].CueTypeHint != "int" {
		t.Errorf("Age cue_type_hint = %q, want int", fieldMap["Age"].CueTypeHint)
	}
	if fieldMap["Balance"].CueTypeHint != "float" {
		t.Errorf("Balance cue_type_hint = %q, want float", fieldMap["Balance"].CueTypeHint)
	}
	if fieldMap["IsActive"].CueTypeHint != "bool" {
		t.Errorf("IsActive cue_type_hint = %q, want bool", fieldMap["IsActive"].CueTypeHint)
	}
	if fieldMap["CreatedAt"].CueTypeHint != "string" {
		t.Errorf("CreatedAt cue_type_hint = %q, want string", fieldMap["CreatedAt"].CueTypeHint)
	}
	if fieldMap["Metadata"].CueTypeHint != "{...}" {
		t.Errorf("Metadata cue_type_hint = %q, want {...}", fieldMap["Metadata"].CueTypeHint)
	}

	if _, ok := entityMap["OrderItem"]; !ok {
		t.Errorf("entity OrderItem not found; got: %v", entityNames(env.Entities))
	}
}

// ---------------------------------------------------------------------------
// detectSourceType tests
// ---------------------------------------------------------------------------

func TestDetectSourceType_SQL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte("CREATE TABLE t (id INT);"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectSourceType(dir)
	if got != "sql" {
		t.Errorf("detectSourceType = %q, want sql", got)
	}
}

func TestDetectSourceType_OpenAPI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte("openapi: 3.0.0"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectSourceType(dir)
	if got != "openapi" {
		t.Errorf("detectSourceType = %q, want openapi", got)
	}
}

func TestDetectSourceType_Go(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectSourceType(dir)
	if got != "go" {
		t.Errorf("detectSourceType = %q, want go", got)
	}
}

func TestDetectSourceType_Java(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	got := detectSourceType(dir)
	if got != "java" {
		t.Errorf("detectSourceType = %q, want java", got)
	}
}

// ---------------------------------------------------------------------------
// Java extractor tests
// ---------------------------------------------------------------------------

func TestExtractJavaFacts_Entity(t *testing.T) {
	dir := t.TempDir()
	java := `package com.example.domain;

import javax.persistence.*;
import java.util.UUID;
import java.time.LocalDateTime;

@Entity
@Table(name = "users")
public class User {

    @Id
    private UUID id;

    @Column(name = "email")
    private String email;

    private int age;

    private boolean active;

    private LocalDateTime createdAt;
}
`
	if err := os.WriteFile(filepath.Join(dir, "User.java"), []byte(java), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	if len(env.Entities) == 0 {
		t.Fatal("expected at least one entity, got none")
	}
	var user *FactEntity
	for i := range env.Entities {
		if env.Entities[i].Name == "User" {
			user = &env.Entities[i]
			break
		}
	}
	if user == nil {
		t.Fatalf("entity User not found; got: %v", entityNames(env.Entities))
	}
	if user.TableHint != "users" {
		t.Errorf("table_hint = %q, want users", user.TableHint)
	}
	fieldMap := make(map[string]FactField)
	for _, f := range user.Fields {
		fieldMap[f.Name] = f
	}
	if fieldMap["Email"].CueTypeHint != "string" {
		t.Errorf("Email cue_type_hint = %q, want string", fieldMap["Email"].CueTypeHint)
	}
	if fieldMap["Age"].CueTypeHint != "int" {
		t.Errorf("Age cue_type_hint = %q, want int", fieldMap["Age"].CueTypeHint)
	}
	if fieldMap["Active"].CueTypeHint != "bool" {
		t.Errorf("Active cue_type_hint = %q, want bool", fieldMap["Active"].CueTypeHint)
	}
	if fieldMap["CreatedAt"].CueTypeHint != "string" {
		t.Errorf("CreatedAt cue_type_hint = %q, want string", fieldMap["CreatedAt"].CueTypeHint)
	}
}

func TestExtractJavaFacts_Repository(t *testing.T) {
	dir := t.TempDir()
	java := `package com.example.repo;

import org.springframework.data.jpa.repository.JpaRepository;
import java.util.UUID;
import java.util.List;
import java.util.Optional;

public interface UserRepository extends JpaRepository<User, UUID> {
    Optional<User> findByEmail(String email);
    List<User> findAllByActive(boolean active);
    long countByActive(boolean active);
    void deleteByEmail(String email);
}
`
	if err := os.WriteFile(filepath.Join(dir, "UserRepository.java"), []byte(java), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	if len(env.Repositories) == 0 {
		t.Fatal("expected at least one repository, got none")
	}
	repo := env.Repositories[0]
	if repo.Entity != "User" {
		t.Errorf("entity = %q, want User", repo.Entity)
	}
	methodMap := make(map[string]FactRepoMethod)
	for _, m := range repo.Methods {
		methodMap[m.Name] = m
	}
	if methodMap["findByEmail"].Returns != "one" {
		t.Errorf("findByEmail returns = %q, want one", methodMap["findByEmail"].Returns)
	}
	if methodMap["findAllByActive"].Returns != "many" {
		t.Errorf("findAllByActive returns = %q, want many", methodMap["findAllByActive"].Returns)
	}
	if methodMap["countByActive"].Returns != "count" {
		t.Errorf("countByActive returns = %q, want count", methodMap["countByActive"].Returns)
	}
	if methodMap["deleteByEmail"].Returns != "none" {
		t.Errorf("deleteByEmail returns = %q, want none", methodMap["deleteByEmail"].Returns)
	}
}

func TestExtractJavaFacts_ControllerMetadataAndCalls(t *testing.T) {
	dir := t.TempDir()
	java := `package com.example.rest;

import org.springframework.web.bind.annotation.*;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.transaction.annotation.Transactional;

@RestController
@RequestMapping("/api")
public class OwnerRestController {
    private final ClinicService clinicService;
    private final OwnerMapper ownerMapper;

    public OwnerRestController(ClinicService clinicService, OwnerMapper ownerMapper) {
        this.clinicService = clinicService;
        this.ownerMapper = ownerMapper;
    }

    @PreAuthorize("hasRole('OWNER_ADMIN')")
    @Transactional
    @PostMapping("/owners")
    public ResponseEntity<OwnerDto> addOwner(OwnerFieldsDto dto) {
        Owner owner = ownerMapper.toOwner(dto);
        clinicService.saveOwner(owner);
        return ResponseEntity.ok(ownerMapper.toOwnerDto(owner));
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "OwnerRestController.java"), []byte(java), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	var op *FactOp
	for i := range env.Operations {
		if env.Operations[i].Name == "AddOwner" {
			op = &env.Operations[i]
			break
		}
	}
	if op == nil {
		t.Fatalf("expected AddOwner operation, got: %v", opNames(env.Operations))
	}
	if op.HTTPMethod != "POST" {
		t.Errorf("http_method = %q, want POST", op.HTTPMethod)
	}
	if op.HTTPPath != "/api/owners" {
		t.Errorf("http_path = %q, want /api/owners", op.HTTPPath)
	}
	if op.AuthExpr == "" || !strings.Contains(op.AuthExpr, "hasRole") {
		t.Errorf("auth_expr = %q, want hasRole(...) expression", op.AuthExpr)
	}
	if !op.Transactional {
		t.Error("expected transactional=true")
	}
	callMap := map[string]string{}
	for _, c := range op.Calls {
		callMap[c.Target] = c.Kind
	}
	if callMap["clinicService.saveOwner"] != "service" {
		t.Errorf("clinicService.saveOwner kind = %q, want service", callMap["clinicService.saveOwner"])
	}
	if callMap["ownerMapper.toOwner"] != "mapper" {
		t.Errorf("ownerMapper.toOwner kind = %q, want mapper", callMap["ownerMapper.toOwner"])
	}
}

func TestExtractJavaFacts_OpenAPIMerge(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "main", "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	openapi := `openapi: 3.0.0
paths:
  /owners:
    post:
      operationId: addOwner
      requestBody:
        content:
          application/json:
            schema:
              type: object
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(filepath.Join(dir, "src", "main", "resources", "openapi.yml"), []byte(openapi), 0644); err != nil {
		t.Fatal(err)
	}
	java := `package com.example.rest;

import org.springframework.web.bind.annotation.RestController;

@RestController
public class OwnerRestController {
    public ResponseEntity<OwnerDto> addOwner(OwnerFieldsDto dto) {
        return null;
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "OwnerRestController.java"), []byte(java), 0644); err != nil {
		t.Fatal(err)
	}
	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	var op *FactOp
	for i := range env.Operations {
		if env.Operations[i].Name == "AddOwner" {
			op = &env.Operations[i]
			break
		}
	}
	if op == nil {
		t.Fatalf("expected AddOwner operation, got: %v", opNames(env.Operations))
	}
	if op.HTTPMethod != "POST" {
		t.Errorf("http_method = %q, want POST", op.HTTPMethod)
	}
	if op.HTTPPath != "/owners" {
		t.Errorf("http_path = %q, want /owners", op.HTTPPath)
	}
}

func TestExtractJavaFacts_RepoQueryMeta(t *testing.T) {
	dir := t.TempDir()
	java := `package com.example.repo;

import java.util.List;

public interface OwnerRepository {
    List<Owner> findByLastNameAndCity(String lastName, String city);
    long countByActive(boolean active);
    void deleteByEmail(String email);
}
`
	if err := os.WriteFile(filepath.Join(dir, "OwnerRepository.java"), []byte(java), 0644); err != nil {
		t.Fatal(err)
	}
	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	if len(env.Repositories) == 0 {
		t.Fatal("expected repository facts")
	}
	methods := map[string]FactRepoMethod{}
	for _, m := range env.Repositories[0].Methods {
		methods[m.Name] = m
	}
	if methods["findByLastNameAndCity"].QueryKind != "find" {
		t.Errorf("query_kind = %q, want find", methods["findByLastNameAndCity"].QueryKind)
	}
	if got := strings.Join(methods["findByLastNameAndCity"].CriteriaFields, ","); got != "lastName,city" {
		t.Errorf("criteria_fields = %q, want %q", got, "lastName,city")
	}
	if methods["countByActive"].QueryKind != "count" {
		t.Errorf("query_kind = %q, want count", methods["countByActive"].QueryKind)
	}
	if methods["deleteByEmail"].QueryKind != "delete" {
		t.Errorf("query_kind = %q, want delete", methods["deleteByEmail"].QueryKind)
	}
}

func TestExtractJavaFacts_MapperErrorAndSecurityFacts(t *testing.T) {
	dir := t.TempDir()
	mapper := `package com.example.mapper;

import org.mapstruct.Mapper;

@Mapper(uses = PetMapper.class)
public interface OwnerMapper {
    OwnerDto toOwnerDto(Owner owner);
}
`
	advice := `package com.example.rest;

import org.springframework.web.bind.annotation.ControllerAdvice;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.http.HttpStatus;

@ControllerAdvice
public class ExceptionControllerAdvice {
    @ExceptionHandler(DataIntegrityViolationException.class)
    public ResponseEntity<ProblemDetail> handleDataIntegrityViolationException(Exception ex) {
        HttpStatus status = HttpStatus.NOT_FOUND;
        return null;
    }
}
`
	security := `package com.example.security;

import org.springframework.security.config.annotation.method.configuration.EnableMethodSecurity;

@EnableMethodSecurity(prePostEnabled = true)
public class BasicAuthenticationConfig {
    public SecurityFilterChain filterChain(HttpSecurity http) throws Exception {
        http.authorizeHttpRequests((authz) -> authz
            .anyRequest().authenticated());
        return null;
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "OwnerMapper.java"), []byte(mapper), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ExceptionControllerAdvice.java"), []byte(advice), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "BasicAuthenticationConfig.java"), []byte(security), 0644); err != nil {
		t.Fatal(err)
	}
	env, err := extractJavaFacts(dir)
	if err != nil {
		t.Fatalf("extractJavaFacts: %v", err)
	}
	if len(env.Mappers) == 0 {
		t.Fatal("expected mapper facts")
	}
	if env.Mappers[0].Name != "OwnerMapper" {
		t.Errorf("mapper name = %q, want OwnerMapper", env.Mappers[0].Name)
	}
	if len(env.ErrorContracts) == 0 {
		t.Fatal("expected error_contracts facts")
	}
	if env.ErrorContracts[0].Status != "NOT_FOUND" {
		t.Errorf("error status = %q, want NOT_FOUND", env.ErrorContracts[0].Status)
	}
	if env.ErrorContracts[0].HTTPCode != 404 {
		t.Errorf("error http_code = %d, want 404", env.ErrorContracts[0].HTTPCode)
	}
	foundAnyRequest := false
	for _, r := range env.SecurityRules {
		if r.Pattern == "anyRequest" && r.Requirement == "authenticated" {
			foundAnyRequest = true
			break
		}
	}
	if !foundAnyRequest {
		t.Fatalf("expected anyRequest.authenticated security rule, got: %+v", env.SecurityRules)
	}
}

// ---------------------------------------------------------------------------
// javaTypeToCUE table tests
// ---------------------------------------------------------------------------

func TestJavaTypeToCUE(t *testing.T) {
	cases := []struct {
		javaType string
		want     string
	}{
		{"String", "string"},
		{"int", "int"},
		{"Integer", "int"},
		{"long", "int"},
		{"Long", "int"},
		{"double", "float"},
		{"BigDecimal", "float"},
		{"boolean", "bool"},
		{"Boolean", "bool"},
		{"UUID", "string"},
		{"LocalDateTime", "string"},
		{"Optional<String>", "string"},
		{"List<String>", "[...string]"},
		{"List<User>", "[...string]"},
		{"Map<String, Object>", "{...}"},
		{"void", "_"},
		{"Object", "_"},
	}
	for _, c := range cases {
		got := javaTypeToCUE(c.javaType)
		if got != c.want {
			t.Errorf("javaTypeToCUE(%q) = %q, want %q", c.javaType, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// goTypeToCUE table tests
// ---------------------------------------------------------------------------

func TestGoTypeToCUE(t *testing.T) {
	cases := []struct {
		goType string
		want   string
	}{
		{"string", "string"},
		{"int", "int"},
		{"int64", "int"},
		{"uint32", "int"},
		{"float64", "float"},
		{"bool", "bool"},
		{"time.Time", "string"},
		{"uuid.UUID", "string"},
		{"*string", "string"},
		{"[]string", "[...string]"},
		{"[]int", "[...int]"},
		{"map[string]string", "{...}"},
		{"interface{}", "_"},
		{"any", "_"},
		{"*User", "string"}, // unknown struct → string
	}
	for _, c := range cases {
		got := goTypeToCUE(c.goType)
		if got != c.want {
			t.Errorf("goTypeToCUE(%q) = %q, want %q", c.goType, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// sqlTypeToCUE table tests
// ---------------------------------------------------------------------------

func TestSQLTypeToCUE(t *testing.T) {
	cases := []struct {
		sqlType string
		want    string
	}{
		{"VARCHAR(255)", "string"},
		{"TEXT", "string"},
		{"INT", "int"},
		{"BIGINT", "int"},
		{"SERIAL", "int"},
		{"BOOLEAN", "bool"},
		{"BOOL", "bool"},
		{"DECIMAL(10,2)", "float"},
		{"FLOAT", "float"},
		{"TIMESTAMP", "string"},
		{"TIMESTAMP WITH TIME ZONE", "string"},
		{"DATE", "string"},
		{"UUID", "string"},
		{"JSONB", "{...}"},
		{"JSON", "{...}"},
		{"BYTEA", "bytes"},
	}
	for _, c := range cases {
		got := sqlTypeToCUE(c.sqlType)
		if got != c.want {
			t.Errorf("sqlTypeToCUE(%q) = %q, want %q", c.sqlType, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func entityNames(entities []FactEntity) []string {
	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Name
	}
	return names
}

func repoEntities(repos []FactRepo) []string {
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Entity
	}
	return names
}

func opNames(ops []FactOp) []string {
	names := make([]string, len(ops))
	for i, o := range ops {
		names[i] = o.Name
	}
	return names
}
