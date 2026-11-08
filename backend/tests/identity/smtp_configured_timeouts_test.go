package identity_test

import (
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"context"
	"testing"
	"time"
)

func TestSMTPConnectBudgetDoesNotTruncateCommandOrDeliveryBudget(t *testing.T) {
	fixture := newSMTPFixture(t, true, 150*time.Millisecond)
	cfg := smtpConfig(t, fixture)
	cfg.Timeout = 0
	cfg.ConnectTimeout = 50 * time.Millisecond
	cfg.CommandTimeout = time.Second
	cfg.DeliveryTimeout = 2 * time.Second
	adapter, err := deliverysmtp.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	recipient, _ := email.Parse("recipient@example.test")
	if err = adapter.DeliverRendered(context.Background(), recipient, "Configured subject", "Configured text", "<p>Configured text</p>"); err != nil {
		t.Fatal("connect budget incorrectly applied to SMTP commands", err)
	}
}
func TestSMTPAcceptsAdminBudgetRangesAndRejectsInvalidPhaseLimits(t *testing.T) {
	fixture := newSMTPFixture(t, false)
	cfg := smtpConfig(t, fixture)
	cfg.Timeout = 0
	cfg.ConnectTimeout = 30 * time.Second
	cfg.CommandTimeout = 60 * time.Second
	cfg.DeliveryTimeout = 120 * time.Second
	if _, err := deliverysmtp.New(cfg); err != nil {
		t.Fatal("admin allowed budgets rejected", err)
	}
	for _, change := range []func(*deliverysmtp.Config){func(c *deliverysmtp.Config) { c.ConnectTimeout = 31 * time.Second }, func(c *deliverysmtp.Config) { c.CommandTimeout = 61 * time.Second }, func(c *deliverysmtp.Config) { c.DeliveryTimeout = 121 * time.Second }} {
		invalid := cfg
		change(&invalid)
		if _, err := deliverysmtp.New(invalid); err == nil {
			t.Fatal("out of range phase limit accepted")
		}
	}
	// The validation-only fixture expects a connection before cleanup.
	adapter, _ := deliverysmtp.New(cfg)
	recipient, _ := email.Parse("recipient@example.test")
	_ = adapter.DeliverRendered(context.Background(), recipient, "Test", "Text", "<p>Text</p>")
}
