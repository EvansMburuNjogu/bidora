package platformusers_test

import (
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	invite "bidora/backend/internal/platformadmin/invitation"
	"context"
	"errors"
	"testing"
	"time"
)

type activationPasswords struct{ hashes int }

func (*activationPasswords) ValidateNew(p []byte) error {
	if len(p) < 12 {
		return errors.New("short")
	}
	return nil
}
func (p *activationPasswords) Hash([]byte) (string, error) {
	p.hashes++
	return "$argon2id$v=19$test-only", nil
}

type activationTx struct {
	atomicInvitationTx
	token, key, request                       [32]byte
	blocked, failPassword, failGrant, expired bool
	passwordSet                               bool
}

func (tx *activationTx) ActivationReplay(_ context.Context, token, key, request [32]byte) (bool, error) {
	return tx.accepted && tx.token == token && tx.key == key && tx.request == request, nil
}
func (tx *activationTx) LockInvitationByToken(_ context.Context, hash [32]byte, _ time.Time) (app.InvitationRecord, error) {
	if tx.accepted || hash != tx.token {
		return app.InvitationRecord{}, app.ErrInvitationInvalid
	}
	expiry := at.Add(time.Hour)
	if tx.expired {
		expiry = at.Add(-time.Second)
	}
	return app.InvitationRecord{ID: app.ID{3}, EmailDisplay: "invited@example.test", EmailLookup: "invited@example.test", FirstName: "Invited", LastName: "Person", Role: domain.SupportAdministrator, State: app.InvitationActive, ExpiresAt: &expiry, Version: 1}, nil
}
func (tx *activationTx) PrepareInvitationIdentity(_ context.Context, r app.InvitationRecord, _ app.ID, _ time.Time) (app.BootstrapIdentity, error) {
	if tx.blocked || r.EmailLookup != "invited@example.test" {
		return app.BootstrapIdentity{}, app.ErrInvitationInvalid
	}
	return app.BootstrapIdentity{UserID: app.ID{1}, AuthEpoch: 1, AffiliationKey: r.EmailLookup}, nil
}
func (tx *activationTx) SetInvitationPassword(context.Context, app.BootstrapIdentity, string, app.ID, app.ID, time.Time) error {
	if tx.failPassword {
		return errors.New("password write failed")
	}
	tx.passwordSet = true
	return nil
}
func (tx *activationTx) AssignInvitedRole(context.Context, app.ID, app.ID, domain.Role, app.ID, string, time.Time) error {
	if tx.failGrant {
		return errors.New("role write failed")
	}
	tx.assigned = true
	return nil
}
func (tx *activationTx) ClaimCommand(_ context.Context, _, _ app.ID, _ string, key, request [32]byte, _ app.ID, _ int32, _ time.Time) error {
	tx.key = key
	tx.request = request
	return nil
}

type activationUOW struct{ state activationTx }

func (u *activationUOW) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	copy := u.state
	if err := fn(ctx, &copy); err != nil {
		return err
	}
	u.state = copy
	return nil
}
func TestInvitationActivationIsAtomicSingleUseAndDoesNotCreateSession(t *testing.T) {
	for _, name := range []string{"new or unfinished account", "protected account", "tenant ties", "expired invitation", "password write failure", "role write failure"} {
		t.Run(name, func(t *testing.T) {
			token, _ := invite.Generate()
			hash, _ := invite.HashPresented(token.RevealForDelivery())
			uow := &activationUOW{state: activationTx{token: hash}}
			switch name {
			case "protected account":
				uow.state.blocked = true
			case "tenant ties":
				uow.state.tenantTies = true
			case "expired invitation":
				uow.state.expired = true
			case "password write failure":
				uow.state.failPassword = true
			case "role write failure":
				uow.state.failGrant = true
			}
			passwords := &activationPasswords{}
			service, _ := app.NewInvitationService(uow, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, &delivery{})
			service.EnablePasswordActivation(passwords)
			input := app.ActivateInvitationInput{PresentedToken: token.RevealForDelivery(), Password: []byte("a secure test password"), CorrelationID: app.ID{4}, IdempotencyKey: app.ID{5}}
			err := service.Activate(context.Background(), input)
			if name != "new or unfinished account" {
				if err == nil || uow.state.passwordSet || uow.state.promoted || uow.state.assigned || uow.state.accepted {
					t.Fatal("failure committed partial activation", err)
				}
				return
			}
			if err != nil || !uow.state.passwordSet || !uow.state.promoted || !uow.state.assigned || !uow.state.accepted || uow.state.session {
				t.Fatal("activation incomplete or manufactured session", err)
			}
			if err = service.Activate(context.Background(), input); err != nil || passwords.hashes != 1 {
				t.Fatal("matching replay rewrote credentials", err)
			}
			input.Password = []byte("a different test password")
			if err = service.Activate(context.Background(), input); err == nil || passwords.hashes != 1 {
				t.Fatal("changed-password replay accepted")
			}
			input.IdempotencyKey = app.ID{9}
			if err = service.Activate(context.Background(), input); err == nil {
				t.Fatal("used capability accepted again")
			}
		})
	}
}
