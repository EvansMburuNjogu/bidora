package platformsettings_test

import (
	settings "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestInformationAuditRecordsActualAddDeleteModifyAndIgnoresSubmittedDelta(t *testing.T) {
	at := time.Now().UTC()
	email, website := "support@example.test", "https://example.test"
	f := &infoFixture{value: settings.Information{PlatformName: "Before", ContactPhone: "+254 700 000 001", SupportEmail: &email, Website: &website, Version: 1}, commands: map[[32]byte]settings.InformationCommand{}, authority: settings.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.admin.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at}}
	service, _ := settings.NewInformationService(f, &infoIDs{}, infoClock{at})
	name, phone := "After", ""
	in := settings.InformationSave{Subject: settings.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}, ExpectedVersion: 1, IdempotencyKey: "audit-delta", PlatformName: &name, ContactPhone: &phone, SupportEmail: email, Website: website, Changes: []settings.FieldChange{{Field: "password"}}}
	if _, err := service.Save(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(f.changes) != 2 || f.changes[0].Field != "platform_name" || f.changes[0].Operation != "MODIFIED" || *f.changes[0].Before != "Before" || *f.changes[0].After != "After" || f.changes[1].Field != "contact_phone" || f.changes[1].Operation != "DELETED" || f.changes[1].After != nil {
		t.Fatalf("incorrect delta: %+v", f.changes)
	}
	in.ExpectedVersion = 2
	in.IdempotencyKey = "audit-add"
	phone = "+254 700 000 002"
	if _, err := service.Save(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(f.changes) != 1 || f.changes[0].Operation != "ADDED" || f.changes[0].Before != nil {
		t.Fatalf("missing addition: %+v", f.changes)
	}
}
func TestGoogleAuditNeverContainsClientSecretValues(t *testing.T) {
	service, f, _, input := googleUnit(t)
	if _, err := service.Save(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(f.stored.Changes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), *input.ClientSecret) || !strings.Contains(string(raw), "[redacted]") || !strings.Contains(string(raw), "client_secret") {
		t.Fatal("secret was leaked or change marker lost")
	}
}
