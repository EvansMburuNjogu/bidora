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

type setupTx struct {
	app.InvitationTx
	record                             app.InvitationRecord
	hash                               [32]byte
	exists, tied                       bool
	created, accepted, roles, evidence int
}

func (tx *setupTx) LockInvitationByToken(_ context.Context, h [32]byte, now time.Time) (app.InvitationRecord, error) {
	if h != tx.hash || tx.record.State != app.InvitationActive || tx.record.ExpiresAt == nil || !now.Before(*tx.record.ExpiresAt) {
		return app.InvitationRecord{}, app.ErrInvitationInvalid
	}
	return tx.record, nil
}
func (tx *setupTx) InvitedAccountExists(context.Context, string) (bool, error) { return tx.exists, nil }
func (tx *setupTx) RegisterInvitedIdentity(_ context.Context, _, _, _ app.ID, r app.InvitationRecord, phc string, _ time.Time) error {
	if r.EmailLookup != "new@example.test" || r.FirstName != "Amina" || r.LastName != "Njoroge" || phc != "test-phc" {
		return errors.New("identity mismatch")
	}
	tx.created++
	return nil
}
func (tx *setupTx) HasTenantTies(context.Context, app.ID, string, time.Time) (bool, error) {
	return tx.tied, nil
}
func (tx *setupTx) CreateAdministratorProfile(context.Context, app.ID, time.Time) error { return nil }
func (tx *setupTx) AssignInvitedRole(_ context.Context, _, _ app.ID, role domain.Role, _ app.ID, _ string, _ time.Time) error {
	if role != tx.record.Role {
		return errors.New("role changed")
	}
	tx.roles++
	return nil
}
func (tx *setupTx) AcceptInvitation(context.Context, app.ID, int64, app.ID, time.Time) error {
	tx.accepted++
	tx.record.State = app.InvitationAccepted
	return nil
}
func (tx *setupTx) LockGovernance(context.Context) (app.GovernanceState, error) {
	return app.GovernanceState{Version: 1}, nil
}
func (tx *setupTx) AdvanceOwnerGeneration(context.Context, int64, time.Time) error { return nil }
func (tx *setupTx) AppendInvitationEvidence(_ context.Context, e app.InvitationEvidence) error {
	if e.Action != "ACCEPTED" {
		return errors.New("wrong evidence")
	}
	tx.evidence++
	return nil
}

type setupUOW struct{ tx *setupTx }

func (s setupUOW) WithinInvitation(ctx context.Context, fn func(context.Context, app.InvitationTx) error) error {
	return fn(ctx, s.tx)
}

type setupPasswords struct{ hashes int }

func (*setupPasswords) ValidateNew(p []byte) error {
	if len(p) < 12 {
		return errors.New("weak")
	}
	return nil
}
func (p *setupPasswords) Hash([]byte) (string, error) { p.hashes++; return "test-phc", nil }
func setupFixture(t *testing.T) (*app.InvitationService, *setupTx, string, *setupPasswords) {
	t.Helper()
	token, e := invite.Generate()
	if e != nil {
		t.Fatal(e)
	}
	expires := at.Add(time.Hour)
	tx := &setupTx{hash: token.Hash(), record: app.InvitationRecord{ID: app.ID{9}, EmailDisplay: "new@example.test", EmailLookup: "new@example.test", FirstName: "Amina", LastName: "Njoroge", State: app.InvitationActive, Role: domain.PlatformOwner, InvitedBy: actor, ExpiresAt: &expires, Version: 1}}
	service, e := app.NewInvitationService(setupUOW{tx}, app.SecureInvitationTokenGenerator{}, &ids{}, clock{}, &delivery{})
	if e != nil {
		t.Fatal(e)
	}
	return service, tx, token.RevealForDelivery(), &setupPasswords{}
}
func TestInvitationPreviewNeverConsumesOrCreatesAnAccount(t *testing.T) {
	s, tx, token, _ := setupFixture(t)
	r, e := s.Preview(context.Background(), token)
	if e != nil || r.Email != "new@example.test" || r.Role != domain.PlatformOwner || tx.created != 0 || tx.accepted != 0 {
		t.Fatal("preview changed security state", e)
	}
}
func TestInvitationPasswordSetupPinsMailboxNamesRoleAndConsumesOnce(t *testing.T) {
	s, tx, token, passwords := setupFixture(t)
	email, e := s.SetUpNewAccount(context.Background(), token, []byte("Safe-invitation-password-1"), "Attacker", "Changed", app.ID{7}, passwords)
	if e != nil || email != "new@example.test" || tx.created != 1 || tx.roles != 1 || tx.accepted != 1 || tx.evidence != 1 {
		t.Fatal("setup failed", e)
	}
	_, e = s.SetUpNewAccount(context.Background(), token, []byte("Different-password-1"), "Amina", "Njoroge", app.ID{7}, passwords)
	if e == nil || passwords.hashes != 1 || tx.created != 1 {
		t.Fatal("consumed invitation replayed")
	}
}
func TestInvitationSetupCannotResetExistingAccountOrConvertTenantIdentity(t *testing.T) {
	for _, mode := range []string{"existing", "tenant", "expired", "cancelled", "weak-password"} {
		t.Run(mode, func(t *testing.T) {
			s, tx, token, passwords := setupFixture(t)
			password := []byte("Safe-invitation-password-1")
			switch mode {
			case "existing":
				tx.exists = true
			case "tenant":
				tx.tied = true
			case "expired":
				expiry := at
				tx.record.ExpiresAt = &expiry
			case "cancelled":
				tx.record.State = app.InvitationCancelled
			case "weak-password":
				password = []byte("short")
			}
			_, e := s.SetUpNewAccount(context.Background(), token, password, "Amina", "Njoroge", app.ID{7}, passwords)
			if e == nil || tx.created != 0 || tx.roles != 0 || tx.accepted != 0 {
				t.Fatal("invalid setup changed state")
			}
			if mode == "existing" && (!errors.Is(e, app.ErrInvitedAccountExists) || passwords.hashes != 0) {
				t.Fatal("existing credentials were touched")
			}
		})
	}
}
func TestOwnerCannotRevokeTheirOwnRoleEvenWithAnotherOwner(t *testing.T) {
	tx := &roleTx{owners: 2, targetOwner: true}
	s, _ := app.NewRoleChangeService(roles{tx}, &ids{}, clock{})
	in := roleInput()
	in.TargetUserID = actor
	in.Action = app.RevokeRole
	in.AssignmentID = app.ID{4}
	in.ExpectedAssignmentVersion = 1
	_, e := s.Change(context.Background(), in)
	if !errors.Is(e, app.ErrPlatformAccessForbidden) || tx.writes != 0 {
		t.Fatal("self role revocation was allowed", e)
	}
}
