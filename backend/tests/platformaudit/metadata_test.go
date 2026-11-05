package platformaudit_test

import (
	control "bidora/backend/internal/controlplane/application"
	platform "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"testing"
	"time"
)

func TestAuditMetadataAccessIsServerAuthorizedAuditedAndExcludedFromExport(t *testing.T) {
	now := time.Now().UTC()
	subject := control.PlatformAuditSubject{UserID: control.ID{1}, SessionID: control.ID{2}, CorrelationID: control.ID{3}, ContextGeneration: 4, MetadataAccess: true}
	transaction := &tx{facts: platform.AuthorityFacts{UserID: platform.ID(subject.UserID), ContextGeneration: 4, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: now, Roles: []domain.Role{domain.BillingAdministrator}}}
	unit := &uow{tx: transaction}
	service, _ := control.NewPlatformAuditService(unit, ids{}, clock{now})
	filter := control.PlatformAuditFilter{Sort: "DESC", IncludeMetadata: true}
	if _, err := service.List(context.Background(), subject, filter, 10, nil); !errors.Is(err, control.ErrPlatformAuditForbidden) || transaction.listed != 0 {
		t.Fatal("caller-supplied metadata flags bypassed authority", err)
	}
	transaction.facts.Roles = []domain.Role{domain.SecurityAuditAdministrator}
	if _, err := service.List(context.Background(), subject, filter, 10, nil); err != nil || !transaction.filter.IncludeMetadata || !transaction.metadataRead || !unit.committed {
		t.Fatal("restricted metadata read was not authorized and evidenced", err)
	}
	filter.From = now.Add(-time.Hour)
	filter.Before = now
	if _, err := service.Export(context.Background(), subject, filter); err != nil || transaction.filter.IncludeMetadata || transaction.exported != 1 {
		t.Fatal("restricted metadata escaped into export", err)
	}
	transaction.facts.AssuredAt = now.Add(-16 * time.Minute)
	before := transaction.listed
	if _, err := service.List(context.Background(), subject, filter, 10, nil); !errors.Is(err, control.ErrPlatformAuditForbidden) || transaction.listed != before {
		t.Fatal("stale assurance accessed metadata", err)
	}
}
