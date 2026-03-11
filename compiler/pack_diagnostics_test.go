package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestCollectCanonicalPackDiagnostics_AuthMissingSelfProfileRoute(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "RegisterUser",
			PrimaryOperationKind: normalizer.OperationKindAuth,
			Capabilities:         []normalizer.CapabilityKind{normalizer.CapabilityAuth},
			Source:               "cue/api/auth.cue:10",
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if !hasDiagCode(got, codePackAuthMissingSelfProfileRoute) {
		t.Fatalf("expected %s, got %#v", codePackAuthMissingSelfProfileRoute, got)
	}
}

func TestCollectCanonicalPackDiagnostics_ModerationMissingTransitions(t *testing.T) {
	entities := []normalizer.Entity{{
		Name:   "ModerationReview",
		Source: "cue/domain/moderation.cue:3",
		Fields: []normalizer.Field{{Name: "status"}},
		FSM: &normalizer.FSM{
			States: []string{"pending"},
		},
	}}
	got := CollectCanonicalPackDiagnostics(entities, nil, nil)
	if !hasDiagCode(got, codePackModerationMissingTransitions) {
		t.Fatalf("expected %s, got %#v", codePackModerationMissingTransitions, got)
	}
}

func TestCollectCanonicalPackDiagnostics_NotifyMissingRecipientSource(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Notifications",
		Methods: []normalizer.Method{{
			Name:         "SendEmailToUser",
			Source:       "cue/api/notify.cue:7",
			Capabilities: []normalizer.CapabilityKind{normalizer.CapabilityNotify},
			SideEffects:  []normalizer.SideEffect{{Kind: "notify.email", Template: "welcome_email"}},
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if !hasDiagCode(got, codePackNotifyMissingRecipientSource) {
		t.Fatalf("expected %s, got %#v", codePackNotifyMissingRecipientSource, got)
	}
}

func TestCollectCanonicalPackDiagnostics_IRMismatch(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "LoginUser",
			PrimaryOperationKind: normalizer.OperationKindAuth,
			Source:               "cue/api/auth.cue:5",
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if !hasDiagCode(got, codeIRCanonicalPackMismatch) {
		t.Fatalf("expected %s, got %#v", codeIRCanonicalPackMismatch, got)
	}
}

func TestCollectCanonicalPackDiagnostics_NoAuthWarningWhenSelfProfileExists(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "RegisterUser",
			PrimaryOperationKind: normalizer.OperationKindAuth,
			Capabilities:         []normalizer.CapabilityKind{normalizer.CapabilityAuth},
		}},
	}}
	endpoints := []normalizer.Endpoint{{Method: "GET", Path: "/auth/profile", ServiceName: "Auth", RPC: "GetProfile"}}
	got := CollectCanonicalPackDiagnostics(nil, services, endpoints)
	if hasDiagCode(got, codePackAuthMissingSelfProfileRoute) {
		t.Fatalf("did not expect %s, got %#v", codePackAuthMissingSelfProfileRoute, got)
	}
}

func TestCollectCanonicalPackDiagnostics_MissingPlannerHints(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "GetMyProfile",
			PrimaryOperationKind: normalizer.OperationKindGet,
			Capabilities:         []normalizer.CapabilityKind{normalizer.CapabilityAuth, normalizer.CapabilityProfile},
			Source:               "cue/api/auth.cue:12",
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if !hasDiagCode(got, codePackMissingPlannerHints) {
		t.Fatalf("expected %s, got %#v", codePackMissingPlannerHints, got)
	}
}

func TestCollectCanonicalPackDiagnostics_NoPlannerWarningWhenPlannerHintsExist(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "GetMyProfile",
			PrimaryOperationKind: normalizer.OperationKindGet,
			Capabilities:         []normalizer.CapabilityKind{normalizer.CapabilityAuth, normalizer.CapabilityProfile},
			Planner: &normalizer.PlannerHints{
				SourcePack: "auth_profile",
				Route:      &normalizer.PlannerRoute{Method: "GET", Path: "/auth/profile"},
				Repository: &normalizer.PlannerRepository{LoadMethod: "FindByUserID", ActorField: "userID"},
			},
			Source: "cue/api/auth.cue:12",
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if hasDiagCode(got, codePackMissingPlannerHints) {
		t.Fatalf("did not expect %s, got %#v", codePackMissingPlannerHints, got)
	}
}

func TestCollectCanonicalPackDiagnostics_MissingPlannerRoutePath(t *testing.T) {
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:                 "GetMyProfile",
			PrimaryOperationKind: normalizer.OperationKindGet,
			Capabilities:         []normalizer.CapabilityKind{normalizer.CapabilityAuth, normalizer.CapabilityProfile},
			Planner: &normalizer.PlannerHints{
				SourcePack: "auth_profile",
				Route:      &normalizer.PlannerRoute{Method: "GET"},
			},
			Source: "cue/api/auth.cue:12",
		}},
	}}
	got := CollectCanonicalPackDiagnostics(nil, services, nil)
	if !hasDiagCode(got, codePackMissingPlannerRoutePath) {
		t.Fatalf("expected %s, got %#v", codePackMissingPlannerRoutePath, got)
	}
}

func hasDiagCode(diags []normalizer.Warning, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
