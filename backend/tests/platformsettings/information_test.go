package platformsettings_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	settings "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
)

type infoClock struct{ at time.Time }

func (c infoClock) Now() time.Time { return c.at }

type infoIDs struct{ next byte }

func (i *infoIDs) NewID() (domain.ID, error) { i.next++; return domain.ID{i.next}, nil }

type infoFixture struct {
	changes          []settings.FieldChange
	value            settings.Information
	authority        settings.Authority
	commands         map[[32]byte]settings.InformationCommand
	writes, evidence int
}

func (f *infoFixture) WithinInformationRead(ctx context.Context, fn func(context.Context, settings.InformationTx) error) error {
	return fn(ctx, f)
}
func (f *infoFixture) WithinInformationWrite(ctx context.Context, fn func(context.Context, settings.InformationTx) error) error {
	old := f.value
	writes, evidence := f.writes, f.evidence
	commands := make(map[[32]byte]settings.InformationCommand, len(f.commands))
	for k, v := range f.commands {
		commands[k] = v
	}
	if err := fn(ctx, f); err != nil {
		f.value = old
		f.writes = writes
		f.evidence = evidence
		f.commands = commands
		return err
	}
	return nil
}
func (f *infoFixture) AuthorizeInformation(_ context.Context, _ settings.Subject, _ time.Time) (settings.Authority, error) {
	return f.authority, nil
}
func (f *infoFixture) ReadInformation(_ context.Context, _ bool) (settings.Information, error) {
	return f.value, nil
}
func (f *infoFixture) FindInformationCommand(_ context.Context, _ domain.ID, key [32]byte) (settings.InformationCommand, bool, error) {
	v, ok := f.commands[key]
	return v, ok, nil
}
func (f *infoFixture) UpdateInformation(_ context.Context, in settings.InformationSave, at time.Time) (int64, error) {
	if f.value.Version != in.ExpectedVersion {
		return 0, nil
	}
	f.value = settings.Information{PlatformName: *in.PlatformName, ContactPhone: *in.ContactPhone, SupportEmail: &in.SupportEmail, Website: &in.Website, Version: f.value.Version + 1, UpdatedAt: &at}
	f.writes++
	return 1, nil
}
func (f *infoFixture) AppendInformationEvidence(_ context.Context, _, _ domain.ID, in settings.InformationSave, version int64, _ time.Time, key, request [32]byte) error {
	f.changes = in.Changes
	f.commands[key] = settings.InformationCommand{RequestHash: request, ResultVersion: version}
	f.evidence++
	return nil
}
func TestPlatformInformationOwnerSaveReplayAndConflict(t *testing.T) {
	at := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	f := &infoFixture{value: settings.Information{PlatformName: "Bidora", Version: 1}, commands: make(map[[32]byte]settings.InformationCommand), authority: settings.Authority{Capabilities: map[string]bool{"platform.admin.read": true}, EffectiveOwner: true, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: at.Add(-time.Minute)}}
	svc, err := settings.NewInformationService(f, &infoIDs{}, infoClock{at})
	if err != nil {
		t.Fatal(err)
	}
	subject := settings.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}
	input := settings.InformationSave{Subject: subject, SupportEmail: "support@example.com", Website: "https://example.com/help", ExpectedVersion: 1, IdempotencyKey: "key-1", CorrelationID: domain.ID{3}}
	first, err := svc.Save(context.Background(), input)
	if err != nil || first.Version != 2 || f.writes != 1 || f.evidence != 1 {
		t.Fatalf("save outcome: version=%d writes=%d evidence=%d err=%v", first.Version, f.writes, f.evidence, err)
	}
	again, err := svc.Save(context.Background(), input)
	if err != nil || again.Version != 2 || f.writes != 1 || f.evidence != 1 {
		t.Fatalf("replay changed state: %v", err)
	}
	input.Website = "https://example.com/other"
	if _, err = svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationConflict) || f.writes != 1 {
		t.Fatalf("changed replay not rejected: %v", err)
	}
	input.IdempotencyKey = "key-2"
	if _, err = svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationConflict) || f.writes != 1 {
		t.Fatalf("stale version not rejected: %v", err)
	}
	f.authority.EffectiveOwner = false
	read, err := svc.Read(context.Background(), subject)
	if err != nil || read.Version != 2 {
		t.Fatalf("nonowner read: %v", err)
	}
	input.ExpectedVersion = 2
	if _, err = svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationForbidden) || f.writes != 1 {
		t.Fatalf("nonowner write: %v", err)
	}
	f.authority.AssuredAt = at.Add(-16 * time.Minute)
	if _, err = svc.Read(context.Background(), subject); err != nil {
		t.Fatalf("valid older administrator session could not read information: %v", err)
	}
	f.authority.EffectiveOwner = true
	if _, err = svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationForbidden) || f.writes != 1 {
		t.Fatal("stale verification permitted a security-sensitive write", err)
	}
}
func TestPlatformInformationPreservesLegacyCommittedReplay(t *testing.T) {
	at := time.Now().UTC()
	mail, site := "support@example.test", "https://example.test"
	key := sha256.Sum256([]byte("platform-information-key-v1\x00legacy-key"))
	encoded, _ := json.Marshal(struct {
		Email, Website string
		Version        int64
	}{mail, site, 1})
	hash := sha256.Sum256(append([]byte("platform-information-request-v1\x00"), encoded...))
	f := &infoFixture{value: settings.Information{PlatformName: "Bidora", SupportEmail: &mail, Website: &site, Version: 2, UpdatedAt: &at}, commands: map[[32]byte]settings.InformationCommand{key: {RequestHash: hash, ResultVersion: 2}}, authority: settings.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.admin.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: at}}
	service, _ := settings.NewInformationService(f, &infoIDs{}, infoClock{at})
	in := settings.InformationSave{Subject: settings.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}, IdempotencyKey: "legacy-key", SupportEmail: mail, Website: site, ExpectedVersion: 1}
	out, err := service.Save(context.Background(), in)
	if err != nil || out.Version != 2 || f.writes != 0 || f.evidence != 0 {
		t.Fatal("legacy receipt replay broken", err)
	}
}

func TestPlatformInformationRejectsUnusableContact(t *testing.T) {
	at := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	f := &infoFixture{value: settings.Information{PlatformName: "Bidora", Version: 1}, commands: make(map[[32]byte]settings.InformationCommand)}
	svc, err := settings.NewInformationService(f, &infoIDs{}, infoClock{at})
	if err != nil {
		t.Fatal(err)
	}
	input := settings.InformationSave{Subject: settings.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, SupportEmail: "support@example.com", Website: "https://example.com", ExpectedVersion: 1, IdempotencyKey: "key", CorrelationID: domain.ID{3}}
	for _, site := range []string{"http://example.com", "https://localhost", "https://127.0.0.1", "https://example.com?", "https://example.com#", "https://example.com:443", "https://user@example.com"} {
		input.Website = site
		if _, err := svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationInvalid) {
			t.Errorf("accepted unsupported website %q: %v", site, err)
		}
	}
	input.Website = "https://example.com"
	for _, email := range []string{" test@example.com", "nobody", "test@example.com\n"} {
		input.SupportEmail = email
		if _, err := svc.Save(context.Background(), input); !errors.Is(err, settings.ErrInformationInvalid) {
			t.Errorf("accepted invalid email %q: %v", email, err)
		}
	}
	if f.writes != 0 || f.evidence != 0 {
		t.Fatal("invalid input wrote state")
	}
}
