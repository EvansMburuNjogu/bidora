package platformusers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	invite "bidora/backend/internal/platformadmin/invitation"
)

type atomicInvitationTx struct {
	app.InvitationTx
	failSession, tenantTies               bool
	promoted, assigned, accepted, session bool
}

func (*atomicInvitationTx) GetCommand(context.Context, app.ID, string, [32]byte) (app.CommandRecord, bool, error) {
	return app.CommandRecord{}, false, nil
}
func (*atomicInvitationTx) LockInvitationByToken(context.Context, [32]byte, time.Time) (app.InvitationRecord, error) {
	return app.InvitationRecord{ID: app.ID{3}, EmailLookup: "invited@example.test", Role: domain.SupportAdministrator, Version: 1}, nil
}
func (*atomicInvitationTx) LockInvitee(_ context.Context, user, session app.ID, email string, _ time.Time) (app.BootstrapIdentity, error) {
	if email != "invited@example.test" || user != (app.ID{1}) || session.IsZero() {
		return app.BootstrapIdentity{}, app.ErrInvitationInvalid
	}
	return app.BootstrapIdentity{UserID: user, AuthEpoch: 1}, nil
}
func (tx *atomicInvitationTx) HasTenantTies(context.Context, app.ID, string, time.Time) (bool, error) {
	return tx.tenantTies, nil
}
func (tx *atomicInvitationTx) PromoteIdentity(context.Context, app.ID, int64, time.Time) (int64, error) {
	tx.promoted = true
	return 2, nil
}
func (*atomicInvitationTx) RevokeSessions(context.Context, app.ID, time.Time) (int64, error) {
	return 1, nil
}
func (*atomicInvitationTx) CreateAdministratorProfile(context.Context, app.ID, time.Time) error {
	return nil
}
func (tx *atomicInvitationTx) AssignInvitedRole(context.Context, app.ID, app.ID, domain.Role, app.ID, string, time.Time) error {
	tx.assigned = true
	return nil
}
func (tx *atomicInvitationTx) AcceptInvitation(context.Context, app.ID, int64, app.ID, time.Time) error {
	tx.accepted = true
	return nil
}
func (*atomicInvitationTx) ClaimCommand(context.Context, app.ID, app.ID, string, [32]byte, [32]byte, app.ID, int32, time.Time) error {
	return nil
}
func (*atomicInvitationTx) AppendInvitationEvidence(context.Context, app.InvitationEvidence) error {
	return nil
}
func (tx *atomicInvitationTx) ContinueInvitationSession(_ context.Context, user app.ID, epoch int64, _ app.InvitationSession) error {
	if !tx.promoted || !tx.assigned || !tx.accepted || user != (app.ID{1}) || epoch != 2 {
		return errors.New("bad transition ordering")
	}
	if tx.failSession {
		return errors.New("session insert failed")
	}
	tx.session = true
	return nil
}

type atomicInvitationUOW struct{ state atomicInvitationTx }

func (u *atomicInvitationUOW) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	pending := u.state
	if err := fn(ctx, &pending); err != nil {
		return err
	}
	u.state = pending
	return nil
}

func TestInvitationRoleGrantAndReplacementSessionShareTransactionAndPreserveTenantDenial(t *testing.T) {
	token, _ := invite.Generate()
	for _, tc := range []struct {
		name       string
		fail, ties bool
	}{{"complete", false, false}, {"session insert fails", true, false}, {"tenant ties denied", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			uow := &atomicInvitationUOW{state: atomicInvitationTx{failSession: tc.fail, tenantTies: tc.ties}}
			service, err := app.NewInvitationService(uow, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, &delivery{})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Accept(context.Background(), app.AcceptInvitationInput{ActorID: app.ID{1}, SessionID: app.ID{2}, CorrelationID: app.ID{4}, IdempotencyKey: app.ID{5}, PresentedToken: token.RevealForDelivery(), Session: &app.InvitationSession{}})
			if tc.fail || tc.ties {
				if err == nil || uow.state.promoted || uow.state.assigned || uow.state.accepted || uow.state.session {
					t.Fatal("failed transition committed partial authority")
				}
			} else if err != nil || result.AuthEpoch != 2 || !uow.state.promoted || !uow.state.assigned || !uow.state.accepted || !uow.state.session {
				t.Fatal("replacement was not part of promotion transaction", err)
			}
		})
	}
}
