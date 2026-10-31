package identity_test

import (
	"context"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
)

type sessionInventoryReaderFake struct {
	rows   []identityapp.SessionInventoryItem
	userID identityapp.ID
	calls  int
}

func (r *sessionInventoryReaderFake) ListActiveUserSessions(_ context.Context, user identityapp.ID, _ time.Time) ([]identityapp.SessionInventoryItem, error) {
	r.calls++
	r.userID = user
	return append([]identityapp.SessionInventoryItem(nil), r.rows...), nil
}

type sessionAdministrationStoreFake struct {
	otherCommit identityapp.RevokeOtherSessionCommit
	allCommit   identityapp.RevokeAllSessionsCommit
	otherResult identityapp.RevokeOtherSessionResult
	allResult   identityapp.RevokeAllSessionsResult
	otherCalls  int
	allCalls    int
}

func (s *sessionAdministrationStoreFake) RevokeOtherSession(_ context.Context, commit identityapp.RevokeOtherSessionCommit) (identityapp.RevokeOtherSessionResult, error) {
	s.otherCalls++
	s.otherCommit = commit
	return s.otherResult, nil
}
func (s *sessionAdministrationStoreFake) RevokeAllSessions(_ context.Context, commit identityapp.RevokeAllSessionsCommit) (identityapp.RevokeAllSessionsResult, error) {
	s.allCalls++
	s.allCommit = commit
	return s.allResult, nil
}

func sessionAdminPrincipal(now time.Time) identityapp.SessionPrincipal {
	var sessionID, userID identityapp.ID
	sessionID[15], userID[15] = 1, 2
	return identityapp.SessionPrincipal{SessionID: sessionID, UserID: userID, AuthEpoch: 4, AbsoluteExpiresAt: now.Add(12 * time.Hour)}
}

func TestSessionInventoryIsAccountOwnedAndMarksOnlyCurrentSession(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	principal := sessionAdminPrincipal(now)
	var other identityapp.ID
	other[15] = 3
	reader := &sessionInventoryReaderFake{rows: []identityapp.SessionInventoryItem{{SessionID: other, Current: true}, {SessionID: principal.SessionID}}}
	service, err := identityapp.NewSessionInventoryService(reader, preparationClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Get(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || reader.userID != principal.UserID || result.ActiveSessionCount != 2 || result.OtherSessionCount != 1 || !result.OtherSessionRevocationAvailable || !result.RevokeAllAvailable || !result.CurrentSessionLogoutAvailable || !result.RevocationPolicyConfigured {
		t.Fatal("account-owned session inventory drifted", result)
	}
	if result.Sessions[0].Current || !result.Sessions[1].Current {
		t.Fatal("current session was not derived from the authenticated principal", result.Sessions)
	}
}

func TestSessionInventoryFailsClosedWhenCurrentSessionIsAbsent(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	var other identityapp.ID
	other[15] = 3
	service, err := identityapp.NewSessionInventoryService(&sessionInventoryReaderFake{rows: []identityapp.SessionInventoryItem{{SessionID: other}}}, preparationClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Get(context.Background(), sessionAdminPrincipal(now)); err == nil || err.Error() != "session_inventory_unavailable" {
		t.Fatal("inventory without current session did not fail closed", err)
	}
}

func TestSessionAdministrationRevokesOnlyOwnedTargetsAndRevokeAllAdvancesEpochResult(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	principal := sessionAdminPrincipal(now)
	var target, correlation identityapp.ID
	target[15], correlation[15] = 3, 4
	ids := []identityapp.ID{{15: 5}, {15: 6}, {15: 7}, {15: 8}, {15: 9}}
	store := &sessionAdministrationStoreFake{otherResult: identityapp.RevokeOtherSessionResult{Revoked: true}, allResult: identityapp.RevokeAllSessionsResult{AuthEpoch: 5, RevokedCount: 2}}
	service, err := identityapp.NewSessionAdministrationService(store, &sequenceIDs{values: ids}, preparationClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.RevokeOther(context.Background(), identityapp.RevokeOtherSessionInput{Principal: principal, TargetSessionID: target, CorrelationID: correlation, IdempotencyKey: "019c0000-0000-4000-8000-000000000010", SessionAuthenticatedAt: now})
	if err != nil || !other.Revoked {
		t.Fatal("revoke-other failed", err)
	}
	if store.otherCalls != 1 || store.otherCommit.Principal.UserID != principal.UserID || store.otherCommit.Principal.SessionID != principal.SessionID || store.otherCommit.TargetSessionID != target || store.otherCommit.KeyHash == ([32]byte{}) || store.otherCommit.RequestHash == ([32]byte{}) {
		t.Fatal("revoke-other ownership/evidence binding drifted", store.otherCommit)
	}
	all, err := service.RevokeAll(context.Background(), principal, correlation, now)
	if err != nil || all.AuthEpoch != 5 || all.RevokedCount != 2 || store.allCalls != 1 || store.allCommit.Principal.UserID != principal.UserID || store.allCommit.Principal.AuthEpoch != 4 {
		t.Fatal("revoke-all binding drifted", all, err)
	}
}

func TestSessionAdministrationRejectsCurrentSessionAndStaleAuthenticationBeforeStore(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	principal := sessionAdminPrincipal(now)
	var correlation identityapp.ID
	correlation[15] = 4
	store := &sessionAdministrationStoreFake{}
	service, err := identityapp.NewSessionAdministrationService(store, &sequenceIDs{values: []identityapp.ID{{15: 5}, {15: 6}, {15: 7}}}, preparationClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	_, currentErr := service.RevokeOther(context.Background(), identityapp.RevokeOtherSessionInput{Principal: principal, TargetSessionID: principal.SessionID, CorrelationID: correlation, IdempotencyKey: "key", SessionAuthenticatedAt: now})
	_, staleErr := service.RevokeAll(context.Background(), principal, correlation, now.Add(-16*time.Minute))
	if currentErr == nil || currentErr.Error() != "session_administration_current_requires_logout" || staleErr == nil || staleErr.Error() != "session_administration_fresh_auth_required" || store.otherCalls != 0 || store.allCalls != 0 {
		t.Fatal("session administration boundary crossed", currentErr, staleErr)
	}
}
