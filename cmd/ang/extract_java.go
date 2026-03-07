package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Java extraction — pure regex, no external deps.
// Targets: Spring Boot / JPA / standard Java domain patterns.

var (
	// Class/interface declaration
	javaClassRe = regexp.MustCompile(`(?m)^(?:public\s+)?(?:abstract\s+)?(?:class|record)\s+(\w+)(?:\s+extends\s+[\w<>, .]+)?(?:\s+implements\s+([\w<>, .]+))?\s*\{`)
	javaIfaceRe = regexp.MustCompile(`(?m)^(?:public\s+)?interface\s+(\w+)(?:\s+extends\s+([\w<>, .]+))?\s*\{`)

	// Annotations
	javaEntityAnnoRe         = regexp.MustCompile(`@(?:Entity|Table|Document|MappedSuperclass)\b`)
	javaServiceAnnoRe        = regexp.MustCompile(`@(?:Service|Component|RestController|Controller)\b`)
	javaControllerAnnoRe     = regexp.MustCompile(`@(?:RestController|Controller)\b`)
	javaControllerAdviceRe   = regexp.MustCompile(`@ControllerAdvice\b`)
	javaMapperAnnoRe         = regexp.MustCompile(`@Mapper\b`)
	javaDomainEventRe        = regexp.MustCompile(`@(?:DomainEvent|EventHandler)\b`)
	javaPreAuthorizeRe       = regexp.MustCompile(`@PreAuthorize\s*\(\s*"([^"]+)"\s*\)`)
	javaSecuredRe            = regexp.MustCompile(`@Secured\s*\(\s*"([^"]+)"\s*\)`)
	javaRolesAllowedRe       = regexp.MustCompile(`@RolesAllowed\s*\(\s*\{?([^)]*?)\}?\s*\)`)
	javaTransactionalRe      = regexp.MustCompile(`@Transactional(?:\s*\(([^)]*)\))?`)
	javaGetMappingRe         = regexp.MustCompile(`@GetMapping(?:\s*\(([^)]*)\))?`)
	javaPostMappingRe        = regexp.MustCompile(`@PostMapping(?:\s*\(([^)]*)\))?`)
	javaPutMappingRe         = regexp.MustCompile(`@PutMapping(?:\s*\(([^)]*)\))?`)
	javaPatchMappingRe       = regexp.MustCompile(`@PatchMapping(?:\s*\(([^)]*)\))?`)
	javaDeleteMappingRe      = regexp.MustCompile(`@DeleteMapping(?:\s*\(([^)]*)\))?`)
	javaRequestMappingRe     = regexp.MustCompile(`@RequestMapping(?:\s*\(([^)]*)\))?`)
	javaExceptionHandlerRe   = regexp.MustCompile(`@ExceptionHandler\s*\(([^)]*)\)`)
	javaEnableMethodSecRe    = regexp.MustCompile(`@EnableMethodSecurity\b`)
	javaAnyRequestAuthRe     = regexp.MustCompile(`anyRequest\s*\(\s*\)\s*\.\s*(authenticated|permitAll)\s*\(\s*\)`)
	javaRequestMatcherRe     = regexp.MustCompile(`requestMatchers\s*\(([^)]*)\)\s*\.\s*(authenticated|permitAll)\s*\(\s*\)`)
	javaRequestMatcherRoleRe = regexp.MustCompile(`requestMatchers\s*\(([^)]*)\)\s*\.\s*(hasRole|hasAnyRole)\s*\(([^)]*)\)`)

	// JPA / Spring Data repo parent
	javaRepoParentRe = regexp.MustCompile(`(?i)extends\s+([\w<>,\s]+(?:Repository|CrudRepository|JpaRepository|MongoRepository|ReactiveCrudRepository|PagingAndSortingRepository)[\w<>,\s]*)`)

	// Field declaration (instance field): modifiers type name ;
	javaFieldRe            = regexp.MustCompile(`(?m)^\s+(?:(?:private|protected|public|final|static|transient|volatile)\s+)+(\w[\w<>\[\].,\s]*?)\s+(\w+)\s*;`)
	javaCtorFieldRe        = regexp.MustCompile(`(?m)^\s*(?:private|protected|public)\s+(?:final\s+)?([\w<>\[\].,?\s]+?)\s+(\w+)\s*;`)
	javaMethodCallRe       = regexp.MustCompile(`(?m)(?:this\.)?([a-zA-Z_]\w*)\.(\w+)\s*\(`)
	javaHTTPStatusInBodyRe = regexp.MustCompile(`HttpStatus\.([A-Z_]+)`)

	// @Column, @Id, @JsonProperty annotations on next line
	javaColumnAnnoRe = regexp.MustCompile(`@(?:Column|Id|GeneratedValue|JsonProperty|JsonAlias)\b`)
	javaColumnNameRe = regexp.MustCompile(`@Column\s*\([^)]*name\s*=\s*"([^"]+)"`)
	javaJsonPropRe   = regexp.MustCompile(`@JsonProperty\s*\(\s*"([^"]+)"\s*\)`)

	// Interface method (allows annotation block before declaration)
	javaIfaceMethodWithAnnoRe = regexp.MustCompile(`(?ms)((?:\s*@[^\n]+\n)*)\s*(?:(?:default|public)\s+)?([\w<>\[\].,?\s]+?)\s+(\w+)\s*\(([^)]*)\)\s*(?:throws\s+([^;]+))?;`)

	// Class method (annotation block + body)
	javaClassMethodWithAnnoRe = regexp.MustCompile(`(?ms)((?:\s*@[^\n]+\n)*)\s*public\s+([\w<>\[\].,?\s]+?)\s+(\w+)\s*\(([^)]*)\)\s*(?:throws\s+([^\{]+))?\{`)

	javaMapperUsesRe = regexp.MustCompile(`uses\s*=\s*\{?([^)}]+)\}?`)

	javaNotNullAnnoRe = regexp.MustCompile(`@(?:NotNull|NotBlank|NotEmpty)\b`)
	javaNullAnnoRe    = regexp.MustCompile(`@Null\b`)
	javaSizeAnnoRe    = regexp.MustCompile(`@Size\s*\(([^)]*)\)`)
	javaLengthAnnoRe  = regexp.MustCompile(`@Length\s*\(([^)]*)\)`)
	javaPatternAnnoRe = regexp.MustCompile(`@Pattern\s*\(([^)]*)\)`)
	javaEmailAnnoRe   = regexp.MustCompile(`@Email\b`)
	javaMinAnnoRe     = regexp.MustCompile(`@Min\s*\(\s*([0-9]+)\s*\)`)
	javaMaxAnnoRe     = regexp.MustCompile(`@Max\s*\(\s*([0-9]+)\s*\)`)
	javaDecMinAnnoRe  = regexp.MustCompile(`@DecimalMin\s*\(\s*"([^"]+)"(?:\s*,\s*inclusive\s*=\s*(true|false))?\s*\)`)
	javaDecMaxAnnoRe  = regexp.MustCompile(`@DecimalMax\s*\(\s*"([^"]+)"(?:\s*,\s*inclusive\s*=\s*(true|false))?\s*\)`)
	javaDigitsAnnoRe  = regexp.MustCompile(`@Digits\s*\(([^)]*)\)`)
	javaPastAnnoRe    = regexp.MustCompile(`@Past\b`)
	javaPastNowAnnoRe = regexp.MustCompile(`@PastOrPresent\b`)
	javaFutureAnnoRe  = regexp.MustCompile(`@Future\b`)
	javaFutureNowAnnoRe = regexp.MustCompile(`@FutureOrPresent\b`)
	javaPositiveAnnoRe = regexp.MustCompile(`@Positive\b`)
	javaPosOrZeroAnnoRe = regexp.MustCompile(`@PositiveOrZero\b`)
	javaNegativeAnnoRe = regexp.MustCompile(`@Negative\b`)
	javaNegOrZeroAnnoRe = regexp.MustCompile(`@NegativeOrZero\b`)
	javaAssertTrueAnnoRe = regexp.MustCompile(`@AssertTrue\b`)
	javaAssertFalseAnnoRe = regexp.MustCompile(`@AssertFalse\b`)
	javaValidAnnoRe = regexp.MustCompile(`@Valid\b`)
	javaNamedArgIntRe = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*=\s*([0-9]+)\b`)
	javaNamedArgStrRe = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*=\s*"([^"]+)"`)
	javaNamedArgBoolRe = regexp.MustCompile(`\b([a-zA-Z_]\w*)\s*=\s*(true|false)\b`)
	javaFirstQuoteRe  = regexp.MustCompile(`"([^"]+)"`)
	javaInlineAnnoRe  = regexp.MustCompile(`@\w+(?:\([^)]*\))?\s*`)
	javaGenericTypeRe = regexp.MustCompile(`<[^>]+>`)
	javaWhitespaceRe  = regexp.MustCompile(`\s+`)
	javaAnnotationNameRe = regexp.MustCompile(`@([a-zA-Z_][\w.]*)`)

	javaOneToManyRe      = regexp.MustCompile(`@OneToMany(?:\s*\(([^)]*)\))?`)
	javaManyToOneRe      = regexp.MustCompile(`@ManyToOne(?:\s*\(([^)]*)\))?`)
	javaOneToOneRe       = regexp.MustCompile(`@OneToOne(?:\s*\(([^)]*)\))?`)
	javaManyToManyRe     = regexp.MustCompile(`@ManyToMany(?:\s*\(([^)]*)\))?`)
	javaElementCollectionRe = regexp.MustCompile(`@ElementCollection(?:\s*\(([^)]*)\))?`)
	javaJoinColumnRe     = regexp.MustCompile(`@JoinColumn\s*\(([^)]*)\)`)
	javaJoinTableRe      = regexp.MustCompile(`@JoinTable\s*\(([^)]*)\)`)
	javaMapsIDRe         = regexp.MustCompile(`@MapsId(?:\s*\(\s*"([^"]*)"\s*\))?`)
	javaEnumeratedRe     = regexp.MustCompile(`@Enumerated\s*\(\s*EnumType\.([A-Z_]+)\s*\)`)
	javaEmbeddedIDRe     = regexp.MustCompile(`@EmbeddedId\b`)
	javaIDRe             = regexp.MustCompile(`@Id\b`)
	javaGeneratedValueRe = regexp.MustCompile(`@GeneratedValue\b`)
	javaEmbeddedRe       = regexp.MustCompile(`@Embedded\b`)
	javaColumnNullableRe = regexp.MustCompile(`@Column\s*\(([^)]*)\)`)

	javaIDClassRe        = regexp.MustCompile(`@IdClass\s*\(\s*([A-Za-z_][\w.]*)\.class\s*\)`)
	javaSQLDeleteRe      = regexp.MustCompile(`@SQLDelete\s*\(([^)]*)\)`)
	javaWhereRe          = regexp.MustCompile(`@Where\s*\(([^)]*)\)`)
	javaFilterRe         = regexp.MustCompile(`@Filter\s*\(([^)]*)\)`)
	javaSQLRestrictionRe = regexp.MustCompile(`@SQLRestriction\s*\(([^)]*)\)`)
	javaRequestParamRe   = regexp.MustCompile(`@RequestParam(?:\s*\(([^)]*)\))?`)
	javaRequestBodyRe    = regexp.MustCompile(`@RequestBody(?:\s*\(([^)]*)\))?`)
	javaPathVarRe        = regexp.MustCompile(`@PathVariable(?:\s*\(([^)]*)\))?`)
	javaRequestPartRe    = regexp.MustCompile(`@RequestPart(?:\s*\(([^)]*)\))?`)
)

type javaMethod struct {
	Name           string
	ReturnType     string
	Params         string
	Throws         []string
	AnnotationsRaw string
	Body           string
	AuthExpr       string
	Transactional  bool
	TxReadOnly     bool
	HTTPMethod     string
	HTTPPath       string
}

type javaExtractOptions struct {
	MergeOpenAPI bool
}

func extractJavaFacts(root string) (FactsEnvelope, error) {
	return extractJavaFactsWithOptions(root, javaExtractOptions{MergeOpenAPI: true})
}

func extractJavaFactsWithOptions(root string, opts javaExtractOptions) (FactsEnvelope, error) {
	var env FactsEnvelope

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == "build" || base == "target" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".java") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		parseJavaFile(string(data), path, &env)
		return nil
	})
	if err != nil {
		return env, err
	}
	if opts.MergeOpenAPI {
		enrichJavaOpsWithOpenAPI(root, &env)
	}
	normalizeJavaFacts(&env)
	return env, nil
}

func parseJavaFile(content, source string, env *FactsEnvelope) {
	parseJavaSecurityHints(content, source, env)

	// Determine if file has entity/service/event/repo markers
	hasEntityAnno := javaEntityAnnoRe.MatchString(content)
	hasServiceAnno := javaServiceAnnoRe.MatchString(content)
	hasDomainEvent := javaDomainEventRe.MatchString(content)

	// --- Interfaces ---
	ifaceMatches := javaIfaceRe.FindAllStringSubmatchIndex(content, -1)
	for _, idx := range ifaceMatches {
		name := content[idx[2]:idx[3]]
		extendsStr := ""
		if idx[4] >= 0 {
			extendsStr = content[idx[4]:idx[5]]
		}
		annos := extractLeadingAnnotations(content, idx[0])
		annoBlock := strings.Join(annos, "\n")
		body := extractJavaBody(content, idx[1])

		switch {
		case isJavaRepoIface(name, extendsStr):
			entity := javaRepoEntityName(name, extendsStr)
			methods := extractJavaRepoMethods(body)
			env.Repositories = append(env.Repositories, FactRepo{
				Entity:  entity,
				Source:  source,
				Methods: methods,
			})
		case javaMapperAnnoRe.MatchString(annoBlock) || strings.HasSuffix(name, "Mapper"):
			mapper := extractJavaMapper(name, source, annoBlock, body)
			if mapper.Name != "" {
				env.Mappers = append(env.Mappers, mapper)
			}
		case isJavaServiceIface(name) || hasServiceAnno || javaControllerAnnoRe.MatchString(annoBlock):
			hint := javaServiceHint(name)
			kind := "service"
			if javaControllerAnnoRe.MatchString(annoBlock) {
				kind = "controller"
			}
			classBasePath := parseClassBasePath(annoBlock)
			classAuthExpr := parseAuthExpr(annoBlock)
			classTx, classTxReadOnly := parseTransactional(annoBlock)
			ops, endpoints := extractJavaInterfaceOps(body, hint, source, name, kind, classBasePath, classAuthExpr, classTx, classTxReadOnly)
			env.Operations = append(env.Operations, ops...)
			env.Endpoints = append(env.Endpoints, endpoints...)
		}
	}

	// --- Classes / Records ---
	classMatches := javaClassRe.FindAllStringSubmatchIndex(content, -1)
	for _, idx := range classMatches {
		name := content[idx[2]:idx[3]]
		implementsStr := ""
		if idx[4] >= 0 {
			implementsStr = content[idx[4]:idx[5]]
		}
		annos := extractLeadingAnnotations(content, idx[0])
		annoBlock := strings.Join(annos, "\n")
		body := extractJavaBody(content, idx[1])

		classBasePath := parseClassBasePath(annoBlock)
		classAuthExpr := parseAuthExpr(annoBlock)
		classTx, classTxReadOnly := parseTransactional(annoBlock)
		classKind := classifyJavaClassKind(name, annoBlock)
		fieldTypes := extractJavaFieldTypeMap(body)

		switch {
		case hasDomainEvent || isJavaEventName(name):
			fields := extractJavaFields(body)
			env.Events = append(env.Events, FactEvent{
				Name:          name,
				Source:        source,
				PayloadFields: fields,
			})
		case hasEntityAnno || isJavaEntityName(name):
			fields := extractJavaFields(body)
			compositeKey, softDelete, softDeleteStrategy, softDeleteClause, whereClause := parseEntityPersistenceHints(annoBlock, fields)
			env.Entities = append(env.Entities, FactEntity{
				Name:               name,
				TableHint:          javaTableHint(name, content),
				Source:             source,
				Fields:             fields,
				CompositeKey:       compositeKey,
				SoftDelete:         softDelete,
				SoftDeleteStrategy: softDeleteStrategy,
				SoftDeleteClause:   softDeleteClause,
				WhereClause:        whereClause,
			})
		}

		if javaControllerAdviceRe.MatchString(annoBlock) || strings.HasSuffix(name, "Advice") {
			errContracts := extractJavaErrorContracts(name, source, body)
			env.ErrorContracts = append(env.ErrorContracts, errContracts...)
		}

		if isJavaServiceName(name) || hasServiceAnno || classKind == "controller" {
			hint := javaServiceHint(name)
			if classKind == "controller" {
				hint = strings.ToLower(name)
			}
			ops, endpoints, edges := extractJavaClassOps(body, hint, source, name, classKind, classBasePath, classAuthExpr, classTx, classTxReadOnly, fieldTypes)
			env.Operations = append(env.Operations, ops...)
			env.Endpoints = append(env.Endpoints, endpoints...)
			env.Calls = append(env.Calls, edges...)
		}

		if javaMapperAnnoRe.MatchString(annoBlock) || strings.HasSuffix(name, "Mapper") {
			mapper := extractJavaMapper(name, source, annoBlock, body)
			if mapper.Name != "" {
				env.Mappers = append(env.Mappers, mapper)
			}
		}

		if implementsStr != "" && strings.Contains(strings.ToLower(implementsStr), "api") && classKind == "controller" {
			env.SecurityRules = append(env.SecurityRules, FactSecurityRule{
				Scope:       "endpoint",
				Pattern:     strings.TrimSpace(name),
				Requirement: "controller-implements-api",
				Source:      source,
			})
		}
	}
}

func parseJavaSecurityHints(content, source string, env *FactsEnvelope) {
	if javaEnableMethodSecRe.MatchString(content) {
		env.SecurityRules = append(env.SecurityRules, FactSecurityRule{
			Scope:       "global",
			Requirement: "method-security-enabled",
			Source:      source,
		})
	}
	for _, m := range javaAnyRequestAuthRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		env.SecurityRules = append(env.SecurityRules, FactSecurityRule{
			Scope:       "global",
			Pattern:     "anyRequest",
			Requirement: strings.TrimSpace(m[1]),
			Source:      source,
		})
	}
	for _, m := range javaRequestMatcherRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 3 {
			continue
		}
		env.SecurityRules = append(env.SecurityRules, FactSecurityRule{
			Scope:       "global",
			Pattern:     cleanQuotedValue(m[1]),
			Requirement: strings.TrimSpace(m[2]),
			Source:      source,
		})
	}
	for _, m := range javaRequestMatcherRoleRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 4 {
			continue
		}
		req := strings.TrimSpace(m[2])
		arg := strings.TrimSpace(cleanQuotedValue(m[3]))
		if arg != "" {
			req += ":" + arg
		}
		env.SecurityRules = append(env.SecurityRules, FactSecurityRule{
			Scope:       "global",
			Pattern:     cleanQuotedValue(m[1]),
			Requirement: req,
			Source:      source,
		})
	}
}

// ---- Repo helpers ----

func isJavaRepoIface(name, extendsStr string) bool {
	for _, suffix := range []string{"Repository", "Repo", "Dao", "Store"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return javaRepoParentRe.MatchString(extendsStr)
}

func javaRepoEntityName(ifaceName, extendsStr string) string {
	// Try to extract from generic: UserRepository extends JpaRepository<User, Long>
	genericRe := regexp.MustCompile(`<\s*(\w+)\s*,`)
	if m := genericRe.FindStringSubmatch(extendsStr); len(m) > 1 {
		return m[1]
	}
	// Strip suffix
	for _, suffix := range []string{"Repository", "Repo", "Dao", "Store"} {
		if strings.HasSuffix(ifaceName, suffix) {
			return strings.TrimSuffix(ifaceName, suffix)
		}
	}
	return ifaceName
}

// ---- Service helpers ----

func isJavaServiceIface(name string) bool {
	for _, suffix := range []string{"Service", "UseCase", "Port", "Application", "Api"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func isJavaServiceName(name string) bool {
	for _, suffix := range []string{"ServiceImpl", "Service", "UseCase", "Application", "Controller"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func javaServiceHint(name string) string {
	for _, suffix := range []string{"RestController", "Controller", "ServiceImpl", "Service", "UseCase", "Port", "Application", "Api"} {
		if strings.HasSuffix(name, suffix) {
			return strings.ToLower(strings.TrimSuffix(name, suffix))
		}
	}
	return strings.ToLower(name)
}

func classifyJavaClassKind(name, annoBlock string) string {
	if javaControllerAdviceRe.MatchString(annoBlock) || strings.HasSuffix(name, "Advice") {
		return "advice"
	}
	if javaControllerAnnoRe.MatchString(annoBlock) || strings.HasSuffix(name, "Controller") {
		return "controller"
	}
	if javaServiceAnnoRe.MatchString(annoBlock) || strings.HasSuffix(name, "Service") || strings.HasSuffix(name, "ServiceImpl") {
		return "service"
	}
	return "other"
}

// ---- Entity helpers ----

func isJavaEntityName(name string) bool {
	for _, suffix := range []string{"Entity", "Model", "DTO", "Dto", "Request", "Response"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func javaTableHint(name, content string) string {
	tableRe := regexp.MustCompile(`@Table\s*\([^)]*name\s*=\s*"([^"]+)"`)
	if m := tableRe.FindStringSubmatch(content); len(m) > 1 {
		return m[1]
	}
	return toSnakePlural(name)
}

// ---- Event helpers ----

func isJavaEventName(name string) bool {
	for _, suffix := range []string{"Event", "Notification", "Payload", "Message"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// ---- Field extraction ----

func extractJavaFields(body string) []FactField {
	var fields []FactField
	lines := strings.Split(body, "\n")
	var pendingAnnos []string
	annoDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Collect annotation hints from preceding lines, including multiline args.
		if strings.HasPrefix(trimmed, "@") || annoDepth > 0 {
			pendingAnnos = append(pendingAnnos, trimmed)
			annoDepth += strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
			if annoDepth < 0 {
				annoDepth = 0
			}
			continue
		}

		m := javaFieldRe.FindStringSubmatch(line)
		if len(m) < 3 {
			pendingAnnos = nil
			continue
		}
		javaType := strings.TrimSpace(m[1])
		fieldName := strings.TrimSpace(m[2])

		// skip static/constant fields (all-caps)
		if fieldName == strings.ToUpper(fieldName) && len(fieldName) > 1 {
			pendingAnnos = nil
			continue
		}
		// skip logger fields
		if strings.Contains(strings.ToLower(javaType), "logger") {
			pendingAnnos = nil
			continue
		}

		annoBlock := strings.Join(pendingAnnos, "\n")
		cueHint := javaTypeToCUE(javaType)
		jsonTag := ""
		dbTag := ""
		validateRules := parseJavaValidationRules(annoBlock)
		required := containsRule(validateRules, "required")
		relationKind, relationTarget, mappedBy, cascade, fetch, orphanRemoval, joinColumn, joinTable, enumType, persistence := parseJavaPersistenceAnnotations(annoBlock, javaType)
		if relationKind == "many_to_one" || relationKind == "one_to_one" {
			if containsTokenInsensitive(persistence, "optional=false") {
				required = true
			}
		}
		if containsTokenInsensitive(persistence, "column_nullable=false") || containsTokenInsensitive(persistence, "id") || containsTokenInsensitive(persistence, "embedded_id") {
			required = true
		}

		if m := javaColumnNameRe.FindStringSubmatch(annoBlock); len(m) > 1 {
			dbTag = strings.TrimSpace(m[1])
		}
		if m := javaJsonPropRe.FindStringSubmatch(annoBlock); len(m) > 1 {
			jsonTag = strings.TrimSpace(m[1])
		}

		exportedName := toPascalCase(fieldName)
		if jsonTag == "" {
			jsonTag = fieldName // Java field name is usually the json name
		}

		fields = append(fields, FactField{
			Name:            exportedName,
			GoType:          javaType,
			CueTypeHint:     cueHint,
			JSONTag:         jsonTag,
			DBTag:           dbTag,
			Validate:        strings.Join(validateRules, ","),
			ValidationRules: validateRules,
			Required:        required,
			RelationKind:    relationKind,
			RelationTarget:  relationTarget,
			MappedBy:        mappedBy,
			Cascade:         cascade,
			Fetch:           fetch,
			OrphanRemoval:   orphanRemoval,
			JoinColumn:      joinColumn,
			JoinTable:       joinTable,
			EnumType:        enumType,
			Persistence:     persistence,
		})
		pendingAnnos = nil
	}
	return fields
}

func parseEntityPersistenceHints(annoBlock string, fields []FactField) (compositeKey string, softDelete bool, strategy string, clause string, whereClause string) {
	trimmed := strings.TrimSpace(annoBlock)
	if trimmed == "" {
		trimmed = ""
	}
	if m := javaIDClassRe.FindStringSubmatch(trimmed); len(m) > 1 {
		compositeKey = "id_class:" + strings.TrimSpace(m[1])
	}
	ids := 0
	for _, f := range fields {
		if containsTokenInsensitive(f.Persistence, "embedded_id") {
			compositeKey = "embedded_id"
		}
		if containsTokenInsensitive(f.Persistence, "id") {
			ids++
		}
		if f.Name == "Deleted" || f.Name == "IsDeleted" || f.Name == "DeletedAt" {
			if !softDelete {
				softDelete = true
				strategy = "field"
				clause = strings.ToLower(f.JSONTag)
			}
		}
	}
	if ids > 1 && compositeKey == "" {
		compositeKey = "multiple_ids"
	}
	if m := javaSQLDeleteRe.FindStringSubmatch(trimmed); len(m) > 1 {
		sql := strings.TrimSpace(m[1])
		if val := extractNamedStringArg(sql, "sql"); val != "" {
			clause = val
		} else if q := javaFirstQuoteRe.FindStringSubmatch(sql); len(q) > 1 {
			clause = strings.TrimSpace(q[1])
		}
		softDelete = true
		strategy = "sql_delete"
	}
	if m := javaWhereRe.FindStringSubmatch(trimmed); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		if val := extractNamedStringArg(args, "clause"); val != "" {
			whereClause = val
			softDelete = true
			if strategy == "" {
				strategy = "where_clause"
			}
			if clause == "" {
				clause = val
			}
		}
	}
	if m := javaSQLRestrictionRe.FindStringSubmatch(trimmed); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		if q := javaFirstQuoteRe.FindStringSubmatch(args); len(q) > 1 {
			whereClause = strings.TrimSpace(q[1])
			softDelete = true
			if strategy == "" {
				strategy = "where_clause"
			}
			if clause == "" {
				clause = whereClause
			}
		}
	}
	if m := javaFilterRe.FindStringSubmatch(trimmed); len(m) > 1 && !softDelete {
		args := strings.TrimSpace(m[1])
		if v := extractNamedStringArg(args, "condition"); v != "" {
			softDelete = true
			strategy = "where_clause"
			whereClause = v
			clause = v
		}
	}
	return compositeKey, softDelete, strategy, clause, whereClause
}

func parseJavaPersistenceAnnotations(annotationBlock, javaType string) (relationKind, relationTarget, mappedBy string, cascade []string, fetch string, orphanRemoval bool, joinColumn string, joinTable string, enumType string, persistence []string) {
	raw := strings.TrimSpace(annotationBlock)
	if raw == "" {
		return "", "", "", nil, "", false, "", "", "", nil
	}
	relationTarget = extractJavaRelationTarget(javaType)
	addPersistence := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		for _, ex := range persistence {
			if strings.EqualFold(ex, v) {
				return
			}
		}
		persistence = append(persistence, v)
	}
	parseRelArgs := func(args string) {
		args = strings.TrimSpace(args)
		if args == "" {
			return
		}
		if v := extractNamedStringArg(args, "mappedBy"); v != "" {
			mappedBy = v
		}
		if vals := extractEnumValues(args, "CascadeType"); len(vals) > 0 {
			cascade = vals
		}
		if vals := extractEnumValues(args, "FetchType"); len(vals) > 0 {
			fetch = strings.ToLower(vals[0])
		}
		if v, ok := extractNamedBoolArg(args, "orphanRemoval"); ok {
			orphanRemoval = v
		}
		if v, ok := extractNamedBoolArg(args, "optional"); ok {
			if !v {
				addPersistence("optional=false")
			} else {
				addPersistence("optional=true")
			}
		}
	}
	if m := javaOneToManyRe.FindStringSubmatch(raw); len(m) > 0 {
		relationKind = "one_to_many"
		if len(m) > 1 {
			parseRelArgs(m[1])
		}
	}
	if m := javaManyToOneRe.FindStringSubmatch(raw); len(m) > 0 {
		relationKind = "many_to_one"
		if len(m) > 1 {
			parseRelArgs(m[1])
		}
	}
	if m := javaOneToOneRe.FindStringSubmatch(raw); len(m) > 0 {
		relationKind = "one_to_one"
		if len(m) > 1 {
			parseRelArgs(m[1])
		}
	}
	if m := javaManyToManyRe.FindStringSubmatch(raw); len(m) > 0 {
		relationKind = "many_to_many"
		if len(m) > 1 {
			parseRelArgs(m[1])
		}
	}
	if m := javaElementCollectionRe.FindStringSubmatch(raw); len(m) > 0 {
		relationKind = "element_collection"
		if len(m) > 1 {
			parseRelArgs(m[1])
		}
	}

	if m := javaJoinColumnRe.FindStringSubmatch(raw); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		joinColumn = extractNamedStringArg(args, "name")
		if joinColumn == "" {
			joinColumn = strings.TrimSpace(args)
		}
		if v, ok := extractNamedBoolArg(args, "nullable"); ok && !v {
			addPersistence("column_nullable=false")
		}
	}
	if m := javaJoinTableRe.FindStringSubmatch(raw); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		joinTable = extractNamedStringArg(args, "name")
	}
	if m := javaMapsIDRe.FindStringSubmatch(raw); len(m) > 0 {
		addPersistence("maps_id")
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			addPersistence("maps_id:" + strings.TrimSpace(m[1]))
		}
	}
	if m := javaEnumeratedRe.FindStringSubmatch(raw); len(m) > 1 {
		enumType = strings.ToLower(strings.TrimSpace(m[1]))
		addPersistence("enum:" + enumType)
	}
	if javaEmbeddedIDRe.MatchString(raw) {
		addPersistence("embedded_id")
	}
	if javaEmbeddedRe.MatchString(raw) {
		addPersistence("embedded")
	}
	if javaIDRe.MatchString(raw) {
		addPersistence("id")
	}
	if javaGeneratedValueRe.MatchString(raw) {
		addPersistence("generated_value")
	}
	if m := javaColumnNullableRe.FindStringSubmatch(raw); len(m) > 1 {
		if v, ok := extractNamedBoolArg(m[1], "nullable"); ok && !v {
			addPersistence("column_nullable=false")
		}
		if v, ok := extractNamedBoolArg(m[1], "unique"); ok && v {
			addPersistence("column_unique=true")
		}
		if v := extractNamedIntArg(m[1], "length"); v != "" {
			addPersistence("column_length=" + v)
		}
		if v := extractNamedIntArg(m[1], "precision"); v != "" {
			addPersistence("column_precision=" + v)
		}
		if v := extractNamedIntArg(m[1], "scale"); v != "" {
			addPersistence("column_scale=" + v)
		}
	}
	return relationKind, relationTarget, mappedBy, uniqueStrings(cascade), fetch, orphanRemoval, joinColumn, joinTable, enumType, persistence
}

func extractJavaFieldTypeMap(body string) map[string]string {
	out := map[string]string{}
	for _, m := range javaCtorFieldRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 3 {
			continue
		}
		t := strings.TrimSpace(m[1])
		n := strings.TrimSpace(m[2])
		if t == "" || n == "" {
			continue
		}
		out[n] = t
	}
	return out
}

// ---- Method / Op extraction ----

func extractJavaRepoMethods(body string) []FactRepoMethod {
	var methods []FactRepoMethod
	for _, m := range javaIfaceMethodWithAnnoRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 4 {
			continue
		}
		returnType := strings.TrimSpace(m[2])
		name := strings.TrimSpace(m[3])
		if name == "" {
			continue
		}
		returns := inferJavaRepoReturns(returnType)
		qKind, criteria := inferJavaRepoQueryMeta(name)
		methods = append(methods, FactRepoMethod{
			Name:           name,
			Returns:        returns,
			QueryKind:      qKind,
			CriteriaFields: criteria,
		})
	}
	return methods
}

func extractJavaInterfaceOps(body, serviceHint, source, className, kind, classBasePath, classAuthExpr string, classTx, classTxReadOnly bool) ([]FactOp, []FactEndpoint) {
	var ops []FactOp
	var endpoints []FactEndpoint
	for _, m := range javaIfaceMethodWithAnnoRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 5 {
			continue
		}
		annotations := strings.TrimSpace(m[1])
		returnType := strings.TrimSpace(m[2])
		name := strings.TrimSpace(m[3])
		params := strings.TrimSpace(m[4])
		throws := splitThrows("")
		if len(m) >= 6 {
			throws = splitThrows(m[5])
		}
		if name == "" {
			continue
		}
		authExpr := parseAuthExpr(annotations)
		if authExpr == "" {
			authExpr = classAuthExpr
		}
		tx, txRO := parseTransactional(annotations)
		if !tx {
			tx = classTx
			txRO = classTxReadOnly
		}
		hMethod, hPath := parseHTTPMappingFromAnnotations(annotations, classBasePath)
		op := FactOp{
			Name:          toPascalCase(name),
			ServiceHint:   serviceHint,
			Source:        source,
			ClassName:     className,
			Kind:          kind,
			HTTPMethod:    hMethod,
			HTTPPath:      hPath,
			AuthExpr:      authExpr,
			Transactional: tx,
			TxReadOnly:    txRO,
			InputFields:   parseJavaParams(params),
			OutputFields:  javaReturnToFields(returnType),
		}
		if len(throws) > 0 {
			op.Calls = append(op.Calls, FactCallRef{Target: "throws:" + strings.Join(throws, ",")})
		}
		ops = append(ops, op)
		if hMethod != "" || hPath != "" {
			endpoints = append(endpoints, FactEndpoint{
				Operation:     op.Name,
				HTTPMethod:    hMethod,
				HTTPPath:      hPath,
				ServiceHint:   serviceHint,
				Source:        source,
				AuthExpr:      authExpr,
				Transactional: tx,
				TxReadOnly:    txRO,
			})
		}
	}
	return ops, endpoints
}

func extractJavaClassOps(body, serviceHint, source, className, classKind, classBasePath, classAuthExpr string, classTx, classTxReadOnly bool, fieldTypes map[string]string) ([]FactOp, []FactEndpoint, []FactCallEdge) {
	methods := extractJavaClassMethods(body)
	var ops []FactOp
	var endpoints []FactEndpoint
	var edges []FactCallEdge

	for _, m := range methods {
		if m.Name == "" || m.Name == "class" || m.Name == "interface" {
			continue
		}
		authExpr := m.AuthExpr
		if authExpr == "" {
			authExpr = classAuthExpr
		}
		tx := m.Transactional
		txRO := m.TxReadOnly
		if !tx {
			tx = classTx
			txRO = classTxReadOnly
		}
		hMethod := m.HTTPMethod
		hPath := m.HTTPPath
		if hMethod == "" && hPath == "" {
			hMethod, hPath = parseHTTPMappingFromAnnotations(m.AnnotationsRaw, classBasePath)
		}
		if classBasePath != "" {
			base := normalizeHTTPPath(classBasePath)
			cur := normalizeHTTPPath(hPath)
			if cur == "" {
				hPath = base
			} else if cur != base && !strings.HasPrefix(cur, strings.TrimSuffix(base, "/")+"/") {
				hPath = joinHTTPPaths(base, cur)
			}
		}

		callRefs, callEdges := extractJavaCallRefs(className, m.Name, m.Body, fieldTypes, source)
		edges = append(edges, callEdges...)
		ops = append(ops, FactOp{
			Name:          toPascalCase(m.Name),
			ServiceHint:   serviceHint,
			Source:        source,
			ClassName:     className,
			Kind:          classKind,
			HTTPMethod:    hMethod,
			HTTPPath:      hPath,
			AuthExpr:      authExpr,
			Transactional: tx,
			TxReadOnly:    txRO,
			InputFields:   parseJavaParams(m.Params),
			OutputFields:  javaReturnToFields(m.ReturnType),
			Calls:         callRefs,
		})
		if hMethod != "" || hPath != "" {
			endpoints = append(endpoints, FactEndpoint{
				Operation:     toPascalCase(m.Name),
				HTTPMethod:    hMethod,
				HTTPPath:      hPath,
				ServiceHint:   serviceHint,
				Source:        source,
				AuthExpr:      authExpr,
				Transactional: tx,
				TxReadOnly:    txRO,
			})
		}
	}
	return ops, endpoints, edges
}

func extractJavaClassMethods(body string) []javaMethod {
	matches := javaClassMethodWithAnnoRe.FindAllStringSubmatchIndex(body, -1)
	methods := make([]javaMethod, 0, len(matches))
	for _, idx := range matches {
		if len(idx) < 12 {
			continue
		}
		ann := strings.TrimSpace(body[idx[2]:idx[3]])
		ret := strings.TrimSpace(body[idx[4]:idx[5]])
		name := strings.TrimSpace(body[idx[6]:idx[7]])
		params := strings.TrimSpace(body[idx[8]:idx[9]])
		throwsRaw := ""
		if idx[10] >= 0 {
			throwsRaw = strings.TrimSpace(body[idx[10]:idx[11]])
		}
		openRel := strings.Index(body[idx[0]:idx[1]], "{")
		if openRel < 0 {
			continue
		}
		openIdx := idx[0] + openRel
		mBody, _ := extractBraceBody(body, openIdx)
		authExpr := parseAuthExpr(ann)
		tx, txRO := parseTransactional(ann)
		hMethod, hPath := parseHTTPMappingFromAnnotations(ann, "")
		methods = append(methods, javaMethod{
			Name:           name,
			ReturnType:     ret,
			Params:         params,
			Throws:         splitThrows(throwsRaw),
			AnnotationsRaw: ann,
			Body:           mBody,
			AuthExpr:       authExpr,
			Transactional:  tx,
			TxReadOnly:     txRO,
			HTTPMethod:     hMethod,
			HTTPPath:       hPath,
		})
	}
	return methods
}

func extractJavaCallRefs(className, methodName, body string, fieldTypes map[string]string, source string) ([]FactCallRef, []FactCallEdge) {
	from := strings.TrimSpace(className) + "." + strings.TrimSpace(methodName)
	seenRef := map[string]struct{}{}
	seenEdge := map[string]struct{}{}
	var refs []FactCallRef
	var edges []FactCallEdge
	for _, m := range javaMethodCallRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 3 {
			continue
		}
		recv := strings.TrimSpace(m[1])
		method := strings.TrimSpace(m[2])
		if recv == "" || method == "" {
			continue
		}
		if strings.ToUpper(recv[:1]) == recv[:1] {
			// Likely static/library call; low signal for migration facts.
			continue
		}
		target := recv + "." + method
		kind := classifyJavaCallKind(recv, fieldTypes[recv], method)
		if kind == "local" {
			continue
		}
		if _, ok := seenRef[target]; !ok {
			refs = append(refs, FactCallRef{Target: target, Kind: kind})
			seenRef[target] = struct{}{}
		}
		edgeKey := from + "|" + target
		if _, ok := seenEdge[edgeKey]; !ok {
			edges = append(edges, FactCallEdge{
				From:   from,
				To:     target,
				Kind:   kind,
				Source: source,
			})
			seenEdge[edgeKey] = struct{}{}
		}
	}
	return refs, edges
}

func classifyJavaCallKind(receiver, receiverType, method string) string {
	h := strings.ToLower(strings.TrimSpace(receiverType))
	if h == "" {
		h = strings.ToLower(strings.TrimSpace(receiver))
	}
	switch {
	case strings.Contains(h, "repository") || strings.HasSuffix(h, "repo") || strings.Contains(h, "dao"):
		return "repo"
	case strings.Contains(h, "service") || strings.Contains(h, "usecase"):
		return "service"
	case strings.Contains(h, "mapper"):
		return "mapper"
	case strings.Contains(h, "client") || strings.Contains(h, "resttemplate") || strings.Contains(h, "http"):
		return "http"
	case strings.Contains(h, "publisher") || strings.Contains(h, "event") || strings.HasPrefix(strings.ToLower(method), "publish"):
		return "event"
	default:
		return "local"
	}
}

func extractJavaMapper(name, source, annoBlock, body string) FactMapper {
	if name == "" {
		return FactMapper{}
	}
	uses := parseMapperUses(annoBlock)
	methods := parseMapperMethods(body)
	if len(methods) == 0 {
		return FactMapper{}
	}
	return FactMapper{
		Name:    name,
		Source:  source,
		Uses:    uses,
		Methods: methods,
	}
}

func parseMapperUses(annoBlock string) []string {
	if !javaMapperAnnoRe.MatchString(annoBlock) {
		return nil
	}
	m := javaMapperUsesRe.FindStringSubmatch(annoBlock)
	if len(m) < 2 {
		return nil
	}
	parts := strings.Split(m[1], ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), ".class"))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return uniqueStrings(out)
}

func parseMapperMethods(body string) []FactMapperMethod {
	var out []FactMapperMethod
	for _, m := range javaIfaceMethodWithAnnoRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 5 {
			continue
		}
		ret := strings.TrimSpace(m[2])
		name := strings.TrimSpace(m[3])
		params := parseJavaParams(strings.TrimSpace(m[4]))
		if name == "" {
			continue
		}
		sourceType := ""
		if len(params) > 0 {
			sourceType = params[0].GoType
		}
		many := strings.HasPrefix(strings.TrimSpace(ret), "List<") || strings.HasPrefix(strings.TrimSpace(ret), "Collection<") || strings.HasPrefix(strings.TrimSpace(ret), "Set<")
		out = append(out, FactMapperMethod{
			Name:       name,
			SourceType: sourceType,
			TargetType: ret,
			Many:       many,
		})
	}
	return out
}

func extractJavaErrorContracts(className, source, body string) []FactErrorContract {
	methods := extractJavaClassMethods(body)
	var out []FactErrorContract
	for _, m := range methods {
		h := javaExceptionHandlerRe.FindStringSubmatch(m.AnnotationsRaw)
		if len(h) < 2 {
			continue
		}
		excs := parseExceptionHandlerTypes(h[1])
		status, code := parseStatusFromMethodBody(m.Body)
		for _, exc := range excs {
			if strings.TrimSpace(exc) == "" {
				continue
			}
			out = append(out, FactErrorContract{
				Exception: exc,
				Status:    status,
				HTTPCode:  code,
				Handler:   className + "." + m.Name,
				Source:    source,
			})
		}
	}
	return out
}

func parseExceptionHandlerTypes(raw string) []string {
	clean := strings.ReplaceAll(raw, "{", "")
	clean = strings.ReplaceAll(clean, "}", "")
	clean = strings.ReplaceAll(clean, ".class", "")
	parts := strings.Split(clean, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 && strings.TrimSpace(clean) != "" {
		out = append(out, strings.TrimSpace(clean))
	}
	return uniqueStrings(out)
}

func parseStatusFromMethodBody(body string) (string, int) {
	m := javaHTTPStatusInBodyRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return "", 0
	}
	status := strings.TrimSpace(m[1])
	return status, httpStatusCode(status)
}

func httpStatusCode(status string) int {
	switch strings.TrimSpace(strings.ToUpper(status)) {
	case "OK":
		return 200
	case "CREATED":
		return 201
	case "NO_CONTENT":
		return 204
	case "BAD_REQUEST":
		return 400
	case "UNAUTHORIZED":
		return 401
	case "FORBIDDEN":
		return 403
	case "NOT_FOUND":
		return 404
	case "CONFLICT":
		return 409
	case "UNPROCESSABLE_ENTITY":
		return 422
	case "INTERNAL_SERVER_ERROR":
		return 500
	default:
		return 0
	}
}

func parseAuthExpr(annotationBlock string) string {
	if m := javaPreAuthorizeRe.FindStringSubmatch(annotationBlock); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if m := javaSecuredRe.FindStringSubmatch(annotationBlock); len(m) > 1 {
		return "secured:" + strings.TrimSpace(m[1])
	}
	if m := javaRolesAllowedRe.FindStringSubmatch(annotationBlock); len(m) > 1 {
		v := cleanQuotedValue(m[1])
		if v != "" {
			return "rolesAllowed:" + v
		}
	}
	return ""
}

func parseTransactional(annotationBlock string) (bool, bool) {
	m := javaTransactionalRe.FindStringSubmatch(annotationBlock)
	if len(m) == 0 {
		return false, false
	}
	if len(m) < 2 {
		return true, false
	}
	args := strings.ToLower(strings.TrimSpace(m[1]))
	if strings.Contains(args, "readonly") && strings.Contains(args, "true") {
		return true, true
	}
	return true, false
}

func parseClassBasePath(annotationBlock string) string {
	for _, line := range strings.Split(annotationBlock, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@RequestMapping") {
			continue
		}
		if m := javaRequestMappingRe.FindStringSubmatch(line); len(m) > 1 {
			return normalizeHTTPPath(extractPathFromArgs(m[1]))
		}
	}
	return ""
}

func parseHTTPMappingFromAnnotations(annotationBlock, classBasePath string) (string, string) {
	method := ""
	path := ""
	if m := javaGetMappingRe.FindStringSubmatch(annotationBlock); len(m) > 0 {
		method = "GET"
		if len(m) > 1 {
			path = extractPathFromArgs(m[1])
		}
	}
	if m := javaPostMappingRe.FindStringSubmatch(annotationBlock); len(m) > 0 {
		method = "POST"
		if len(m) > 1 {
			path = extractPathFromArgs(m[1])
		}
	}
	if m := javaPutMappingRe.FindStringSubmatch(annotationBlock); len(m) > 0 {
		method = "PUT"
		if len(m) > 1 {
			path = extractPathFromArgs(m[1])
		}
	}
	if m := javaPatchMappingRe.FindStringSubmatch(annotationBlock); len(m) > 0 {
		method = "PATCH"
		if len(m) > 1 {
			path = extractPathFromArgs(m[1])
		}
	}
	if m := javaDeleteMappingRe.FindStringSubmatch(annotationBlock); len(m) > 0 {
		method = "DELETE"
		if len(m) > 1 {
			path = extractPathFromArgs(m[1])
		}
	}
	if method == "" {
		if m := javaRequestMappingRe.FindStringSubmatch(annotationBlock); len(m) > 1 {
			args := m[1]
			if reqM := regexp.MustCompile(`RequestMethod\.([A-Z]+)`).FindStringSubmatch(args); len(reqM) > 1 {
				method = strings.ToUpper(strings.TrimSpace(reqM[1]))
			}
			path = extractPathFromArgs(args)
		}
	}
	if classBasePath != "" || path != "" {
		path = joinHTTPPaths(classBasePath, path)
	}
	return method, path
}

func extractPathFromArgs(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}
	if m := regexp.MustCompile(`(?:value|path)\s*=\s*"([^"]+)"`).FindStringSubmatch(args); len(m) > 1 {
		return m[1]
	}
	if m := regexp.MustCompile(`"([^"]+)"`).FindStringSubmatch(args); len(m) > 1 {
		return m[1]
	}
	return ""
}

func normalizeHTTPPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

func joinHTTPPaths(base, path string) string {
	b := normalizeHTTPPath(base)
	p := normalizeHTTPPath(path)
	if b == "" {
		return p
	}
	if p == "" {
		return b
	}
	return normalizeHTTPPath(strings.TrimSuffix(b, "/") + "/" + strings.TrimPrefix(p, "/"))
}

func splitThrows(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.TrimSuffix(p, ";"))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return uniqueStrings(out)
}

func parseJavaParams(params string) []FactField {
	if strings.TrimSpace(params) == "" {
		return nil
	}
	var fields []FactField
	parts := splitTopLevel(params, ',')
	for _, p := range parts {
		p = strings.TrimSpace(p)
		rules := parseJavaValidationRules(p)
		// HTTP param/body annotations can override required/nullability.
		required := containsRule(rules, "required")
		if m := javaRequestParamRe.FindStringSubmatch(p); len(m) > 1 {
			if v, ok := extractNamedBoolArg(m[1], "required"); ok {
				required = v
				if !v {
					rules = appendUniqueRule(rules, "optional")
				}
			}
		}
		if m := javaRequestBodyRe.FindStringSubmatch(p); len(m) > 1 {
			if v, ok := extractNamedBoolArg(m[1], "required"); ok {
				required = v
			} else {
				required = true
			}
		}
		if javaPathVarRe.MatchString(p) {
			required = true
		}
		if javaRequestPartRe.MatchString(p) && !containsRule(rules, "optional") {
			required = true
		}
		// Remove annotations inline: @Valid UserDto dto
		p = javaInlineAnnoRe.ReplaceAllString(p, "")
		p = strings.TrimSpace(p)
		toks := strings.Fields(p)
		if len(toks) < 2 {
			continue
		}
		javaType := strings.Join(toks[:len(toks)-1], " ")
		paramName := toks[len(toks)-1]
		// strip trailing artifacts
		paramName = strings.TrimRight(paramName, ";)")
		if strings.ToLower(javaType) == "void" {
			continue
		}
		fields = append(fields, FactField{
			Name:            paramName,
			GoType:          javaType,
			CueTypeHint:     javaTypeToCUE(javaType),
			Validate:        strings.Join(rules, ","),
			ValidationRules: rules,
			Required:        required,
		})
	}
	return fields
}

func parseJavaValidationAnnotations(raw string) string {
	return strings.Join(parseJavaValidationRules(raw), ",")
}

func parseJavaValidationRules(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var rules []string
	addRule := func(rule string) {
		rules = appendUniqueRule(rules, rule)
	}

	if javaNotNullAnnoRe.MatchString(raw) {
		addRule("required")
	}
	if javaNullAnnoRe.MatchString(raw) {
		addRule("nullable")
	}
	if javaValidAnnoRe.MatchString(raw) {
		addRule("nested_validate")
	}
	if javaEmailAnnoRe.MatchString(raw) {
		addRule("format=email")
	}
	if javaAssertTrueAnnoRe.MatchString(raw) {
		addRule("const=true")
	}
	if javaAssertFalseAnnoRe.MatchString(raw) {
		addRule("const=false")
	}
	if javaPositiveAnnoRe.MatchString(raw) {
		addRule("gt=0")
	}
	if javaPosOrZeroAnnoRe.MatchString(raw) {
		addRule("min=0")
	}
	if javaNegativeAnnoRe.MatchString(raw) {
		addRule("lt=0")
	}
	if javaNegOrZeroAnnoRe.MatchString(raw) {
		addRule("max=0")
	}
	if javaPastAnnoRe.MatchString(raw) {
		addRule("time<present")
	}
	if javaPastNowAnnoRe.MatchString(raw) {
		addRule("time<=present")
	}
	if javaFutureAnnoRe.MatchString(raw) {
		addRule("time>present")
	}
	if javaFutureNowAnnoRe.MatchString(raw) {
		addRule("time>=present")
	}
	if m := javaSizeAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		parseMinMaxArgs(m[1], &rules, "minLen=", "maxLen=")
	}
	if m := javaLengthAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		parseMinMaxArgs(m[1], &rules, "minLen=", "maxLen=")
	}
	if m := javaPatternAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		pattern := extractNamedStringArg(args, "regexp")
		if pattern == "" {
			pattern = extractNamedStringArg(args, "value")
		}
		if pattern == "" {
			if mm := javaFirstQuoteRe.FindStringSubmatch(args); len(mm) > 1 {
				pattern = strings.TrimSpace(mm[1])
			}
		}
		if pattern != "" && !strings.Contains(pattern, ",") {
			addRule("pattern=" + pattern)
		}
	}
	if m := javaMinAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		addRule("min=" + strings.TrimSpace(m[1]))
	}
	if m := javaMaxAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		addRule("max=" + strings.TrimSpace(m[1]))
	}
	if m := javaDecMinAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		v := strings.TrimSpace(m[1])
		inclusive := true
		if len(m) > 2 && strings.EqualFold(strings.TrimSpace(m[2]), "false") {
			inclusive = false
		}
		if inclusive {
			addRule("min=" + v)
		} else {
			addRule("gt=" + v)
		}
	}
	if m := javaDecMaxAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		v := strings.TrimSpace(m[1])
		inclusive := true
		if len(m) > 2 && strings.EqualFold(strings.TrimSpace(m[2]), "false") {
			inclusive = false
		}
		if inclusive {
			addRule("max=" + v)
		} else {
			addRule("lt=" + v)
		}
	}
	if m := javaDigitsAnnoRe.FindStringSubmatch(raw); len(m) > 1 {
		args := strings.TrimSpace(m[1])
		if integer := extractNamedIntArg(args, "integer"); integer != "" {
			addRule("maxIntegerDigits=" + integer)
		}
		if fraction := extractNamedIntArg(args, "fraction"); fraction != "" {
			addRule("maxFractionDigits=" + fraction)
		}
	}

	for _, custom := range parseJavaCustomValidationAnnotations(raw) {
		addRule("custom=" + custom)
	}
	return rules
}

func parseMinMaxArgs(args string, rules *[]string, minPrefix, maxPrefix string) {
	args = strings.TrimSpace(args)
	if args == "" {
		return
	}
	for _, mm := range javaNamedArgIntRe.FindAllStringSubmatch(args, -1) {
		if len(mm) < 3 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(mm[1])) {
		case "min":
			*rules = appendUniqueRule(*rules, minPrefix+strings.TrimSpace(mm[2]))
		case "max":
			*rules = appendUniqueRule(*rules, maxPrefix+strings.TrimSpace(mm[2]))
		}
	}
}

func parseJavaCustomValidationAnnotations(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	known := map[string]struct{}{
		"NotNull": {}, "NotBlank": {}, "NotEmpty": {}, "Null": {}, "Size": {}, "Length": {}, "Pattern": {},
		"Min": {}, "Max": {}, "DecimalMin": {}, "DecimalMax": {}, "Digits": {}, "Email": {}, "Past": {}, "PastOrPresent": {},
		"Future": {}, "FutureOrPresent": {}, "Positive": {}, "PositiveOrZero": {}, "Negative": {}, "NegativeOrZero": {},
		"AssertTrue": {}, "AssertFalse": {}, "Valid": {},
		// non-validation common annotations to ignore
		"RequestParam": {}, "RequestBody": {}, "PathVariable": {}, "RequestPart": {}, "JsonProperty": {}, "Column": {},
		"Id": {}, "GeneratedValue": {}, "EmbeddedId": {}, "Embedded": {}, "OneToMany": {}, "ManyToOne": {}, "OneToOne": {},
		"ManyToMany": {}, "JoinColumn": {}, "JoinTable": {}, "MapsId": {}, "Enumerated": {}, "ElementCollection": {},
	}
	var out []string
	for _, m := range javaAnnotationNameRe.FindAllStringSubmatch(raw, -1) {
		if len(m) < 2 {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		parts := strings.Split(name, ".")
		simple := parts[len(parts)-1]
		if _, ok := known[simple]; ok {
			continue
		}
		if strings.HasSuffix(simple, "Validator") || strings.HasSuffix(simple, "Constraint") || strings.HasPrefix(strings.ToLower(simple), "valid") {
			out = append(out, simple)
		}
	}
	return uniqueStrings(out)
}

func appendUniqueRule(rules []string, rule string) []string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return rules
	}
	for _, ex := range rules {
		if ex == rule {
			return rules
		}
	}
	return append(rules, rule)
}

func containsRule(rules []string, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r == prefix || strings.HasPrefix(r, prefix+"=") {
			return true
		}
	}
	return false
}

func containsTokenInsensitive(tokens []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return false
	}
	for _, t := range tokens {
		if strings.ToLower(strings.TrimSpace(t)) == want {
			return true
		}
	}
	return false
}

func extractNamedStringArg(args, key string) string {
	args = strings.TrimSpace(args)
	key = strings.TrimSpace(key)
	if args == "" || key == "" {
		return ""
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]+)"`)
	if m := re.FindStringSubmatch(args); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractNamedBoolArg(args, key string) (bool, bool) {
	args = strings.TrimSpace(args)
	key = strings.TrimSpace(key)
	if args == "" || key == "" {
		return false, false
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(key) + `\s*=\s*(true|false)`)
	if m := re.FindStringSubmatch(args); len(m) > 1 {
		return strings.EqualFold(strings.TrimSpace(m[1]), "true"), true
	}
	return false, false
}

func extractNamedIntArg(args, key string) string {
	args = strings.TrimSpace(args)
	key = strings.TrimSpace(key)
	if args == "" || key == "" {
		return ""
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(key) + `\s*=\s*([0-9]+)`)
	if m := re.FindStringSubmatch(args); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractEnumValues(args, enumName string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	re := regexp.MustCompile(regexp.QuoteMeta(enumName) + `\.([A-Z_]+)`)
	matches := re.FindAllStringSubmatch(args, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		out = append(out, strings.ToUpper(strings.TrimSpace(m[1])))
	}
	return uniqueStrings(out)
}

func extractJavaRelationTarget(javaType string) string {
	t := strings.TrimSpace(javaType)
	if t == "" {
		return ""
	}
	if strings.HasPrefix(t, "Optional<") && strings.HasSuffix(t, ">") {
		return extractJavaRelationTarget(t[len("Optional<") : len(t)-1])
	}
	if strings.HasPrefix(t, "List<") && strings.HasSuffix(t, ">") {
		return extractJavaRelationTarget(t[len("List<") : len(t)-1])
	}
	if strings.HasPrefix(t, "Set<") && strings.HasSuffix(t, ">") {
		return extractJavaRelationTarget(t[len("Set<") : len(t)-1])
	}
	if strings.HasPrefix(t, "Collection<") && strings.HasSuffix(t, ">") {
		return extractJavaRelationTarget(t[len("Collection<") : len(t)-1])
	}
	if strings.HasPrefix(t, "Map<") {
		return ""
	}
	if i := strings.Index(t, "<"); i > 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.TrimPrefix(t, "? extends ")
	t = strings.TrimPrefix(t, "? super ")
	t = strings.TrimPrefix(t, "?")
	if t == "" {
		return ""
	}
	if strings.Contains(t, ".") {
		parts := strings.Split(t, ".")
		t = parts[len(parts)-1]
	}
	if strings.EqualFold(t, "string") || strings.EqualFold(t, "int") || strings.EqualFold(t, "long") || strings.EqualFold(t, "boolean") {
		return ""
	}
	return strings.TrimSpace(t)
}

func splitTopLevel(s string, sep rune) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	depthParen := 0
	depthAngle := 0
	depthBrace := 0
	inString := false
	escaped := false
	for i, r := range s {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '<':
			depthAngle++
		case '>':
			if depthAngle > 0 {
				depthAngle--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		default:
			if r == sep && depthParen == 0 && depthAngle == 0 && depthBrace == 0 {
				part := strings.TrimSpace(s[start:i])
				if part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}
	if start <= len(s) {
		part := strings.TrimSpace(s[start:])
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func javaReturnToFields(returnType string) []FactField {
	if returnType == "void" || returnType == "Void" || returnType == "" {
		return nil
	}
	return []FactField{{
		Name:        "_",
		GoType:      returnType,
		CueTypeHint: javaTypeToCUE(returnType),
	}}
}

func inferJavaRepoReturns(returnType string) string {
	r := strings.TrimSpace(returnType)
	switch {
	case r == "void" || r == "Void":
		return "none"
	case strings.HasPrefix(r, "List<") || strings.HasPrefix(r, "Collection<") ||
		strings.HasPrefix(r, "Iterable<") || strings.HasPrefix(r, "Page<") ||
		strings.HasPrefix(r, "Flux<") || strings.HasPrefix(r, "Stream<") ||
		strings.HasSuffix(r, "[]"):
		return "many"
	case strings.HasPrefix(r, "Optional<") || strings.HasPrefix(r, "Mono<"):
		return "one"
	case r == "long" || r == "Long" || r == "int" || r == "Integer" || r == "boolean" || r == "Boolean":
		return "count"
	default:
		return "one"
	}
}

func inferJavaRepoQueryMeta(name string) (string, []string) {
	method := strings.TrimSpace(name)
	lower := strings.ToLower(method)
	switch {
	case strings.HasPrefix(lower, "count"):
		return "count", parseCriteriaFieldsFromMethodName(method)
	case strings.HasPrefix(lower, "exists"):
		return "exists", parseCriteriaFieldsFromMethodName(method)
	case strings.HasPrefix(lower, "delete") || strings.HasPrefix(lower, "remove"):
		return "delete", parseCriteriaFieldsFromMethodName(method)
	case strings.HasPrefix(lower, "save") || strings.HasPrefix(lower, "update"):
		return "write", nil
	case strings.HasPrefix(lower, "find") || strings.HasPrefix(lower, "read") || strings.HasPrefix(lower, "get"):
		return "find", parseCriteriaFieldsFromMethodName(method)
	default:
		return "other", parseCriteriaFieldsFromMethodName(method)
	}
}

func parseCriteriaFieldsFromMethodName(name string) []string {
	idx := strings.Index(name, "By")
	if idx < 0 || idx+2 >= len(name) {
		return nil
	}
	part := name[idx+2:]
	if oi := strings.Index(part, "OrderBy"); oi >= 0 {
		part = part[:oi]
	}
	if part == "" {
		return nil
	}
	parts := splitByConjunctions(part)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = trimQueryOperatorSuffix(p)
		if p == "" {
			continue
		}
		out = append(out, lowerFirst(p))
	}
	return uniqueStrings(out)
}

func splitByConjunctions(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "And") || strings.HasPrefix(s[i:], "Or") {
			if i > start {
				out = append(out, s[start:i])
			}
			if strings.HasPrefix(s[i:], "And") {
				i += 2
			} else {
				i += 1
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimQueryOperatorSuffix(s string) string {
	suffixes := []string{
		"IsNotNull", "IsNull", "NotNull", "NotIn", "IgnoreCase", "StartingWith", "EndingWith",
		"Containing", "Contains", "Between", "GreaterThanEqual", "GreaterThan", "LessThanEqual",
		"LessThan", "Like", "Not", "In", "True", "False", "Equals", "Is",
	}
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) && len(s) > len(suf) {
			return strings.TrimSuffix(s, suf)
		}
	}
	return s
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = []rune(strings.ToLower(string(r[0])))[0]
	return string(r)
}

// ---- Type mapping ----

// javaTypeToCUE maps Java type -> CUE type hint.
func javaTypeToCUE(javaType string) string {
	t := strings.TrimSpace(javaType)
	// unwrap Optional<X>, Mono<X>, Flux<X> -> recurse on X
	for _, wrapper := range []string{"Optional", "Mono", "CompletableFuture", "ResponseEntity", "HttpEntity", "ApiResponse"} {
		if strings.HasPrefix(t, wrapper+"<") && strings.HasSuffix(t, ">") {
			inner := t[len(wrapper)+1 : len(t)-1]
			return javaTypeToCUE(inner)
		}
	}
	// Lists
	for _, wrapper := range []string{"List", "Collection", "Set", "Iterable", "Stream", "Flux", "Page", "Slice"} {
		if strings.HasPrefix(t, wrapper+"<") && strings.HasSuffix(t, ">") {
			inner := t[len(wrapper)+1 : len(t)-1]
			return "[..." + javaTypeToCUE(inner) + "]"
		}
	}
	// Arrays
	if strings.HasSuffix(t, "[]") {
		return "[..." + javaTypeToCUE(t[:len(t)-2]) + "]"
	}
	// Map
	if strings.HasPrefix(t, "Map<") {
		return "{...}"
	}
	switch t {
	case "String":
		return "string"
	case "int", "Integer", "long", "Long", "short", "Short", "byte", "Byte":
		return "int"
	case "double", "Double", "float", "Float", "BigDecimal", "BigInteger":
		return "float"
	case "boolean", "Boolean":
		return "bool"
	case "UUID":
		return "string"
	case "LocalDate", "LocalDateTime", "ZonedDateTime", "OffsetDateTime", "Instant", "Date":
		return "string"
	case "Object", "?", "T":
		return "_"
	case "void", "Void":
		return "_"
	case "byte[]":
		return "bytes"
	default:
		// unknown class -> string hint
		return "string"
	}
}

// ---- Body extraction ----

// extractJavaBody returns the content between the first { and matching }
// starting from offset startIdx in content.
func extractJavaBody(content string, startIdx int) string {
	// startIdx points just after the opening brace position in the regex match.
	openIdx := strings.Index(content[startIdx-1:], "{")
	if openIdx < 0 {
		return ""
	}
	start := startIdx - 1 + openIdx + 1
	depth := 1
	i := start
	for i < len(content) && depth > 0 {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		i++
	}
	if depth != 0 {
		return content[start:]
	}
	return content[start : i-1]
}

func extractBraceBody(content string, openBraceIdx int) (string, int) {
	if openBraceIdx < 0 || openBraceIdx >= len(content) || content[openBraceIdx] != '{' {
		return "", openBraceIdx
	}
	depth := 1
	i := openBraceIdx + 1
	for i < len(content) && depth > 0 {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		i++
	}
	if depth != 0 {
		return content[openBraceIdx+1:], len(content)
	}
	return content[openBraceIdx+1 : i-1], i
}

func extractLeadingAnnotations(content string, declStart int) []string {
	if declStart <= 0 || declStart > len(content) {
		return nil
	}
	prefix := content[:declStart]
	lines := strings.Split(prefix, "\n")
	var out []string
	capturing := false
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if capturing {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			if capturing {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "@") {
			capturing = true
			out = append([]string{line}, out...)
			continue
		}
		break
	}
	return out
}

func cleanQuotedValue(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	parts := strings.Split(raw, ",")
	vals := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"`)
		if p == "" {
			continue
		}
		vals = append(vals, p)
	}
	return strings.Join(vals, ",")
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func normalizeFactID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func mergeFactFields(a, b []FactField) []FactField {
	byName := map[string]FactField{}
	for _, f := range a {
		k := normalizeFactID(f.Name)
		if k == "" {
			k = f.Name
		}
		byName[k] = f
	}
	for _, f := range b {
		k := normalizeFactID(f.Name)
		if k == "" {
			k = f.Name
		}
		if cur, ok := byName[k]; ok {
			if cur.GoType == "" {
				cur.GoType = f.GoType
			}
			if cur.CueTypeHint == "" {
				cur.CueTypeHint = f.CueTypeHint
			}
			if cur.JSONTag == "" {
				cur.JSONTag = f.JSONTag
			}
			if cur.DBTag == "" {
				cur.DBTag = f.DBTag
			}
			if cur.Validate == "" {
				cur.Validate = f.Validate
			}
			cur.ValidationRules = uniqueStrings(append(cur.ValidationRules, f.ValidationRules...))
			cur.Required = cur.Required || f.Required
			if cur.RelationKind == "" {
				cur.RelationKind = f.RelationKind
			}
			if cur.RelationTarget == "" {
				cur.RelationTarget = f.RelationTarget
			}
			if cur.MappedBy == "" {
				cur.MappedBy = f.MappedBy
			}
			cur.Cascade = uniqueStrings(append(cur.Cascade, f.Cascade...))
			if cur.Fetch == "" {
				cur.Fetch = f.Fetch
			}
			cur.OrphanRemoval = cur.OrphanRemoval || f.OrphanRemoval
			if cur.JoinColumn == "" {
				cur.JoinColumn = f.JoinColumn
			}
			if cur.JoinTable == "" {
				cur.JoinTable = f.JoinTable
			}
			if cur.EnumType == "" {
				cur.EnumType = f.EnumType
			}
			cur.Persistence = uniqueStrings(append(cur.Persistence, f.Persistence...))
			byName[k] = cur
		} else {
			byName[k] = f
		}
	}
	out := make([]FactField, 0, len(byName))
	for _, f := range byName {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func mergeCallRefs(a, b []FactCallRef) []FactCallRef {
	byTarget := map[string]FactCallRef{}
	for _, c := range a {
		byTarget[c.Target] = c
	}
	for _, c := range b {
		if cur, ok := byTarget[c.Target]; ok {
			if cur.Kind == "" {
				cur.Kind = c.Kind
			}
			byTarget[c.Target] = cur
		} else {
			byTarget[c.Target] = c
		}
	}
	out := make([]FactCallRef, 0, len(byTarget))
	for _, c := range byTarget {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

func normalizeJavaFacts(env *FactsEnvelope) {
	// Operations: merge duplicates from interface+impl.
	type opKey struct {
		name string
		svc  string
	}
	opMap := map[opKey]FactOp{}
	for _, op := range env.Operations {
		k := opKey{name: normalizeFactID(op.Name), svc: normalizeFactID(op.ServiceHint)}
		if k.name == "" {
			continue
		}
		if cur, ok := opMap[k]; ok {
			if cur.Source == "" {
				cur.Source = op.Source
			}
			if cur.ClassName == "" {
				cur.ClassName = op.ClassName
			}
			if cur.Kind == "" {
				cur.Kind = op.Kind
			}
			if cur.HTTPMethod == "" {
				cur.HTTPMethod = op.HTTPMethod
			}
			if cur.HTTPPath == "" {
				cur.HTTPPath = op.HTTPPath
			}
			if cur.AuthExpr == "" {
				cur.AuthExpr = op.AuthExpr
			}
			cur.Transactional = cur.Transactional || op.Transactional
			cur.TxReadOnly = cur.TxReadOnly || op.TxReadOnly
			cur.InputFields = mergeFactFields(cur.InputFields, op.InputFields)
			cur.OutputFields = mergeFactFields(cur.OutputFields, op.OutputFields)
			cur.Calls = mergeCallRefs(cur.Calls, op.Calls)
			opMap[k] = cur
		} else {
			op.InputFields = mergeFactFields(nil, op.InputFields)
			op.OutputFields = mergeFactFields(nil, op.OutputFields)
			op.Calls = mergeCallRefs(nil, op.Calls)
			opMap[k] = op
		}
	}
	env.Operations = env.Operations[:0]
	for _, op := range opMap {
		env.Operations = append(env.Operations, op)
	}
	sort.Slice(env.Operations, func(i, j int) bool {
		a := env.Operations[i]
		b := env.Operations[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.ServiceHint != b.ServiceHint {
			return a.ServiceHint < b.ServiceHint
		}
		return a.Source < b.Source
	})

	// Repositories/methods normalize.
	for i := range env.Repositories {
		m := env.Repositories[i].Methods
		sort.Slice(m, func(a, b int) bool { return m[a].Name < m[b].Name })
		env.Repositories[i].Methods = m
	}
	sort.Slice(env.Repositories, func(i, j int) bool {
		if env.Repositories[i].Entity != env.Repositories[j].Entity {
			return env.Repositories[i].Entity < env.Repositories[j].Entity
		}
		return env.Repositories[i].Source < env.Repositories[j].Source
	})

	// Entities/events sort.
	sort.Slice(env.Entities, func(i, j int) bool {
		if env.Entities[i].Name != env.Entities[j].Name {
			return env.Entities[i].Name < env.Entities[j].Name
		}
		return env.Entities[i].Source < env.Entities[j].Source
	})
	sort.Slice(env.Events, func(i, j int) bool {
		if env.Events[i].Name != env.Events[j].Name {
			return env.Events[i].Name < env.Events[j].Name
		}
		return env.Events[i].Source < env.Events[j].Source
	})

	// Endpoints dedupe + supplement from operations.
	eMap := map[string]FactEndpoint{}
	for _, e := range env.Endpoints {
		k := normalizeFactID(e.Operation) + "|" + strings.ToUpper(strings.TrimSpace(e.HTTPMethod)) + "|" + normalizeHTTPPath(e.HTTPPath)
		e.HTTPMethod = strings.ToUpper(strings.TrimSpace(e.HTTPMethod))
		e.HTTPPath = normalizeHTTPPath(e.HTTPPath)
		eMap[k] = e
	}
	for _, op := range env.Operations {
		if op.HTTPMethod == "" && op.HTTPPath == "" {
			continue
		}
		e := FactEndpoint{
			Operation:     op.Name,
			HTTPMethod:    strings.ToUpper(strings.TrimSpace(op.HTTPMethod)),
			HTTPPath:      normalizeHTTPPath(op.HTTPPath),
			ServiceHint:   op.ServiceHint,
			Source:        op.Source,
			AuthExpr:      op.AuthExpr,
			Transactional: op.Transactional,
			TxReadOnly:    op.TxReadOnly,
		}
		k := normalizeFactID(e.Operation) + "|" + e.HTTPMethod + "|" + e.HTTPPath
		if cur, ok := eMap[k]; ok {
			if cur.AuthExpr == "" {
				cur.AuthExpr = e.AuthExpr
			}
			cur.Transactional = cur.Transactional || e.Transactional
			cur.TxReadOnly = cur.TxReadOnly || e.TxReadOnly
			eMap[k] = cur
		} else {
			eMap[k] = e
		}
	}
	env.Endpoints = env.Endpoints[:0]
	for _, e := range eMap {
		env.Endpoints = append(env.Endpoints, e)
	}
	sort.Slice(env.Endpoints, func(i, j int) bool {
		a := env.Endpoints[i]
		b := env.Endpoints[j]
		if a.Operation != b.Operation {
			return a.Operation < b.Operation
		}
		if a.HTTPMethod != b.HTTPMethod {
			return a.HTTPMethod < b.HTTPMethod
		}
		return a.HTTPPath < b.HTTPPath
	})

	// Call edges dedupe/sort.
	cMap := map[string]FactCallEdge{}
	for _, c := range env.Calls {
		k := c.From + "|" + c.To
		if cur, ok := cMap[k]; ok {
			if cur.Kind == "" {
				cur.Kind = c.Kind
			}
			if cur.Source == "" {
				cur.Source = c.Source
			}
			cMap[k] = cur
		} else {
			cMap[k] = c
		}
	}
	env.Calls = env.Calls[:0]
	for _, c := range cMap {
		env.Calls = append(env.Calls, c)
	}
	sort.Slice(env.Calls, func(i, j int) bool {
		if env.Calls[i].From != env.Calls[j].From {
			return env.Calls[i].From < env.Calls[j].From
		}
		return env.Calls[i].To < env.Calls[j].To
	})

	// Mapper/security/error sorting.
	sort.Slice(env.Mappers, func(i, j int) bool {
		if env.Mappers[i].Name != env.Mappers[j].Name {
			return env.Mappers[i].Name < env.Mappers[j].Name
		}
		return env.Mappers[i].Source < env.Mappers[j].Source
	})
	sort.Slice(env.ErrorContracts, func(i, j int) bool {
		a := env.ErrorContracts[i]
		b := env.ErrorContracts[j]
		if a.Exception != b.Exception {
			return a.Exception < b.Exception
		}
		if a.Handler != b.Handler {
			return a.Handler < b.Handler
		}
		return a.Source < b.Source
	})
	sort.Slice(env.SecurityRules, func(i, j int) bool {
		a := env.SecurityRules[i]
		b := env.SecurityRules[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Pattern != b.Pattern {
			return a.Pattern < b.Pattern
		}
		if a.Requirement != b.Requirement {
			return a.Requirement < b.Requirement
		}
		return a.Source < b.Source
	})
}

func enrichJavaOpsWithOpenAPI(root string, env *FactsEnvelope) {
	path, ok := findJavaProjectOpenAPI(root)
	if !ok {
		return
	}
	openapiFacts, err := extractOpenAPIFacts(path)
	if err != nil {
		return
	}
	byName := map[string]FactOp{}
	for _, op := range openapiFacts.Operations {
		key := normalizeFactID(op.Name)
		if key == "" {
			continue
		}
		byName[key] = op
	}
	for i := range env.Operations {
		key := normalizeFactID(env.Operations[i].Name)
		o, ok := byName[key]
		if !ok {
			continue
		}
		if env.Operations[i].HTTPMethod == "" {
			env.Operations[i].HTTPMethod = o.HTTPMethod
		}
		curPath := normalizeHTTPPath(env.Operations[i].HTTPPath)
		apiPath := normalizeHTTPPath(o.HTTPPath)
		switch {
		case curPath == "":
			env.Operations[i].HTTPPath = apiPath
		case apiPath != "" && isLikelyClassBasePath(curPath):
			env.Operations[i].HTTPPath = joinHTTPPaths(curPath, apiPath)
		}
		if env.Operations[i].ServiceHint == "" {
			env.Operations[i].ServiceHint = o.ServiceHint
		}
		if len(env.Operations[i].InputFields) == 0 {
			env.Operations[i].InputFields = o.InputFields
		}
		if len(env.Operations[i].OutputFields) == 0 {
			env.Operations[i].OutputFields = o.OutputFields
		}
	}
}

func isLikelyClassBasePath(path string) bool {
	p := normalizeHTTPPath(path)
	if p == "" || p == "/" {
		return false
	}
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "{") || strings.Contains(trimmed, "}") {
		return false
	}
	// Base path usually has a single segment like /api or /v1.
	return !strings.Contains(trimmed, "/")
}

func findJavaProjectOpenAPI(root string) (string, bool) {
	candidates := []string{
		filepath.Join(root, "openapi.yaml"),
		filepath.Join(root, "openapi.yml"),
		filepath.Join(root, "openapi.json"),
		filepath.Join(root, "swagger.yaml"),
		filepath.Join(root, "swagger.yml"),
		filepath.Join(root, "swagger.json"),
		filepath.Join(root, "src", "main", "resources", "openapi.yaml"),
		filepath.Join(root, "src", "main", "resources", "openapi.yml"),
		filepath.Join(root, "src", "main", "resources", "openapi.json"),
		filepath.Join(root, "src", "main", "resources", "swagger.yaml"),
		filepath.Join(root, "src", "main", "resources", "swagger.yml"),
		filepath.Join(root, "src", "main", "resources", "swagger.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, true
		}
	}
	return "", false
}

func parseMethodSecurityRequirement(authExpr string) string {
	a := strings.TrimSpace(authExpr)
	if a == "" {
		return ""
	}
	if strings.HasPrefix(a, "hasRole") || strings.HasPrefix(a, "hasAnyRole") {
		return a
	}
	return "preauthorize:" + a
}

func maybeParseHTTPCode(code string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(code))
	return v
}
