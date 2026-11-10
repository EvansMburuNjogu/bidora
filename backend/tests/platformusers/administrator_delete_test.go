package platformusers_test

import (
	app "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type deletionTx struct {
	*roleTx
	deleteCalls int
	blocked     bool
}

func (tx *deletionTx) DeleteDisabledAdministrator(context.Context, app.AdministratorIdentity, time.Time) error {
	tx.deleteCalls++
	if tx.blocked {
		return app.ErrInvitationConflict
	}
	tx.writes++
	return nil
}

type deletionUOW struct{ tx *deletionTx }

func (u deletionUOW) WithinRoleChange(ctx context.Context, fn func(context.Context, app.RoleChangeTx) error) error {
	return fn(ctx, u.tx)
}
func TestAdministratorDeletionRequiresDisabledOtherAccountAndFreshOwner(t *testing.T) {
	for _, name := range []string{"disabled", "active", "self", "security auditor", "stale verification", "stale version", "tenant access"} {
		t.Run(name, func(t *testing.T) {
			tx := &deletionTx{roleTx: &roleTx{owners: 2, disabled: true}}
			in := identityInput()
			in.Action = app.DeleteAdministrator
			switch name {
			case "active":
				tx.disabled = false
			case "self":
				in.TargetUserID = actor
			case "security auditor":
				tx.auth.role = domain.SecurityAuditAdministrator
			case "stale verification":
				tx.auth.age = 16 * time.Minute
			case "stale version":
				in.ExpectedAdministratorVersion = 2
			case "tenant access":
				tx.blocked = true
			}
			service, _ := app.NewRoleChangeService(deletionUOW{tx}, &ids{}, clock{})
			result, err := service.ChangeIdentityState(context.Background(), in)
			if name != "disabled" {
				if err == nil || tx.writes != 0 || tx.evidence != 0 {
					t.Fatal("forbidden deletion executed", err)
				}
				return
			}
			if err != nil || result.Action != app.DeleteAdministrator || result.State != "EXECUTED" || tx.deleteCalls != 1 || tx.evidence != 1 {
				t.Fatal("disabled user deletion failed", err)
			}
			replay, e := service.ChangeIdentityState(context.Background(), in)
			if e != nil || !replay.Replay || tx.deleteCalls != 1 {
				t.Fatal("replay repeated deletion", e)
			}
			in.Reason = "different deletion"
			if _, e = service.ChangeIdentityState(context.Background(), in); !errors.Is(e, app.ErrInvitationConflict) {
				t.Fatal("conflicting deletion replay accepted")
			}
		})
	}
}
