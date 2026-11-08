package notification_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"bidora/backend/internal/identity/email"
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	invite "bidora/backend/internal/platformadmin/invitation"
)

type reissueIDs struct{ n byte }

func (i *reissueIDs) NewID() (app.ID, error) { i.n++; return app.ID{i.n}, nil }

type reissueClock struct{ at time.Time }

func (c reissueClock) Now() time.Time { return c.at }

type reissueTokens struct{}

func (reissueTokens) Generate() (invite.Token, error) { return invite.Generate() }

type reissueDelivery struct {
	calls  int
	failed bool
	mu     sync.Mutex
}

func (d *reissueDelivery) DeliverPlatformInvitation(_ context.Context, recipient email.Address, token invite.Token, role domain.Role, expiry time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	if recipient.Lookup() != "recipient@example.test" || token.IsZero() || role != domain.SupportAdministrator || expiry.IsZero() {
		return errors.New("invalid fresh message")
	}
	if d.failed {
		return errors.New("ambiguous SMTP outcome")
	}
	return nil
}

type reissueStore struct {
	app.InvitationTx
	mu       sync.Mutex
	old, new app.InvitationRecord
	command  app.CommandRecord
	key      [32]byte
	found    bool
	events   []string
	actor    app.ID
	now      time.Time
	role     domain.Role
}

func (s *reissueStore) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	savedOld, savedNew, savedCommand, savedKey, savedFound, savedEvents := s.old, s.new, s.command, s.key, s.found, append([]string(nil), s.events...)
	e := fn(ctx, s)
	if e != nil {
		s.old, s.new, s.command, s.key, s.found, s.events = savedOld, savedNew, savedCommand, savedKey, savedFound, savedEvents
	}
	return e
}
func (s *reissueStore) GetIdentityAuthority(_ context.Context, _ app.AuthorityQuery) (app.IdentityAuthorityFacts, error) {
	return app.IdentityAuthorityFacts{UserID: s.actor, AuthEpoch: 1, AffiliationKey: "PLATFORM", AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: s.now.Add(-time.Minute)}, nil
}
func (s *reissueStore) GetPlatformAuthority(context.Context, app.ID) (app.PlatformAuthorityFacts, error) {
	role := s.role
	if role == "" {
		role = domain.PlatformOwner
	}
	return app.PlatformAuthorityFacts{UserID: s.actor, ContextGeneration: 1, AdministratorVersion: 1, RoleKeys: []string{string(role)}}, nil
}
func (s *reissueStore) GetCommand(_ context.Context, _ app.ID, op string, key [32]byte) (app.CommandRecord, bool, error) {
	if s.found && op == "INVITATION_REISSUE" && key == s.key {
		return s.command, true, nil
	}
	return app.CommandRecord{}, false, nil
}
func (s *reissueStore) GetInvitation(_ context.Context, id app.ID) (app.InvitationRecord, error) {
	if s.new.ID == id {
		return s.new, nil
	}
	return app.InvitationRecord{}, app.ErrInvitationNotFound
}
func (s *reissueStore) CancelInvitation(_ context.Context, id app.ID, version int64, _ time.Time) (app.InvitationRecord, error) {
	if id != s.old.ID || version != s.old.Version || s.old.State != app.InvitationActive {
		return app.InvitationRecord{}, app.ErrInvitationConflict
	}
	s.old.State = app.InvitationCancelled
	s.old.Version++
	return s.old, nil
}
func (s *reissueStore) ReplaceOpenInvitations(context.Context, string, time.Time) error { return nil }
func (s *reissueStore) InsertInvitation(_ context.Context, record app.InvitationRecord, hash *[32]byte) error {
	if hash == nil || *hash == ([32]byte{}) {
		return errors.New("no new token")
	}
	s.new = record
	return nil
}
func (s *reissueStore) ClaimCommand(_ context.Context, _ app.ID, _ app.ID, op string, key, hash [32]byte, target app.ID, _ int32, _ time.Time) error {
	if op != "INVITATION_REISSUE" {
		return errors.New("wrong command")
	}
	s.command = app.CommandRecord{RequestHash: hash, TargetID: target}
	s.key = key
	s.found = true
	return nil
}
func (s *reissueStore) AppendInvitationEvidence(_ context.Context, v app.InvitationEvidence) error {
	s.events = append(s.events, v.Action)
	return nil
}
func TestReissueExplicitFreshTokenAndReplayNeverSends(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Hour)
	s := &reissueStore{actor: app.ID{1}, now: now, old: app.InvitationRecord{ID: app.ID{9}, EmailDisplay: "recipient@example.test", EmailLookup: "recipient@example.test", Role: domain.SupportAdministrator, State: app.InvitationActive, AuthorizationPath: app.InvitationOwnerDirect, ActivatedAt: &now, ExpiresAt: &expiry, Version: 1}}
	sender := &reissueDelivery{failed: true}
	service, e := app.NewInvitationService(s, reissueTokens{}, &reissueIDs{}, reissueClock{now}, sender)
	if e != nil {
		t.Fatal(e)
	}
	in := app.ReissueInvitationInput{ActorID: s.actor, SessionID: app.ID{2}, CorrelationID: app.ID{3}, IdempotencyKey: app.ID{4}, InvitationID: s.old.ID, PlatformContextGeneration: 1, ExpectedVersion: 1, Reason: "Replace an invitation whose delivery cannot be confirmed"}
	first, e := service.Reissue(context.Background(), in)
	if e != nil || first.Replay || first.Delivery != app.DeliveryUnconfirmed || first.InvitationID == in.InvitationID || sender.calls != 1 || s.old.State != app.InvitationCancelled || len(s.events) != 2 || s.events[0] != "CANCELLED" || s.events[1] != "ISSUED" {
		t.Fatal(first, e, s.events, sender.calls)
	}
	second, e := service.Reissue(context.Background(), in)
	if e != nil || !second.Replay || sender.calls != 1 {
		t.Fatal("replay sent again", second, e, sender.calls)
	}
	in.IdempotencyKey = app.ID{5}
	if _, e = service.Reissue(context.Background(), in); !errors.Is(e, app.ErrInvitationConflict) || sender.calls != 1 {
		t.Fatal("stale invitation reissued", e, sender.calls)
	}
}
func TestConcurrentReissueSameKeyMakesOneFreshSend(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Hour)
	s := &reissueStore{actor: app.ID{1}, now: now, old: app.InvitationRecord{ID: app.ID{9}, EmailDisplay: "recipient@example.test", EmailLookup: "recipient@example.test", Role: domain.SupportAdministrator, State: app.InvitationActive, ActivatedAt: &now, ExpiresAt: &expiry, Version: 1}}
	sender := &reissueDelivery{}
	service, _ := app.NewInvitationService(s, reissueTokens{}, &reissueIDs{}, reissueClock{now}, sender)
	in := app.ReissueInvitationInput{ActorID: s.actor, SessionID: app.ID{2}, CorrelationID: app.ID{3}, IdempotencyKey: app.ID{4}, InvitationID: s.old.ID, PlatformContextGeneration: 1, ExpectedVersion: 1, Reason: "Explicit replacement"}
	var wg sync.WaitGroup
	wg.Add(8)
	results := make(chan app.IssueInvitationResult, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			defer wg.Done()
			result, e := service.Reissue(context.Background(), in)
			results <- result
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	fresh := 0
	for result := range results {
		if !result.Replay {
			fresh++
		}
	}
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if fresh != 1 || sender.calls != 1 || len(s.events) != 2 {
		t.Fatal("concurrent duplicate delivery", fresh, sender.calls, s.events)
	}
}
func TestReissueRequiresOwnerEvenWithBillingCapability(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(time.Hour)
	s := &reissueStore{actor: app.ID{1}, now: now, role: domain.BillingAdministrator, old: app.InvitationRecord{ID: app.ID{9}, EmailDisplay: "recipient@example.test", EmailLookup: "recipient@example.test", Role: domain.SupportAdministrator, State: app.InvitationActive, ActivatedAt: &now, ExpiresAt: &expiry, Version: 1}}
	sender := &reissueDelivery{}
	service, _ := app.NewInvitationService(s, reissueTokens{}, &reissueIDs{}, reissueClock{now}, sender)
	in := app.ReissueInvitationInput{ActorID: s.actor, SessionID: app.ID{2}, CorrelationID: app.ID{3}, IdempotencyKey: app.ID{4}, InvitationID: s.old.ID, PlatformContextGeneration: 1, ExpectedVersion: 1, Reason: "Explicit replacement"}
	if _, e := service.Reissue(context.Background(), in); !errors.Is(e, app.ErrPlatformAccessForbidden) || sender.calls != 0 || s.old.State != app.InvitationActive {
		t.Fatal("non-owner reissue allowed", e)
	}
}
func TestReissueExcludesOwnerAndExpiredInvitations(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(-time.Minute)
	s := &reissueStore{actor: app.ID{1}, now: now, old: app.InvitationRecord{ID: app.ID{9}, EmailDisplay: "recipient@example.test", EmailLookup: "recipient@example.test", Role: domain.SupportAdministrator, State: app.InvitationActive, ActivatedAt: &now, ExpiresAt: &expiry, Version: 1}}
	sender := &reissueDelivery{}
	service, _ := app.NewInvitationService(s, reissueTokens{}, &reissueIDs{}, reissueClock{now}, sender)
	in := app.ReissueInvitationInput{ActorID: s.actor, SessionID: app.ID{2}, CorrelationID: app.ID{3}, IdempotencyKey: app.ID{4}, InvitationID: s.old.ID, PlatformContextGeneration: 1, ExpectedVersion: 1, Reason: "Explicit replacement"}
	if _, e := service.Reissue(context.Background(), in); !errors.Is(e, app.ErrInvitationConflict) || sender.calls != 0 || s.old.State != app.InvitationActive {
		t.Fatal("expired invitation changed", e)
	}
	s.old.Role = domain.PlatformOwner
	s.old.ExpiresAt = &now
	future := now.Add(time.Hour)
	s.old.ExpiresAt = &future
	if _, e := service.Reissue(context.Background(), in); !errors.Is(e, app.ErrInvitationConflict) || sender.calls != 0 || s.old.State != app.InvitationActive {
		t.Fatal("owner governance bypassed", e)
	}
}
