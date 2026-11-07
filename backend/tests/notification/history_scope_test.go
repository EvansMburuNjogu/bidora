package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
)

type historyScopeFixture struct {
	facts         platformapp.AuthorityFacts
	reads, audits int
}

func (f *historyScopeFixture) WithinPlatformHistory(ctx context.Context, fn func(context.Context, app.PlatformHistoryTx) error) error {
	return fn(ctx, f)
}
func (f *historyScopeFixture) ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return f.facts, nil
}
func (f *historyScopeFixture) ListPlatformAttempts(context.Context, int32, *app.HistoryCursor) ([]app.AttemptHistoryRow, error) {
	f.reads++
	return []app.AttemptHistoryRow{{Category: "PASSWORD_RECOVERY"}, {Category: "LOGIN_CODE"}, {Category: "TENANT_INVITATION"}}, nil
}
func (f *historyScopeFixture) AppendHistoryReadAudit(context.Context, domain.ID, app.HistorySubject, time.Time, int, bool) error {
	f.audits++
	return nil
}

func TestAllNotificationHistoryRequiresFreshPlatformAuthority(t *testing.T) {
	at := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	subject := app.HistorySubject{UserID: domain.ID{1}, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 1}
	f := &historyScopeFixture{facts: platformapp.AuthorityFacts{UserID: platformapp.ID(subject.UserID), ContextGeneration: 1, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at}}
	service, err := app.NewPlatformHistoryService(f, &templateIDs{}, templateClock{at: at})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.List(context.Background(), subject, 25, nil); !errors.Is(err, app.ErrHistoryForbidden) || f.reads != 0 {
		t.Fatal("ordinary/tenant user read global history")
	}
	f.facts.Roles = []platformdomain.Role{platformdomain.PlatformOwner}
	page, err := service.List(context.Background(), subject, 25, nil)
	if err != nil || len(page.Rows) != 3 || f.reads != 1 || f.audits != 1 {
		t.Fatal("authorized global metadata not audited", err)
	}
	f.facts.AssuredAt = at.Add(-16 * time.Minute)
	if _, err = service.List(context.Background(), subject, 25, nil); !errors.Is(err, app.ErrHistoryForbidden) || f.reads != 1 {
		t.Fatal("stale assurance read history")
	}
	f.facts.AssuredAt = at
	f.facts.ContextGeneration++
	if _, err = service.List(context.Background(), subject, 25, nil); !errors.Is(err, app.ErrHistoryForbidden) || f.reads != 1 {
		t.Fatal("stale context read history")
	}
}
