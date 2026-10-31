package notification_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/notification/emailtemplate"
	"bidora/backend/internal/notification/platformemail"
	"bidora/backend/internal/platformadmin/domain"
)

type identityReader struct {
	value platformemail.PlatformIdentity
	err   error
}

func (r *identityReader) PlatformIdentity(context.Context) (platformemail.PlatformIdentity, error) {
	return r.value, r.err
}

type sender struct{ messages []emailtemplate.Message }

func (s *sender) Send(_ context.Context, _ email.Address, m emailtemplate.Message) error {
	s.messages = append(s.messages, m)
	return nil
}
func TestPlatformCommunicationsConsumeCurrentIdentityAndFailClosed(t *testing.T) {
	reader := &identityReader{value: platformemail.PlatformIdentity{Name: "Bidora <Operations>", SupportEmail: "support@example.test", Website: "https://example.test", Phone: "+254 700 000 001"}}
	transport := &sender{}
	delivery, err := platformemail.NewWithIdentity(transport, reader)
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := email.Parse("recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err = delivery.DeliverPlatformRoleChange(context.Background(), recipient, domain.PlatformOwner, true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	first := transport.messages[0]
	if !strings.Contains(first.Text, reader.value.Name) || !strings.Contains(first.HTML, "Bidora &lt;Operations&gt;") || !strings.Contains(first.Text, "support@example.test") {
		t.Fatal("active identity not applied safely")
	}
	reader.value.Name = "Updated Platform"
	if err = delivery.DeliverPlatformSessionRevocation(context.Background(), recipient, time.Now().UTC()); err != nil || !strings.Contains(transport.messages[1].Text, "Updated Platform") {
		t.Fatal("consumer reused stale identity", err)
	}
	reader.err = errors.New("synthetic unavailable")
	if err = delivery.DeliverPlatformSessionRevocation(context.Background(), recipient, time.Now().UTC()); err == nil || len(transport.messages) != 2 {
		t.Fatal("unavailable identity still sent a message", err)
	}
}
