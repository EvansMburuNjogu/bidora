package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"bidora/backend/internal/identity/email"
	platforminvite "bidora/backend/internal/platformadmin/invitation"
)

// Reissue invalidates the old invitation token and creates a completely new
// invitation/token for the same recipient. A fingerprint is never a token.
// Every role follows the verified-Owner direct-invitation policy. Expired active
// records may be explicitly replaced; the old token is never reactivated.
type ReissueInvitationInput struct {
	ActorID, SessionID, CorrelationID, IdempotencyKey, InvitationID ID
	PlatformContextGeneration, ExpectedVersion                      int64
	Reason                                                          string
}

func (s *InvitationService) Reissue(ctx context.Context, in ReissueInvitationInput) (result IssueInvitationResult, err error) {
	if s == nil || ctx == nil || s.uow == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.delivery == nil || in.ActorID.IsZero() || in.SessionID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey.IsZero() || in.InvitationID.IsZero() || in.PlatformContextGeneration < 1 || in.ExpectedVersion < 1 || len(in.Reason) < 1 || len(in.Reason) > 2000 || strings.TrimSpace(in.Reason) != in.Reason {
		return result, ErrInvitationInvalid
	}
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return result, ErrInvitationInvalid
	}
	keyHash := hashKey("platform-invitation-reissue:v1", in.IdempotencyKey)
	hash := hashRequest("platform-invitation-reissue:v1", struct {
		InvitationID ID
		Version      int64
		Reason       string
	}{in.InvitationID, in.ExpectedVersion, in.Reason})
	var token platforminvite.Token
	var recipient email.Address
	err = s.uow.WithinInvitation(ctx, func(c context.Context, tx InvitationTx) error {
		if _, e := requireInvitationOwner(c, tx, in.ActorID, in.SessionID, in.PlatformContextGeneration, now); e != nil {
			return e
		}
		replay, e := invitationReplay(c, tx, in.ActorID, "INVITATION_REISSUE", keyHash, hash)
		if e != nil {
			return e
		}
		if replay != nil {
			record, e := tx.GetInvitation(c, replay.TargetID)
			if e != nil {
				return e
			}
			result = issueResult(record, nil, DeliveryUnconfirmed, true)
			return nil // a replay never sends again
		}
		previous, e := tx.CancelInvitation(c, in.InvitationID, in.ExpectedVersion, now)
		if e != nil {
			// A concurrent claim can commit while this transaction waits on
			// the original invitation row. Recheck its command under the
			// latest read-committed snapshot; never trigger another SMTP send.
			if errors.Is(e, ErrInvitationNotFound) || errors.Is(e, ErrConcurrentChange) {
				claimed, checkErr := invitationReplay(c, tx, in.ActorID, "INVITATION_REISSUE", keyHash, hash)
				if checkErr != nil {
					return checkErr
				}
				if claimed != nil {
					record, readErr := tx.GetInvitation(c, claimed.TargetID)
					if readErr != nil {
						return readErr
					}
					result = issueResult(record, nil, DeliveryUnconfirmed, true)
					return nil
				}
			}
			return e
		}
		// CancelInvitation locks the exact expected version; any invalid state rolls
		// back the cancellation, its audit and the new token together.
		if previous.State != InvitationCancelled || previous.ActivatedAt == nil || previous.ExpiresAt == nil || previous.Version != in.ExpectedVersion+1 {
			return ErrInvitationConflict
		}
		recipient, e = email.Parse(previous.EmailDisplay)
		if e != nil || recipient.Lookup() != previous.EmailLookup {
			return ErrInvitationConflict
		}
		token, e = s.tokens.Generate()
		if e != nil || token.IsZero() {
			return ErrInvitationConflict
		}
		expires, e := platforminvite.ExpiresAt(now)
		if e != nil {
			return e
		}
		ids, e := s.newIDs(6)
		if e != nil {
			return e
		}
		record := InvitationRecord{FirstName: previous.FirstName, LastName: previous.LastName, ID: ids[2], EmailDisplay: previous.EmailDisplay, EmailLookup: previous.EmailLookup, Role: previous.Role, AuthorizationPath: InvitationOwnerDirect, State: InvitationActive, InvitedBy: in.ActorID, Reason: in.Reason, CreatedAt: now, ActivatedAt: &now, ExpiresAt: &expires, Version: 1}
		if e = tx.ReplaceOpenInvitations(c, recipient.Lookup(), now); e != nil {
			return e
		}
		fingerprint := token.Hash()
		if e = tx.InsertInvitation(c, record, &fingerprint); e != nil {
			return e
		}
		if e = tx.ClaimCommand(c, ids[3], in.ActorID, "INVITATION_REISSUE", keyHash, hash, record.ID, 201, now); e != nil {
			return e
		}
		if e = tx.AppendInvitationEvidence(c, InvitationEvidence{AuditID: ids[0], OutboxID: ids[1], CorrelationID: in.CorrelationID, ActorID: in.ActorID, InvitationID: previous.ID, Action: "CANCELLED", Role: previous.Role, Version: previous.Version, OccurredAt: now}); e != nil {
			return e
		}
		if e = tx.AppendInvitationEvidence(c, InvitationEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ActorID: in.ActorID, InvitationID: record.ID, Action: "ISSUED", Role: record.Role, Version: record.Version, OccurredAt: now}); e != nil {
			return e
		}
		result = issueResult(record, nil, DeliveryNotReady, false)
		return nil
	})
	if err != nil {
		return IssueInvitationResult{}, err
	}
	if token.IsZero() {
		return result, nil
	}
	// External SMTP is outside the transaction. An uncertain outcome is not a
	// second send permission; retrying requires another explicit reissue request.
	result.Delivery = DeliveryAccepted
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if delivery, ok := s.delivery.(EvidencedPlatformInvitationDelivery); ok {
		err = delivery.DeliverPlatformInvitationWithEvidence(sendCtx, PlatformInvitationDeliveryRequest{InvitationID: result.InvitationID, ActorID: in.ActorID, CorrelationID: in.CorrelationID, Recipient: recipient, Token: token, Role: result.Role, ExpiresAt: *result.ExpiresAt})
	} else {
		err = s.delivery.DeliverPlatformInvitation(sendCtx, recipient, token, result.Role, *result.ExpiresAt)
	}
	if err != nil {
		result.Delivery = DeliveryUnconfirmed
	}
	return result, nil
}
