package platformplans_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	pa "bidora/backend/internal/platformadmin/application"
	pd "bidora/backend/internal/platformadmin/domain"
)

type subscriptionStore struct {
	actor                         domain.ID
	at                            time.Time
	generation                    int64
	roles                         []pd.Role
	summary                       app.SubscriptionSummary
	commands                      map[string]app.PlatformSubscriptionCommand
	audits, reads, inventoryReads int
	inventoryRows                 []app.PlatformSubscriptionInventoryRow
}

func (s *subscriptionStore) WithinPlatformSubscription(ctx context.Context, fn func(context.Context, app.PlatformSubscriptionTx) error) error {
	return fn(ctx, s)
}
func (s *subscriptionStore) ResolvePlatformAuthority(_ context.Context, _ pa.AuthorityQuery) (pa.AuthorityFacts, error) {
	return pa.AuthorityFacts{UserID: pa.ID(s.actor), ContextGeneration: s.generation, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: s.at.Add(-time.Minute), Roles: s.roles}, nil
}
func (s *subscriptionStore) ReadPlatformSubscription(_ context.Context, tenant domain.ID, _ bool) (app.SubscriptionSummary, bool, error) {
	return s.summary, s.summary.Subscription.TenantID == tenant, nil
}
func (s *subscriptionStore) ListPlatformSubscriptionInventory(_ context.Context, after domain.ID, limit int32, state string) ([]app.PlatformSubscriptionInventoryRow, error) {
	result := []app.PlatformSubscriptionInventoryRow{}
	for _, row := range s.inventoryRows {
		if bytes.Compare(row.SubscriptionID[:], after[:]) > 0 && (state == "" || string(row.State) == state) {
			result = append(result, row)
			if len(result) >= int(limit) {
				break
			}
		}
	}
	return result, nil
}
func (s *subscriptionStore) AppendPlatformSubscriptionInventoryRead(_ context.Context, _, _, _ domain.ID, _ int32, _ string, _ time.Time) error {
	s.inventoryReads++
	return nil
}
func (s *subscriptionStore) GetPlatformSubscriptionCommand(_ context.Context, _ domain.ID, op string, key [32]byte) (app.PlatformSubscriptionCommand, bool, error) {
	v, ok := s.commands[op+string(key[:])]
	return v, ok, nil
}
func (s *subscriptionStore) CommitPlatformSubscription(_ context.Context, c app.PlatformSubscriptionCommit) error {
	if c.After.Subscription.Version != s.summary.Subscription.Version+1 {
		return errors.New("stale")
	}
	s.summary = c.After
	s.commands[c.Operation+string(c.KeyHash[:])] = app.PlatformSubscriptionCommand{RequestHash: c.RequestHash, Receipt: app.PlatformSubscriptionReceipt{TenantID: c.After.Subscription.TenantID, SubscriptionID: c.After.SubscriptionID, Version: c.After.Subscription.Version, State: c.After.Subscription.State, CancelAt: c.After.Subscription.CancelAt}}
	s.audits++
	return nil
}
func (s *subscriptionStore) AppendPlatformSubscriptionRead(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, time.Time) error {
	s.reads++
	return nil
}
func TestPlatformSubscriptionMigrationKeepsTenantAndPlatformCommandsSeparate(t *testing.T) {
	body, e := os.ReadFile("../../migrations/control/00047_platform_subscription_commands.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"CREATE TABLE commercial.platform_subscription_commands", "platform_subscription_commands_immutable", "PLATFORM_SCHEDULE_CANCELLATION", "PLATFORM_WITHDRAW_CANCELLATION", "PlatformSubscriptionCancellationScheduled", "schema_version=47"} {
		if !strings.Contains(string(body), v) {
			t.Fatal("missing subscription invariant", v)
		}
	}
	projection, e := os.ReadFile("../../migrations/control/00049_platform_subscription_inventory_projection.sql")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(projection), "PlatformSubscriptionInventoryRead") || !strings.Contains(string(projection), "schema_version=49") {
		t.Fatal("inventory audit projection missing")
	}
	other, e := os.ReadFile("../../migrations/control/00048_platform_invitation_reissue.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"'INVITATION_REISSUE'", "schema_version=48"} {
		if !strings.Contains(string(other), v) {
			t.Fatal("missing fresh invitation operation", v)
		}
	}
}
func TestPlatformSubscriptionInventoryBoundedAuthorizedAndAudited(t *testing.T) {
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	end := at.Add(14 * 24 * time.Hour)
	s := &subscriptionStore{actor: domain.ID{1}, at: at, generation: 4, roles: []pd.Role{pd.BillingAdministrator}}
	for i := byte(1); i <= 3; i++ {
		s.inventoryRows = append(s.inventoryRows, app.PlatformSubscriptionInventoryRow{TenantID: domain.ID{i + 10}, SubscriptionID: domain.ID{i}, State: domain.TrialActive, Version: 1, PlanKey: domain.Trial, Cadence: domain.TrialCadence, HasCurrentTerm: true, PeriodEnd: &end, UpdatedAt: at})
	}
	svc, _ := app.NewPlatformSubscriptionService(s, &ids{}, clock{at})
	subject := app.PlatformSubscriptionSubject{UserID: s.actor, SessionID: domain.ID{8}, CorrelationID: domain.ID{9}, ContextGeneration: 4}
	in := app.PlatformSubscriptionInventoryInput{Subject: subject, Limit: 2}
	first, e := svc.Inventory(context.Background(), in)
	if e != nil || len(first.Rows) != 2 || first.Next == nil || *first.Next != (domain.ID{2}) || s.inventoryReads != 1 {
		t.Fatal(first, e, s.inventoryReads)
	}
	in.AfterID = *first.Next
	second, e := svc.Inventory(context.Background(), in)
	if e != nil || len(second.Rows) != 1 || second.Next != nil || s.inventoryReads != 2 {
		t.Fatal(second, e)
	}
	s.inventoryRows[1].State = domain.Active
	in.AfterID = domain.ID{}
	in.State = "ACTIVE"
	filtered, e := svc.Inventory(context.Background(), in)
	if e != nil || len(filtered.Rows) != 1 || filtered.Rows[0].SubscriptionID != (domain.ID{2}) || filtered.Next != nil || s.inventoryReads != 3 {
		t.Fatal("exact state filter failed", filtered, e)
	}
	in.State = "UNKNOWN"
	if _, e = svc.Inventory(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionInvalid) || s.inventoryReads != 3 {
		t.Fatal("invalid filter accepted", e)
	}
	in.State = ""
	in.Limit = 51
	if _, e = svc.Inventory(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionInvalid) {
		t.Fatal("unbounded page accepted", e)
	}
	in.Limit = 2
	s.roles = []pd.Role{pd.SupportAdministrator}
	if _, e = svc.Inventory(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionForbidden) || s.inventoryReads != 3 {
		t.Fatal("non-billing inventory allowed", e)
	}
	s.roles = []pd.Role{pd.BillingAdministrator}
	in.AfterID = domain.ID{}
	s.inventoryRows[0].TenantID = domain.ID{}
	if _, e = svc.Inventory(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionUnavailable) || s.inventoryReads != 3 {
		t.Fatal("invalid control row exposed", e)
	}
}
func TestPlatformSubscriptionIndependentAuthorityAndIdempotentCommands(t *testing.T) {
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tenant := domain.ID{7}
	subscription, e := domain.StartTrial(tenant, domain.TrialEligibility{TenantActive: true, ProvisioningComplete: true, EffectiveOrganizationAdmin: true}, at)
	if e != nil {
		t.Fatal(e)
	}
	store := &subscriptionStore{actor: domain.ID{1}, at: at, generation: 4, roles: []pd.Role{pd.BillingAdministrator}, summary: app.SubscriptionSummary{SubscriptionID: domain.ID{8}, Subscription: subscription}, commands: map[string]app.PlatformSubscriptionCommand{}}
	service, _ := app.NewPlatformSubscriptionService(store, &ids{}, clock{at})
	subject := app.PlatformSubscriptionSubject{UserID: store.actor, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 4}
	if _, e = service.Read(context.Background(), subject, tenant); e != nil || store.reads != 1 {
		t.Fatal(e, store.reads)
	}
	in := app.PlatformSubscriptionChange{Subject: subject, TenantID: tenant, ExpectedVersion: 1, Operation: "SCHEDULE_CANCELLATION", IdempotencyKey: "b1000000-0000-4000-8000-000000000001"}
	receipt, e := service.Change(context.Background(), in)
	if e != nil || receipt.Version != 2 || receipt.CancelAt == nil || store.audits != 1 {
		t.Fatal(receipt, e)
	}
	replay, e := service.Change(context.Background(), in)
	if e != nil || !replay.Replay || store.audits != 1 {
		t.Fatal(replay, e)
	}
	in.IdempotencyKey = "b1000000-0000-4000-8000-000000000002"
	if _, e = service.Change(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionConflict) {
		t.Fatal("stale change accepted", e)
	}
	in.Operation = "WITHDRAW_CANCELLATION"
	in.ExpectedVersion = 2
	withdrawn, e := service.Change(context.Background(), in)
	if e != nil || withdrawn.CancelAt != nil || withdrawn.Version != 3 || store.audits != 2 {
		t.Fatal(withdrawn, e)
	}
	store.roles = []pd.Role{pd.SupportAdministrator}
	in.IdempotencyKey = "b1000000-0000-4000-8000-000000000003"
	if _, e = service.Change(context.Background(), in); !errors.Is(e, app.ErrPlatformSubscriptionForbidden) {
		t.Fatal("unsupported capability allowed", e)
	}
	subject.ContextGeneration = 3
	if _, e = service.Read(context.Background(), subject, tenant); !errors.Is(e, app.ErrPlatformSubscriptionForbidden) {
		t.Fatal("stale Platform context allowed", e)
	}
}
