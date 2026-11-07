package notification_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
)

type recipientHistoryFixture struct {
	historyScopeFixture
	selected app.HistoryFilters
}

func (f *recipientHistoryFixture) WithinPlatformHistory(ctx context.Context, fn func(context.Context, app.PlatformHistoryTx) error) error {
	return fn(ctx, f)
}
func (f *recipientHistoryFixture) ListPlatformAttempts(ctx context.Context, limit int32, cursor *app.HistoryCursor, filters app.HistoryFilters) ([]app.AttemptHistoryRow, error) {
	f.selected = filters
	return f.historyScopeFixture.ListPlatformAttempts(ctx, limit, cursor, filters)
}
func TestRecipientFilterUsesOriginalPrivateCaseInsensitiveFingerprint(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	codec, _ := message.New(key, 3)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("bidora-notification-v1\x00recipient\x00person+tag@example.test"))
	got := codec.RecipientFingerprint("Person+Tag@Example.Test")
	if !hmac.Equal(got, mac.Sum(nil)) || codec.KeyVersion() != 3 {
		t.Fatal("recipient search does not match delivery fingerprint")
	}
	if hmac.Equal(got, codec.RecipientFingerprint("person@example.test")) {
		t.Fatal("provider alias rewritten")
	}
	at := time.Now().UTC()
	f := &recipientHistoryFixture{historyScopeFixture: historyScopeFixture{facts: platformapp.AuthorityFacts{UserID: platformapp.ID{1}, ContextGeneration: 1, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at, Roles: []platformdomain.Role{platformdomain.PlatformOwner}}}}
	service, _ := app.NewPlatformHistoryService(f, &templateIDs{}, templateClock{at: at})
	subject := app.HistorySubject{UserID: domain.ID{1}, SessionID: domain.ID{2}, CorrelationID: domain.ID{3}, ContextGeneration: 1}
	if _, err := service.List(context.Background(), subject, 25, nil, app.HistoryFilters{Recipient: "Person+Tag@Example.Test"}); err != nil || f.selected.Recipient != "Person+Tag@Example.Test" {
		t.Fatal("recipient filter lost before paginated query", err)
	}
	before := f.reads
	if _, err := service.List(context.Background(), subject, 25, nil, app.HistoryFilters{Recipient: "not a mailbox"}); !errors.Is(err, app.ErrHistoryInvalid) || f.reads != before {
		t.Fatal("invalid mailbox reached query")
	}
}
