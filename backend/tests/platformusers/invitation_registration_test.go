package platformusers_test

import (
	app "bidora/backend/internal/platformadmin/application"
	invite "bidora/backend/internal/platformadmin/invitation"
	"context"
	"errors"
	"testing"
	"time"
)

type setupTx struct {
	app.InvitationTx
	record app.InvitationRecord
}

func (s *setupTx) LockInvitationByToken(context.Context, [32]byte, time.Time) (app.InvitationRecord, error) {
	return s.record, nil
}

type setupUOW struct {
	tx    *setupTx
	calls int
}

func (u *setupUOW) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	u.calls++
	return fn(ctx, u.tx)
}
func TestInvitationRegistrationRequiresActiveNamedUnexpiredToken(t *testing.T) {
	token, err := invite.Generate()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"active", "expired", "cancelled", "accepted", "missing names", "invalid token"} {
		t.Run(name, func(t *testing.T) {
			expiry := at.Add(time.Hour)
			r := app.InvitationRecord{State: app.InvitationActive, ExpiresAt: &expiry, FirstName: "Saved", LastName: "Person", EmailDisplay: "saved@example.test"}
			switch name {
			case "expired":
				expiry = at.Add(-time.Second)
			case "cancelled":
				r.State = app.InvitationCancelled
			case "accepted":
				r.State = app.InvitationAccepted
			case "missing names":
				r.FirstName = ""
			}
			uow := &setupUOW{tx: &setupTx{record: r}}
			service, _ := app.NewInvitationService(uow, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, &delivery{})
			raw := token.RevealForDelivery()
			if name == "invalid token" {
				raw = "invalid"
			}
			result, err := service.RegistrationIdentity(context.Background(), raw)
			if name == "active" {
				if err != nil || result.FirstName != "Saved" || result.Email != "saved@example.test" {
					t.Fatal("saved identity not resolved")
				}
			} else if !errors.Is(err, app.ErrInvitationInvalid) {
				t.Fatal("invalid invitation used for setup")
			}
			if name == "invalid token" && uow.calls != 0 {
				t.Fatal("invalid token reached database")
			}
		})
	}
}
