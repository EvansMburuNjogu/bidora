package platformsettings_test

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type regionalFixture struct {
	rows                 map[string]domain.CountryConfiguration
	policy               app.OperationalPolicy
	receipts             map[[32]byte]app.RegionalReceipt
	denied, failEvidence bool
	writes, evidence     int
	last                 app.RegionalSubject
}

func (f *regionalFixture) WithinRegionalPolicy(ctx context.Context, _ bool, fn func(context.Context, app.RegionalPolicyTx) error) error {
	rows := map[string]domain.CountryConfiguration{}
	for k, v := range f.rows {
		rows[k] = v
	}
	receipts := map[[32]byte]app.RegionalReceipt{}
	for k, v := range f.receipts {
		receipts[k] = v
	}
	p, w, e := f.policy, f.writes, f.evidence
	if err := fn(ctx, f); err != nil {
		f.rows, f.receipts, f.policy, f.writes, f.evidence = rows, receipts, p, w, e
		return err
	}
	return nil
}
func (f *regionalFixture) AuthorizeRegional(_ context.Context, s app.RegionalSubject, _ string, _ bool, _ time.Time) error {
	f.last = s
	if f.denied {
		return app.ErrInformationForbidden
	}
	return nil
}
func (f *regionalFixture) Countries(context.Context) (out []domain.CountryConfiguration, _ error) {
	for _, v := range f.rows {
		out = append(out, v)
	}
	return out, nil
}
func (f *regionalFixture) Country(_ context.Context, code string, _ bool) (domain.CountryConfiguration, error) {
	v, ok := f.rows[code]
	if !ok {
		return v, app.ErrInformationConflict
	}
	return v, nil
}
func (f *regionalFixture) SaveCountry(_ context.Context, v domain.CountryConfiguration, _ app.RegionalSubject, at time.Time) error {
	old, exists := f.rows[v.Code]
	if exists && old.Version != v.Version || !exists && v.Version != 0 {
		return app.ErrInformationConflict
	}
	if v.Default {
		for code, other := range f.rows {
			if other.Default && code != v.Code {
				other.Default = false
				other.Version++
				f.rows[code] = other
			}
		}
	}
	v.Version++
	v.UpdatedAt = at
	f.rows[v.Code] = v
	defaults := 0
	for _, row := range f.rows {
		if row.Default {
			defaults++
		}
	}
	if defaults != 1 {
		return app.ErrInformationConflict
	}
	f.writes++
	return nil
}
func (f *regionalFixture) CountryHistory(context.Context, string) ([]domain.CountryConfiguration, error) {
	return nil, nil
}
func (f *regionalFixture) Policy(context.Context, bool) (app.OperationalPolicy, error) {
	return f.policy, nil
}
func (f *regionalFixture) SavePolicy(_ context.Context, v app.OperationalPolicy, _ app.RegionalSubject, at time.Time) error {
	if f.policy.Version != v.Version {
		return app.ErrInformationConflict
	}
	v.Version++
	v.UpdatedAt = at
	f.policy = v
	f.writes++
	return nil
}
func (f *regionalFixture) PolicyHistory(context.Context) ([]app.OperationalPolicy, error) {
	return []app.OperationalPolicy{f.policy}, nil
}
func (f *regionalFixture) Command(_ context.Context, _ app.RegionalSubject, _ string, key [32]byte) (app.RegionalReceipt, bool, error) {
	v, ok := f.receipts[key]
	return v, ok, nil
}
func (f *regionalFixture) Evidence(_ context.Context, _ app.RegionalSubject, _ string, target string, version int64, key, hash [32]byte, _, _ domain.ID, _ time.Time) error {
	if f.failEvidence {
		return errors.New("synthetic evidence failure")
	}
	f.receipts[key] = app.RegionalReceipt{Hash: hash, Version: version, Target: target}
	f.evidence++
	return nil
}
func validPolicy() app.OperationalPolicy {
	return app.OperationalPolicy{Version: 1, RegistrationEnabled: true, AIEnabled: true, UploadsEnabled: true, MaxDocumentBytes: 25 << 20, MetadataRetentionDays: 90}
}
func TestRegionalReferencesAndOperationalBounds(t *testing.T) {
	if len(domain.CountryReferences()) < 240 {
		t.Fatal("reference snapshot incomplete")
	}
	ke := domain.CountryConfiguration{Code: "KE", Currency: "KES", Language: "en", Timezone: "Africa/Nairobi", Locale: "en-KE", Enabled: true, Default: true}
	if !ke.Valid() || ke.WithName().Name != "Kenya" {
		t.Fatal("Kenya default invalid")
	}
	for _, change := range []func(*domain.CountryConfiguration){func(c *domain.CountryConfiguration) { c.Currency = "USD" }, func(c *domain.CountryConfiguration) { c.Code = "ZZ" }, func(c *domain.CountryConfiguration) { c.Timezone = "untrusted/zone" }, func(c *domain.CountryConfiguration) { c.Enabled = false }, func(c *domain.CountryConfiguration) { c.Locale = "en-US" }} {
		bad := ke
		change(&bad)
		if bad.Valid() {
			t.Fatal("invalid country accepted")
		}
	}
	p := validPolicy()
	if !p.Valid() {
		t.Fatal("default policy invalid")
	}
	for _, days := range []int32{0, 6, 91, 365} {
		bad := p
		bad.MetadataRetentionDays = days
		if bad.Valid() {
			t.Fatal("unbounded retention")
		}
	}
	for _, size := range []int64{0, (1 << 20) - 1, (25 << 20) + 1} {
		bad := p
		bad.MaxDocumentBytes = size
		if bad.Valid() {
			t.Fatal("mandatory upload bounds weakened")
		}
	}
	start := time.Date(2026, 1, 2, 1, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	p.MaintenanceEnabled = true
	p.MaintenanceMessage = "Scheduled maintenance"
	p.MaintenanceStartsAt = &start
	p.MaintenanceEndsAt = &end
	if !p.Valid() || p.UnderMaintenance(start.Add(-time.Second)) || !p.UnderMaintenance(start) || p.UnderMaintenance(end) {
		t.Fatal("UTC half-open maintenance window invalid")
	}
	p.MaintenanceMessage = "injected\r\nheader"
	if p.Valid() {
		t.Fatal("control characters accepted")
	}
}
func TestRegionalCommandsAuthorizeReplayConflictAndRollback(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	ke := domain.CountryConfiguration{Code: "KE", Currency: "KES", Language: "en", Timezone: "Africa/Nairobi", Locale: "en-KE", Enabled: true, Default: true, Version: 1}
	f := &regionalFixture{rows: map[string]domain.CountryConfiguration{"KE": ke}, policy: validPolicy(), receipts: map[[32]byte]app.RegionalReceipt{}}
	s, e := app.NewRegionalPolicyService(f, &infoIDs{}, infoClock{at})
	if e != nil {
		t.Fatal(e)
	}
	subject := app.RegionalSubject{Subject: app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}}
	f.denied = true
	if _, e = s.SavePolicy(ctx, subject, f.policy, "first"); !errors.Is(e, app.ErrInformationForbidden) || f.writes != 0 {
		t.Fatal("authorization did not precede writes")
	}
	f.denied = false
	p := f.policy
	p.AIEnabled = false
	got, e := s.SavePolicy(ctx, subject, p, "first")
	if e != nil || got.Version != 2 || f.evidence != 1 || f.last.ActorID != subject.ActorID {
		t.Fatal("save evidence failed", e)
	}
	if _, e = s.SavePolicy(ctx, subject, p, "first"); e != nil || f.writes != 1 {
		t.Fatal("identical replay wrote again", e)
	}
	p.AIEnabled = true
	if _, e = s.SavePolicy(ctx, subject, p, "first"); !errors.Is(e, app.ErrInformationConflict) {
		t.Fatal("changed replay accepted")
	}
	f.failEvidence = true
	p = f.policy
	p.AIEnabled = true
	if _, e = s.SavePolicy(ctx, subject, p, "second"); !errors.Is(e, app.ErrInformationUnavailable) || f.policy.Version != 2 || f.policy.AIEnabled || f.writes != 1 {
		t.Fatal("evidence failure did not roll back")
	}
	f.failEvidence = false
	ke.Enabled = false
	ke.Default = false
	if _, e = s.SaveCountry(ctx, subject, ke, "country-one"); !errors.Is(e, app.ErrInformationConflict) || !f.rows["KE"].Default {
		t.Fatal("last enabled default removed")
	}
	ng := domain.CountryConfiguration{Code: "NG", Currency: "NGN", Language: "en", Timezone: "Africa/Lagos", Enabled: true, Default: true}
	v, e := s.SaveCountry(ctx, subject, ng, "country-two")
	if e != nil || !v.Default || v.Locale != "en-NG" || f.rows["KE"].Default {
		t.Fatal("new country default not consumed", e)
	}
	if _, e = s.SaveCountry(ctx, subject, ng, "country-two"); e != nil || f.writes != 2 {
		t.Fatal("country replay duplicated writes", e)
	}
}
