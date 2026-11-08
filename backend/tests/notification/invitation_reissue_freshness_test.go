package notification_test

import (
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"context"
	"testing"
	"time"
)

func TestReissueOwnerStillRequiresFreshVerification(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(time.Hour)
	s := &reissueStore{actor: app.ID{1}, now: now, stale: true, old: app.InvitationRecord{ID: app.ID{9}, Role: domain.PlatformOwner, State: app.InvitationActive, ActivatedAt: &now, ExpiresAt: &expiry, Version: 1}}
	sender := &reissueDelivery{role: domain.PlatformOwner}
	service, _ := app.NewInvitationService(s, reissueTokens{}, &reissueIDs{}, reissueClock{now}, sender)
	_, e := service.Reissue(context.Background(), app.ReissueInvitationInput{ActorID: s.actor, SessionID: app.ID{2}, CorrelationID: app.ID{3}, IdempotencyKey: app.ID{4}, InvitationID: s.old.ID, PlatformContextGeneration: 1, ExpectedVersion: 1, Reason: "Replace invitation"})
	if e == nil || sender.calls != 0 || s.old.State != app.InvitationActive || len(s.events) != 0 {
		t.Fatal("stale authority mutated invitation", e)
	}
}
