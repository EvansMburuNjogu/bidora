package application

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/platformadmin/domain"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvitedAccountExists = errors.New("invited_account_exists")

type InvitationSetupTx interface {
	InvitationTx
	InvitedAccountExists(context.Context, string) (bool, error)
	RegisterInvitedIdentity(context.Context, ID, ID, ID, InvitationRecord, string, time.Time) error
}
type InvitationPreview struct {
	Email         string      `json:"email"`
	FirstName     string      `json:"first_name"`
	LastName      string      `json:"last_name"`
	Role          domain.Role `json:"role_key"`
	ExpiresAt     time.Time   `json:"expires_at"`
	AccountExists bool        `json:"account_exists"`
}

func (InvitationPreview) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED invitation preview]")
}

// Preview validates the emailed capability without consuming or activating it.
// Mail scanners and page loads must never accept an invitation.
func (s *InvitationService) Preview(ctx context.Context, token string) (out InvitationPreview, err error) {
	if ctx == nil || s == nil || s.uow == nil {
		return out, ErrInvitationInvalid
	}
	hash, e := platforminvite.HashPresented(token)
	if e != nil {
		return out, ErrInvitationInvalid
	}
	err = s.uow.WithinInvitation(ctx, func(c context.Context, tx InvitationTx) error {
		record, e := tx.LockInvitationByToken(c, hash, s.clock.Now().UTC())
		if e != nil {
			return ErrInvitationInvalid
		}
		setup, ok := tx.(InvitationSetupTx)
		if !ok {
			return errors.New("invitation_setup_unavailable")
		}
		exists, e := setup.InvitedAccountExists(c, record.EmailLookup)
		if e != nil {
			return e
		}
		if record.ExpiresAt == nil {
			return ErrInvitationInvalid
		}
		out = InvitationPreview{Email: record.EmailDisplay, FirstName: record.FirstName, LastName: record.LastName, Role: record.Role, ExpiresAt: *record.ExpiresAt, AccountExists: exists}
		return nil
	})
	return
}

// SetUpNewAccount accepts only a fresh, valid invitation. Its emailed secret is
// proof of the invited mailbox, not a sign-in session or MFA assurance. No cookie
// is issued here: the ordinary password + email-OTP login must still complete.
// Existing identities are never reset, re-enabled or promoted by this route.
func (s *InvitationService) SetUpNewAccount(ctx context.Context, token string, password []byte, first, last string, correlation ID, hasher identityapp.RegistrationPassword) (address string, err error) {
	if correlation.IsZero() || hasher == nil {
		return "", ErrInvitationInvalid
	}
	preview, e := s.Preview(ctx, token)
	if e != nil {
		return "", e
	}
	if preview.AccountExists {
		return "", ErrInvitedAccountExists
	}
	if preview.FirstName != "" {
		first = preview.FirstName
	}
	if preview.LastName != "" {
		last = preview.LastName
	}
	if !identityapp.ValidPersonalName(first) || !identityapp.ValidPersonalName(last) || !securitypolicy.PasswordAllowed(ctx, password) || hasher.ValidateNew(password) != nil {
		return "", ErrInvitationInvalid
	}
	phc, e := hasher.Hash(password)
	if e != nil {
		return "", e
	}
	if phc == "" {
		return "", errors.New("invitation_password_unavailable")
	}
	hash, e := platforminvite.HashPresented(token)
	if e != nil {
		return "", ErrInvitationInvalid
	}
	now := s.clock.Now().UTC()
	err = s.uow.WithinInvitation(ctx, func(c context.Context, tx InvitationTx) error {
		record, e := tx.LockInvitationByToken(c, hash, now)
		if e != nil {
			return ErrInvitationInvalid
		}
		setup, ok := tx.(InvitationSetupTx)
		if !ok {
			return errors.New("invitation_setup_unavailable")
		}
		exists, e := setup.InvitedAccountExists(c, record.EmailLookup)
		if e != nil {
			return e
		}
		if exists {
			return ErrInvitedAccountExists
		}
		recipient, e := email.Parse(record.EmailDisplay)
		if e != nil || recipient.Lookup() != record.EmailLookup {
			return ErrInvitationInvalid
		}
		ids, e := s.newIDs(5)
		if e != nil {
			return e
		}
		record.FirstName = first
		record.LastName = last
		tied, e := tx.HasTenantTies(c, ids[0], record.EmailLookup, now)
		if e != nil {
			return e
		}
		if tied {
			return ErrInvitationInvalid
		}
		// Registration, mailbox proof, role, consumption and evidence share one commit.
		if e = setup.RegisterInvitedIdentity(c, ids[0], ids[1], correlation, record, phc, now); e != nil {
			return e
		}
		if e = tx.CreateAdministratorProfile(c, ids[0], now); e != nil {
			return e
		}
		if e = tx.AssignInvitedRole(c, ids[2], ids[0], record.Role, record.InvitedBy, record.Reason, now); e != nil {
			return e
		}
		if e = tx.AcceptInvitation(c, record.ID, record.Version, ids[0], now); e != nil {
			return e
		}
		if record.Role == domain.PlatformOwner {
			g, e := tx.LockGovernance(c)
			if e != nil {
				return e
			}
			if e = tx.AdvanceOwnerGeneration(c, g.Version, now); e != nil {
				return e
			}
		}
		if e = tx.AppendInvitationEvidence(c, InvitationEvidence{AuditID: ids[3], OutboxID: ids[4], CorrelationID: correlation, ActorID: ids[0], InvitationID: record.ID, Action: "ACCEPTED", Role: record.Role, Version: record.Version + 1, OccurredAt: now}); e != nil {
			return e
		}
		address = recipient.Display()
		return nil
	})
	return
}
