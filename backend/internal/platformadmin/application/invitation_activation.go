package application

import (
	"bidora/backend/internal/identity/securitypolicy"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

type InvitationPassword interface {
	ValidateNew([]byte) error
	Hash([]byte) (string, error)
}
type ActivateInvitationInput struct {
	PresentedToken                string
	Password                      []byte
	CorrelationID, IdempotencyKey ID
}

func (ActivateInvitationInput) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED invitation activation]")
}

type InvitationActivationTx interface {
	ActivationReplay(context.Context, [32]byte, [32]byte, [32]byte) (bool, error)
	PrepareInvitationIdentity(context.Context, InvitationRecord, ID, time.Time) (BootstrapIdentity, error)
	SetInvitationPassword(context.Context, BootstrapIdentity, string, ID, ID, time.Time) error
}

func (s *InvitationService) EnablePasswordActivation(passwords InvitationPassword) error {
	if s == nil || passwords == nil {
		return errors.New("invitation_activation_unavailable")
	}
	s.passwords = passwords
	return nil
}

// Activate consumes the strong mailed invitation as purpose-bound enrollment
// proof, not a login session. It can enroll only a new/unassigned identity with
// no protected account, external identity, MFA or tenant ties. It never signs in.
func (s *InvitationService) Activate(ctx context.Context, input ActivateInvitationInput) error {
	if ctx == nil || s == nil || s.uow == nil || s.passwords == nil || input.CorrelationID.IsZero() || input.IdempotencyKey.IsZero() {
		return ErrInvitationInvalid
	}
	tokenHash, err := platforminvite.HashPresented(input.PresentedToken)
	if err != nil {
		return ErrInvitationInvalid
	}
	if !securitypolicy.PasswordAllowed(ctx, input.Password) || s.passwords.ValidateNew(input.Password) != nil {
		return errors.New("invitation_password_invalid")
	}
	keyHash := hashKey("platform-invitation-accept:v1", input.IdempotencyKey)
	// Bind retries to the same password without persisting a fast password hash.
	// The HMAC key is the high-entropy raw invitation capability, never its stored hash.
	binding := hmac.New(sha256.New, []byte(input.PresentedToken))
	binding.Write([]byte("invitation-password:v1:"))
	binding.Write(input.Password)
	var credentialBinding [32]byte
	copy(credentialBinding[:], binding.Sum(nil))
	requestHash := hashRequest("platform-invitation-activation:v1", struct{ TokenHash, CredentialBinding [32]byte }{tokenHash, credentialBinding})
	now := s.clock.Now().UTC()
	return s.uow.WithinInvitation(ctx, func(txCtx context.Context, tx InvitationTx) error {
		enrollment, ok := tx.(InvitationActivationTx)
		if !ok {
			return errors.New("invitation_activation_unavailable")
		}
		replay, e := enrollment.ActivationReplay(txCtx, tokenHash, keyHash, requestHash)
		if e != nil || replay {
			return e
		}
		record, e := tx.LockInvitationByToken(txCtx, tokenHash, now)
		if e != nil {
			if !errors.Is(e, ErrInvitationInvalid) {
				return e
			}
			// Recheck after waiting on a concurrent completion of this same command.
			replay, replayErr := enrollment.ActivationReplay(txCtx, tokenHash, keyHash, requestHash)
			if replayErr != nil {
				return replayErr
			}
			if replay {
				return nil
			}
			return ErrInvitationInvalid
		}
		// Another identical request may have committed while this one waited on the
		// invitation lock. The locked ACTIVE query then returns no row; a normal retry
		// finds the receipt above without replacing the password again.
		if record.State != InvitationActive || record.ExpiresAt == nil || !record.ExpiresAt.After(now) || record.FirstName == "" || record.LastName == "" {
			return ErrInvitationInvalid
		}
		ids, e := s.newIDs(2)
		if e != nil {
			return e
		}
		identity, e := enrollment.PrepareInvitationIdentity(txCtx, record, ids[0], now)
		if e != nil {
			return e
		}
		tied, e := tx.HasTenantTies(txCtx, identity.UserID, identity.AffiliationKey, now)
		if e != nil {
			return e
		}
		if tied {
			return ErrInvitationInvalid
		}
		// Hash only after the invitation and locked candidate are eligible. The
		// transport applies the existing identity/network quotas before entering here.
		phc, e := s.passwords.Hash(input.Password)
		if e != nil || phc == "" {
			return errors.New("invitation_activation_unavailable")
		}
		if e = enrollment.SetInvitationPassword(txCtx, identity, phc, ids[1], input.CorrelationID, now); e != nil {
			return e
		}
		_, e = s.acceptIdentityWithin(txCtx, tx, record, identity, AcceptInvitationInput{ActorID: identity.UserID, CorrelationID: input.CorrelationID, IdempotencyKey: input.IdempotencyKey, PresentedToken: input.PresentedToken}, keyHash, requestHash, now)
		return e
	})
}
