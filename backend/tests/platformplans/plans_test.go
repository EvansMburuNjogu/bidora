package platformplans_test

import (
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

type clock struct{ at time.Time }

func (c clock) Now() time.Time { return c.at }

type ids struct{ n byte }

func (i *ids) NewID() (domain.ID, error) { i.n++; return domain.ID{i.n}, nil }

type fakeStore struct {
	head        app.PlanHead
	versions    map[int64]app.PlanConfig
	commands    map[string]app.PlanCommandRecord
	actor       domain.ID
	assured     time.Time
	generation  int64
	roles       []pd.Role
	audit, read int
}

func (s *fakeStore) WithinPlan(ctx context.Context, fn func(context.Context, app.PlanTx) error) error {
	return fn(ctx, s)
}
func (s *fakeStore) ResolvePlatformAuthority(_ context.Context, in pa.AuthorityQuery) (pa.AuthorityFacts, error) {
	return pa.AuthorityFacts{UserID: pa.ID(s.actor), ContextGeneration: s.generation, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: s.assured, Roles: s.roles}, nil
}
func (s *fakeStore) ListPublished(context.Context) ([]app.PlanConfig, error) { return nil, nil }
func (s *fakeStore) LockPlanHead(context.Context, domain.PlanKey) (app.PlanHead, error) {
	return s.head, nil
}
func (s *fakeStore) GetPlanVersion(_ context.Context, _ domain.PlanKey, version int64) (app.PlanConfig, error) {
	v, ok := s.versions[version]
	if !ok {
		return v, app.ErrPlatformPlanConflict
	}
	return v, nil
}
func (s *fakeStore) GetPlanCommand(_ context.Context, _ domain.ID, op string, key [32]byte) (app.PlanCommandRecord, bool, error) {
	v, ok := s.commands[op+string(key[:])]
	return v, ok, nil
}
func (s *fakeStore) InsertPlanVersion(_ context.Context, v app.PlanConfig, _ domain.ID, _ time.Time) error {
	s.versions[v.Version] = v
	return nil
}
func (s *fakeStore) AdvancePlanDraft(_ context.Context, _ domain.PlanKey, head, version int64, _ time.Time) (bool, error) {
	if head != s.head.HeadVersion || version != s.head.NextVersion {
		return false, nil
	}
	s.head.NextVersion++
	s.head.HeadVersion++
	return true, nil
}
func (s *fakeStore) PublishPlan(_ context.Context, _ domain.PlanKey, head, version int64, _ time.Time) (bool, error) {
	if head != s.head.HeadVersion || version >= s.head.NextVersion {
		return false, nil
	}
	s.head.ActiveVersion = version
	s.head.HeadVersion++
	return true, nil
}
func (s *fakeStore) AppendPlanCommand(_ context.Context, _ domain.ID, op string, key, hash [32]byte, r app.PlanReceipt, _ time.Time) error {
	s.commands[op+string(key[:])] = app.PlanCommandRecord{RequestHash: hash, Key: r.Key, Version: r.Version, HeadVersion: r.HeadVersion}
	return nil
}
func (s *fakeStore) AppendPlanEvidence(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, domain.PlanKey, string, int64, time.Time) error {
	s.audit++
	return nil
}
func (s *fakeStore) AppendPlanRead(context.Context, domain.ID, domain.ID, domain.ID, time.Time) error {
	s.read++
	return nil
}
func plan() app.PlanConfig {
	return app.PlanConfig{Key: domain.Basic, MonthlyMinor: 450000, AnnualMinor: 4500000, AdditionalLicenceMinor: 75000, BidWorkspaces: 3, IncludedLicences: 3, StorageBytes: 10737418240, DocumentVersions: 250, AICredits: 5000, ConcurrentAIJobs: 2}
}
func TestPlanMakerCheckerReplayAndSnapshots(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	maker, checker := domain.ID{11}, domain.ID{12}
	s := &fakeStore{head: app.PlanHead{ActiveVersion: 1, NextVersion: 2, HeadVersion: 1}, versions: map[int64]app.PlanConfig{}, commands: map[string]app.PlanCommandRecord{}, actor: maker, assured: now.Add(-time.Minute), generation: 1, roles: []pd.Role{pd.BillingAdministrator}}
	service, e := app.NewPlanService(s, &ids{}, clock{now})
	if e != nil {
		t.Fatal(e)
	}
	subject := app.PlanSubject{UserID: maker, SessionID: domain.ID{20}, CorrelationID: domain.ID{21}, ContextGeneration: 1}
	draft := app.PlanChange{Subject: subject, Operation: "DRAFT", Config: plan(), ExpectedHeadVersion: 1, IdempotencyKey: "a1000000-0000-4000-8000-000000000001"}
	draft.Config.MonthlyMinor = 500000
	receipt, e := service.Change(context.Background(), draft)
	if e != nil || receipt.Version != 2 || receipt.Published || s.audit != 1 {
		t.Fatal(receipt, e, s.audit)
	}
	replay, e := service.Change(context.Background(), draft)
	if e != nil || !replay.Replay || s.audit != 1 {
		t.Fatal(replay, e)
	}
	changed := draft
	changed.Config.MonthlyMinor = 510000
	if _, e = service.Change(context.Background(), changed); !errors.Is(e, app.ErrPlatformPlanConflict) {
		t.Fatal("idempotency key reuse accepted", e)
	}
	publish := app.PlanChange{Subject: subject, Operation: "PUBLISH", Config: app.PlanConfig{Key: domain.Basic}, ExpectedHeadVersion: 2, PublishVersion: 2, IdempotencyKey: "a1000000-0000-4000-8000-000000000002"}
	if _, e = service.Change(context.Background(), publish); !errors.Is(e, app.ErrPlatformPlanConflict) {
		t.Fatal("maker published own draft", e)
	}
	s.actor = checker
	publish.Subject.UserID = checker
	published, e := service.Change(context.Background(), publish)
	if e != nil || !published.Published || published.HeadVersion != 3 || s.audit != 2 {
		t.Fatal(published, e, s.audit)
	}
	old, _ := domain.FindPlan(domain.Basic)
	if old.MonthlyBasePrice.MinorUnits != 450000 {
		t.Fatal("static trial/reference catalogue mutated")
	}
	selected, e := app.ApplyPublishedPlan(s.versions[2])
	if e != nil || selected.MonthlyBasePrice.MinorUnits != 500000 || selected.CatalogueVersion != "PHASE1_V1-BASIC-v2" {
		t.Fatal(selected, e)
	}
	term, e := domain.NewTermFromPlan(selected, domain.MonthlyCadence, now)
	if e != nil || term.Price.MinorUnits != 500000 || term.CatalogueVersion != selected.CatalogueVersion {
		t.Fatal(term, e)
	}
	if replay, e = service.Change(context.Background(), publish); e != nil || !replay.Replay || s.audit != 2 {
		t.Fatal("publication replay not safe", replay, e)
	}
}
func TestPlanMigrationVersionAndAuditFence(t *testing.T) {
	body, err := os.ReadFile("../../migrations/control/00046_platform_plan_versions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, required := range []string{"CREATE TABLE commercial.platform_plan_versions", "('BASIC',1,450000,4500000,75000", "('GROWTH',1,1250000,12500000,65000", "('PROFESSIONAL',1,2950000,29500000,50000", "platform_plan_versions_immutable", "platform_plan_commands_immutable", "schema_version=46", "PlatformPlanPublished", "PlatformPlansRead"} {
		if !strings.Contains(sql, required) {
			t.Fatal("missing migration invariant", required)
		}
	}
}

type publishedReader struct {
	plans []app.PlanConfig
	err   error
}

func (r publishedReader) PublishedPlans(context.Context) ([]app.PlanConfig, error) {
	return r.plans, r.err
}
func TestPublishedCatalogueQuoteAndTermFailClosed(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	basic := plan()
	basic.Version = 2
	basic.MonthlyMinor = 510000
	growth := basic
	growth.Key = domain.Growth
	growth.Version = 1
	professional := basic
	professional.Key = domain.Professional
	professional.Version = 1
	catalogue := app.NewCatalogueService()
	if e := catalogue.ConfigurePublished(publishedReader{plans: []app.PlanConfig{basic, growth, professional}}); e != nil {
		t.Fatal(e)
	}
	quote, e := catalogue.QuoteBasePrice(context.Background(), "BASIC", "MONTHLY")
	if e != nil || quote.Price.MinorUnits != 510000 || quote.Plan.CatalogueVersion != "PHASE1_V1-BASIC-v2" {
		t.Fatal(quote, e)
	}
	term, e := catalogue.PublishedTerm(context.Background(), domain.Basic, domain.MonthlyCadence, now)
	if e != nil || term.Price.MinorUnits != quote.Price.MinorUnits || term.CatalogueVersion != quote.Plan.CatalogueVersion {
		t.Fatal(term, e)
	}
	if _, e = domain.NewCatalogueTerm(domain.Basic, domain.MonthlyCadence, now); !errors.Is(e, domain.ErrSubscriptionInvalid) {
		t.Fatal("stale source price allowed for paid term", e)
	}
	if e = catalogue.ConfigurePublished(publishedReader{err: errors.New("database unavailable")}); e != nil {
		t.Fatal(e)
	}
	if _, e = catalogue.QuoteBasePrice(context.Background(), "BASIC", "MONTHLY"); !errors.Is(e, app.ErrPlatformPlanUnavailable) {
		t.Fatal("static fallback after DB failure", e)
	}
}
func TestPlanAuthorityFailClosed(t *testing.T) {
	now := time.Now().UTC()
	s := &fakeStore{head: app.PlanHead{ActiveVersion: 1, NextVersion: 2, HeadVersion: 1}, versions: map[int64]app.PlanConfig{}, commands: map[string]app.PlanCommandRecord{}, actor: domain.ID{1}, assured: now.Add(-20 * time.Minute), generation: 1, roles: []pd.Role{pd.BillingAdministrator}}
	service, _ := app.NewPlanService(s, &ids{}, clock{now})
	in := app.PlanChange{Subject: app.PlanSubject{UserID: domain.ID{1}, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 1}, Operation: "DRAFT", Config: plan(), ExpectedHeadVersion: 1, IdempotencyKey: "a1000000-0000-4000-8000-000000000001"}
	if _, e := service.Change(context.Background(), in); !errors.Is(e, app.ErrPlatformPlanForbidden) || s.audit != 0 {
		t.Fatal(e)
	}
	s.assured = now
	s.generation = 2
	if _, e := service.Change(context.Background(), in); !errors.Is(e, app.ErrPlatformPlanForbidden) || s.audit != 0 {
		t.Fatal(e)
	}
}
