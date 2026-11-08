// Package persistence composes control-plane transaction-scoped adapters. It
// contains no Platform Administration policy sequencing; that belongs to the
// application services.
package persistence

import (
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	controlapp "bidora/backend/internal/controlplane/application"
	controlpersistence "bidora/backend/internal/controlplane/persistence"
	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
	platformsql "bidora/backend/internal/platformadmin/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
	WithinConsistentReadTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type Store struct{ runner transactionRunner }

func NewStore(runner transactionRunner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("platform_store_invalid")
	}
	return &Store{runner: runner}, nil
}

func (s *Store) WithinBootstrap(ctx context.Context, fn func(context.Context, platformapp.BootstrapTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_bootstrap_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newBootstrapTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (s *Store) WithinAuthority(ctx context.Context, fn func(context.Context, platformapp.AuthorityTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_authority_unit_of_work_invalid")
	}
	return s.runner.WithinConsistentReadTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newAuthorityTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (s *Store) WithinGovernanceRead(ctx context.Context, fn func(context.Context, platformapp.GovernanceReadTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_governance_read_unit_of_work_invalid")
	}
	return s.runner.WithinConsistentReadTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newGovernanceReadTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (s *Store) WithinInvitation(ctx context.Context, fn func(context.Context, platformapp.InvitationTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_invitation_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newInvitationTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (s *Store) WithinRoleChange(ctx context.Context, fn func(context.Context, platformapp.RoleChangeTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_role_change_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newRoleChangeTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

type bootstrapTx struct {
	platform    *platformsql.Queries
	identity    *identitypersistence.PlatformAdministrationIdentityTx
	affiliation *authzpersistence.PlatformAdministrationAffiliationTx
	evidence    *controlpersistence.EvidenceTx
}

func newBootstrapTx(db postgres.DBTX) (*bootstrapTx, error) {
	identity, err := identitypersistence.NewPlatformAdministrationIdentityTx(db)
	if err != nil {
		return nil, err
	}
	affiliation, err := authzpersistence.NewPlatformAdministrationAffiliationTx(db)
	if err != nil {
		return nil, err
	}
	evidence, err := controlpersistence.NewEvidenceTx(db)
	if err != nil {
		return nil, err
	}
	return &bootstrapTx{platform: platformsql.New(db), identity: identity, affiliation: affiliation, evidence: evidence}, nil
}

func (tx *bootstrapTx) LockGovernance(ctx context.Context) (platformapp.GovernanceState, error) {
	row, err := tx.platform.LockPlatformGovernanceState(ctx)
	if err != nil {
		return platformapp.GovernanceState{}, err
	}
	return platformapp.GovernanceState{
		BootstrapConsumed:                     row.BootstrapConsumed,
		FirstAdditionalOwnerExceptionConsumed: row.FirstAdditionalOwnerExceptionConsumed,
		EffectiveOwnerGeneration:              row.EffectiveOwnerGeneration, Version: row.Version,
	}, nil
}
func (tx *bootstrapTx) CountOwnerHistory(ctx context.Context) (int64, error) {
	return tx.platform.CountPlatformOwnerAssignmentHistory(ctx)
}
func (tx *bootstrapTx) ProfileExists(ctx context.Context, userID platformapp.ID) (bool, error) {
	return tx.platform.PlatformAdministratorProfileExists(ctx, platformID(userID))
}
func (tx *bootstrapTx) LockIdentity(ctx context.Context, userID platformapp.ID, _ time.Time) (platformapp.BootstrapIdentity, error) {
	candidate, err := tx.identity.LockBootstrapCandidate(ctx, identityapp.ID(userID))
	if errors.Is(err, identityapp.ErrPlatformIdentityIneligible) {
		return platformapp.BootstrapIdentity{}, platformapp.ErrBootstrapIneligible
	}
	if err != nil {
		return platformapp.BootstrapIdentity{}, err
	}
	return platformapp.BootstrapIdentity{UserID: platformapp.ID(candidate.UserID), AuthEpoch: candidate.AuthEpoch, AffiliationKey: candidate.AffiliationKey}, nil
}
func (tx *bootstrapTx) HasTenantTies(ctx context.Context, userID platformapp.ID, affiliationKey string, at time.Time) (bool, error) {
	return tx.affiliation.HasTenantTies(ctx, authzapp.PlatformAffiliationQuery{UserID: authzapp.ID(userID), AffiliationKey: affiliationKey, ObservedAt: at})
}
func (tx *bootstrapTx) PromoteIdentity(ctx context.Context, userID platformapp.ID, epoch int64, at time.Time) (int64, error) {
	newEpoch, err := tx.identity.Promote(ctx, identityapp.ID(userID), epoch, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityConflict) {
		return 0, platformapp.ErrConcurrentChange
	}
	return newEpoch, err
}
func (tx *bootstrapTx) RevokeSessions(ctx context.Context, userID platformapp.ID, at time.Time) (int64, error) {
	return tx.identity.RevokeSessions(ctx, identityapp.ID(userID), at)
}
func (tx *bootstrapTx) CreateAdministratorProfile(ctx context.Context, userID platformapp.ID, at time.Time) error {
	return tx.platform.InsertPlatformAdministratorProfile(ctx, platformsql.InsertPlatformAdministratorProfileParams{UserID: platformID(userID), At: platformTime(at)})
}
func (tx *bootstrapTx) AssignInitialOwner(ctx context.Context, assignmentID, userID platformapp.ID, at time.Time) error {
	return tx.platform.InsertPlatformRoleAssignment(ctx, platformsql.InsertPlatformRoleAssignmentParams{
		ID: platformID(assignmentID), UserID: platformID(userID), RoleKey: "PLATFORM_OWNER", Source: "BOOTSTRAP",
		AssignedBy: pgtype.UUID{}, AssignedAt: platformTime(at), Reason: "Initial deployment bootstrap",
	})
}
func (tx *bootstrapTx) ConsumeBootstrap(ctx context.Context, userID platformapp.ID, version int64, at time.Time) (int64, error) {
	changed, err := tx.platform.ConsumePlatformBootstrap(ctx, platformsql.ConsumePlatformBootstrapParams{OwnerUserID: platformID(userID), At: platformTime(at), ExpectedVersion: version})
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, platformapp.ErrConcurrentChange
	}
	return version + 1, nil
}
func (tx *bootstrapTx) AppendBootstrapEvidence(ctx context.Context, evidence platformapp.BootstrapEvidence) error {
	return tx.evidence.AppendPlatformBootstrap(ctx, controlapp.PlatformBootstrapEvidence{
		AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID),
		CorrelationID: controlapp.ID(evidence.CorrelationID), OwnerUserID: controlapp.ID(evidence.OwnerUserID),
		GovernanceVersion: evidence.GovernanceVersion, OccurredAt: evidence.OccurredAt,
	})
}

type invitationTx struct {
	*bootstrapTx
	authority *authorityTx
}

func newInvitationTx(db postgres.DBTX) (*invitationTx, error) {
	bootstrap, err := newBootstrapTx(db)
	if err != nil {
		return nil, err
	}
	authority, err := newAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	return &invitationTx{bootstrapTx: bootstrap, authority: authority}, nil
}

func (tx *invitationTx) GetIdentityAuthority(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.IdentityAuthorityFacts, error) {
	return tx.authority.GetIdentityAuthority(ctx, query)
}
func (tx *invitationTx) GetPlatformAuthority(ctx context.Context, userID platformapp.ID) (platformapp.PlatformAuthorityFacts, error) {
	return tx.authority.GetPlatformAuthority(ctx, userID)
}

func (tx *invitationTx) GetCommand(ctx context.Context, actor platformapp.ID, operation string, keyHash [32]byte) (platformapp.CommandRecord, bool, error) {
	row, err := tx.platform.GetPlatformCommand(ctx, platformsql.GetPlatformCommandParams{ActorUserID: platformID(actor), Operation: operation, KeyHash: keyHash[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.CommandRecord{}, false, nil
	}
	if err != nil {
		return platformapp.CommandRecord{}, false, err
	}
	if len(row.RequestHash) != 32 || !row.TargetID.Valid {
		return platformapp.CommandRecord{}, false, errors.New("platform_command_result_invalid")
	}
	var requestHash [32]byte
	copy(requestHash[:], row.RequestHash)
	return platformapp.CommandRecord{RequestHash: requestHash, TargetID: platformAppID(row.TargetID), ResponseStatus: row.ResponseStatus}, true, nil
}

func (tx *invitationTx) LockActiveOwnerIDs(ctx context.Context) ([]platformapp.ID, error) {
	rows, err := tx.platform.ListActivePlatformOwnerIDsForUpdate(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]platformapp.ID, len(rows))
	for index, row := range rows {
		if !row.Valid {
			return nil, errors.New("platform_owner_result_invalid")
		}
		ids[index] = platformAppID(row)
	}
	return ids, nil
}

func (tx *invitationTx) CountEffectiveOwnerIdentities(ctx context.Context, ids []platformapp.ID) (int64, error) {
	identityIDs := make([]identityapp.ID, len(ids))
	for index, id := range ids {
		identityIDs[index] = identityapp.ID(id)
	}
	return tx.identity.CountEffective(ctx, identityIDs)
}

func (tx *invitationTx) FindOpenOwnerInvitation(ctx context.Context) (platformapp.InvitationRecord, bool, error) {
	row, err := tx.platform.FindOpenPlatformOwnerInvitation(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.InvitationRecord{}, false, nil
	}
	if err != nil {
		return platformapp.InvitationRecord{}, false, err
	}
	record, err := invitationFromFind(row)
	return record, err == nil, err
}

func (tx *invitationTx) ExpireReplacedInvitationChanges(ctx context.Context, lookup string, at time.Time) error {
	changed, err := tx.platform.ExpirePendingChangesForOpenInvitationEmail(ctx, platformsql.ExpirePendingChangesForOpenInvitationEmailParams{At: platformTime(at), EmailLookup: lookup})
	if err != nil {
		return err
	}
	if changed > 1 {
		return errors.New("platform_invitation_change_result_invalid")
	}
	return nil
}

func (tx *invitationTx) ReplaceOpenInvitations(ctx context.Context, lookup string, at time.Time) error {
	return tx.platform.ReplaceOpenPlatformInvitations(ctx, platformsql.ReplaceOpenPlatformInvitationsParams{At: platformTime(at), EmailLookup: lookup})
}

func (tx *invitationTx) InsertInvitation(ctx context.Context, record platformapp.InvitationRecord, tokenHash *[32]byte) error {
	var hash []byte
	if tokenHash != nil {
		hash = tokenHash[:]
	}
	return tx.platform.InsertPlatformInvitation(ctx, platformsql.InsertPlatformInvitationParams{
		ID: platformID(record.ID), Email: record.EmailDisplay, EmailLookup: record.EmailLookup,
		RoleKey: string(record.Role), AuthorizationPath: string(record.AuthorizationPath), State: string(record.State), TokenHash: hash,
		InvitedBy: platformID(record.InvitedBy), Reason: record.Reason, CreatedAt: platformTime(record.CreatedAt),
		ActivatedAt: optionalPlatformTime(record.ActivatedAt), ExpiresAt: optionalPlatformTime(record.ExpiresAt),
	})
}

func (tx *invitationTx) InsertOwnerInvitationChange(ctx context.Context, changeID platformapp.ID, invitation platformapp.InvitationRecord, expiresAt time.Time) error {
	role := string(invitation.Role)
	return tx.platform.InsertPlatformAdministratorChange(ctx, platformsql.InsertPlatformAdministratorChangeParams{
		ID: platformID(changeID), Action: "ACTIVATE_OWNER_INVITATION", InvitationID: platformID(invitation.ID),
		RoleKey: &role, Reason: invitation.Reason, State: "PENDING_APPROVAL", MakerID: platformID(invitation.InvitedBy),
		CreatedAt: platformTime(invitation.CreatedAt), ExpiresAt: platformTime(expiresAt),
	})
}

func (tx *invitationTx) ConsumeFirstOwnerException(ctx context.Context, version int64, at time.Time) error {
	changed, err := tx.platform.ConsumeFirstAdditionalOwnerException(ctx, platformsql.ConsumeFirstAdditionalOwnerExceptionParams{At: platformTime(at), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrConcurrentChange
	}
	return nil
}

func (tx *invitationTx) GetInvitation(ctx context.Context, id platformapp.ID) (platformapp.InvitationRecord, error) {
	row, err := tx.platform.GetPlatformInvitation(ctx, platformID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.InvitationRecord{}, platformapp.ErrInvitationNotFound
	}
	if err != nil {
		return platformapp.InvitationRecord{}, err
	}
	return invitationFromGet(row)
}

func (tx *invitationTx) GetInvitationChangeID(ctx context.Context, invitationID platformapp.ID) (platformapp.ID, bool, error) {
	id, err := tx.platform.GetPlatformInvitationChangeID(ctx, platformID(invitationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.ID{}, false, nil
	}
	if err != nil {
		return platformapp.ID{}, false, err
	}
	if !id.Valid {
		return platformapp.ID{}, false, errors.New("platform_invitation_change_result_invalid")
	}
	return platformAppID(id), true, nil
}

func (tx *invitationTx) LockPendingOwnerInvitation(ctx context.Context, changeID platformapp.ID, observedAt time.Time) (platformapp.PendingOwnerInvitation, error) {
	row, err := tx.platform.LockPendingOwnerInvitationChange(ctx, platformsql.LockPendingOwnerInvitationChangeParams{ChangeID: platformID(changeID), ObservedAt: platformTime(observedAt)})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.PendingOwnerInvitation{}, platformapp.ErrInvitationConflict
	}
	if err != nil {
		return platformapp.PendingOwnerInvitation{}, err
	}
	invitation, err := invitationRecord(row.ID, row.Email, row.EmailLookup, row.RoleKey, row.AuthorizationPath, row.State, row.InvitedBy, row.InvitationReason, row.CreatedAt, row.ActivatedAt, row.InvitationExpiresAt, row.InvitationVersion)
	if err != nil || !row.ChangeID.Valid || !row.InvitationID.Valid || !row.MakerID.Valid || !row.ExpiresAt.Valid || row.ChangeVersion < 1 || row.InvitationID != row.ID {
		return platformapp.PendingOwnerInvitation{}, errors.New("platform_owner_invitation_change_result_invalid")
	}
	return platformapp.PendingOwnerInvitation{ChangeID: platformAppID(row.ChangeID), MakerID: platformAppID(row.MakerID), ChangeVersion: row.ChangeVersion, ChangeExpiresAt: row.ExpiresAt.Time.UTC(), Invitation: invitation}, nil
}

func (tx *invitationTx) ActivateOwnerInvitation(ctx context.Context, id platformapp.ID, version int64, tokenHash [32]byte, at, expiresAt time.Time) error {
	if !expiresAt.Equal(at.UTC().Add(72 * time.Hour)) {
		return errors.New("platform_owner_invitation_activation_invalid")
	}
	changed, err := tx.platform.ActivateApprovedOwnerInvitation(ctx, platformsql.ActivateApprovedOwnerInvitationParams{TokenHash: tokenHash[:], At: platformTime(at), ID: platformID(id), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrInvitationConflict
	}
	return nil
}

func (tx *invitationTx) RejectOwnerInvitation(ctx context.Context, id platformapp.ID, version int64, at time.Time) error {
	changed, err := tx.platform.RejectPlatformOwnerInvitation(ctx, platformsql.RejectPlatformOwnerInvitationParams{At: platformTime(at), ID: platformID(id), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrInvitationConflict
	}
	return nil
}

func (tx *invitationTx) DecideOwnerInvitationChange(ctx context.Context, changeID platformapp.ID, version int64, checkerID platformapp.ID, state, reason string, at time.Time) error {
	changed, err := tx.platform.DecidePlatformAdministratorChange(ctx, platformsql.DecidePlatformAdministratorChangeParams{State: state, DecisionReason: &reason, CheckerID: platformID(checkerID), At: platformTime(at), ID: platformID(changeID), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrInvitationConflict
	}
	return nil
}

func (tx *invitationTx) CancelInvitation(ctx context.Context, id platformapp.ID, version int64, at time.Time) (platformapp.InvitationRecord, error) {
	row, err := tx.platform.LockCancelablePlatformInvitation(ctx, platformsql.LockCancelablePlatformInvitationParams{ID: platformID(id), ExpectedVersion: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.InvitationRecord{}, platformapp.ErrInvitationNotFound
	}
	if err != nil {
		return platformapp.InvitationRecord{}, err
	}
	record, err := invitationFromCancelable(row)
	if err != nil {
		return platformapp.InvitationRecord{}, err
	}
	changed, err := tx.platform.CancelPlatformInvitation(ctx, platformsql.CancelPlatformInvitationParams{At: platformTime(at), ID: platformID(id), ExpectedVersion: version})
	if err != nil {
		return platformapp.InvitationRecord{}, err
	}
	if changed != 1 {
		return platformapp.InvitationRecord{}, platformapp.ErrConcurrentChange
	}
	record.State, record.Version = platformapp.InvitationCancelled, record.Version+1
	return record, nil
}

func (tx *invitationTx) ExpireInvitationChange(ctx context.Context, invitationID platformapp.ID, at time.Time) error {
	changed, err := tx.platform.ExpirePendingInvitationChangeOnCancellation(ctx, platformsql.ExpirePendingInvitationChangeOnCancellationParams{At: platformTime(at), InvitationID: platformID(invitationID)})
	if err != nil {
		return err
	}
	if changed > 1 {
		return errors.New("platform_invitation_change_result_invalid")
	}
	return nil
}

func (tx *invitationTx) LockInvitationByToken(ctx context.Context, tokenHash [32]byte, at time.Time) (platformapp.InvitationRecord, error) {
	row, err := tx.platform.LockActivePlatformInvitationByToken(ctx, platformsql.LockActivePlatformInvitationByTokenParams{TokenHash: tokenHash[:], ObservedAt: platformTime(at)})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.InvitationRecord{}, platformapp.ErrInvitationInvalid
	}
	if err != nil {
		return platformapp.InvitationRecord{}, err
	}
	return invitationFromActive(row)
}

func (tx *invitationTx) LockInvitee(ctx context.Context, userID, sessionID platformapp.ID, lookup string, at time.Time) (platformapp.BootstrapIdentity, error) {
	candidate, err := tx.identity.LockInvitee(ctx, identityapp.ID(userID), identityapp.ID(sessionID), lookup, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityIneligible) {
		return platformapp.BootstrapIdentity{}, platformapp.ErrInvitationInvalid
	}
	if err != nil {
		return platformapp.BootstrapIdentity{}, err
	}
	return platformapp.BootstrapIdentity{UserID: platformapp.ID(candidate.UserID), AuthEpoch: candidate.AuthEpoch, AffiliationKey: candidate.AffiliationKey}, nil
}

func (tx *invitationTx) AssignInvitedRole(ctx context.Context, assignmentID, userID platformapp.ID, role platformdomain.Role, invitedBy platformapp.ID, reason string, at time.Time) error {
	return tx.platform.InsertPlatformRoleAssignment(ctx, platformsql.InsertPlatformRoleAssignmentParams{
		ID: platformID(assignmentID), UserID: platformID(userID), RoleKey: string(role), Source: "INVITATION",
		AssignedBy: platformID(invitedBy), AssignedAt: platformTime(at), Reason: reason,
	})
}

func (tx *invitationTx) AcceptInvitation(ctx context.Context, invitationID platformapp.ID, version int64, userID platformapp.ID, at time.Time) error {
	changed, err := tx.platform.AcceptPlatformInvitation(ctx, platformsql.AcceptPlatformInvitationParams{AcceptedBy: platformID(userID), At: platformTime(at), ID: platformID(invitationID), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrConcurrentChange
	}
	return nil
}

func (tx *invitationTx) AdvanceOwnerGeneration(ctx context.Context, version int64, at time.Time) error {
	changed, err := tx.platform.AdvancePlatformGovernanceOwnerGeneration(ctx, platformsql.AdvancePlatformGovernanceOwnerGenerationParams{At: platformTime(at), ExpectedVersion: version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrConcurrentChange
	}
	return nil
}

func (tx *invitationTx) ClaimCommand(ctx context.Context, commandID, actorID platformapp.ID, operation string, keyHash, requestHash [32]byte, targetID platformapp.ID, status int32, at time.Time) error {
	changed, err := tx.platform.ClaimPlatformCommand(ctx, platformsql.ClaimPlatformCommandParams{
		ID: platformID(commandID), ActorUserID: platformID(actorID), Operation: operation,
		KeyHash: keyHash[:], RequestHash: requestHash[:], TargetID: platformID(targetID), ResponseStatus: status, At: platformTime(at),
	})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrInvitationConflict
	}
	return nil
}

func (tx *invitationTx) AppendInvitationEvidence(ctx context.Context, evidence platformapp.InvitationEvidence) error {
	return tx.evidence.AppendPlatformInvitation(ctx, controlapp.PlatformInvitationEvidence{
		AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID), CorrelationID: controlapp.ID(evidence.CorrelationID),
		ActorID: controlapp.ID(evidence.ActorID), InvitationID: controlapp.ID(evidence.InvitationID), Action: evidence.Action,
		RoleKey: string(evidence.Role), Version: evidence.Version, OccurredAt: evidence.OccurredAt,
	})
}

type roleChangeTx struct{ *invitationTx }

func newRoleChangeTx(db postgres.DBTX) (*roleChangeTx, error) {
	tx, err := newInvitationTx(db)
	if err != nil {
		return nil, err
	}
	return &roleChangeTx{invitationTx: tx}, nil
}
func (tx *roleChangeTx) GetIdentityStateChange(ctx context.Context, changeID platformapp.ID) (platformapp.IdentityStateChangeResult, error) {
	row, err := tx.platform.GetPlatformAdministratorChangeState(ctx, platformID(changeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.IdentityStateChangeResult{}, platformapp.ErrInvitationConflict
	}
	if err != nil {
		return platformapp.IdentityStateChangeResult{}, err
	}
	action := platformapp.IdentityStateAction(row.Action)
	if !row.TargetUserID.Valid || (action != platformapp.DisableAdministrator && action != platformapp.ReactivateAdministrator) || (row.State != "EXECUTED" && row.State != "PENDING_APPROVAL" && row.State != "REJECTED") {
		return platformapp.IdentityStateChangeResult{}, errors.New("platform_identity_state_change_result_invalid")
	}
	return platformapp.IdentityStateChangeResult{TargetUserID: platformAppID(row.TargetUserID), Action: action, State: row.State}, nil
}
func (tx *roleChangeTx) LockAdministratorProfile(ctx context.Context, userID platformapp.ID) (platformapp.AdministratorProfile, error) {
	row, err := tx.platform.LockPlatformAdministratorProfile(ctx, platformID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.AdministratorProfile{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.AdministratorProfile{}, err
	}
	if !row.UserID.Valid || row.ContextGeneration < 1 || row.Version < 1 {
		return platformapp.AdministratorProfile{}, errors.New("platform_profile_result_invalid")
	}
	return platformapp.AdministratorProfile{UserID: platformAppID(row.UserID), ContextGeneration: row.ContextGeneration, Version: row.Version}, nil
}
func (tx *roleChangeTx) LockAdministratorIdentity(ctx context.Context, userID platformapp.ID) (platformapp.AdministratorIdentity, error) {
	identity, err := tx.identity.LockAdministrator(ctx, identityapp.ID(userID))
	if errors.Is(err, identityapp.ErrPlatformIdentityNotFound) {
		return platformapp.AdministratorIdentity{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.AdministratorIdentity{}, err
	}
	return platformapp.AdministratorIdentity{UserID: platformapp.ID(identity.UserID), EmailDisplay: identity.EmailDisplay, AuthEpoch: identity.AuthEpoch, Disabled: identity.Disabled}, nil
}
func (tx *roleChangeTx) ActiveRoleExists(ctx context.Context, userID platformapp.ID, role platformdomain.Role) (bool, error) {
	return tx.platform.PlatformActiveRoleExists(ctx, platformsql.PlatformActiveRoleExistsParams{UserID: platformID(userID), RoleKey: string(role)})
}
func (tx *roleChangeTx) LockRoleAssignment(ctx context.Context, assignmentID, userID platformapp.ID) (platformapp.RoleAssignment, error) {
	row, err := tx.platform.LockActivePlatformRoleAssignment(ctx, platformsql.LockActivePlatformRoleAssignmentParams{ID: platformID(assignmentID), UserID: platformID(userID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.RoleAssignment{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.RoleAssignment{}, err
	}
	role, err := platformdomain.ParseRole(row.RoleKey)
	if err != nil || !row.ID.Valid || !row.UserID.Valid || row.Version < 1 {
		return platformapp.RoleAssignment{}, errors.New("platform_assignment_result_invalid")
	}
	return platformapp.RoleAssignment{ID: platformAppID(row.ID), UserID: platformAppID(row.UserID), Role: role, Version: row.Version}, nil
}
func (tx *roleChangeTx) LockPendingOwnerIdentityStateChange(ctx context.Context, changeID platformapp.ID, observedAt time.Time) (platformapp.PendingOwnerIdentityStateChange, error) {
	row, err := tx.platform.LockPendingPlatformOwnerIdentityStateChange(ctx, platformsql.LockPendingPlatformOwnerIdentityStateChangeParams{ChangeID: platformID(changeID), ObservedAt: platformTime(observedAt)})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.PendingOwnerIdentityStateChange{}, platformapp.ErrInvitationConflict
	}
	if err != nil {
		return platformapp.PendingOwnerIdentityStateChange{}, err
	}
	action := platformapp.IdentityStateAction(row.Action)
	if !row.ID.Valid || !row.TargetUserID.Valid || !row.MakerID.Valid || row.ExpectedUserVersion == nil || row.ExpectedTargetContextGeneration == nil || !row.ExpiresAt.Valid || row.Version < 1 || (action != platformapp.DisableAdministrator && action != platformapp.ReactivateAdministrator) {
		return platformapp.PendingOwnerIdentityStateChange{}, errors.New("platform_owner_identity_change_result_invalid")
	}
	return platformapp.PendingOwnerIdentityStateChange{ChangeID: platformAppID(row.ID), TargetUserID: platformAppID(row.TargetUserID), MakerID: platformAppID(row.MakerID), Action: action, ExpectedAdministratorVersion: *row.ExpectedUserVersion, ExpectedContextGeneration: *row.ExpectedTargetContextGeneration, Reason: row.Reason, ExpiresAt: row.ExpiresAt.Time.UTC(), Version: row.Version}, nil
}
func (tx *roleChangeTx) LockPendingOwnerRoleChange(ctx context.Context, changeID platformapp.ID, observedAt time.Time) (platformapp.PendingOwnerRoleChange, error) {
	row, err := tx.platform.LockPendingPlatformOwnerRoleChange(ctx, platformsql.LockPendingPlatformOwnerRoleChangeParams{ChangeID: platformID(changeID), ObservedAt: platformTime(observedAt)})
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.PendingOwnerRoleChange{}, platformapp.ErrInvitationConflict
	}
	if err != nil {
		return platformapp.PendingOwnerRoleChange{}, err
	}
	if !row.ID.Valid || !row.TargetUserID.Valid || !row.MakerID.Valid || row.RoleKey == nil || *row.RoleKey != string(platformdomain.PlatformOwner) || row.ExpectedUserVersion == nil || row.ExpectedTargetContextGeneration == nil || !row.ExpiresAt.Valid || row.Version < 1 {
		return platformapp.PendingOwnerRoleChange{}, errors.New("platform_owner_role_change_result_invalid")
	}
	action := platformapp.RoleChangeAction(row.Action)
	if action != platformapp.AssignRole && action != platformapp.RevokeRole {
		return platformapp.PendingOwnerRoleChange{}, errors.New("platform_owner_role_change_result_invalid")
	}
	result := platformapp.PendingOwnerRoleChange{ChangeID: platformAppID(row.ID), TargetUserID: platformAppID(row.TargetUserID), MakerID: platformAppID(row.MakerID), Action: action, ExpectedAdministratorVersion: *row.ExpectedUserVersion, ExpectedContextGeneration: *row.ExpectedTargetContextGeneration, Reason: row.Reason, ExpiresAt: row.ExpiresAt.Time.UTC(), Version: row.Version}
	if action == platformapp.RevokeRole {
		if !row.AssignmentID.Valid || row.ExpectedAssignmentVersion == nil {
			return platformapp.PendingOwnerRoleChange{}, errors.New("platform_owner_role_change_result_invalid")
		}
		result.AssignmentID = platformAppID(row.AssignmentID)
		result.ExpectedAssignmentVersion = *row.ExpectedAssignmentVersion
	}
	return result, nil
}
func (tx *roleChangeTx) DecideOwnerRoleChange(ctx context.Context, changeID platformapp.ID, version int64, checkerID platformapp.ID, state, reason string, at time.Time) error {
	return tx.DecideOwnerInvitationChange(ctx, changeID, version, checkerID, state, reason, at)
}

func (tx *roleChangeTx) InsertRoleChange(ctx context.Context, changeID platformapp.ID, input platformapp.RoleChangeInput, assignmentID *platformapp.ID, state string, at time.Time, expiresAt *time.Time) error {
	var assignment pgtype.UUID
	if assignmentID != nil {
		assignment = platformID(*assignmentID)
	}
	role := string(input.Role)
	adminVersion := input.ExpectedAdministratorVersion
	contextGeneration := input.ExpectedTargetContextGeneration
	var assignmentVersion *int64
	if input.Action == platformapp.RevokeRole {
		assignmentVersion = &input.ExpectedAssignmentVersion
	}
	decidedAt := pgtype.Timestamptz{}
	if state == "EXECUTED" {
		decidedAt = platformTime(at)
	}
	return tx.platform.InsertPlatformAdministratorChange(ctx, platformsql.InsertPlatformAdministratorChangeParams{ID: platformID(changeID), Action: string(input.Action), TargetUserID: platformID(input.TargetUserID), RoleKey: &role, AssignmentID: assignment, ExpectedUserVersion: &adminVersion, ExpectedTargetContextGeneration: &contextGeneration, ExpectedAssignmentVersion: assignmentVersion, Reason: input.Reason, State: state, MakerID: platformID(input.ActorID), CreatedAt: platformTime(at), ExpiresAt: optionalPlatformTime(expiresAt), DecidedAt: decidedAt})
}
func (tx *roleChangeTx) InsertRoleAssignment(ctx context.Context, assignmentID, userID platformapp.ID, role platformdomain.Role, actorID platformapp.ID, reason string, at time.Time) error {
	return tx.platform.InsertPlatformRoleAssignment(ctx, platformsql.InsertPlatformRoleAssignmentParams{ID: platformID(assignmentID), UserID: platformID(userID), RoleKey: string(role), Source: "OWNER_CHANGE", AssignedBy: platformID(actorID), AssignedAt: platformTime(at), Reason: reason})
}
func (tx *roleChangeTx) RevokeRoleAssignment(ctx context.Context, assignment platformapp.RoleAssignment, actorID platformapp.ID, reason string, at time.Time) error {
	changed, err := tx.platform.RevokePlatformRoleAssignment(ctx, platformsql.RevokePlatformRoleAssignmentParams{RevokedBy: platformID(actorID), At: platformTime(at), RevocationReason: &reason, ID: platformID(assignment.ID), UserID: platformID(assignment.UserID), ExpectedVersion: assignment.Version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrConcurrentChange
	}
	return nil
}
func (tx *roleChangeTx) AdvanceAdministratorProfile(ctx context.Context, profile platformapp.AdministratorProfile, at time.Time) error {
	changed, err := tx.platform.AdvancePlatformAdministratorProfile(ctx, platformsql.AdvancePlatformAdministratorProfileParams{At: platformTime(at), UserID: platformID(profile.UserID), ExpectedContextGeneration: profile.ContextGeneration, ExpectedVersion: profile.Version})
	if err != nil {
		return err
	}
	if changed != 1 {
		return platformapp.ErrConcurrentChange
	}
	return nil
}
func (tx *roleChangeTx) AdvanceIdentityEpoch(ctx context.Context, identity platformapp.AdministratorIdentity, at time.Time) error {
	_, err := tx.identity.AdvanceEpoch(ctx, identityapp.ID(identity.UserID), identity.AuthEpoch, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityConflict) {
		return platformapp.ErrConcurrentChange
	}
	return err
}
func (tx *roleChangeTx) SetAdministratorDisabled(ctx context.Context, identity platformapp.AdministratorIdentity, at time.Time) error {
	_, err := tx.identity.SetDisabled(ctx, identityapp.ID(identity.UserID), identity.AuthEpoch, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityConflict) {
		return platformapp.ErrConcurrentChange
	}
	return err
}
func (tx *roleChangeTx) SetAdministratorActive(ctx context.Context, identity platformapp.AdministratorIdentity, at time.Time) error {
	_, err := tx.identity.SetActive(ctx, identityapp.ID(identity.UserID), identity.AuthEpoch, at)
	if errors.Is(err, identityapp.ErrPlatformIdentityConflict) {
		return platformapp.ErrConcurrentChange
	}
	return err
}
func (tx *roleChangeTx) InsertIdentityStateChange(ctx context.Context, changeID platformapp.ID, input platformapp.IdentityStateChangeInput, state string, at time.Time, expiresAt *time.Time) error {
	adminVersion, contextGeneration := input.ExpectedAdministratorVersion, input.ExpectedTargetContextGeneration
	decidedAt := pgtype.Timestamptz{}
	if state == "EXECUTED" {
		decidedAt = platformTime(at)
	}
	return tx.platform.InsertPlatformAdministratorChange(ctx, platformsql.InsertPlatformAdministratorChangeParams{ID: platformID(changeID), Action: string(input.Action), TargetUserID: platformID(input.TargetUserID), ExpectedUserVersion: &adminVersion, ExpectedTargetContextGeneration: &contextGeneration, Reason: input.Reason, State: state, MakerID: platformID(input.ActorID), CreatedAt: platformTime(at), ExpiresAt: optionalPlatformTime(expiresAt), DecidedAt: decidedAt})
}
func (tx *roleChangeTx) AppendRoleChangeEvidence(ctx context.Context, evidence platformapp.RoleChangeEvidence) error {
	return tx.evidence.AppendPlatformRoleChange(ctx, controlapp.PlatformRoleChangeEvidence{AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID), CorrelationID: controlapp.ID(evidence.CorrelationID), ActorID: controlapp.ID(evidence.ActorID), TargetUserID: controlapp.ID(evidence.TargetUserID), ChangeID: controlapp.ID(evidence.ChangeID), Action: string(evidence.Action), RoleKey: string(evidence.Role), State: evidence.State, Version: evidence.Version, OccurredAt: evidence.OccurredAt})
}
func (tx *roleChangeTx) AppendIdentityStateEvidence(ctx context.Context, evidence platformapp.IdentityStateEvidence) error {
	return tx.evidence.AppendPlatformIdentityStateChange(ctx, controlapp.PlatformIdentityStateEvidence{AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID), CorrelationID: controlapp.ID(evidence.CorrelationID), ActorID: controlapp.ID(evidence.ActorID), TargetUserID: controlapp.ID(evidence.TargetUserID), ChangeID: controlapp.ID(evidence.ChangeID), Action: string(evidence.Action), State: evidence.State, Version: evidence.Version, OccurredAt: evidence.OccurredAt})
}
func (tx *roleChangeTx) AppendAdministratorSessionRevocationEvidence(ctx context.Context, evidence platformapp.AdministratorSessionRevocationEvidence) error {
	return tx.evidence.AppendPlatformAdministratorSessionRevocation(ctx, controlapp.PlatformAdministratorSessionRevocationEvidence{AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID), CorrelationID: controlapp.ID(evidence.CorrelationID), ActorID: controlapp.ID(evidence.ActorID), TargetUserID: controlapp.ID(evidence.TargetUserID), TargetAuthEpoch: evidence.TargetAuthEpoch, Reason: evidence.Reason, OccurredAt: evidence.OccurredAt})
}

type authorityTx struct {
	platform *platformsql.Queries
	identity *identitypersistence.PlatformAdministrationIdentityTx
}

func newAuthorityTx(db postgres.DBTX) (*authorityTx, error) {
	identity, err := identitypersistence.NewPlatformAdministrationIdentityTx(db)
	if err != nil {
		return nil, err
	}
	return &authorityTx{platform: platformsql.New(db), identity: identity}, nil
}
func (tx *authorityTx) GetIdentityAuthority(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.IdentityAuthorityFacts, error) {
	facts, err := tx.identity.GetAuthority(ctx, identityapp.PlatformAuthorityQuery{UserID: identityapp.ID(query.UserID), SessionID: identityapp.ID(query.SessionID), ObservedAt: query.ObservedAt})
	if errors.Is(err, identityapp.ErrPlatformIdentityNotFound) {
		return platformapp.IdentityAuthorityFacts{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.IdentityAuthorityFacts{}, err
	}
	return platformapp.IdentityAuthorityFacts{
		UserID: platformapp.ID(facts.UserID), AuthEpoch: facts.AuthEpoch, AffiliationKey: facts.AffiliationKey,
		AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod,
		AssuredAt: facts.AssuredAt, IdleExpiresAt: facts.IdleExpiresAt, AbsoluteExpiresAt: facts.AbsoluteExpiresAt,
	}, nil
}
func (tx *authorityTx) GetPlatformAuthority(ctx context.Context, userID platformapp.ID) (platformapp.PlatformAuthorityFacts, error) {
	row, err := tx.platform.GetPlatformAuthorityProfile(ctx, platformID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.PlatformAuthorityFacts{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.PlatformAuthorityFacts{}, err
	}
	if !row.UserID.Valid || row.ContextGeneration < 1 || row.AdministratorVersion < 1 {
		return platformapp.PlatformAuthorityFacts{}, platformapp.ErrAuthorityNotFound
	}
	roles, err := tx.platform.ListActivePlatformRoleKeys(ctx, platformID(userID))
	if err != nil {
		return platformapp.PlatformAuthorityFacts{}, err
	}
	return platformapp.PlatformAuthorityFacts{UserID: platformAppID(row.UserID), ContextGeneration: row.ContextGeneration, AdministratorVersion: row.AdministratorVersion, RoleKeys: roles}, nil
}

type governanceReadTx struct{ *authorityTx }

func newGovernanceReadTx(db postgres.DBTX) (*governanceReadTx, error) {
	authority, err := newAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	return &governanceReadTx{authorityTx: authority}, nil
}
func (tx *governanceReadTx) CountEffectiveOwners(ctx context.Context) (int64, error) {
	ids, err := tx.platform.ListActivePlatformOwnerIDs(ctx)
	if err != nil {
		return 0, err
	}
	values := make([]identityapp.ID, len(ids))
	for i, id := range ids {
		if !id.Valid {
			return 0, errors.New("platform_owner_id_invalid")
		}
		values[i] = identityapp.ID(id.Bytes)
	}
	return tx.identity.CountEffective(ctx, values)
}
func (tx *governanceReadTx) ListAdministratorProfiles(ctx context.Context, filter platformapp.AdministratorFilter, page platformapp.PageCursor) ([]platformapp.AdministratorProfileView, error) {
	beforeTime, beforeID := readCursor(page)
	var userID pgtype.UUID
	if filter.EmailLookup != nil {
		id, found, err := tx.identity.FindAdministratorByEmailLookup(ctx, *filter.EmailLookup)
		if err != nil {
			return nil, err
		}
		if !found {
			return []platformapp.AdministratorProfileView{}, nil
		}
		userID = platformID(platformapp.ID(id))
	}
	var role *string
	if filter.Role != nil {
		value := string(*filter.Role)
		role = &value
	}
	rows, err := tx.platform.ListPlatformAdministratorProfiles(ctx, platformsql.ListPlatformAdministratorProfilesParams{FilterUserID: userID, FilterRoleKey: role, FilterState: filter.State, BeforeCreatedAt: beforeTime, BeforeUserID: beforeID, PageLimit: page.Limit + 1})
	if err != nil {
		return nil, err
	}
	result := make([]platformapp.AdministratorProfileView, len(rows))
	for i, row := range rows {
		view, viewErr := profileView(row)
		if viewErr != nil {
			return nil, viewErr
		}
		result[i] = view
	}
	return result, nil
}
func (tx *governanceReadTx) GetAdministratorProfile(ctx context.Context, userID platformapp.ID) (platformapp.AdministratorProfileView, error) {
	row, err := tx.platform.GetPlatformAdministratorProfile(ctx, platformID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.AdministratorProfileView{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.AdministratorProfileView{}, err
	}
	return profileView(row)
}
func (tx *governanceReadTx) ListAdministratorIdentities(ctx context.Context, userIDs []platformapp.ID) ([]platformapp.AdministratorIdentityView, error) {
	ids := make([]identityapp.ID, len(userIDs))
	for i, id := range userIDs {
		ids[i] = identityapp.ID(id)
	}
	rows, err := tx.identity.ListAdministratorSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]platformapp.AdministratorIdentityView, len(rows))
	for i, row := range rows {
		result[i] = platformapp.AdministratorIdentityView{UserID: platformapp.ID(row.UserID), Email: row.EmailDisplay, Disabled: row.Disabled, TOTPEnrolled: row.TOTPEnrolled, EmailOTPVerified: row.EmailOTPVerified}
	}
	return result, nil
}
func (tx *governanceReadTx) ListLatestAssignments(ctx context.Context, userIDs []platformapp.ID) ([]platformapp.AssignmentView, error) {
	ids := make([]pgtype.UUID, len(userIDs))
	for i, id := range userIDs {
		ids[i] = platformID(id)
	}
	rows, err := tx.platform.ListLatestPlatformRoleAssignmentsForUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	return assignmentViews(rows)
}
func (tx *governanceReadTx) ListAssignmentHistory(ctx context.Context, userID platformapp.ID, page platformapp.PageCursor) ([]platformapp.AssignmentView, error) {
	beforeTime, beforeID := readCursor(page)
	rows, err := tx.platform.ListPlatformRoleAssignmentHistory(ctx, platformsql.ListPlatformRoleAssignmentHistoryParams{UserID: platformID(userID), BeforeAssignedAt: beforeTime, BeforeID: beforeID, PageLimit: page.Limit + 1})
	if err != nil {
		return nil, err
	}
	return assignmentViews(rows)
}
func (tx *governanceReadTx) ListInvitations(ctx context.Context, page platformapp.PageCursor) ([]platformapp.InvitationView, error) {
	beforeTime, beforeID := readCursor(page)
	rows, err := tx.platform.ListPlatformInvitations(ctx, platformsql.ListPlatformInvitationsParams{BeforeCreatedAt: beforeTime, BeforeID: beforeID, PageLimit: page.Limit + 1})
	if err != nil {
		return nil, err
	}
	result := make([]platformapp.InvitationView, len(rows))
	for i, row := range rows {
		role, parseErr := platformdomain.ParseRole(row.RoleKey)
		if parseErr != nil || !row.ID.Valid || !row.CreatedAt.Valid || row.Version < 1 {
			return nil, errors.New("platform_invitation_view_invalid")
		}
		delivery := platformapp.DeliveryUnconfirmed
		if row.State == "PENDING_APPROVAL" {
			delivery = platformapp.DeliveryNotReady
		}
		result[i] = platformapp.InvitationView{ID: platformAppID(row.ID), Email: row.Email, Role: role, State: row.State, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: platformTimePointer(row.ExpiresAt), Delivery: delivery, GovernanceChangeID: platformIDPointer(row.GovernanceChangeID)}
	}
	return result, nil
}
func (tx *governanceReadTx) GetInvitation(ctx context.Context, id platformapp.ID) (platformapp.InvitationView, error) {
	row, err := tx.platform.GetPlatformInvitation(ctx, platformID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.InvitationView{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.InvitationView{}, err
	}
	role, err := platformdomain.ParseRole(row.RoleKey)
	if err != nil || !row.ID.Valid || !row.CreatedAt.Valid || row.Version < 1 {
		return platformapp.InvitationView{}, errors.New("platform_invitation_view_invalid")
	}
	var changeID *platformapp.ID
	change, changeErr := tx.platform.GetPlatformInvitationChangeID(ctx, platformID(id))
	if changeErr == nil {
		changeID = platformIDPointer(change)
	} else if !errors.Is(changeErr, pgx.ErrNoRows) {
		return platformapp.InvitationView{}, changeErr
	}
	delivery := platformapp.DeliveryUnconfirmed
	if row.State == "PENDING_APPROVAL" {
		delivery = platformapp.DeliveryNotReady
	}
	return platformapp.InvitationView{ID: platformAppID(row.ID), Email: row.Email, Role: role, State: row.State, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: platformTimePointer(row.ExpiresAt), Delivery: delivery, GovernanceChangeID: changeID}, nil
}
func (tx *governanceReadTx) ListChanges(ctx context.Context, state *string, page platformapp.PageCursor, observedAt time.Time) ([]platformapp.ChangeView, error) {
	beforeTime, beforeID := readCursor(page)
	rows, err := tx.platform.ListPlatformAdministratorChanges(ctx, platformsql.ListPlatformAdministratorChangesParams{ObservedAt: platformTime(observedAt), FilterState: state, BeforeCreatedAt: beforeTime, BeforeID: beforeID, PageLimit: page.Limit + 1})
	if err != nil {
		return nil, err
	}
	result := make([]platformapp.ChangeView, len(rows))
	for i, row := range rows {
		view, viewErr := changeView(row)
		if viewErr != nil {
			return nil, viewErr
		}
		result[i] = view
	}
	return result, nil
}
func (tx *governanceReadTx) GetChange(ctx context.Context, changeID platformapp.ID) (platformapp.ChangeView, error) {
	row, err := tx.platform.GetPlatformAdministratorChange(ctx, platformID(changeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return platformapp.ChangeView{}, platformapp.ErrAuthorityNotFound
	}
	if err != nil {
		return platformapp.ChangeView{}, err
	}
	return changeView(row)
}
func readCursor(page platformapp.PageCursor) (pgtype.Timestamptz, pgtype.UUID) {
	if page.BeforeTime == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return platformTime(*page.BeforeTime), platformID(*page.BeforeID)
}
func profileView(row platformsql.PlatformAdministrationAdministratorProfile) (platformapp.AdministratorProfileView, error) {
	if !row.UserID.Valid || !row.CreatedAt.Valid || !row.UpdatedAt.Valid || row.ContextGeneration < 1 || row.Version < 1 {
		return platformapp.AdministratorProfileView{}, errors.New("platform_profile_view_invalid")
	}
	return platformapp.AdministratorProfileView{UserID: platformAppID(row.UserID), ContextGeneration: row.ContextGeneration, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, nil
}
func assignmentViews(rows []platformsql.PlatformAdministrationRoleAssignment) ([]platformapp.AssignmentView, error) {
	result := make([]platformapp.AssignmentView, len(rows))
	for i, row := range rows {
		role, err := platformdomain.ParseRole(row.RoleKey)
		if err != nil || !row.ID.Valid || !row.UserID.Valid || !row.AssignedAt.Valid || row.Version < 1 {
			return nil, errors.New("platform_assignment_view_invalid")
		}
		result[i] = platformapp.AssignmentView{ID: platformAppID(row.ID), UserID: platformAppID(row.UserID), Role: role, State: row.State, Source: row.Source, Reason: row.Reason, AssignedBy: platformIDPointer(row.AssignedBy), AssignedAt: row.AssignedAt.Time.UTC(), RevokedBy: platformIDPointer(row.RevokedBy), RevokedAt: platformTimePointer(row.RevokedAt), RevocationReason: row.RevocationReason, Version: row.Version}
	}
	return result, nil
}
func changeView(row platformsql.PlatformAdministrationAdministratorChange) (platformapp.ChangeView, error) {
	if !row.ID.Valid || !row.MakerID.Valid || !row.CreatedAt.Valid || row.Version < 1 {
		return platformapp.ChangeView{}, errors.New("platform_change_view_invalid")
	}
	var role *platformdomain.Role
	if row.RoleKey != nil {
		parsed, err := platformdomain.ParseRole(*row.RoleKey)
		if err != nil {
			return platformapp.ChangeView{}, err
		}
		role = &parsed
	}
	return platformapp.ChangeView{ID: platformAppID(row.ID), Action: row.Action, SessionsRevoked: row.State == "EXECUTED" && row.Action != "ACTIVATE_OWNER_INVITATION", TargetUserID: platformIDPointer(row.TargetUserID), InvitationID: platformIDPointer(row.InvitationID), AssignmentID: platformIDPointer(row.AssignmentID), Role: role, ExpectedAdministratorVersion: row.ExpectedUserVersion, ExpectedContextGeneration: row.ExpectedTargetContextGeneration, ExpectedAssignmentVersion: row.ExpectedAssignmentVersion, Reason: row.Reason, DecisionReason: row.DecisionReason, State: row.State, Version: row.Version, MakerID: platformAppID(row.MakerID), CheckerID: platformIDPointer(row.CheckerID), CreatedAt: row.CreatedAt.Time.UTC(), ExpiresAt: platformTimePointer(row.ExpiresAt), DecidedAt: platformTimePointer(row.DecidedAt)}, nil
}
func platformIDPointer(value pgtype.UUID) *platformapp.ID {
	if !value.Valid {
		return nil
	}
	id := platformAppID(value)
	return &id
}

func invitationFromFind(row platformsql.FindOpenPlatformOwnerInvitationRow) (platformapp.InvitationRecord, error) {
	return invitationRecord(row.ID, row.Email, row.EmailLookup, row.RoleKey, row.AuthorizationPath, row.State, row.InvitedBy, row.Reason, row.CreatedAt, row.ActivatedAt, row.ExpiresAt, row.Version)
}
func invitationFromGet(row platformsql.GetPlatformInvitationRow) (platformapp.InvitationRecord, error) {
	return invitationRecord(row.ID, row.Email, row.EmailLookup, row.RoleKey, row.AuthorizationPath, row.State, row.InvitedBy, row.Reason, row.CreatedAt, row.ActivatedAt, row.ExpiresAt, row.Version)
}
func invitationFromCancelable(row platformsql.LockCancelablePlatformInvitationRow) (platformapp.InvitationRecord, error) {
	return invitationRecord(row.ID, row.Email, row.EmailLookup, row.RoleKey, row.AuthorizationPath, row.State, row.InvitedBy, row.Reason, row.CreatedAt, row.ActivatedAt, row.ExpiresAt, row.Version)
}
func invitationFromActive(row platformsql.LockActivePlatformInvitationByTokenRow) (platformapp.InvitationRecord, error) {
	return invitationRecord(row.ID, row.Email, row.EmailLookup, row.RoleKey, row.AuthorizationPath, string(platformapp.InvitationActive), row.InvitedBy, row.Reason, row.CreatedAt, row.ActivatedAt, row.ExpiresAt, row.Version)
}
func invitationRecord(id pgtype.UUID, emailDisplay, emailLookup, roleKey, authorizationPath, state string, invitedBy pgtype.UUID, reason string, createdAt, activatedAt, expiresAt pgtype.Timestamptz, version int64) (platformapp.InvitationRecord, error) {
	role, err := platformdomain.ParseRole(roleKey)
	if err != nil || !id.Valid || !invitedBy.Valid || emailDisplay == "" || emailLookup == "" || reason == "" || !createdAt.Valid || version < 1 {
		return platformapp.InvitationRecord{}, errors.New("platform_invitation_result_invalid")
	}
	path := platformapp.InvitationAuthorizationPath(authorizationPath)
	switch path {
	case platformapp.InvitationOwnerDirect, platformapp.InvitationFirstOwnerException, platformapp.InvitationMakerChecker:
	default:
		return platformapp.InvitationRecord{}, errors.New("platform_invitation_result_invalid")
	}
	invitationState := platformapp.InvitationState(state)
	switch invitationState {
	case platformapp.InvitationPendingApproval, platformapp.InvitationActive, platformapp.InvitationAccepted, platformapp.InvitationCancelled:
	default:
		return platformapp.InvitationRecord{}, errors.New("platform_invitation_result_invalid")
	}
	return platformapp.InvitationRecord{
		ID: platformAppID(id), EmailDisplay: emailDisplay, EmailLookup: emailLookup, Role: role,
		AuthorizationPath: path, State: invitationState, InvitedBy: platformAppID(invitedBy), Reason: reason, CreatedAt: createdAt.Time.UTC(),
		ActivatedAt: platformTimePointer(activatedAt), ExpiresAt: platformTimePointer(expiresAt), Version: version,
	}, nil
}
func platformTimePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
func optionalPlatformTime(value *time.Time) pgtype.Timestamptz {
	if value == nil || value.IsZero() {
		return pgtype.Timestamptz{}
	}
	return platformTime(*value)
}
func platformID(id platformapp.ID) pgtype.UUID    { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }
func platformAppID(id pgtype.UUID) platformapp.ID { return platformapp.ID(id.Bytes) }
func platformTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
