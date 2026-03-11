package emitter

import (
	"strings"
	"testing"
)

func TestRenderCueEmitProjectCode_NormalizesServiceContext(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	if !strings.Contains(code, `_serviceName := _contextName(_usecases.ServiceName)`) {
		t.Fatalf("expected service context normalization in generated code, got:\n%s", code)
	}
	if !strings.Contains(code, `owner: \"%s\"\n", _serviceName`) {
		t.Fatalf("expected entities to use normalized service context, got:\n%s", code)
	}
	if !strings.Contains(code, `service:     \"%s\"\n", _serviceName`) {
		t.Fatalf("expected operations to use normalized service context, got:\n%s", code)
	}
}

func TestRenderCueEmitProjectCode_UsesCanonicalSplitLayout(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"split"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`files["cue/domain/entities.cue"] = entitiesB.String()`,
		`files["cue/api/http.cue"] = httpB.String()`,
		`files["cue/repo/repositories.cue"] = repoB.String()`,
		`files["cue/infra/handlers.cue"] = infraB.String()`,
		`_opsFileStem := func(kind, entity string, capabilities []string) string`,
		`path := "cue/api/operations_" + group + ".cue"`,
		`operationFilePaths := make([]string, 0, len(opsOrder))`,
		`sort.Strings(opsOrder)`,
		`if _layout == "single_file" {`,
		`for _, path := range operationFilePaths {`,
		`sections = append(sections, _stripPackage(files[path]))`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected canonical split layout snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_AddsCreateAndReplyHeuristics(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`PrimaryOperationKind string ` + "`json:\"primary_operation_kind\"`",
		`Capabilities []string ` + "`json:\"capabilities\"`",
		`SideEffectsTyped []_metaPlanSideEffect ` + "`json:\"side_effects_typed\"`",
		`ManualRequired bool ` + "`json:\"manual_required\"`",
		`_normalizeSideEffects := func(effects []string) []_metaPlanSideEffect`,
		`_normalizeCapabilities := func(explicit []string, kind string, effects []_metaPlanSideEffect, name string, inputFields, outputFields, entityFields []struct{ Name, Type string }) []string`,
		`_primaryKindOf := func(name, method string, inputFields, outputFields, entityFields []struct{ Name, Type string }, isTransition bool, explicit string) string`,
		`_looksLikeProfileOrMediaMutation(name, inputFields, outputFields, entityFields)`,
		`kind := _primaryKindOf(uc.Name, uc.Method, uc.InputFields, uc.OutputFields, entity.Fields, uc.IsStateTransition, uc.PrimaryOperationKind)`,
		`typedEffects := append([]_metaPlanSideEffect(nil), uc.SideEffectsTyped...)`,
		`capabilities := _normalizeCapabilities(uc.Capabilities, kind, typedEffects, uc.Name, uc.InputFields, uc.OutputFields, entity.Fields)`,
		`manualReason = "missing_email_recipient"`,
		`map[string]any{"p": "notify_email", "to": recipientExpr, "text": textExpr}`,
		`"primary_operation_kind": kind`,
		`"capabilities": capabilities`,
		`"side_effects": typedEffects`,
		`"manual_required": false`,
		`_startsWithAny(name, "approve", "reject", "resolve", "close", "open", "cancel", "complete", "activate", "deactivate")`,
		`_appendEntityReplies(&steps, uc.OutputFields, uc.PrimaryEntity, entityVar)`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected generated micro-plan heuristics %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_RendersNotifyEmailSteps(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`case "notify":`,
		`_notifyVerb := func(opName, entity string) string`,
		`strings.Contains(verb, "send-email")`,
		`case "notify_email":`,
		`"action: \"notify.Email\""`,
		`"to: " + _cueExprArg(step["to"])`,
		`"text: " + _cueExprArg(step["text"])`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected notify/email rendering snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_SeparatesPrimaryKindFromEmailSideEffects(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`_canonicalSideEffectKind := func(kind string) string`,
		`case _containsAny(effect, "welcome email"):`,
		`appendEffect(_metaPlanSideEffect{Kind: "notify.email", Channel: "email", Template: "welcome_email"})`,
		`appendEffect(_metaPlanSideEffect{Kind: "notify_user"})`,
		`appendEffect(_metaPlanSideEffect{Kind: "create_review"})`,
		`appendEffect(_metaPlanSideEffect{Kind: "upload_media", TargetField: targetField})`,
		`return "auth"`,
		`return "message"`,
		`return "upload"`,
		`if _startsWithAny(name, "send", "notify", "email") {`,
		`if _looksLikeProfileOrMediaMutation(name, inputFields, outputFields, entityFields) && _startsWithAny(name, "add", "set", "update", "upload", "attach", "change") {`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected side-effect/primary-kind separation snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_LowersMessagingAndCommunityPatterns(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`if kind == "message" || _containsAny(name, "message", "chat", "conversation", "reply", "community", "listing") { appendCap("messaging") }`,
		`if strings.TrimSpace(actor) == "" {`,
		`if _containsAny(field.Name, "senderid", "authorid", "userid", "memberid") {`,
		`if conversationField := _findFieldByNames(entity.Fields, "conversationID"); conversationField != "" {`,
		`if conversationInput := _findFieldByNames(uc.InputFields, conversationField, "conversationID", "conversationId"); conversationInput != "" {`,
		`finder = "ListBy" + _pascal(conversationField)`,
		`finderInput = "req." + _pascal(conversationInput)`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected messaging/community lowering snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_CanonicalizesMediaProfileFields(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`_canonicalFieldName := func(name string) string`,
		`case "avatar", "avatarurl", "photo", "photourl", "image", "imageurl", "picture", "pictureurl", "profilepicture", "profilepictureurl", "profilephoto", "profilephotourl", "profileimage", "profileimageurl":`,
		`return "photoURL"`,
		`uc.InputFields = _canonicalizeFields(uc.InputFields)`,
		`uc.OutputFields = _canonicalizeFields(uc.OutputFields)`,
		`entity.Fields = _canonicalizeFields(entity.Fields)`,
		`typedEffects[i].TargetField = _canonicalFieldName(typedEffects[i].TargetField)`,
		`if kind == "upload" || _containsAny(name, "upload", "avatar", "photo", "image", "profile picture", "media", "attachment") {`,
		`if _canonicalFieldName(field.Name) == "photoURL" || _containsAny(field.Name, "media", "attachment") {`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected media/profile field normalization snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_DistinguishesExplicitNotifyFromNotifySideEffect(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`if _startsWithAny(name, "send", "notify", "email") {`,
		`return "notify"`,
		`case "notify.email":`,
		`notifyStep := map[string]any{"p": "notify_email", "to": recipientExpr, "text": textExpr}`,
		`if sideEffectReason := _appendNormalizedSideEffects(&steps, typedEffects, uc.PrimaryEntity, entity.Fields, entityVar, uc.InputFields, uc.OutputFields, uc.Name); sideEffectReason != "" {`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected explicit notify vs side-effect snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_LowersCanonicalAuthProfileFlows(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`requiresSession := (kind != "auth" && (lowerKind == "create" || lowerKind == "update" || lowerKind == "delete" || lowerKind == "transition" || lowerKind == "list" || lowerKind == "notify")) || (isProfileCapability && _containsAny(uc.Name, "profile", "me"))`,
		`case "auth":`,
		`case _startsWithAny(uc.Name, "register", "signup", "sign-up"):`,
		`map[string]any{"p": "hash_password", "input": "req." + _pascal(inField.Name), "output": "passwordHash"}`,
		`map[string]any{"p": "logic_call", "func": "generateTokens", "args": []string{entityVar}, "output": "tokens"}`,
		`case _startsWithAny(uc.Name, "login", "signin", "sign-in"):`,
		`map[string]any{"p": "load", "entity": uc.PrimaryEntity, "method": "FindByEmail", "input_expr": "req." + _pascal(emailField), "output": entityVar, "error": "Invalid credentials"}`,
		`map[string]any{"p": "logic_call", "func": "verifyPassword", "args": []string{entityVar + ".PasswordHash", "req." + _pascal(passwordField)}, "output": "passwordValid"}`,
		`map[string]any{"p": "guard_bool", "condition": "passwordValid", "throw": "Invalid credentials"}`,
		`if isProfileCapability && _containsAny(uc.Name, "profile", "me") {`,
		`map[string]any{"p": "load", "entity": uc.PrimaryEntity, "input_expr": "sessionID", "output": entityVar, "error": uc.PrimaryEntity + " not found"}`,
		`steps[len(steps)-1]["method"] = "FindByUserID"`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected canonical auth/profile lowering snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_StripsNotifyVerbToCanonicalSendEmail(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`verb = "send-email"`,
		`verb = strings.TrimSuffix(verb, "-to-"+entityKebab)`,
		`verb = strings.TrimSuffix(verb, "-for-"+entityKebab)`,
		`return base + "/{id}/" + verb`,
		`opsB.WriteString(fmt.Sprintf("    primary_operation_kind: \"%s\"\n", op.PrimaryOperationKind))`,
		`opsB.WriteString(fmt.Sprintf("    capabilities: [%s]\n", strings.Join(caps, ", ")))`,
		`opsB.WriteString("    side_effects: [\n")`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected canonical notify path snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_RendersCanonicalAuthAndProfilePaths(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`_authPath := func(opName string) string`,
		`return "/auth/register"`,
		`return "/auth/login"`,
		`return "/auth/logout"`,
		`return "/auth/refresh"`,
		`return "/auth/profile"`,
		`_messagePath := func(entity, opName string) string`,
		`return "/conversations/{id}/messages"`,
		`if strings.EqualFold(entity.Name, "User") && (_entityHasCapability(entity.Name, "auth") || _entityHasCapability(entity.Name, "profile")) {`,
		`{"email", "string"}`,
		`{"displayName", "string"}`,
		`{"photoURL", "string"}`,
		`{"createdAt", "time"}`,
		`{"passwordHash", "string"}`,
		`if strings.EqualFold(entity.Name, "UserProfile") && _entityHasCapability(entity.Name, "profile") {`,
		`if strings.EqualFold(entity.Name, "Conversation") && _entityHasCapability(entity.Name, "messaging") {`,
		`if strings.EqualFold(entity.Name, "ConversationMessage") && _entityHasCapability(entity.Name, "messaging") {`,
		`{"conversationID", "uuid"}`,
		`{"senderID", "uuid"}`,
		`{"body", "string"}`,
		`if strings.EqualFold(entity.Name, "CommunityListing") && _entityHasCapability(entity.Name, "messaging") {`,
		`{"title", "string"}`,
		`case "load":`,
		`field := strings.TrimPrefix(strings.TrimPrefix(method, "FindBy"), "GetBy")`,
		`repoFinders[entity][method] = _lowerCamel(field)`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected canonical auth/profile emit snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_CanonicalizesMediaProfileFields(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`_canonicalFieldName := func(name string) string`,
		`entity.Fields = _canonicalizeFields(entity.Fields)`,
		`uc.InputFields = _canonicalizeFields(uc.InputFields)`,
		`uc.OutputFields = _canonicalizeFields(uc.OutputFields)`,
		`{"photoURL", "string"}`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected media/profile emit normalization snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_LowersModerationPack(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`_isModerationEntity := func(name string) bool`,
		`return strings.EqualFold(strings.TrimSpace(name), "ModerationReview") || strings.EqualFold(strings.TrimSpace(name), "Report")`,
		`_inferTransitionTo := func(opName string, explicit *string, entityName string) string`,
		`if strings.HasPrefix(name, "approve") {`,
		`return "approved"`,
		`if strings.HasPrefix(name, "reject") {`,
		`return "rejected"`,
		`statusStates := append([]string(nil), entity.Statuses...)`,
		`statusStates = []string{"pending", "approved", "rejected"}`,
		`case "create_review":`,
		`map[string]any{"p": "create_review", "entity": "ModerationReview", "output": "moderationReview"`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected moderation lowering snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_RendersModerationEntitiesAndReviewSteps(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`_hasEntity := func(name string) bool`,
		`_anyOperationUsesSideEffect := func(kind string) bool`,
		`if (strings.EqualFold(entity.Name, "ModerationReview") || strings.EqualFold(entity.Name, "Report")) && _entityHasCapability(entity.Name, "moderation") {`,
		`required,oneof=pending approved rejected`,
		`if !_hasEntity("ModerationReview") && _anyOperationUsesSideEffect("create_review") {`,
		`#ModerationReview: {`,
		`case "create_review":`,
		`{action: \"mapping.Map\", output: \"%s\", entity: \"%s\"}`,
		`{action: \"mapping.Assign\", to: \"%s.TargetEntity\", value: %q}`,
		`{action: \"repo.Save\", source: \"%s\", input: \"%s\"}`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected moderation emit snippet %q, got:\n%s", snippet, code)
		}
	}
}
