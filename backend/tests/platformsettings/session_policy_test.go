package platformsettings_test

import (
	"bidora/backend/internal/identity/securitypolicy"
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
)

type policyFixture struct {
	policy                        app.SessionPolicy
	authority                     app.Authority
	receipts                      map[[32]byte]app.InformationCommand
	writes, revocations, evidence int
	failEvidence                  bool
}

func (f *policyFixture) WithinSessionPolicy(ctx context.Context, _ bool, fn func(context.Context, app.SessionPolicyTx) error) error {
	old := f.policy
	w, r, e := f.writes, f.revocations, f.evidence
	receipts := make(map[[32]byte]app.InformationCommand)
	for k, v := range f.receipts {
		receipts[k] = v
	}
	if err := fn(ctx, f); err != nil {
		f.policy = old
		f.writes, f.revocations, f.evidence = w, r, e
		f.receipts = receipts
		return err
	}
	return nil
}
func (f *policyFixture) AuthorizeInformation(context.Context, app.Subject, time.Time) (app.Authority, error) {
	return f.authority, nil
}
func (f *policyFixture) ReadSessionPolicy(context.Context, bool) (app.SessionPolicy, error) {
	return f.policy, nil
}
func (f *policyFixture) FindSessionPolicyCommand(_ context.Context, _ domain.ID, key [32]byte) (app.InformationCommand, bool, error) {
	v, ok := f.receipts[key]
	return v, ok, nil
}
func (f *policyFixture) UpdateSessionPolicy(_ context.Context, in app.SessionPolicySave, _ time.Time) (int64, error) {
	if f.policy.Version != in.Policy.Version {
		return 0, nil
	}
	f.policy = in.Policy
	f.policy.Version++
	f.writes++
	return 1, nil
}
func (f *policyFixture) ApplySessionPolicy(context.Context, app.SessionPolicySave, time.Time) (int64, error) {
	f.revocations++
	return 4, nil
}
func (f *policyFixture) AppendSessionPolicyEvidence(_ context.Context, _, _ domain.ID, in app.SessionPolicySave, _ int64, _ time.Time, key, hash [32]byte) error {
	if f.failEvidence {
		return errors.New("synthetic evidence failure")
	}
	f.evidence++
	f.receipts[key] = app.InformationCommand{RequestHash: hash, ResultVersion: in.Policy.Version + 1}
	return nil
}
func TestSessionPolicyOwnerSaveRevocationReplayAndRollback(t *testing.T) {
	at := time.Now().UTC()
	f := &policyFixture{policy: app.SessionPolicy{IdleSeconds: 1800, AbsoluteSeconds: 43200, Security: securitypolicy.DefaultControls(), Version: 1}, receipts: map[[32]byte]app.InformationCommand{}, authority: app.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.admin.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: at}}
	service, err := app.NewSessionPolicyService(f, &infoIDs{}, infoClock{at})
	if err != nil {
		t.Fatal(err)
	}
	in := app.SessionPolicySave{Subject: app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}, IdempotencyKey: "policy-1", Policy: app.SessionPolicy{IdleSeconds: 300, AbsoluteSeconds: 3600, Security: securitypolicy.DefaultControls(), Version: 1}}
	out, err := service.Save(context.Background(), in)
	if err != nil || out.Version != 2 || f.writes != 1 || f.revocations != 1 || f.evidence != 1 {
		t.Fatal("policy not committed atomically", out, err)
	}
	out, err = service.Save(context.Background(), in)
	if err != nil || out.Version != 2 || f.revocations != 1 || f.evidence != 1 {
		t.Fatal("replay repeated effects", err)
	}
	in.Policy.IdleSeconds = 600
	if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationConflict) {
		t.Fatal("changed replay accepted", err)
	}
	in.IdempotencyKey = "policy-2"
	in.Policy.Version = 2
	f.failEvidence = true
	if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationUnavailable) || f.policy.Version != 2 || f.revocations != 1 || f.evidence != 1 {
		t.Fatal("failed evidence left policy/session effects", err)
	}
	f.failEvidence = false
	f.authority.EffectiveOwner = false
	if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationForbidden) {
		t.Fatal("non-owner saved policy", err)
	}
	f.authority.EffectiveOwner = true
	f.authority.AssuredAt = at.Add(-16 * time.Minute)
	if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationForbidden) {
		t.Fatal("stale TOTP accepted", err)
	}
	for _, p := range []app.SessionPolicy{{IdleSeconds: 1801, AbsoluteSeconds: 43200, Security: securitypolicy.DefaultControls(), Version: 1}, {IdleSeconds: 1800, AbsoluteSeconds: 604800, Security: securitypolicy.DefaultControls(), Version: 1}, {IdleSeconds: 0, AbsoluteSeconds: 3600, Security: securitypolicy.DefaultControls(), Version: 1}} {
		in.Policy = p
		if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationInvalid) {
			t.Fatal("mandatory bounds weakened", err)
		}
	}
}

func TestInformationIdentitySavedAndSharedWithoutAdminProjection(t *testing.T) {
	at := time.Now().UTC()
	f := &infoFixture{value: app.Information{PlatformName: "Bidora", Version: 1}, commands: map[[32]byte]app.InformationCommand{}, authority: app.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.admin.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: at}}
	service, _ := app.NewInformationService(f, &infoIDs{}, infoClock{at})
	name, phone := "Bidora Pilot", "+254 700 000 001"
	in := app.InformationSave{Subject: app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}, IdempotencyKey: "identity-1", ExpectedVersion: 1, PlatformName: &name, ContactPhone: &phone, SupportEmail: "support@example.test", Website: "https://example.test"}
	out, err := service.Save(context.Background(), in)
	if err != nil || out.PlatformName != name || out.ContactPhone != phone {
		t.Fatal("identity not saved", err)
	}
	f.authority = app.Authority{}
	shared, err := service.Shared(context.Background())
	if err != nil || shared.PlatformName != name || shared.ContactPhone != phone {
		t.Fatal("shared projection depends on Platform authority", err)
	}
	for _, bad := range []string{"", " whitespace ", "name\nheader"} {
		in.PlatformName = &bad
		if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationInvalid) {
			t.Fatal("unsafe display name accepted", err)
		}
	}
	in.PlatformName = &name
	badPhone := "+254<script>"
	in.ContactPhone = &badPhone
	if _, err = service.Save(context.Background(), in); !errors.Is(err, app.ErrInformationInvalid) {
		t.Fatal("unsafe phone accepted", err)
	}
}
